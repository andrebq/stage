package server

import (
	"context"
	"net"
	"runtime"
	"testing"
	"time"

	"github.com/andrebq/stage/client"
	"github.com/andrebq/stage/internal/protocol"
	"github.com/rs/zerolog"
)

func acquireListener(t *testing.T) (net.Listener, func()) {
	lst, err := net.Listen("tcp", "127.0.01:0")
	if err != nil {
		t.Fatal(err)
	}
	return lst, func() {
		lst.Close()
	}
}

func acquireClientServerPair(t *testing.T) (context.Context, Exchange, protocol.ExchangeClient, func()) {
	ctx := context.Background()
	lst, closeListener := acquireListener(t)
	e := NewExchange("", zerolog.Nop())
	go e.Serve(lst)
	runtime.Gosched()

	cli, err := client.New(lst.Addr().String())
	if err != nil {
		t.Fatalf("Unable to open a client connection to %v, cause %v", lst.Addr(), err)
	}
	return ctx, e, cli, func() {
		e.Shutdown()
		closeListener()
		// add any client cleanup if needed
	}
}

func TestStartExchange(t *testing.T) {
	ctx, _, cli, done := acquireClientServerPair(t)
	defer done()
	_, err := cli.Ping(ctx, &protocol.PingRequest{
		PingID:       "",
		SenderMoment: time.Now().Format(time.RFC3339Nano),
	})
	if err != nil {
		t.Fatalf("Unable to ping agent: %v", err)
	}
}

func TestSimpleExchange(t *testing.T) {
	ctx, exchange, cli, done := acquireClientServerPair(t)
	defer done()

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	bobState := struct {
		msgs int
		done chan struct{}
	}{
		done: make(chan struct{}),
	}
	exchange.AddActor("bob", func(_ *protocol.Message) {
		select {
		case <-bobState.done:
			t.Fatal("Multiple delivery attempts")
			return
		default:
		}
		bobState.msgs++
		if err := exchange.RemoveActor("bob"); err != nil {
			t.Fatal("Should have removed bob without problems")
		}
		close(bobState.done)
	})

	delivered, err := client.Deliver(ctx, cli, "bob", "alice", nil)
	if err != nil {
		t.Fatalf("Unable to ping agent: %v", err)
	}
	if !delivered {
		t.Fatalf("Expecting the message to be delivered to the actor")
	}
	select {
	case <-ctx.Done():
		t.Fatal("Timeout before bob had time to finish its work")
	case <-bobState.done:
		if bobState.msgs != 1 {
			t.Fatalf("Bob should have received %v msgs but got %v", 1, bobState.msgs)
		}
	}
}
