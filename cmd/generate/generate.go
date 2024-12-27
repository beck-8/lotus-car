package generate

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	commcid "github.com/filecoin-project/go-fil-commcid"
	commp "github.com/filecoin-project/go-fil-commp-hashhash"
	"github.com/google/uuid"
	"github.com/minerdao/lotus-car/config"
	"github.com/minerdao/lotus-car/db"
	"github.com/minerdao/lotus-car/util"
	"github.com/urfave/cli/v2"
)

const BufSize = (4 << 20) / 128 * 127

type Input []util.Finfo

func Command() *cli.Command {
	return &cli.Command{
		Name:  "generate",
		Usage: "Generate car archive from list of files and compute commp",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "input",
				Aliases: []string{"i"},
				Usage:   "File to read list of files, or '-' if from stdin",
				Value:   "-",
			},
			&cli.Uint64Flag{
				Name:    "quantity",
				Aliases: []string{"q"},
				Usage:   "Quantity of car files",
				Value:   3,
			},
			&cli.Uint64Flag{
				Name:  "file-size",
				Usage: "Target car file size, default to 32GiB size sector",
				Value: 19327352832,
			},
			&cli.Uint64Flag{
				Name:    "piece-size",
				Aliases: []string{"s"},
				Usage:   "Target piece size, default to minimum possible value",
				Value:   34359738368,
			},
			&cli.StringFlag{
				Name:  "out-file",
				Usage: "Output file as .csv format to save the car file",
				Value: "./source.csv",
			},
			&cli.StringFlag{
				Name:    "out-dir",
				Aliases: []string{"o"},
				Usage:   "Output directory to save the car file",
				Value:   ".",
			},
			&cli.StringFlag{
				Name:    "tmp-dir",
				Aliases: []string{"t"},
				Usage:   "Optionally copy the files to a temporary (and much faster) directory",
				Value:   "",
			},
			&cli.StringFlag{
				Name:     "parent",
				Aliases:  []string{"p"},
				Usage:    "Parent path of the dataset",
				Required: true,
			},
		},
		Action: func(c *cli.Context) error {
			ctx := context.Background()
			// Load configuration
			cfg, err := config.LoadConfig(c.String("config"))
			if err != nil {
				return fmt.Errorf("failed to load config: %v", err)
			}

			inputFile := c.String("input")
			fileSizeInput := c.Uint64("file-size")
			pieceSizeInput := c.Uint64("piece-size")
			quantity := c.Uint64("quantity")
			outFile := c.String("out-file")
			outDir := c.String("out-dir")
			parent := c.String("parent")
			tmpDir := c.String("tmp-dir")

			var inputBytes []byte
			if inputFile == "-" {
				reader := bufio.NewReader(os.Stdin)
				buf := new(bytes.Buffer)
				_, err := buf.ReadFrom(reader)
				if err != nil {
					return err
				}
				inputBytes = buf.Bytes()
			} else {
				bytes, err := os.ReadFile(inputFile)
				if err != nil {
					return err
				}
				inputBytes = bytes
			}

			var inputFiles Input
			err = json.Unmarshal(inputBytes, &inputFiles)
			if err != nil {
				return err
			}

			csvF, err := os.Create(outFile)
			if err != nil {
				return err
			}
			defer csvF.Close()

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

			for i := 0; i < int(quantity); i++ {
				start := time.Now()
				var selectedFiles []util.Finfo
				totalSize := 0
				rng := rand.New(rand.NewSource(time.Now().UnixNano()))
				for totalSize < int(fileSizeInput) {
					choicedFile := inputFiles[rng.Intn(len(inputFiles))]
					totalSize += int(choicedFile.Size)
					fileInfo := choicedFile
					selectedFiles = append(selectedFiles, fileInfo)
				}

				fmt.Printf("Will generate file with %d bytes\n", totalSize)

				outFilename := uuid.New().String() + ".car"
				outPath := path.Join(outDir, outFilename)
				carF, err := os.Create(outPath)
				if err != nil {
					return err
				}

				cp := new(commp.Calc)
				writer := bufio.NewWriterSize(io.MultiWriter(carF, cp), BufSize)
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
					return err
				}
				if pieceSizeInput > 0 {
					rawCommP, err = commp.PadCommP(
						rawCommP,
						pieceSize,
						pieceSizeInput,
					)
					if err != nil {
						return err
					}
					pieceSize = pieceSizeInput
				}
				commCid, err := commcid.DataCommitmentV1ToCID(rawCommP)
				if err != nil {
					return err
				}

				generatedFile := path.Join(outDir, commCid.String()+".car")
				err = os.Rename(outPath, generatedFile)
				if err != nil {
					return err
				}
				elapsed := time.Since(start)
				fmt.Printf("Generated %d car %s took %s \n", i+1, generatedFile, elapsed)

				// get car file size
				carFi, err := os.Stat(generatedFile)
				if err != nil {
					return err
				}

				// 将选中的文件信息转换为优化后的结构
				var rawFileInfos []db.RawFileInfo
				for _, f := range selectedFiles {
					relPath := strings.TrimPrefix(f.Path, parent)
					relPath = strings.TrimPrefix(relPath, "/")
					rawFileInfos = append(rawFileInfos, db.RawFileInfo{
						Name:         filepath.Base(f.Path),
						Size:         f.Size,
						RelativePath: relPath,
					})
				}

				rawFilesBytes, err := json.Marshal(rawFileInfos)
				if err != nil {
					return fmt.Errorf("failed to marshal raw files: %v", err)
				}

				carFile := &db.CarFile{
					CommP:      commCid.String(),
					DataCid:    cid,
					PieceCid:   commCid.String(),
					PieceSize:  pieceSize,
					CarSize:    uint64(carFi.Size()),
					FilePath:   generatedFile,
					RawFiles:   string(rawFilesBytes),
					DealStatus: db.DealStatusPending,
				}

				err = database.InsertFile(carFile)
				if err != nil {
					return fmt.Errorf("failed to insert car file: %v", err)
				}

				fmt.Printf("Car file information saved to database with ID: %s\n", carFile.ID)

				outItem := []string{
					commCid.String(),
					strconv.Itoa(int(carFi.Size())),
					strconv.Itoa(int(pieceSize)),
					cid,
				}

				csvWriter := csv.NewWriter(csvF)
				csvWriter.Write(outItem)
				csvWriter.Flush()

				fmt.Printf("Saved %s to database and CSV\n", commCid.String())
			}
			return nil
		},
	}
}
