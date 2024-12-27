package server

import (
	"fmt"
	"log"
	"net/http"

	"github.com/minerdao/lotus-car/api"
	"github.com/minerdao/lotus-car/config"
	"github.com/minerdao/lotus-car/db"
	"github.com/minerdao/lotus-car/middleware"
	"github.com/urfave/cli/v2"
)

func Command() *cli.Command {
	return &cli.Command{
		Name:  "serve",
		Usage: "Start API server",
		Action: func(c *cli.Context) error {
			// Load configuration
			cfg, err := config.LoadConfig(c.String("config"))
			if err != nil {
				return fmt.Errorf("failed to load config: %v", err)
			}

			dbConfig := &db.DBConfig{
				Host:     cfg.Database.Host,
				Port:     cfg.Database.Port,
				User:     cfg.Database.User,
				Password: cfg.Database.Password,
				DBName:   cfg.Database.DBName,
				SSLMode:  cfg.Database.SSLMode,
			}

			authConfig := middleware.AuthConfig{
				JWTSecret:        cfg.Auth.JWTSecret,
				TokenExpireHours: cfg.Auth.TokenExpireHours,
			}

			apiServer, err := api.NewAPIServer(dbConfig, authConfig)
			if err != nil {
				return fmt.Errorf("failed to create API server: %v", err)
			}

			mux := http.NewServeMux()

			// 公开的路由（不需要认证）
			mux.HandleFunc("/api/login", apiServer.Login)

			// 需要认证的路由
			authMiddleware := middleware.AuthMiddleware(authConfig)
			mux.HandleFunc("/api/files", authMiddleware(apiServer.ListFiles))
			mux.HandleFunc("/api/file", authMiddleware(apiServer.GetFile))       // GET with ?id=X
			mux.HandleFunc("/api/delete", authMiddleware(apiServer.DeleteFile))  // DELETE with ?id=X
			mux.HandleFunc("/api/search", authMiddleware(apiServer.SearchFiles)) // GET with query params

			log.Printf("Starting API server on %s", cfg.Server.Address)
			return http.ListenAndServe(cfg.Server.Address, mux)
		},
	}
}
