package deal

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/minerdao/lotus-car/config"
	"github.com/minerdao/lotus-car/db"
	"github.com/minerdao/lotus-car/util"
	"github.com/urfave/cli/v2"
)

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

// parseDealResponse parses the deal response string and returns a Deal struct
func parseDealResponse(response string) (*db.Deal, error) {
	lines := strings.Split(response, "\n")
	deal := &db.Deal{
		Status: "proposed", // Initial status when deal is created
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "deal uuid":
			deal.UUID = value
		case "storage provider":
			deal.StorageProvider = value
		case "client wallet":
			deal.ClientWallet = value
		case "payload cid":
			deal.PayloadCid = value
		case "commp":
			deal.CommP = value
		case "start epoch":
			epoch, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse start epoch: %v", err)
			}
			deal.StartEpoch = epoch
		case "end epoch":
			epoch, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse end epoch: %v", err)
			}
			deal.EndEpoch = epoch
		case "provider collateral":
			// Parse "X.XXX mFIL" format
			collateralStr := strings.Split(value, " ")[0]
			collateral, err := strconv.ParseFloat(collateralStr, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse provider collateral: %v", err)
			}
			deal.ProviderCollateral = collateral
		}
	}

	// Validate required fields
	if deal.UUID == "" {
		return nil, fmt.Errorf("deal UUID not found in response")
	}

	return deal, nil
}

func Command() *cli.Command {
	return &cli.Command{
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
				Usage:    "Client wallet address",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "api",
				Usage:    "Lotus API endpoint",
				Required: true,
				Value:    "https://api.node.glif.io",
			},
			&cli.StringFlag{
				Name:  "boost-client-path",
				Usage: "Path to boost client executable (overrides config file)",
			},
			&cli.Int64Flag{
				Name:  "start-epoch-day",
				Value: 10,
				Usage: "Start epoch in days",
			},
			&cli.Int64Flag{
				Name:  "duration",
				Value: 3513600,
				Usage: "Deal duration in epochs (default: 3.55 years)",
			},
			&cli.IntFlag{
				Name:  "total",
				Usage: "Number of deals to send",
				Value: 1,
			},
			&cli.BoolFlag{
				Name:  "really-do-it",
				Usage: "Actually send the deals",
				Value: false,
			},
			&cli.Int64Flag{
				Name:  "interval",
				Usage: "Loop interval in seconds (0 means run once)",
				Value: 0,
			},
		},
		Action: func(c *cli.Context) error {
			// Load configuration
			cfg, err := config.LoadConfig(c.String("config"))
			if err != nil {
				return fmt.Errorf("failed to load config: %v", err)
			}

			miner := c.String("miner")
			fromWallet := c.String("from-wallet")
			api := c.String("api")
			boostClientPath := c.String("boost-client-path")
			startEpochDay := c.Int64("start-epoch-day")
			duration := c.Int64("duration")
			total := c.Int("total")
			reallyDoIt := c.Bool("really-do-it")
			interval := c.Int64("interval")

			// Use command line boost path if provided, otherwise use config
			if boostClientPath == "" {
				boostClientPath = cfg.Deal.BoostPath
			}

			for {
				if err := sendDeals(cfg, miner, fromWallet, api, boostClientPath, startEpochDay, duration, total, reallyDoIt); err != nil {
					log.Printf("Error sending deals: %v", err)
				}

				if interval <= 0 {
					break // Run once and exit
				}

				time.Sleep(time.Duration(interval) * time.Second)
			}

			return nil
		},
	}
}

func sendDeals(cfg *config.Config, miner, fromWallet, api, boostClientPath string, startEpochDay, duration int64, total int, reallyDoIt bool) error {
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
	files, err := database.ListFiles()
	if err != nil {
		return fmt.Errorf("failed to list car files: %v", err)
	}

	log.Printf("Start epoch days: %d", startEpochDay)
	startEpoch := util.CurrentHeight() + (startEpochDay * 2880)

	// Filter pending deals
	var pendingDeals []db.CarFile
	for _, file := range files {
		if file.DealStatus == db.DealStatusPending {
			pendingDeals = append(pendingDeals, file)
		}
	}

	// Determine how many deals to process
	totalPending := len(pendingDeals)
	if totalPending == 0 {
		log.Println("No pending deals found")
		return nil
	}

	dealsToProcess := totalPending
	if total > 0 && total < totalPending {
		dealsToProcess = total
	}

	log.Printf("Found %d pending deals, will process %d deals", totalPending, dealsToProcess)

	// Process deals
	successCount := 0
	failureCount := 0

	for i := 0; i < dealsToProcess; i++ {
		file := pendingDeals[i]

		// Prepare deal command
		cmd := boostClientPath + " offline-deal " +
			"--provider=" + miner + " " +
			"--commp=" + file.CommP + " " +
			"--piece-size=" + strconv.FormatUint(file.PieceSize, 10) + " " +
			"--wallet=" + fromWallet + " " +
			"--payload-cid=" + file.DataCid + " " +
			"--verified=true " +
			"--duration=" + strconv.FormatInt(duration, 10) + " " +
			"--storage-price=0 " +
			"--start-epoch=" + strconv.FormatInt(startEpoch, 10)

		log.Printf("[%d/%d] Processing file %s", i+1, dealsToProcess, file.FilePath)
		log.Printf("Command: %s", cmd)

		if reallyDoIt {
			dealResponse, err := execCmd(api, cmd)
			if err != nil {
				errMsg := fmt.Sprintf("Failed to send deal: %v", err)
				log.Printf("Failed to send deal for file %s: %v", file.FilePath, errMsg)
				// Update deal status to failed with error message
				if err = database.UpdateDealSentStatus(file.ID, db.DealStatusFailed, errMsg); err != nil {
					log.Printf("Failed to update deal status: %v", err)
				}
				failureCount++
				continue
			}

			log.Printf("Deal sent successfully for file %s: %s", file.FilePath, dealResponse)

			// Parse deal response
			deal, err := parseDealResponse(dealResponse)
			if err != nil {
				log.Printf("Failed to parse deal response: %v", err)
				failureCount++
				continue
			}

			// Save deal to database
			if err = database.InsertDeal(deal); err != nil {
				log.Printf("Failed to save deal: %v", err)
				failureCount++
				continue
			}

			// Update car_files with deal UUID
			if err = database.UpdateDealSentStatus(file.ID, db.DealStatusSuccess, deal.UUID); err != nil {
				log.Printf("Failed to update deal status: %v", err)
			}
			successCount++

			// Add delay between deals
			if i < dealsToProcess-1 {
				time.Sleep(time.Duration(cfg.Deal.DealDelay) * time.Millisecond)
				log.Printf("[%d/%d] Delayed for %d seconds", i+1, dealsToProcess, cfg.Deal.DealDelay/1000)
			}
		}
	}

	// Print summary
	if reallyDoIt {
		log.Printf("\nDeal Summary:")
		log.Printf("Total Processed: %d", dealsToProcess)
		log.Printf("Successful: %d", successCount)
		log.Printf("Failed: %d", failureCount)
	}

	return nil
}
