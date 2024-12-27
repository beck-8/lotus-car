package initcfg

import (
	"fmt"

	"github.com/minerdao/lotus-car/config"
	"github.com/urfave/cli/v2"
)

func Command() *cli.Command {
	return &cli.Command{
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
	}
}
