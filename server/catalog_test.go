package server

import (
	"context"
	"testing"

	"github.com/andrebq/stage/internal/protocol"
)

func TestCatalog(t *testing.T) {
	c := &Catalog{}
	listener, done := startServer(t, c.Export())
	defer done()

	cc, clientDone := acquireClientConnection(t, listener.Addr().String())
	defer clientDone()
	cli := protocol.NewCatalogClient(cc)
	_, err := cli.Register(context.Background(), &protocol.RegisterRequest{
		Actors:   []string{"actor1", "actor2"},
		Exchange: "grpc://localhost:1234",
	})
	noFailure(t, err, "Unable to register actors in catalog")

	list, err := cli.Address(context.Background(), &protocol.AddressList{
		Addresses: []*protocol.Address{
			{Actor: "actor1"},
			{Actor: "actor2"},
		},
	})
	noFailure(t, err, "Unable to get list of endpoints for actors")
	if len(list.GetAddresses()) != 2 {
		t.Fatalf("Should have found 2 actors but got %v", len(list.GetAddresses()))
	}
	for _, v := range list.GetAddresses() {
		if v.GetEndpoint() != "grpc://localhost:1234" {
			t.Fatalf("Actor %v got an invalid endpoint %v", v.GetActor(), v.GetEndpoint())
		}
	}
}
