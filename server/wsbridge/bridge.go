package wsbridge

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/andrebq/stage/client"
	"github.com/andrebq/stage/internal/protocol"
	"github.com/andrebq/stage/server"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/valyala/fastjson"
)

type (
	bridge struct {
		exchange server.Exchange
		logger   zerolog.Logger
		client   protocol.ExchangeClient
		sampled  zerolog.Logger
	}

	jsonStageMessage struct {
		Sender  string           `json:"sender"`
		Actor   string           `json:"actor"`
		Payload *json.RawMessage `json:"payload"`
	}
)

var (
	ErrMessageNotJSON = errors.New("message is not a valid JSON string")
)

var upgrader = websocket.Upgrader{
	// TODO: make this a bit safer
	CheckOrigin: func(r *http.Request) bool { return true },
	Subprotocols: []string{
		"stage-binary-message",
		"stage-json-message",
	},
} // use default options

func New(ex server.Exchange, cli protocol.ExchangeClient, logger zerolog.Logger) http.Handler {
	b := &bridge{
		client:   cli,
		exchange: ex,
		logger:   logger.With().Str("package", "github.com/andrebq/stage").Str("module", "internal/server/wsbridge").Logger(),
	}
	b.sampled = b.logger //.Sample(zerolog.Often)
	return b
}

func (b *bridge) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	_ = ctx
	log := b.sampled.With().Str("remoteAddr", req.RemoteAddr).Logger()
	err := req.ParseForm()
	if err != nil {
		log.Error().Err(err).Msg("Unable to parse parameters from client")
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	actorID := req.FormValue("actorid")
	if actorID == "" {
		log.Error().Err(err).Msg("Actor ID not provided")
		http.Error(w, "missing actorid parameter", http.StatusBadRequest)
		return
	}
	log = b.sampled.With().Str("actorID", actorID).Logger()
	c, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Error().Err(err).Msg("Unable to upgrade client to a websocket connection")
		return
	}
	defer c.Close()
	err = b.exchange.AddActor(actorID, func(m *protocol.Message) {
		var pl json.RawMessage
		pl = json.RawMessage(m.Payload)
		if err := fastjson.ValidateBytes([]byte(pl)); err != nil {
			log.Error().Err(err).Str("sender", m.Sender).Msg("Invalid JSON")
			return
		}
		out := jsonStageMessage{
			Sender:  m.Sender,
			Actor:   m.Actor,
			Payload: &pl,
		}
		err = c.WriteJSON(out)
		if err != nil {
			log.Error().Err(err).Msg("Closing after failed attempt to send a JSON object")
			c.Close()
			return
		}
	})

	if err != nil {
		log.Error().Err(err).Msg("Unable to register actor on this exchange")
		return
	}

	defer func() {
		b.exchange.RemoveActor(actorID)
		c.Close()
	}()

	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Error().Err(err).Msg("Unable to read data from client")
			break
		}
		var in jsonStageMessage
		err = json.Unmarshal(message, &in)
		if err != nil {
			log.Error().Err(err).Msg("Unable to parse msg form client")
			continue
		}
		client.Deliver(ctx, b.client, in.Actor, actorID, *in.Payload)
	}
}
