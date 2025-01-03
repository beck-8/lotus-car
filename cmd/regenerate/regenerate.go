package regenerate

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	commcid "github.com/filecoin-project/go-fil-commcid"
	commp "github.com/filecoin-project/go-fil-commp-hashhash"
	"github.com/google/uuid"
	"github.com/minerdao/lotus-car/config"
	"github.com/minerdao/lotus-car/db"
	"github.com/minerdao/lotus-car/util"
	"github.com/urfave/cli/v2"
)

const BufSize = (4 << 20) / 128 * 127

func Command() *cli.Command {
	return &cli.Command{
		Name:  "regenerate",
		Usage: "Regenerate car file from saved raw files information",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "id",
				Usage:    "UUID of the car file to regenerate",
				Required: false,
			},
			&cli.StringFlag{
				Name:     "from-piece-cids",
				Usage:    "Path to file containing piece CIDs (one per line)",
				Required: false,
			},
			&cli.StringFlag{
				Name:     "parent",
				Aliases:  []string{"p"},
				Usage:    "Parent path of the dataset",
				Required: true,
			},
			&cli.StringFlag{
				Name:    "tmp-dir",
				Aliases: []string{"t"},
				Usage:   "Optionally copy the files to a temporary (and much faster) directory",
				Value:   "",
			},
			&cli.StringFlag{
				Name:    "out-dir",
				Aliases: []string{"o"},
				Usage:   "Output directory to save the car file",
				Value:   ".",
			},
		},
		Action: func(c *cli.Context) error {
			// Load configuration
			cfg, err := config.LoadConfig(c.String("config"))
			if err != nil {
				return fmt.Errorf("failed to load config: %v", err)
			}

			id := c.String("id")
			fromPieceCids := c.String("from-piece-cids")
			parent := c.String("parent")
			tmpDir := c.String("tmp-dir")
			outDir := c.String("out-dir")

			dbConfig := &db.DBConfig{
				Host:     cfg.Database.Host,
				Port:     cfg.Database.Port,
				User:     cfg.Database.User,
				Password: cfg.Database.Password,
				DBName:   cfg.Database.DBName,
				SSLMode:  cfg.Database.SSLMode,
			}

			database, err := db.InitDB(dbConfig)
			if err != nil {
				return fmt.Errorf("failed to initialize database: %v", err)
			}
			defer database.Close()

			var files []db.CarFile
			if fromPieceCids != "" {
				// 从文件读取 piece CIDs
				pieceCids, err := readPieceCidsFromFile(fromPieceCids)
				if err != nil {
					return fmt.Errorf("failed to read piece CIDs from file: %v", err)
				}

				// 获取文件信息
				files, err = database.GetFilesByPieceCids(pieceCids)
				if err != nil {
					return fmt.Errorf("failed to get files by piece CIDs: %v", err)
				}

				if len(files) == 0 {
					return fmt.Errorf("no files found for the provided piece CIDs")
				}

				log.Printf("Found %d files to regenerate", len(files))
			} else {
				if id == "" {
					return fmt.Errorf("either --id or --from-piece-cids must be specified")
				}

				// 获取单个文件信息
				file, err := database.GetFile(id)
				if err != nil {
					return fmt.Errorf("failed to get car file: %v", err)
				}
				if file == nil {
					return fmt.Errorf("car file not found with id: %s", id)
				}

				files = []db.CarFile{*file}
			}

			// 处理每个文件
			for _, file := range files {
				err = regenerateFile(database, file, parent, tmpDir, outDir)
				if err != nil {
					log.Printf("Failed to regenerate file %s: %v", file.ID, err)
					continue
				}
			}

			return nil
		},
	}
}

