package client

import (
	"context"
	"fmt"

	"github.com/andrebq/stage/internal/protocol"
	"google.golang.org/grpc"
)

type (
	Message struct {
		Payload       []byte
		SenderID      string
		DestinationID string
	}
)

func New(ctx context.Context, addr string) (protocol.ExchangeClient, error) {
	conn, err := grpc.DialContext(ctx, addr, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return nil, fmt.Errorf("unable to setup grcp connection with %v, cause %w", addr, err)
	}
	return protocol.NewExchangeClient(conn), nil
}

// Stream messages to actorID that were sent via the given exchange,
//
// This is a simplification over calling exchange.Receive directly,
// use that instead if you need more control of the process.
func Stream(ctx context.Context, exchange protocol.ExchangeClient, actorID string) (<-chan Message, error) {
	stream, err := exchange.Receive(ctx, &protocol.ReceiveRequest{
		ActorID: actorID,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to receive data from exchange, cause %v", err)
	}
	msgs := make(chan Message, 1)
	go func() {
		for {
			msg, err := stream.Recv()
			if err != nil {
				close(msgs)
				return
			}
			msgs <- Message{
				SenderID:      msg.GetNextMessage().GetSender(),
				DestinationID: msg.GetNextMessage().GetActor(),
				Payload:       msg.GetNextMessage().GetPayload(),
			}
		}
	}()
	return msgs, nil
}

// Deliver the payload to the given destination using the provided ExchangeClient
func Deliver(ctx context.Context, exchange protocol.ExchangeClient, destinationID string, senderID string, payload []byte) (bool, error) {
	if destinationID == "" {
		return false, ErrMissingDestination
	}
	if senderID == "" {
		return false, ErrMissingSenderID
	}
	res, err := exchange.Deliver(ctx, &protocol.DeliverRequest{
		Message: &protocol.Message{
			Sender:  senderID,
			Actor:   destinationID,
			Payload: payload,
		},
	})
	if err != nil {
		return false, err
	}
	return res.Delivered, nil
}
