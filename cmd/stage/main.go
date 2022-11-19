package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/andrebq/stage/upstream"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

func envvarName(str string) string {
	return strings.ToUpper(strings.ReplaceAll(str, "-", "_"))
}

func stringFlag(out *string, name string, usage string) cli.Flag {
	return &cli.StringFlag{
		Name:        name,
		Usage:       usage,
		Value:       *out,
		Destination: out,
		EnvVars:     []string{envvarName(name)},
	}
}
func uintFlag(out *uint, name string, usage string) cli.Flag {
	return &cli.UintFlag{
		Name:        name,
		Usage:       usage,
		Value:       *out,
		Destination: out,
		EnvVars:     []string{envvarName(name)},
	}
}

func appCmd() *cli.App {
	return &cli.App{
		Name: "stage",
		Commands: []*cli.Command{
			upstreamCmd(),
		},
	}
}

func upstreamCmd() *cli.Command {
	var addr string = "127.0.0.1"
	var port uint = 8001
	return &cli.Command{
		Name: "upstream",
		Flags: []cli.Flag{
			stringFlag(&addr, "addr", "Address to bind to"),
			uintFlag(&port, "port", "Port to bind to"),
		},
		Action: func(appCtx *cli.Context) error {
			return upstream.ListenAndServe(appCtx.Context, fmt.Sprintf("%v:%v", addr, port), upstream.NewUpstreamServer())
		},
	}
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	app := appCmd()
	if err := app.RunContext(ctx, os.Args); !errors.Is(err, context.Canceled) {
		log.Fatal().Err(err).Msg("Application failed")
	}
}
