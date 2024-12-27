package regenerate

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
				Required: true,
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
			ctx := context.Background()
			// Load configuration
			cfg, err := config.LoadConfig(c.String("config"))
			if err != nil {
				return fmt.Errorf("failed to load config: %v", err)
			}

			id := c.String("id")
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

			// 获取原始 car 文件信息
			carFile, err := database.GetFile(id)
			if err != nil {
				return fmt.Errorf("failed to get car file: %v", err)
			}

			// 解析原始文件列表
			var rawFiles []db.RawFileInfo
			err = json.Unmarshal([]byte(carFile.RawFiles), &rawFiles)
			if err != nil {
				return fmt.Errorf("failed to unmarshal raw files: %v", err)
			}

			// 生成临时 car 文件
			outFilename := uuid.New().String() + ".car"
			outPath := path.Join(outDir, outFilename)
			carF, err := os.Create(outPath)
			if err != nil {
				return err
			}

			cp := new(commp.Calc)
			writer := bufio.NewWriterSize(io.MultiWriter(carF, cp), BufSize)
			var selectedFiles []util.Finfo
			for _, f := range rawFiles {
				selectedFiles = append(selectedFiles, util.Finfo{
					Path: filepath.Join(parent, strings.TrimPrefix(f.RelativePath, "/")),
					Size: f.Size,
				})
			}
			_, cid, _, err := util.GenerateCar(ctx, selectedFiles, parent, tmpDir, writer)
			if err != nil {
				carF.Close()
				os.Remove(outPath)
				return err
			}
			err = writer.Flush()
			if err != nil {
				carF.Close()
				os.Remove(outPath)
				return err
			}
			err = carF.Close()
			if err != nil {
				os.Remove(outPath)
				return err
			}

			rawCommP, pieceSize, err := cp.Digest()
			if err != nil {
				os.Remove(outPath)
				return err
			}

			// 使用原始 piece size
			rawCommP, err = commp.PadCommP(
				rawCommP,
				pieceSize,
				carFile.PieceSize,
			)
			if err != nil {
				os.Remove(outPath)
				return err
			}
			pieceSize = carFile.PieceSize

			commCid, err := commcid.DataCommitmentV1ToCID(rawCommP)
			if err != nil {
				os.Remove(outPath)
				return err
			}

			// 验证生成的 car 文件是否与原始文件一致
			if commCid.String() != carFile.CommP {
				os.Remove(outPath)
				return fmt.Errorf("generated car file does not match original: expected CommP %s, got %s", carFile.CommP, commCid.String())
			}

			// 重命名为最终文件名
			generatedFile := path.Join(outDir, commCid.String()+".car")
			err = os.Rename(outPath, generatedFile)
			if err != nil {
				return err
			}

			fmt.Printf("Successfully regenerated car file: %s\n", generatedFile)
			fmt.Printf("CommP: %s\n", commCid.String())
			fmt.Printf("DataCid: %s\n", cid)
			fmt.Printf("PieceSize: %d\n", pieceSize)

			return nil
		},
	}
}
