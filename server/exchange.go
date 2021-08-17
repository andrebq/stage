package server

import (
	"context"
	"errors"
	"net"
	"sync"

	"github.com/andrebq/stage/internal/protocol"
	"google.golang.org/grpc"
)

type (
	exchange struct {
		sync.RWMutex
		protocol.UnimplementedExchangeServer
		catalog map[string]*protocol.Agent

		lst       net.Listener
		shutdown  chan struct{}
		stopped   chan struct{}
		lastError error
	}

	Exchange interface {
		protocol.ExchangeServer
		Serve(net.Listener) error
		Shutdown() error
	}
)

func NewExchange() Exchange {
	return &exchange{
		catalog:  make(map[string]*protocol.Agent),
		shutdown: make(chan struct{}),
		stopped:  make(chan struct{}),
	}
}

func (e *exchange) Serve(ln net.Listener) error {
	select {
	case <-e.shutdown:
		return errors.New("server is already in shutdown mode")
	default:
	}
	e.lst = ln
	s := grpc.NewServer()
	protocol.RegisterExchangeServer(s, e)
	e.lastError = s.Serve(ln)
	close(e.stopped)
	return e.lastError
}

func (e *exchange) Shutdown() error {
	select {
	case <-e.shutdown:
		return errors.New("server is already in shutdown mode")
	case <-e.stopped:
		return errors.New("server is already stopped")
	default:
	}
	close(e.shutdown)
	if e.lst != nil {
		e.lst.Close()
	}
	<-e.stopped
	return e.lastError
}

func (e *exchange) Register(ctx context.Context, req *protocol.RegisterRequest) (*protocol.RegisterResponse, error) {
	e.Lock()
	defer e.Unlock()
	e.catalog[req.Agent.Actor] = req.Agent
	return &protocol.RegisterResponse{Active: true}, nil
}

func (e *exchange) Deliver(ctx context.Context, req *protocol.DeliverRequest) (*protocol.DeliverResponse, error) {
	return nil, errors.New("not implemented")
}
