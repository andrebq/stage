package exchange

import (
	"github.com/andrebq/stage/server"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

func Cmd() *cli.Command {
	bindAddr := "localhost:31400"
	external := ""
	return &cli.Command{
		Name:  "exchange",
		Usage: "Starts a new exchange at a given address",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "bind",
				Aliases:     []string{"b"},
				Usage:       "Address to bind for incoming connections",
				EnvVars:     []string{"STAGE_EXCHANGE_BIND"},
				Destination: &bindAddr,
				Value:       bindAddr,
			},
			&cli.StringFlag{
				Name:        "external",
				Aliases:     []string{"e"},
				Usage:       "External address to announce for other exchanges, defaults to bindAddr if not configured",
				EnvVars:     []string{"STAGE_EXCHANGE_EXTERNAL_ADDRESS"},
				Destination: &external,
				Value:       external,
			},
		},
		Action: func(c *cli.Context) error {
			log.Logger = log.Logger.With().Str("fullCmd", c.Command.FullName()).Logger()
			if external == "" {
				external = bindAddr
			}
			log.Info().Str("external", external).Str("bind", bindAddr).Msg("Starting exchange.")
			return server.ListenAndServe(c.Context, bindAddr, server.NewExchange(external, log.Logger))
		},
	}
}
