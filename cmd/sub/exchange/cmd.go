package exchange

import (
	"context"
	"fmt"

	"github.com/andrebq/stage/client"
	"github.com/andrebq/stage/internal/protocol"
	"github.com/andrebq/stage/server"
	"github.com/andrebq/stage/server/wsbridge"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

func Cmd() *cli.Command {
	bindAddr := fmt.Sprintf("localhost:%v", protocol.DefaultExchangePort)
	bridgeBind := fmt.Sprintf("localhost:%v", protocol.DefaultBridgePort)
	enableBridge := false
	external := ""
	return &cli.Command{
		Name:  "exchange",
		Usage: "Starts a new exchange at a given address, users can optionally enable a websocket bridge",
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
			&cli.StringFlag{
				Name:        "bridge-bind",
				Usage:       "Address to listen for websocket connections to be bridged",
				EnvVars:     []string{"STAGE_EXCHANGE_BRIDGE_BIND"},
				Destination: &bridgeBind,
				Value:       bridgeBind,
			},
			&cli.BoolFlag{
				Name:        "enable-bridge",
				Aliases:     []string{"bridge"},
				Usage:       "Enable the Websocket bridge",
				EnvVars:     []string{"STAGE_EXCHANGE_ENABLE_BRIDGE"},
				Destination: &enableBridge,
				Value:       enableBridge,
			},
		},
		Action: func(c *cli.Context) error {
			log.Logger = log.Logger.With().Str("fullCmd", c.Command.FullName()).Logger()
			if external == "" {
				external = bindAddr
			}
			ctx, cancel := context.WithCancel(c.Context)
			exchange := server.NewExchange(external, log.Logger)

			var exchangeDone <-chan error
			var bridgeDone <-chan error

			exchangeDone = async(func() error {
				log.Info().Str("external", external).Str("bind", bindAddr).Msg("Starting exchange.")
				return server.ListenAndServe(ctx, bindAddr, exchange)
			})
			if enableBridge {
				cli, err := client.New(ctx, bindAddr)
				if err != nil {
					cancel()
					return err
				}
				bridgeDone = async(func() error {
					log.Info().Str("external", external).Str("bind", bindAddr).Str("bridgeBind", bridgeBind).Msg("Starting bridge.")
					return wsbridge.ListenAndServe(ctx, wsbridge.New(exchange, cli, log.Logger), bridgeBind)
				})
			}

			exchangeError := <-exchangeDone
			cancel()
			if exchangeError != nil {
				log.Error().Err(exchangeError).Msg("Exchange error")
			}
			if bridgeDone != nil {
				bridgeError := <-bridgeDone
				if bridgeError != nil {
					log.Error().Err(bridgeError).Msg("Bridge error")
				}
			}
			return exchangeError
		},
	}
}

type (
	asyncCtx struct {
		parent context.Context
		done   chan struct{}
		err    error
	}
)

func async(fn func() error) <-chan error {
	err := make(chan error, 1)
	go func() {
		innerErr := fn()
		err <- innerErr
	}()
	return err
}
