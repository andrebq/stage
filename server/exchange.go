package server

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/andrebq/stage/internal/protocol"
	"google.golang.org/grpc"
)

type (
	exchange struct {
		sync.RWMutex
		protocol.UnimplementedExchangeServer
		catalog map[string]*protocol.Agent

		endpoints *endpoints

		lst       net.Listener
		shutdown  chan struct{}
		stopped   chan struct{}
		lastError error

		externalAddress string
		deliverTimeout  time.Duration

		actors *actors

		self *protocol.Agent
	}

	actors struct {
		entries map[string]*actor
	}

	actor struct {
		inbox                  chan *protocol.Message
		failedDeliveriesInARow int
	}

	endpoints struct {
		clients map[string]protocol.ExchangeClient
	}

	ActorCallback func(*protocol.Message)

	Exchange interface {
		protocol.ExchangeServer
		AddActor(string, ActorCallback) error
		Serve(net.Listener) error
		Shutdown() error
	}
)

func NewExchange(externalAddr string) Exchange {
	return &exchange{
		catalog:  make(map[string]*protocol.Agent),
		shutdown: make(chan struct{}),
		stopped:  make(chan struct{}),
		endpoints: &endpoints{
			clients: make(map[string]protocol.ExchangeClient),
		},
		actors: &actors{
			entries: make(map[string]*actor),
		},
		deliverTimeout: time.Minute,
		self: &protocol.Agent{
			AgentBindAddr: externalAddr,
		},
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
	if e.self.AgentBindAddr == "" {
		e.self.AgentBindAddr = ln.Addr().String()
	}
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
	var agent *protocol.Agent
	var has bool
	e.readLock(func() {
		agent, has = e.catalog[req.GetMessage().GetActor()]
	})
	if !has {
		return nil, errors.New("agent does not have address for the given agent")
	}
	if e.amIThis(agent) {
		e.deliverLocal(ctx, req)
	}
	return e.deliverRemote(ctx, req, agent)
}

func (e *exchange) Ping(ctx context.Context, req *protocol.PingRequest) (*protocol.PingResponse, error) {
	if req.TargetAgent == "" || req.TargetAgent == e.externalAddress {
		t, err := time.Parse(time.RFC3339Nano, req.SenderMoment)
		if err != nil {
			return nil, err
		}
		now := time.Now()
		return &protocol.PingResponse{
			PingID:          req.PingID,
			Hops:            1,
			SenderMoment:    req.SenderMoment,
			ResponderMoment: now.Format(time.RFC3339Nano),
			DelayMillis:     now.Sub(t).Milliseconds(),
		}, nil
	}
	return nil, errors.New("cannot ping other agents for now...")
}

func (e *exchange) AddActor(actorID string, callback ActorCallback) error {
	var err error
	e.writeLock(func() {
		_, has := e.catalog[actorID]
		if has {
			err = errors.New("actor with the given ID is already registered in this exchange")
			return
		}
		e.catalog[actorID] = e.self
	})
	return err
}

func (e *exchange) readLock(fn func()) {
	e.RLock()
	defer e.RUnlock()
	fn()
}

func (e *exchange) writeLock(fn func()) {
	e.Lock()
	defer e.RUnlock()
	fn()
}

func (e *exchange) deliverRemote(ctx context.Context, req *protocol.DeliverRequest, agent *protocol.Agent) (*protocol.DeliverResponse, error) {
	var cli protocol.ExchangeClient
	var has bool
	e.readLock(func() {
		cli, has = e.endpoints.clients[agent.GetAgentBindAddr()]
	})
	if !has {
		return nil, errors.New("internal inconsistency... agent for actor is known but there is no connection to it")
	}
	ctx, cancel := context.WithTimeout(ctx, e.deliverTimeout)
	defer cancel()
	return cli.Deliver(ctx, req)
}

func (e *exchange) amIThis(agent *protocol.Agent) bool {
	return agent.GetAgentBindAddr() == e.self.GetAgentBindAddr()
}

func (e *exchange) deliverLocal(ctx context.Context, req *protocol.DeliverRequest) (*protocol.DeliverResponse, error) {
	return nil, errors.New("local delivery not implemented")
}
