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
				Name:  "boost-path",
				Usage: "Path to boostd executable",
				Value: "boostd",
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

			carDir := c.String("car-dir")
			boostPath := c.String("boost-path")

			// Get all proposed deals
			deals, err := database.GetDealsByStatus("proposed")
			if err != nil {
				return fmt.Errorf("failed to get proposed deals: %v", err)
			}

			if len(deals) == 0 {
				fmt.Println("No proposed deals found")
				return nil
			}

			fmt.Printf("Found %d proposed deals\n", len(deals))

			for _, deal := range deals {
				// Construct car file path
				carFile := filepath.Join(carDir, deal.CommP+".car")

				// Check if car file exists
				if _, err := os.Stat(carFile); os.IsNotExist(err) {
					log.Printf("Car file not found for deal %s: %s\n", deal.UUID, carFile)
					continue
				}

				// Construct import command
				cmd := exec.Command(boostPath, "import-data", deal.UUID, carFile)

				// Run the command
				fmt.Printf("Importing deal %s with car file %s\n", deal.UUID, carFile)
				output, err := cmd.CombinedOutput()
				if err != nil {
					log.Printf("Failed to import deal %s: %v\nOutput: %s\n", deal.UUID, err, string(output))
					continue
				}

				fmt.Printf("Successfully imported deal %s\n", deal.UUID)

				// Update deal status to imported
				err = database.UpdateDealStatus(deal.UUID, "imported")
				if err != nil {
					log.Printf("Failed to update deal status: %v\n", err)
				}

				log.Printf("Sleeping for 10 seconds before importing next deal\n")
				time.Sleep(time.Second * 10)
			}

			return nil
		},
	}
}
