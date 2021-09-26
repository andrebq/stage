package server

import (
	"context"
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
	t.Fail()
}
