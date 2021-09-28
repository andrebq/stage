package server

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/andrebq/stage/internal/protocol"
)

func TestStartExchange(t *testing.T) {
	e := &Exchange{}
	c := &Catalog{}
	catalogListener, closeCatalog := startServer(t, c.Export())
	defer closeCatalog()
	if err := e.Catalog(context.Background(), catalogListener.Addr().String()); err != nil {
		t.Fatal(err)
	}
	exchangeListener, closeExchange := startServer(t, e.Export())
	defer closeExchange()
	cc, closeClient := acquireClientConnection(t, exchangeListener.Addr().String())
	defer closeClient()
	exchangeClient := protocol.NewExchangeClient(cc)
	if _, err := exchangeClient.Ping(context.Background(), &protocol.Echo{SentTime: time.Now().UnixNano()}); err != nil {
		t.Fatal(err)
	}
}

func TestSimpleExchange(t *testing.T) {
	e := &Exchange{}
	c := &Catalog{}
	catalogListener, closeCatalog := startServer(t, c.Export())
	defer closeCatalog()
	if err := e.Catalog(context.Background(), catalogListener.Addr().String()); err != nil {
		t.Fatal(err)
	}
	exchangeListener, closeExchange := startServer(t, e.Export())
	defer closeExchange()
	cc, closeClient := acquireClientConnection(t, exchangeListener.Addr().String())
	defer closeClient()
	exchangeClient := protocol.NewExchangeClient(cc)
	_, err := exchangeClient.Register(context.Background(), &protocol.RegisterRequest{
		Actors: []string{"a1", "a2"},
	})
	noFailure(t, err, "Unable to register actors a1, a2 in exchange")
	msgs := []*protocol.Message{
		{
			Payload: []byte("hello world"),
			Identity: &protocol.MessageIdentity{
				Sender:      "a1",
				Destination: "a2",
				Id:          "m1",
			},
		},
		{
			Payload: []byte("hello world"),
			Identity: &protocol.MessageIdentity{
				Sender:      "a1",
				Destination: "a2",
				Id:          "m2",
			},
		},
	}
	_, err = exchangeClient.Send(context.Background(), &protocol.SendRequest{Messages: msgs})
	noFailure(t, err, "Unable to send message from a1 to a2")
	output, err := exchangeClient.Recv(context.Background(), &protocol.RecvRequest{Actors: []string{"a2"}})
	noFailure(t, err, "Unable to receive messages to a2")
	if len(output.GetItems()) != 2 {
		t.Fatalf("a2 should have two messages on its inbox but got %v", len(output.GetItems()))
	}
	if !reflect.DeepEqual(output.GetItems()[0].GetPayload(), msgs[0].GetPayload()) {
		t.Fatalf("Received messages are different from expected")
	}
	output, err = exchangeClient.Recv(context.Background(), &protocol.RecvRequest{Actors: []string{"a2"}})
	noFailure(t, err, "Unable to receive messages to a2 after the inbox was consumed")
	if len(output.GetItems()) != 0 {
		t.Fatalf("Inbox of a2 should be empty but got %v", len(output.GetItems()))
	}
}
