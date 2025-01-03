package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/minerdao/lotus-car/cmd/deal"
	exportfile "github.com/minerdao/lotus-car/cmd/export-file"
	"github.com/minerdao/lotus-car/cmd/generate"
	importdeal "github.com/minerdao/lotus-car/cmd/import-deal"
	"github.com/minerdao/lotus-car/cmd/index"
	initcfg "github.com/minerdao/lotus-car/cmd/init-cfg"
	initdb "github.com/minerdao/lotus-car/cmd/init-db"
	"github.com/minerdao/lotus-car/cmd/regenerate"
	"github.com/minerdao/lotus-car/cmd/server"
	updatedeal "github.com/minerdao/lotus-car/cmd/update-deal"
	"github.com/minerdao/lotus-car/cmd/user"
	"github.com/minerdao/lotus-car/version"
	"github.com/urfave/cli/v2"
)

func main() {
	ctx := context.Background()

	app := &cli.App{
		Name:    "lotus-car",
		Usage:   "A tool for generating car files",
		Version: version.Version,
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
			exportfile.Command(),
			updatedeal.Command(),
			{
				Name:  "version",
				Usage: "Print version information",
				Action: func(c *cli.Context) error {
					fmt.Printf("Version: %s\n", version.Version)
					fmt.Printf("Commit:  %s\n", version.Commit)
					fmt.Printf("Date:    %s\n", version.Date)
					return nil
				},
			},
		},
	}

	if err := app.RunContext(ctx, os.Args); err != nil {
		log.Fatal(err)
	}
}
