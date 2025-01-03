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
			&cli.IntFlag{
				Name:    "interval",
				Aliases: []string{"i"},
				Usage:   "Loop interval in seconds (0 means run once)",
				Value:   0,
			},
			&cli.StringFlag{
				Name:    "boost-path",
				Aliases: []string{"b"},
				Usage:   "Path to boost executable",
				Value:   "boost",
			},
			&cli.IntFlag{
				Name:    "delay",
				Aliases: []string{"d"},
				Usage:   "Delay between each deal status check in seconds",
				Value:   5,
			},
		},
		Action: func(c *cli.Context) error {
			// Load configuration
			cfg, err := config.LoadConfig(c.String("config"))
			if err != nil {
				return fmt.Errorf("failed to load config: %v", err)
			}

			interval := c.Int("interval")
			boostPath := c.String("boost-path")
			delay := c.Int("delay")

			// 启动时立即运行一次
			if err := updateDeals(cfg, boostPath, delay); err != nil {
				log.Printf("Error updating deals: %v", err)
			}

			// 如果interval大于0，则继续循环运行
			if interval > 0 {
				for {
					time.Sleep(time.Duration(interval) * time.Second)
					if err := updateDeals(cfg, boostPath, delay); err != nil {
						log.Printf("Error updating deals: %v", err)
					}
				}
			}

			return nil
		},
	}
}

func updateDeals(cfg *config.Config, boostPath string, delay int) error {
	// Get all deals with "imported" status
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

	deals, err := database.GetDealsByStatus("imported")
	if err != nil {
		return fmt.Errorf("failed to get imported deals: %w", err)
	}

	log.Printf("Found %d imported deals to check", len(deals))

	successCount := 0
	failureCount := 0

	for i, deal := range deals {
		// 添加延迟，防止API请求过快
		time.Sleep(time.Duration(delay) * time.Second)

		// Query deal status using boost CLI
		log.Printf("[%d/%d] Checking deal %s status", i+1, len(deals), deal.UUID)
		cmd := fmt.Sprintf("%s deal-status --provider=%s --deal-uuid=%s --wallet=%s", boostPath, deal.StorageProvider, deal.UUID, deal.ClientWallet)
		output, err := util.ExecCmd("", cmd)
		if err != nil {
			log.Printf("[%d/%d] Error querying deal status for %s: %v", i+1, len(deals), deal.UUID, err)
			failureCount++
			continue
		}

		// Map boost status to our status
		var newStatus string
		if strings.Contains(output, "Proving") {
			newStatus = "success"
		} else if strings.Contains(output, "Error") {
			newStatus = "failed"
		} else {
			// Deal is still in progress
			newStatus = "sealing"
		}

		log.Printf("[%d/%d] Deal %s raw status is %s, update status is %s", i+1, len(deals), deal.UUID, output, newStatus)

		// Update deal status in database
		if err := database.UpdateDealStatus(deal.UUID, newStatus); err != nil {
			log.Printf("[%d/%d] Error updating deal status for %s: %v", i+1, len(deals), deal.UUID, err)
			failureCount++
			continue
		}

		log.Printf("[%d/%d] Updated deal %s status from imported to %s", i+1, len(deals), deal.UUID, newStatus)
		if newStatus == "success" {
			successCount++
		} else if newStatus == "failed" {
			failureCount++
		}
	}

	log.Printf("\nUpdate Summary:")
	log.Printf("Total Deals: %d", len(deals))
	log.Printf("Success: %d", successCount)
	log.Printf("Failed: %d", failureCount)
	log.Printf("In Progress: %d", len(deals)-successCount-failureCount)

	return nil
}
