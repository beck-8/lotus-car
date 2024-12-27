package initdb

import (
	"database/sql"
	"fmt"

	"github.com/minerdao/lotus-car/config"
	"github.com/minerdao/lotus-car/db"
	"github.com/urfave/cli/v2"
)

func Command() *cli.Command {
	return &cli.Command{
		Name:  "init-db",
		Usage: "Initialize database",
		Action: func(c *cli.Context) error {
			// Load configuration
			cfg, err := config.LoadConfig(c.String("config"))
			if err != nil {
				return fmt.Errorf("failed to load config: %v", err)
			}

			// 连接到 postgres 数据库来创建新数据库
			connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=%s",
				cfg.Database.Host,
				cfg.Database.Port,
				cfg.Database.User,
				cfg.Database.Password,
				cfg.Database.SSLMode,
			)

			sqlDB, err := sql.Open("postgres", connStr)
			if err != nil {
				return fmt.Errorf("failed to connect to postgres: %v", err)
			}
			defer sqlDB.Close()

			// 检查数据库是否存在
			var exists bool
			err = sqlDB.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", cfg.Database.DBName).Scan(&exists)
			if err != nil {
				return fmt.Errorf("failed to check if database exists: %v", err)
			}

			// 如果数据库不存在，创建它
			if !exists {
				_, err = sqlDB.Exec(fmt.Sprintf("CREATE DATABASE %s", cfg.Database.DBName))
				if err != nil {
					return fmt.Errorf("failed to create database: %v", err)
				}
				fmt.Printf("Created database %s\n", cfg.Database.DBName)
			} else {
				fmt.Printf("Database %s already exists\n", cfg.Database.DBName)
			}

			// 初始化数据库表结构
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
				return fmt.Errorf("failed to initialize database tables: %v", err)
			}
			defer database.Close()

			fmt.Println("Database initialization completed successfully")
			return nil
		},
	}
}
