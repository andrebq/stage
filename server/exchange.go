package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/andrebq/stage/internal/protocol"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

	ActorCallback func(*protocol.Message, bool)

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
	e.log.Debug().Str("actor", req.Agent.GetActor()).Str("agent", req.Agent.GetAgentBindAddr()).Msg("got new actor registration")
	var err error
	var response *protocol.RegisterResponse
	e.writeLock(func() {
		if err = e.stillRunning(); err != nil {
			return
		}
		e.catalog[req.Agent.Actor] = req.Agent
		response = &protocol.RegisterResponse{Active: true}
	})
	return response, err
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

func (e *exchange) Receive(req *protocol.ReceiveRequest, stream protocol.Exchange_ReceiveServer) error {
	actor := req.GetActorID()
	policy := req.GetDropPolicy()
	if actor == "" {
		return status.Errorf(codes.InvalidArgument, "Invalid actorID")
	}
	buf := make([]*protocol.Message, 100)
	e.RemoveActor(actor)
	dropCount := int32(0)
	newMessage := make(chan struct{}, 1)
	handle := func(m *protocol.Message, removed bool) {
		if len(buf) == cap(buf) {
			dropCount++
			var full bool
			buf, full = applyDropPolicy(buf, policy)
			if full {
				return
			}
		}
		buf = append(buf, m)
		select {
		case newMessage <- struct{}{}:
		default:
		}
	}
	err := e.AddActor(actor, handle)
	if err != nil {
		return status.Errorf(codes.Internal, "Unable to register actor %v, cause %v", actor, err)
	}
	defer e.RemoveActor(actor)
	for {
		select {
		case <-stream.Context().Done():
			return status.Errorf(codes.Canceled, stream.Context().Err().Error())
		case _, open := <-newMessage:
			if !open {
				return status.Errorf(codes.Aborted, "Actor %v was removed by external actions", actor)
			}
			var msg *protocol.Message
			var size int
			msg, size, buf = popHead(buf)
			stream.Send(&protocol.ReceiveResponse{
				NextMessage:   msg,
				DropCount:     dropCount,
				CurrentBuffer: int32(size),
			})
		}
	}
}

func popHead(buf []*protocol.Message) (*protocol.Message, int, []*protocol.Message) {
	if len(buf) == 0 {
		return nil, 0, buf
	}
	m := buf[0]
	copy(buf[:], buf[1:])
	buf[len(buf)-1] = nil
	buf = buf[:len(buf)-2]
	return m, len(buf), buf
}

func applyDropPolicy(buf []*protocol.Message, policy protocol.MessageDropPolicy) ([]*protocol.Message, bool) {
	if policy == protocol.MessageDropPolicy_Unspecified {
		policy = protocol.MessageDropPolicy_Oldest
	}
	switch policy {
	case protocol.MessageDropPolicy_Newest:
		buf[len(buf)-1] = nil
		buf = buf[:len(buf)-2]
	case protocol.MessageDropPolicy_Oldest:
		_, _, buf = popHead(buf)
	case protocol.MessageDropPolicy_Incoming:
		return buf, true
	}
	return buf, false
}

func (e *exchange) RemoveActor(actorID string) error {
	e.log.Debug().Str("actor", actorID).Msg("remove actor")
	var err error
	e.writeLock(func() {
		if err = e.stillRunning(); err != nil {
			return
		}
		actor, has := e.actors.entries[actorID]
		if !has {
			err = ErrActorNotFound
		}
		println("calling stop on ", actorID)
		actor.stop()
		delete(e.catalog, actorID)
		delete(e.actors.entries, actorID)
	})
	return err
}

func (e *exchange) AddActor(actorID string, callback ActorCallback) error {
	e.log.Debug().Str("actor", actorID).Str("callback", fmt.Sprintf("%p", callback)).Msg("got a new actor")
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
