package main

import (
	"context"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

func main() {
	ctx := context.Background()
	app := &cli.App{
		Name:  "stage",
		Usage: "Actor stages",
	}

	err := app.RunContext(ctx, os.Args)
	if err != nil {
		log.Fatal().Err(err).Msg("App failed")
	}
}