// 从文件读取 piece CIDs
func readPieceCidsFromFile(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	var pieceCids []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		pieceCid := scanner.Text()
		if pieceCid != "" {
			pieceCids = append(pieceCids, pieceCid)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	return pieceCids, nil
}

// 重新生成单个文件
func regenerateFile(database *db.Database, file db.CarFile, parent, tmpDir, outDir string) error {
	log.Printf("Start regenerating car file for id: %s, piece cid: %s", file.ID, file.PieceCid)

	// 更新状态为进行中
	err := database.UpdateRegenerateStatus(file.ID, db.RegenerateStatusPending)
	if err != nil {
		return fmt.Errorf("failed to update regenerate status: %v", err)
	}

	// 解析原始文件信息
	var rawFiles []db.RawFileInfo
	err = json.Unmarshal([]byte(file.RawFiles), &rawFiles)
	if err != nil {
		// 更新状态为失败
		_ = database.UpdateRegenerateStatus(file.ID, db.RegenerateStatusFailed)
		return fmt.Errorf("failed to unmarshal raw files: %v", err)
	}

	// 检查所有原始文件是否存在
	for _, rawFile := range rawFiles {
		fullPath := filepath.Join(parent, rawFile.RelativePath)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			// 更新状态为失败
			_ = database.UpdateRegenerateStatus(file.ID, db.RegenerateStatusFailed)
			return fmt.Errorf("raw file not found: %s", fullPath)
		}
	}

	// 生成临时 car 文件
	outFilename := uuid.New().String() + ".car"
	outPath := path.Join(outDir, outFilename)

	// 创建输出目录
	err = os.MkdirAll(outDir, 0755)
	if err != nil {
		// 更新状态为失败
		_ = database.UpdateRegenerateStatus(file.ID, db.RegenerateStatusFailed)
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	// 生成 car 文件
	carF, err := os.Create(outPath)
	if err != nil {
		// 更新状态为失败
		_ = database.UpdateRegenerateStatus(file.ID, db.RegenerateStatusFailed)
		return fmt.Errorf("failed to create car file: %v", err)
	}

	// 准备文件列表
	var selectedFiles []util.Finfo
	for _, rawFile := range rawFiles {
		selectedFiles = append(selectedFiles, util.Finfo{
			Path: filepath.Join(parent, strings.TrimPrefix(rawFile.RelativePath, "/")),
			Size: rawFile.Size,
		})
	}

	// 生成 car 文件
	ctx := context.Background()
	cp := new(commp.Calc)
	writer := bufio.NewWriterSize(io.MultiWriter(carF, cp), BufSize)

	_, cid, _, err := util.GenerateCar(ctx, selectedFiles, parent, tmpDir, writer)
	if err != nil {
		// 更新状态为失败
		_ = database.UpdateRegenerateStatus(file.ID, db.RegenerateStatusFailed)
		carF.Close()
		os.Remove(outPath)
		return fmt.Errorf("failed to generate car file: %v", err)
	}

	err = writer.Flush()
	if err != nil {
		// 更新状态为失败
		_ = database.UpdateRegenerateStatus(file.ID, db.RegenerateStatusFailed)
		carF.Close()
		os.Remove(outPath)
		return fmt.Errorf("failed to flush writer: %v", err)
	}

	rawCommP, pieceSize, err := cp.Digest()
	if err != nil {
		// 更新状态为失败
		_ = database.UpdateRegenerateStatus(file.ID, db.RegenerateStatusFailed)
		carF.Close()
		os.Remove(outPath)
		return fmt.Errorf("failed to compute commp: %v", err)
	}

	rawCommP, err = commp.PadCommP(
		rawCommP,
		pieceSize,
		file.PieceSize,
	)
	if err != nil {
		// 更新状态为失败
		_ = database.UpdateRegenerateStatus(file.ID, db.RegenerateStatusFailed)
		carF.Close()
		os.Remove(outPath)
		return fmt.Errorf("failed to pad commp: %v", err)
	}
	pieceSize = file.PieceSize

	commCid, err := commcid.DataCommitmentV1ToCID(rawCommP)
	if err != nil {
		// 更新状态为失败
		_ = database.UpdateRegenerateStatus(file.ID, db.RegenerateStatusFailed)
		carF.Close()
		os.Remove(outPath)
		return fmt.Errorf("failed to convert commp to CID: %v", err)
	}

	// 验证生成的 car 文件
	if commCid.String() != file.CommP {
		// 更新状态为失败
		_ = database.UpdateRegenerateStatus(file.ID, db.RegenerateStatusFailed)
		carF.Close()
		os.Remove(outPath)
		return fmt.Errorf("generated car file does not match original: expected CommP %s, got %s", file.CommP, commCid.String())
	}

	err = carF.Close()
	if err != nil {
		// 更新状态为失败
		_ = database.UpdateRegenerateStatus(file.ID, db.RegenerateStatusFailed)
		os.Remove(outPath)
		return fmt.Errorf("failed to close car file: %v", err)
	}

	// 重命名为最终文件名
	generatedFile := filepath.Join(outDir, fmt.Sprintf("%s.car", file.PieceCid))
	err = os.Rename(outPath, generatedFile)
	if err != nil {
		// 更新状态为失败
		_ = database.UpdateRegenerateStatus(file.ID, db.RegenerateStatusFailed)
		return fmt.Errorf("failed to rename car file: %v", err)
	}

	// 更新状态为成功
	err = database.UpdateRegenerateStatus(file.ID, db.RegenerateStatusSuccess)
	if err != nil {
		return fmt.Errorf("failed to update regenerate status: %v", err)
	}

	log.Printf("Successfully regenerated car file: %s", generatedFile)
	log.Printf("CommP: %s", commCid.String())
	log.Printf("DataCid: %s", cid)
	log.Printf("PieceSize: %d", pieceSize)

	return nil
}
