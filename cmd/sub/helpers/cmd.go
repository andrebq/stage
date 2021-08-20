package helpers

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/andrebq/stage/client"
	"github.com/andrebq/stage/internal/protocol"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

func Cmd() *cli.Command {
	return &cli.Command{
		Name:  "helper",
		Usage: "A bunch of helpers to help tools interact with stage",
		Subcommands: []*cli.Command{
			catCmd(),
		},
	}
}

func catCmd() *cli.Command {
	exchange := fmt.Sprintf("localhost:%v", protocol.DefaultExchangePort)
	target := ""
	lines := true
	return &cli.Command{
		Name:  "cat",
		Usage: "Read stdin and send it as payload to a given actor, sender ID is random",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "exchange",
				Value:       exchange,
				Destination: &exchange,
				EnvVars:     []string{"STAGE_HELPER_CAT_EXCHANGE"},
				Usage:       "ID of the exchange to connect",
			},
			&cli.StringFlag{
				Name:        "target",
				Value:       target,
				Destination: &target,
				EnvVars:     []string{"STAGE_HELPER_CAT_TARGET"},
				Usage:       "Actor ID to whom the message(s) will be set",
				Required:    true,
			},
			&cli.BoolFlag{
				Name:        "lines",
				Value:       lines,
				Destination: &lines,
				EnvVars:     []string{"STAGE_HELPER_CAT_LINES"},
				Usage:       "Treat stdin as a series of lines rather than as raw binary, will generate multiple messages",
			},
		},
		Action: func(c *cli.Context) error {
			actorID := uuid.Must(uuid.NewRandom())
			log.Debug().Str("exchange", exchange).Str("sendingAs", actorID.String()).Msg("connecting to exchange")
			cli, err := client.New(c.Context, exchange)
			if err != nil {
				return err
			}

			if !lines {
				payload, err := ioutil.ReadAll(os.Stdin)
				if err != nil {
					return err
				}
				ok, err := client.Deliver(c.Context, cli, target, actorID.String(), payload)
				if err != nil {
					return err
				}
				// TODO: handle failed deliveries
				_ = ok
				return nil
			}

			sc := bufio.NewScanner(os.Stdin)
			for sc.Scan() {
				ok, err := client.Deliver(c.Context, cli, target, actorID.String(), bytes.TrimSpace(sc.Bytes()))
				if err != nil {
					return err
				}
				// TODO: handle failed deliveries
				_ = ok
			}
			return nil
		},
	}
}
