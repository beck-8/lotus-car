package user

import (
	"fmt"

	"github.com/minerdao/lotus-car/config"
	"github.com/minerdao/lotus-car/db"
	"github.com/urfave/cli/v2"
)

func Command() *cli.Command {
	return &cli.Command{
		Name:  "user",
		Usage: "Manage users",
		Subcommands: []*cli.Command{
			{
				Name:  "add",
				Usage: "Add a new user",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "username",
						Usage:    "Username",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "password",
						Usage:    "Password",
						Required: true,
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

					username := c.String("username")
					password := c.String("password")

					// 创建用户
					err = database.CreateUser(username, password)
					if err != nil {
						return fmt.Errorf("failed to create user: %v", err)
					}

					fmt.Printf("User %s created successfully\n", username)
					return nil
				},
			},
		},
	}
}
