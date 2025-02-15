package importdeal

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/minerdao/lotus-car/config"
	"github.com/minerdao/lotus-car/db"
	"github.com/urfave/cli/v2"
)

func Command() *cli.Command {
	return &cli.Command{
		Name:  "import-deal",
		Usage: "Import proposed deals data to boost",
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:     "car-dir",
				Usage:    "Directories containing car files (can be specified multiple times)",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "boostd-path",
				Usage: "Path to boostd executable",
				Value: "boostd",
			},
			&cli.IntFlag{
				Name:  "total",
				Usage: "Number of deals to import (0 means all)",
				Value: 0,
			},
			&cli.Int64Flag{
				Name:  "interval",
				Usage: "Loop interval in seconds (0 means run once)",
				Value: 0,
			},
			&cli.BoolFlag{
				Name:  "regenerated",
				Usage: "Only import deals with regenerated car files",
				Value: false,
			},
		},
		Action: func(c *cli.Context) error {
			// Load configuration
			cfg, err := config.LoadConfig(c.String("config"))
			if err != nil {
				return fmt.Errorf("failed to load config: %v", err)
			}

			carDirs := c.StringSlice("car-dir")
			boostdPath := c.String("boostd-path")
			total := c.Int("total")
			interval := c.Int64("interval")
			regenerated := c.Bool("regenerated")

			for {
				if err := importDeals(cfg, carDirs, boostdPath, total, regenerated); err != nil {
					log.Printf("Error importing deals: %v", err)
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

func importDeals(cfg *config.Config, carDirs []string, boostdPath string, total int, regenerated bool) error {
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

	var deals []db.Deal
	if regenerated {
		// 获取status为proposed且对应文件regenerate_status为success的订单
		deals, err = database.GetProposedDealsWithRegeneratedFiles()
	} else {
		// 获取所有proposed状态的订单
		deals, err = database.GetDealsByStatus("proposed")
	}

	if err != nil {
		return fmt.Errorf("failed to get deals: %v", err)
	}

	if len(deals) == 0 {
		log.Println("No deals found")
		return nil
	}

	// Determine how many deals to process
	dealsToProcess := len(deals)
	if total > 0 && total < dealsToProcess {
		dealsToProcess = total
	}

	log.Printf("Found %d deals, will process %d deals", len(deals), dealsToProcess)

	successCount := 0
	failureCount := 0

	for i := 0; i < dealsToProcess; i++ {
		deal := deals[i]

		// Search for car file in all directories
		var carFile string
		var found bool
		for _, dir := range carDirs {
			path := filepath.Join(dir, deal.CommP+".car")
			if _, err := os.Stat(path); err == nil {
				carFile = path
				found = true
				break
			}
		}
		
		if !found {
			log.Printf("Car file not found for deal %s in any of the specified directories", deal.UUID)
			failureCount++
			continue
		}

		// Construct import command
		cmd := exec.Command(boostdPath, "import-data", deal.UUID, carFile)

		// Run the command
		log.Printf("[%d/%d] Importing deal %s with car file %s", i+1, dealsToProcess, deal.UUID, carFile)
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("Failed to import deal %s: %v\nOutput: %s", deal.UUID, err, string(output))
			failureCount++
			continue
		}

		// Update deal status to imported
		if err := database.UpdateDealStatus(deal.UUID, "imported"); err != nil {
			log.Printf("Failed to update deal status for %s: %v", deal.UUID, err)
			failureCount++
			continue
		}

		successCount++
	}

	log.Printf("Import completed. Success: %d, Failure: %d", successCount, failureCount)
	return nil
}
