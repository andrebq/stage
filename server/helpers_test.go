package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"

	"google.golang.org/grpc"
)

func acquireClientConnection(t *testing.T, endpoint string) (grpc.ClientConnInterface, func()) {
	cc, err := grpc.DialContext(context.Background(), endpoint, grpc.WithInsecure())
	noFailure(t, err, "Unable to establish a connection to %v", endpoint)
	return cc, func() {
		err = cc.Close()
		if err != nil {
			t.Logf("Unable to close client connection cleanly, cause %v", err)
		}
	}
}

func startServer(t *testing.T, server Server) (net.Listener, func()) {
	lst, done := acquireListener(t)
	started := make(chan struct{})
	go func() {
		close(started)
		err := server.Serve(lst)
		if errors.Is(err, net.ErrClosed) {
			err = nil
		}
		if err != nil {
			t.Log("start-server", "listener", lst.Addr().String(), err)
		}
	}()
	<-started
	return lst, done
}

func acquireListener(t *testing.T) (net.Listener, func()) {
	lst, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return lst, func() {
		lst.Close()
	}
}

func noFailure(t *testing.T, err error, str string, args ...interface{}) {
	if err == nil {
		return
	}
	t.Fatalf("%v! cause: %v", fmt.Sprintf(str, args...), err)
}
