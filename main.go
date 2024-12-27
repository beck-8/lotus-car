package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"database/sql"

	commcid "github.com/filecoin-project/go-fil-commcid"
	commp "github.com/filecoin-project/go-fil-commp-hashhash"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	_ "github.com/lib/pq"
	"github.com/minerdao/lotus-car/api"
	"github.com/minerdao/lotus-car/config"
	"github.com/minerdao/lotus-car/db"
	"github.com/minerdao/lotus-car/util"
	"github.com/urfave/cli/v2"
)

type CommpResult struct {
	commp     string
	pieceSize uint64
}

type Result struct {
	Ipld *util.FsNode
	// FileSize  uint64
	DataCid   string
	PieceCid  string
	PieceSize uint64
	CidMap    map[string]util.CidMapValue
}

type Input []util.Finfo

type CarHeader struct {
	Roots   []cid.Cid
	Version uint64
}

func init() {
	cbor.RegisterCborType(CarHeader{})
}

const BufSize = (4 << 20) / 128 * 127

// 本地 lotus 二进制文件位置
const localLotusPath = "/Users/max/workspace/filecoin/lotus-optimized/lotus"

const localBoostPath = "/Users/max/workspace/filecoin/boost/boost --repo=/Users/max/.boost-client"

// 每轮重试的间隔时间
const dealDelay = 100 * time.Millisecond

