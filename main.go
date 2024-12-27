package main

import (
	"context"
	"log"
	"os"

	"github.com/minerdao/lotus-car/cmd/deal"
	"github.com/minerdao/lotus-car/cmd/generate"
	importdeal "github.com/minerdao/lotus-car/cmd/import-deal"
	"github.com/minerdao/lotus-car/cmd/index"
	initcfg "github.com/minerdao/lotus-car/cmd/init-cfg"
	initdb "github.com/minerdao/lotus-car/cmd/init-db"
	"github.com/minerdao/lotus-car/cmd/regenerate"
	"github.com/minerdao/lotus-car/cmd/server"
	"github.com/minerdao/lotus-car/cmd/user"
	"github.com/urfave/cli/v2"
)

func main() {
	ctx := context.Background()

	app := &cli.App{
		Name:  "lotus-car",
		Usage: "A tool for generating car files",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "Path to config file",
				Value:   "config.yaml",
			},
		},
		Commands: []*cli.Command{
			initcfg.Command(),
			initdb.Command(),
			index.Command(),
			generate.Command(),
			regenerate.Command(),
			deal.Command(),
			importdeal.Command(),
			server.Command(),
			user.Command(),
		},
	}

	if err := app.RunContext(ctx, os.Args); err != nil {
		log.Fatal(err)
	}
}
