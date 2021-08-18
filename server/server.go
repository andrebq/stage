package server

import (
	"context"
	"fmt"
	"net"
)

func ListenAndServe(ctx context.Context, bind string, s Exchange) error {
	lst, err := net.Listen("tcp", bind)
	if err != nil {
		return fmt.Errorf("unable to setup tcp listener on %v, cause %w", bind, err)
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		<-ctx.Done()
		s.Shutdown()
	}()
	err = s.Serve(lst)
	cancel()
	return err
}