// execCmd 执行命令并返回输出
func execCmd(env, c string) (string, error) {
	cmd := exec.Command("bash", "-c", c)
	cmd.Env = append(os.Environ(), env)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("command failed: %v, stderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

func main() {
	ctx := context.Background()

	app := &cli.App{
		Name:  "lotus-car",
		Usage: "A tool for generating car files",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "Path to config file",
				Value:   "config.yaml",
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "init",
				Usage: "Initialize default configuration file",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "config",
						Aliases: []string{"c"},
						Usage:   "Path to config file",
						Value:   "config.yaml",
					},
				},
				Action: func(c *cli.Context) error {
					configPath := c.String("config")
					if err := config.SaveDefaultConfig(configPath); err != nil {
						return fmt.Errorf("failed to save default config: %v", err)
					}
					fmt.Printf("Default configuration saved to %s\n", configPath)
					return nil
				},
			},
			{
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
						fmt.Printf("Generated %d car %s took %s \n", i, generatedFile, elapsed)

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
							CommP:     commCid.String(),
							DataCid:   cid,
							PieceCid:  commCid.String(),
							PieceSize: pieceSize,
							CarSize:   uint64(carFi.Size()),
							FilePath:  generatedFile,
							RawFiles:  string(rawFilesBytes),
						}

						err = database.InsertCarFile(carFile)
						if err != nil {
							return fmt.Errorf("failed to insert car file record: %v", err)
						}

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
			},
			{
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
					carFile, err := database.GetCarFile(id)
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
			},
			{
				Name:  "init-db",
				Usage: "Initialize the database",
				Action: func(c *cli.Context) error {
					// Load configuration
					cfg, err := config.LoadConfig(c.String("config"))
					if err != nil {
						return fmt.Errorf("failed to load config: %v", err)
					}

					// 连接到 postgres 数据库来创建新数据库
					connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=%s",
						cfg.Database.Host,
						cfg.Database.Port,
						cfg.Database.User,
						cfg.Database.Password,
						cfg.Database.SSLMode,
					)

					sqlDB, err := sql.Open("postgres", connStr)
					if err != nil {
						return fmt.Errorf("failed to connect to postgres: %v", err)
					}
					defer sqlDB.Close()

					// 检查数据库是否存在
					var exists bool
					err = sqlDB.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", cfg.Database.DBName).Scan(&exists)
					if err != nil {
						return fmt.Errorf("failed to check if database exists: %v", err)
					}

					// 如果数据库不存在，创建它
					if !exists {
						_, err = sqlDB.Exec(fmt.Sprintf("CREATE DATABASE %s", cfg.Database.DBName))
						if err != nil {
							return fmt.Errorf("failed to create database: %v", err)
						}
						fmt.Printf("Created database %s\n", cfg.Database.DBName)
					} else {
						fmt.Printf("Database %s already exists\n", cfg.Database.DBName)
					}

					// 初始化数据库表结构
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
						return fmt.Errorf("failed to initialize database tables: %v", err)
					}
					defer database.Close()

					fmt.Println("Database initialization completed successfully")
					return nil
				},
			},
			{
				Name:  "serve",
				Usage: "Start the API server",
				Action: func(c *cli.Context) error {
					// Load configuration
					cfg, err := config.LoadConfig(c.String("config"))
					if err != nil {
						return fmt.Errorf("failed to load config: %v", err)
					}

					dbConfig := &db.DBConfig{
						Host:     cfg.Database.Host,
						Port:     cfg.Database.Port,
						User:     cfg.Database.User,
						Password: cfg.Database.Password,
						DBName:   cfg.Database.DBName,
						SSLMode:  cfg.Database.SSLMode,
					}

					apiServer, err := api.NewAPIServer(dbConfig)
					if err != nil {
						return fmt.Errorf("failed to create API server: %v", err)
					}

					mux := http.NewServeMux()
					mux.HandleFunc("/api/car-files", apiServer.ListCarFiles)
					mux.HandleFunc("/api/car-file", apiServer.GetCarFile)   // GET with ?id=X
					mux.HandleFunc("/api/delete", apiServer.DeleteCarFile)  // DELETE with ?id=X
					mux.HandleFunc("/api/search", apiServer.SearchCarFiles) // GET with query params

					log.Printf("Starting API server on %s", cfg.Server.Address)
					return http.ListenAndServe(cfg.Server.Address, mux)
				},
			},
			{
				Name:  "deal",
				Usage: "Send deals for car files",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "miner",
						Usage:    "Storage provider ID",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "from-wallet",
						Usage:    "Wallet address to send deals from",
						Required: true,
					},
					&cli.Int64Flag{
						Name:  "start-epoch-day",
						Value: 10,
						Usage: "Start epoch in days",
					},
					&cli.Int64Flag{
						Name:  "duration",
						Value: 1512000,
						Usage: "Deal duration in epochs",
					},
					&cli.BoolFlag{
						Name:  "use-boost",
						Value: true,
						Usage: "Use Boost for deal making",
					},
					&cli.BoolFlag{
						Name:  "really-do-it",
						Usage: "Actually execute the deal commands",
					},
					&cli.StringFlag{
						Name:     "api",
						Usage:    "FULLNODE_API_INFO",
						Required: true,
					},
				},
				Action: func(c *cli.Context) error {
					// Load configuration
					cfg, err := config.LoadConfig(c.String("config"))
					if err != nil {
						return fmt.Errorf("failed to load config: %v", err)
					}

					// Initialize database connection
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

					// Get pending deals from database
					files, err := database.ListCarFiles()
					if err != nil {
						return fmt.Errorf("failed to list car files: %v", err)
					}

					miner := c.String("miner")
					fromWallet := c.String("from-wallet")
					startEpochDays := c.Int64("start-epoch-day")
					duration := c.Int64("duration")
					useBoost := c.Bool("use-boost")
					reallyDoIt := c.Bool("really-do-it")
					api := c.String("api")

					startEpoch := uint64(time.Now().Unix() + startEpochDays*24*60*60)

					// Process each pending deal
					for _, file := range files {
						if file.DealStatus != db.DealStatusPending {
							continue
						}

						// Prepare deal command
						cmd := ""
						if useBoost {
							cmd = fmt.Sprintf("%s client deal --verified-deal=false --provider=%s "+
								"--commp=%s --piece-size=%d --car-size=%d "+
								"--payload-cid=%s --storage-price-per-epoch=0 "+
								"--start-epoch=%d --duration=%d --wallet=%s %s",
								localBoostPath, miner, file.CommP, file.PieceSize, file.CarSize,
								file.DataCid, startEpoch, duration, fromWallet, file.FilePath)
						} else {
							cmd = fmt.Sprintf("%s client deal --verified-deal=false --provider=%s "+
								"--piece-cid=%s --piece-size=%d --car-size=%d "+
								"--payload-cid=%s --start-epoch=%d --duration=%d "+
								"--wallet=%s %s",
								localLotusPath, miner, file.PieceCid, file.PieceSize, file.CarSize,
								file.DataCid, startEpoch, duration, fromWallet, file.FilePath)
						}

						fmt.Printf("Processing file %s with command: %s\n", file.FilePath, cmd)

						if reallyDoIt {
							dealResponse, err := execCmd(api, cmd)
							if err != nil {
								errMsg := fmt.Sprintf("Failed to send deal: %v", err)
								fmt.Printf("Failed to send deal for file %s: %v\n", file.FilePath, errMsg)
								// Update deal status to failed with error message
								err = database.UpdateDealStatus(file.ID, db.DealStatusFailed, errMsg)
								if err != nil {
									fmt.Printf("Failed to update deal status: %v\n", err)
								}
								continue
							}

							fmt.Printf("Deal sent successfully for file %s: %s\n", file.FilePath, dealResponse)

							// Update deal status to success with no error message
							err = database.UpdateDealStatus(file.ID, db.DealStatusSuccess, "")
							if err != nil {
								fmt.Printf("Failed to update deal status: %v\n", err)
							}

							// Add delay between deals
							time.Sleep(dealDelay)
						}
					}

					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
