package clearcar

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v2"

	"github.com/minerdao/lotus-car/config"
	"github.com/minerdao/lotus-car/db"
)

func Command() *cli.Command {
	return &cli.Command{
		Name:  "clear-car",
		Usage: "Clear .car files for completed deals",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "Path to config file",
				Value:   "config.yaml",
			},
			&cli.StringSliceFlag{
				Name:     "car-dirs",
				Aliases:  []string{"d"},
				Usage:    "Directories containing .car files to clean",
				Required: true,
			},
			&cli.BoolFlag{
				Name:  "really-do-it",
				Usage: "Actually delete the files. If not set, will only show what would be deleted",
				Value: false,
			},
		},
		Action: func(c *cli.Context) error {
			// Load configuration
			cfg, err := config.LoadConfig(c.String("config"))
			if err != nil {
				return fmt.Errorf("failed to load config: %v", err)
			}

			database, err := db.InitFromConfig(cfg)
			if err != nil {
				return fmt.Errorf("failed to connect to database: %v", err)
			}
			defer database.Close()

			// 获取所有成功的订单
			filteredDeals, err := database.GetDealsForClear()
			if err != nil {
				return fmt.Errorf("failed to get success deals: %v", err)
			}

			if len(filteredDeals) == 0 {
				log.Printf("No success deals found")
				return nil
			}

			log.Printf("Found %d success deals", len(filteredDeals))

			// 获取所有目录
			carDirs := c.StringSlice("car-dirs")
			log.Printf("Searching in directories: %v", carDirs)

			// 检查目录是否可访问
			for _, dir := range carDirs {
				if _, err := os.Stat(dir); err != nil {
					log.Printf("Warning: Cannot access directory %s: %v", dir, err)
					continue
				}
				log.Printf("Directory %s is accessible", dir)
			}

			// 统计信息
			totalFound := 0
			totalDeleted := 0
			totalErrors := 0

			// 遍历每个成功的订单
			for i, deal := range filteredDeals {
				log.Printf("[%d/%d] Processing deal %s (CommP: %s)", i+1, len(filteredDeals), deal.UUID, deal.CommP)

				// 在每个目录中查找对应的car文件
				for _, dir := range carDirs {
					carPath := filepath.Join(dir, fmt.Sprintf("%s.car", deal.CommP))
					log.Printf("[%d/%d] Looking for file: %s", i+1, len(filteredDeals), carPath)

					// 检查文件是否存在
					if fileInfo, err := os.Stat(carPath); err != nil {
						if os.IsNotExist(err) {
							log.Printf("[%d/%d] File not found: %s", i+1, len(filteredDeals), carPath)
							continue
						}
						log.Printf("[%d/%d] Error checking file %s: %v", i+1, len(filteredDeals), carPath, err)
						totalErrors++
						continue
					} else {
						log.Printf("[%d/%d] Found file %s (size: %d, mode: %v)", i+1, len(filteredDeals), carPath, fileInfo.Size(), fileInfo.Mode())
					}

					totalFound++
					log.Printf("[%d/%d] Found car file: %s", i+1, len(filteredDeals), carPath)

					// 删除文件
					if c.Bool("really-do-it") {
						if err := os.Remove(carPath); err != nil {
							log.Printf("[%d/%d] Failed to delete file %s: %v", i+1, len(filteredDeals), carPath, err)
							totalErrors++
							continue
						}
						totalDeleted++
						log.Printf("[%d/%d] Successfully deleted car file: %s", i+1, len(filteredDeals), carPath)
					} else {
						totalDeleted++
						log.Printf("[%d/%d] Would delete car file: %s (dry run)", i+1, len(filteredDeals), carPath)
					}
				}
			}

			// 打印总结信息
			log.Printf("\nClear Summary:")
			log.Printf("Total Success Deals: %d", len(filteredDeals))
			log.Printf("Total Car Files Found: %d", totalFound)
			log.Printf("Successfully Deleted: %d", totalDeleted)
			log.Printf("Errors: %d", totalErrors)

			return nil
		},
	}
}
