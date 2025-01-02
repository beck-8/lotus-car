package exportfile

import (
	"fmt"
	"log"
	"time"

	"github.com/minerdao/lotus-car/config"
	"github.com/minerdao/lotus-car/db"
	"github.com/urfave/cli/v2"
)

func Command() *cli.Command {
	return &cli.Command{
		Name:  "export-file",
		Usage: "Export files by deal status and deal time",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "deal-status",
				Usage: "Filter by deal status (pending/success/failed)",
			},
			&cli.StringFlag{
				Name:  "start-time",
				Usage: "Filter by deal time start (format: 2006-01-02 15:04:05)",
			},
			&cli.StringFlag{
				Name:  "end-time",
				Usage: "Filter by deal time end (format: 2006-01-02 15:04:05)",
			},
		},
		Action: func(c *cli.Context) error {
			// Load configuration
			cfg, err := config.LoadConfig(c.String("config"))
			if err != nil {
				return fmt.Errorf("failed to load config: %v", err)
			}

			dealStatus := c.String("deal-status")
			startTimeStr := c.String("start-time")
			endTimeStr := c.String("end-time")

			// Parse time strings if provided
			var startTime, endTime *time.Time
			if startTimeStr != "" {
				t, err := time.ParseInLocation("2006-01-02 15:04:05", startTimeStr, time.Local)
				if err != nil {
					return fmt.Errorf("invalid start time format: %v", err)
				}
				startTime = &t
			}
			if endTimeStr != "" {
				t, err := time.ParseInLocation("2006-01-02 15:04:05", endTimeStr, time.Local)
				if err != nil {
					return fmt.Errorf("invalid end time format: %v", err)
				}
				endTime = &t
			}

			return exportFiles(cfg, dealStatus, startTime, endTime)
		},
	}
}

func exportFiles(cfg *config.Config, dealStatus string, startTime, endTime *time.Time) error {
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

	// Get files with filters
	files, err := database.GetFilesByDealStatus(dealStatus, startTime, endTime)
	if err != nil {
		return fmt.Errorf("failed to get files: %v", err)
	}

	// Print piece IDs
	for _, file := range files {
		fmt.Println(file.PieceCid)
	}

	log.Printf("Total files exported: %d", len(files))
	return nil
}
