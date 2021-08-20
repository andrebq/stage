package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/andrebq/stage/cmd/sub/exchange"
	"github.com/andrebq/stage/cmd/sub/helpers"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

func main() {
	ctx, done := signal.NotifyContext(context.Background(), os.Interrupt)
	defer done()

	var debug bool
	app := &cli.App{
		Name:  "stage",
		Usage: "Actor stages",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "debug",
				Usage:       "Run the exchange in debug mode rather than info mode",
				EnvVars:     []string{"STAGE_DEBUG"},
				Destination: &debug,
			},
		},
		Commands: []*cli.Command{
			exchange.Cmd(),
			helpers.Cmd(),
		},
		Before: func(c *cli.Context) error {
			if debug {
				log.Logger = log.Logger.Level(zerolog.DebugLevel)
			} else {
				log.Logger = log.Logger.Level(zerolog.InfoLevel)
			}
			return nil
		},
	}

	err := app.RunContext(ctx, os.Args)
	if err != nil {
		log.Fatal().Err(err).Msg("App failed")
	}
}
