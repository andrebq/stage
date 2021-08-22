package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

type (
	// WSBridge provides a client connection to a websocket bridge for stage
	WSBridge struct {
		remote *url.URL
		c      *websocket.Conn

		logger zerolog.Logger

		toBridge   chan json.RawMessage
		fromBridge chan json.RawMessage
	}
)

// NewBridge connects to the given bridge, if authentication is required
// it must have been executed prior to callnew new bridge.
func NewBridge(ctx context.Context, endpoint string, logger zerolog.Logger) (*WSBridge, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, errors.New("endpoint is not a valid url")
	}

	ws := &WSBridge{
		remote: u,

		fromBridge: make(chan json.RawMessage, 1000),
		toBridge:   make(chan json.RawMessage),
		logger:     logger,
	}
	err = ws.dial(ctx)
	if err != nil {
		return nil, err
	}
	return ws, nil
}

// Incoming returns a channel with all messages received from the bridge
func (w *WSBridge) Input() <-chan json.RawMessage {
	return w.fromBridge
}

func (w *WSBridge) Output() chan<- json.RawMessage {
	return w.toBridge
}

func (w *WSBridge) Run(ctx context.Context) error {
	log := w.logger.With().Bool("running", true).Logger()
	doneReading := make(chan error, 1)
	doneWriting := make(chan error, 1)
	go func() {
		defer close(doneReading)
		defer w.c.Close()
		for {
			_, buf, err := w.c.ReadMessage()
			if err != nil {
				doneReading <- err
				return
			}
			select {
			case <-ctx.Done():
				return
			case w.fromBridge <- json.RawMessage(buf):
				continue
			}
		}
	}()
	go func() {
		defer close(doneWriting)
		defer w.c.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case m := <-w.toBridge:
				err := w.c.WriteMessage(websocket.TextMessage, m)
				if err != nil {
					doneWriting <- err
					return
				}
			}
		}
	}()
	for doneWriting != nil && doneReading != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-doneReading:
			log.Error().Err(err).Msg("Error reading data from bridge")
			doneReading = nil
		case err := <-doneWriting:
			log.Error().Err(err).Msg("Error writing data to bridge")
			doneWriting = nil
		}
	}
	return nil
}

func (w *WSBridge) dial(ctx context.Context) error {
	if w.c != nil {
		w.c.Close()
	}
	c, _, err := websocket.DefaultDialer.Dial(w.remote.String(), nil)
	if err != nil {
		return fmt.Errorf("unable to dial to remote bridge, cause %w", err)
	}
	w.c = c
	return nil
}
