package server

import (
	"context"
	"net"
	"runtime"
	"testing"

	"github.com/andrebq/stage/client"
	"github.com/andrebq/stage/internal/protocol"
)

func acquireListener(t *testing.T) (net.Listener, func()) {
	lst, err := net.Listen("tcp", "")
	if err != nil {
		t.Fatal(err)
	}
	return lst, func() {
		lst.Close()
	}
}

func TestStartExchange(t *testing.T) {
	ctx := context.Background()
	lst, done := acquireListener(t)
	defer done()
	e := NewExchange()
	go e.Serve(lst)
	runtime.Gosched()

	cli, err := client.New(lst.Addr().String())
	if err != nil {
		t.Fatalf("Unable to open a client connection to %v, cause %v", lst.Addr(), err)
	}
	_, err = cli.Register(ctx, &protocol.RegisterRequest{
		Agent: &protocol.Agent{
			Actor:         "bob",
			AgentBindAddr: lst.Addr().String(),
		},
	})
	if err != nil {
		t.Fatalf("Unable to register actor: %v", err)
	}
	e.Shutdown()
}
