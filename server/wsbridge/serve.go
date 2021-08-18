package wsbridge

import (
	"errors"
	"net/http"

	"github.com/andrebq/stage/internal/protocol"
	"github.com/andrebq/stage/server"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

type (
	bridge struct {
		exchange server.Exchange
		logger   zerolog.Logger
		sampled  zerolog.Logger
	}
)

var (
	ErrMessageNotJSON = errors.New("message is not a valid JSON string")
)

var upgrader = websocket.Upgrader{} // use default options

func New(ex server.Exchange, logger zerolog.Logger) http.Handler {
	b := &bridge{
		exchange: ex,
		logger:   logger.With().Str("package", "github.com/andrebq/stage").Str("module", "internal/server/wsbridge").Logger(),
	}
	b.sampled = b.logger.Sample(zerolog.Often)
	return b
}

func (b *bridge) ServeHTTP(w http.ResponseWriter, req *http.Request) {
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
	err = b.exchange.AddActor(actorID, func(m *protocol.Message) {
		panic("TODO: implement this")
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
		mt, message, err := c.ReadMessage()
		if err != nil {
			log.Error().Err(err).Msg("Unable to read data from client")
			break
		}
		_ = mt
		_ = message
		panic("TODO: implement this")
	}
}
