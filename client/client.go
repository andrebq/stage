package client

import (
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
