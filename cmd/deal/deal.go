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
			&cli.StringFlag{
				Name:  "from-piece-cids",
				Usage: "Path to file containing piece CIDs (one per line)",
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
			fromPieceCids := c.String("from-piece-cids")
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
				if err := sendDeals(cfg, miner, fromWallet, api, boostClientPath, fromPieceCids, startEpochDay, duration, total, reallyDoIt); err != nil {
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

func sendDeals(cfg *config.Config, miner, fromWallet, api, boostClientPath, fromPieceCids string, startEpochDay, duration int64, total int, reallyDoIt bool) error {
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

	log.Printf("Start epoch days: %d", startEpochDay)
	startEpoch := util.CurrentHeight() + (startEpochDay * 2880)

	var pendingDeals []db.CarFile

	if fromPieceCids != "" {
		// Read piece CIDs from file
		content, err := os.ReadFile(fromPieceCids)
		if err != nil {
			return fmt.Errorf("failed to read piece CIDs file: %v", err)
		}

		// Parse piece CIDs
		var pieceCids []string
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				pieceCids = append(pieceCids, line)
			}
		}

		if len(pieceCids) == 0 {
			return fmt.Errorf("no piece CIDs found in file: %s", fromPieceCids)
		}

		log.Printf("Loaded %d piece CIDs from file", len(pieceCids))

		// Query files by piece CIDs
		files, err := database.GetFilesByPieceCids(pieceCids)
		if err != nil {
			return fmt.Errorf("failed to get files by piece CIDs: %v", err)
		}

		pendingDeals = files

		// Log unmatched piece CIDs
		foundCids := make(map[string]bool)
		for _, file := range files {
			foundCids[file.CommP] = true
		}

		var unmatchedCids []string
		for _, cid := range pieceCids {
			if !foundCids[cid] {
				unmatchedCids = append(unmatchedCids, cid)
			}
		}

		if len(unmatchedCids) > 0 {
			log.Printf("Warning: %d piece CIDs from file not found in database:", len(unmatchedCids))
			for _, cid := range unmatchedCids {
				log.Printf("  - %s", cid)
			}
		}

		log.Printf("Found %d matching files for specified piece CIDs", len(pendingDeals))
	} else {
		// Get pending deals from database
		files, err := database.ListPendingFiles()
		if err != nil {
			return fmt.Errorf("failed to list pending files: %v", err)
		}

		pendingDeals = files

		// Apply total limit if no piece CIDs specified
		if total > 0 && total < len(pendingDeals) {
			pendingDeals = pendingDeals[:total]
		}
	}

	if len(pendingDeals) == 0 {
		log.Println("No files found to process")
		return nil
	}

	log.Printf("Will process %d deals", len(pendingDeals))

	successCount := 0
	failureCount := 0

	for i, file := range pendingDeals {
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

		log.Printf("[%d/%d] Processing file %s", i+1, len(pendingDeals), file.FilePath)
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
			if i < len(pendingDeals)-1 {
				time.Sleep(time.Duration(cfg.Deal.DealDelay) * time.Millisecond)
				log.Printf("[%d/%d] Delayed for %d seconds", i+1, len(pendingDeals), cfg.Deal.DealDelay/1000)
			}
		}
	}

	// Print summary
	if reallyDoIt {
		log.Printf("\nDeal Summary:")
		log.Printf("Total Processed: %d", len(pendingDeals))
		log.Printf("Successful: %d", successCount)
		log.Printf("Failed: %d", failureCount)
	}

	return nil
}
