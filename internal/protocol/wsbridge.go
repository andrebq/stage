package protocol

import "encoding/json"

type (
	WebsocketMessage struct {
		Sender  string          `json:"sender"`
		Actor   string          `json:"actor"`
		Payload json.RawMessage `json:"payload"`
	}
)
