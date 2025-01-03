package updatedeal

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/minerdao/lotus-car/config"
	"github.com/minerdao/lotus-car/db"
	"github.com/minerdao/lotus-car/util"
)

type DealResponse struct {
	ID         string `json:"ID"`
	Checkpoint string `json:"Checkpoint"`
	Message    string `json:"Message"`
}

// BoostDealStatus represents the parsed response from boost deal-status command
type BoostDealStatus struct {
	UUID        string
	Status      string
	Label       string
	PublishCid  string
	ChainDealID int64
}

func parseDealStatus(output string) (*BoostDealStatus, error) {
	status := &BoostDealStatus{}
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "deal uuid:") {
			status.UUID = strings.TrimSpace(strings.TrimPrefix(line, "deal uuid:"))
		} else if strings.HasPrefix(line, "deal status:") {
			status.Status = strings.TrimSpace(strings.TrimPrefix(line, "deal status:"))
		} else if strings.HasPrefix(line, "deal label:") {
			status.Label = strings.TrimSpace(strings.TrimPrefix(line, "deal label:"))
		} else if strings.HasPrefix(line, "publish cid:") {
			status.PublishCid = strings.TrimSpace(strings.TrimPrefix(line, "publish cid:"))
		} else if strings.HasPrefix(line, "chain deal id:") {
			idStr := strings.TrimSpace(strings.TrimPrefix(line, "chain deal id:"))
			id, err := strconv.ParseInt(idStr, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse chain deal id: %w", err)
			}
			status.ChainDealID = id
		}
	}
	return status, nil
}

func Command() *cli.Command {
	return &cli.Command{
		Name:  "update-deal",
		Usage: "Update deal statuses for imported deal",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "config",
				Aliases:  []string{"c"},
				Usage:    "path to config file",
				Required: true,
			},
			&cli.IntFlag{
				Name:    "interval",
				Aliases: []string{"i"},
				Usage:   "update interval in seconds",
				Value:   300, // default 5 minutes
			},
		},
		Action: func(cctx *cli.Context) error {
			cfg, err := config.LoadConfig(cctx.String("config"))
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

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
				return fmt.Errorf("failed to init database: %w", err)
			}
			defer database.Close()

			interval := time.Duration(cctx.Int("interval")) * time.Second
			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			log.Printf("Starting deal status update service with interval: %v", interval)

			for {
				select {
				case <-cctx.Context.Done():
					return nil
				case <-ticker.C:
					if err := updateDeals(database, cfg.Deal.BoostPath); err != nil {
						log.Printf("Error updating deals: %v", err)
					}
				}
			}
		},
	}
}

func updateDeals(database *db.Database, boostPath string) error {
	// Get all deals with "imported" status
	deals, err := database.GetDealsByStatus("imported")
	if err != nil {
		return fmt.Errorf("failed to get imported deals: %w", err)
	}

	log.Printf("Found %d imported deals to check", len(deals))

	for _, deal := range deals {
		// Query deal status using boost CLI
		cmd := fmt.Sprintf("%s deal-status --provider=%s --deal-uuid=%s --wallet=%s", boostPath, deal.StorageProvider, deal.UUID, deal.ClientWallet)
		output, err := util.ExecCmd("", cmd)
		if err != nil {
			log.Printf("Error querying deal status for %s: %v", deal.UUID, err)
			continue
		}

		dealStatus, err := parseDealStatus(output)
		if err != nil {
			log.Printf("Error parsing deal status response for %s: %v", deal.UUID, err)
			continue
		}

		// Map boost status to our status
		var newStatus string
		if strings.Contains(dealStatus.Status, "Proving") {
			newStatus = "success"
		} else if strings.Contains(dealStatus.Status, "Error") {
			newStatus = "failed"
		} else {
			// Deal is still in progress
			continue
		}

		// Update deal status in database
		if err := database.UpdateDealStatus(deal.UUID, newStatus); err != nil {
			log.Printf("Error updating deal status for %s: %v", deal.UUID, err)
			continue
		}

		log.Printf("Updated deal %s status from imported to %s", deal.UUID, newStatus)
	}

	return nil
}
