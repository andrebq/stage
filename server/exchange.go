package server

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/andrebq/stage/internal/protocol"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
)

type (
	exchange struct {
		sync.RWMutex
		log zerolog.Logger
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

	endpoints struct {
		clients map[string]protocol.ExchangeClient
	}

	ActorCallback func(*protocol.Message)

	Exchange interface {
		protocol.ExchangeServer
		AddActor(string, ActorCallback) error
		RemoveActor(string) error
		Serve(net.Listener) error
		Shutdown() error
	}
)

func NewExchange(externalAddr string, log zerolog.Logger) Exchange {
	return &exchange{
		log:      log.With().Str("package", "github.com/andrebq/stage").Str("component", "internal/server/exchange").Logger(),
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
	e.writeLock(func() {
		for _, a := range e.actors.entries {
			a.stop()
		}
		// nobody should touch the actor table anymore
		e.actors.entries = nil
	})
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
		return e.deliverLocal(ctx, req)
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

func (e *exchange) RemoveActor(actorID string) error {
	var err error
	e.writeLock(func() {
		if err = e.stillRunning(); err != nil {
			return
		}
		actor, has := e.actors.entries[actorID]
		if !has {
			err = ErrActorNotFound
		}
		actor.stop()
		delete(e.actors.entries, actorID)
	})
	return err
}

func (e *exchange) AddActor(actorID string, callback ActorCallback) error {
	var err error
	e.writeLock(func() {
		if err = e.stillRunning(); err != nil {
			return
		}
		_, has := e.catalog[actorID]
		if has {
			err = errors.New("actor with the given ID is already registered in this exchange")
			return
		}
		e.catalog[actorID] = e.self
		actor := newActor(actorID, callback, e.log)
		e.actors.entries[actorID] = actor
		go actor.act()
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
	defer e.Unlock()
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
	var actor *actor
	var has bool
	var err error
	e.readLock(func() {
		if err = e.stillRunning(); err != nil {
			return
		}
		actor, has = e.actors.entries[req.Message.Actor]
	})
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrActorNotFound
	}
	return &protocol.DeliverResponse{
		Delivered: actor.deliver(req.Message),
	}, nil
}

func (e *exchange) stillRunning() error {
	select {
	case <-e.shutdown:
		return ErrServerShutdown
	case <-e.stopped:
		return ErrServerStopped
	default:
		return nil
	}
}
