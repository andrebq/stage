package client

import (
	"context"
	"fmt"

	"github.com/andrebq/stage/internal/protocol"
	"google.golang.org/grpc"
)

func New(addr string) (protocol.ExchangeClient, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return nil, fmt.Errorf("unable to setup grcp connection with %v, cause %w", addr, err)
	}
	return protocol.NewExchangeClient(conn), nil
}

// Deliver the payload to the given destination using the provided ExchangeClient
func Deliver(ctx context.Context, exchange protocol.ExchangeClient, destinationID string, senderID string, payload []byte) (bool, error) {
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
