package wsbridge

import (
	"context"
	"net"
	"net/http"
	"time"
)

func ListenAndServe(ctx context.Context, handler http.Handler, bindAddr string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	lst, err := createListener(bindAddr)
	server := createServer()
	server.Handler = handler
	go func() {
		<-ctx.Done()
		lst.Close()
	}()
	if err != nil {
		return err
	}
	return server.Serve(lst)
}

func createServer() *http.Server {
	s := &http.Server{
		ReadTimeout:       time.Minute / 2,
		WriteTimeout:      time.Minute / 2,
		ReadHeaderTimeout: time.Minute / 2,
		IdleTimeout:       time.Minute * 30,
		MaxHeaderBytes:    100_000,
	}
	return s
}

func createListener(bind string) (net.Listener, error) {
	return net.Listen("tcp", bind)
}
