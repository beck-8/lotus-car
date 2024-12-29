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
			&cli.StringFlag{
				Name:     "car-dir",
				Usage:    "Directory containing car files",
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
		},
		Action: func(c *cli.Context) error {
			// Load configuration
			cfg, err := config.LoadConfig(c.String("config"))
			if err != nil {
				return fmt.Errorf("failed to load config: %v", err)
			}

			carDir := c.String("car-dir")
			boostdPath := c.String("boostd-path")
			total := c.Int("total")
			interval := c.Int64("interval")

			for {
				if err := importDeals(cfg, carDir, boostdPath, total); err != nil {
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

func importDeals(cfg *config.Config, carDir, boostdPath string, total int) error {
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

	// Get all proposed deals
	deals, err := database.GetDealsByStatus("proposed")
	if err != nil {
		return fmt.Errorf("failed to get proposed deals: %v", err)
	}

	if len(deals) == 0 {
		log.Println("No proposed deals found")
		return nil
	}

	// Determine how many deals to process
	dealsToProcess := len(deals)
	if total > 0 && total < dealsToProcess {
		dealsToProcess = total
	}

	log.Printf("Found %d proposed deals, will process %d deals", len(deals), dealsToProcess)

	successCount := 0
	failureCount := 0

	for i := 0; i < dealsToProcess; i++ {
		deal := deals[i]

		// Construct car file path
		carFile := filepath.Join(carDir, deal.CommP+".car")

		// Check if car file exists
		if _, err := os.Stat(carFile); os.IsNotExist(err) {
			log.Printf("Car file not found for deal %s: %s", deal.UUID, carFile)
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

		log.Printf("Successfully imported deal %s", deal.UUID)

		// Update deal status to imported
		if err = database.UpdateDealStatus(deal.UUID, "imported"); err != nil {
			log.Printf("Failed to update deal status: %v", err)
			failureCount++
			continue
		}

		successCount++

		// Wait before next import if not the last one
		if i < dealsToProcess-1 {
			log.Printf("[%d/%d] Waiting %d seconds before next import...", i+1, dealsToProcess, 10)
			time.Sleep(10 * time.Second)
		}
	}

	// Print summary
	log.Printf("\nImport Summary:")
	log.Printf("Total Processed: %d", dealsToProcess)
	log.Printf("Successful: %d", successCount)
	log.Printf("Failed: %d", failureCount)

	return nil
}
