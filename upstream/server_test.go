package upstream

import (
	"context"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/andrebq/stage"
	"github.com/andrebq/stage/protocol"
)

func TestRemoteStage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(),
		time.Second)
	defer cancel()
	addr, done := openServer(t, NewUpstreamServer())
	defer done()

	var cli1, cli2 stage.Upstream
	var err error
	if cli1, err = Dial(ctx, "st1", addr); err != nil {
		t.Fatal(err)
	}
	defer cli1.Close()
	if cli2, err = Dial(ctx, "st2", addr); err != nil {
		t.Fatal(err)
	}
	defer cli2.Close()

	if err := cli1.RegisterPIDs(ctx, stage.Identity{PID: "st1.a.1"}); err != nil {
		t.Fatal(err)
	}
	msg := stage.Message{
		To:      stage.Identity{PID: "st1.a.1"},
		From:    stage.Identity{PID: "st2.a.1"},
		Method:  "method",
		Content: nil,
	}
	if err := cli2.Proxy(ctx, msg); err != nil {
		t.Fatal(err)
	}
	if actual, err := cli1.Fetch(ctx); err != nil {
		t.Fatal(err)
	} else if !reflect.DeepEqual(actual, []stage.Message{msg}) {
		t.Fatalf("Should be %#v got %#v", []stage.Message{msg}, actual)
	}
	done()
}

func openServer(t *testing.T, us protocol.UpstreamServer) (string, func()) {
	lst, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(us)
	go func() {
		s.Serve(lst)
	}()
	return lst.Addr().String(), func() {
		s.GracefulStop()
		lst.Close()
	}
}
