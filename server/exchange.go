package server

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/andrebq/stage/internal/protocol"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type (
	Exchange struct {
		lock    sync.Mutex
		catalog protocol.CatalogClient
		peers   map[string]protocol.ExchangeClient
		actors  map[string]*actor

		initDone bool
	}

	exchangeServer struct {
		protocol.UnimplementedExchangeServer
		e        *Exchange
		selfAddr string
	}

	ExchangeServer interface {
		Server
		protocol.ExchangeServer
	}

	actor struct {
		mailbox []*protocol.Message
	}
)

// Catalog changes the catalog instance used to discover peers for a given actor,
// as well as a way to register itself when accepting new actors.
func (e *Exchange) Catalog(ctx context.Context, endpoint string) error {
	// TODO: remove the hard-coded with insecure
	cc, err := grpc.DialContext(ctx, endpoint, grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("unable to connect to catalog at %v, cause %w", endpoint, err)
	}
	catalog := protocol.NewCatalogClient(cc)
	now := time.Now()
	_, err = catalog.Ping(ctx, &protocol.Echo{SentTime: now.UnixNano()})
	if err != nil {
		return fmt.Errorf("unable to ping catalog at %v, cause %w", endpoint, err)
	}
	after := time.Now()
	if after.Sub(now).Milliseconds() > 300 {
		return fmt.Errorf("catalog took %v millis to respond and that is more than we can accept", after.Sub(now).Milliseconds())
	}
	e.init()
	e.lock.Lock()
	e.catalog = catalog
	e.lock.Unlock()
	return nil
}

func (e *Exchange) Push(m *protocol.Message) error {
	e.init()
	e.lock.Lock()
	defer e.lock.Unlock()
	ac, ok := e.actors[m.GetIdentity().GetDestination()]
	if !ok {
		// TODO: use the catalog to discover a peer which might be responsible
		// for this specific actor ID
		return ErrActorNotFound
	}
	ac.mailbox = append(ac.mailbox, m)
	return nil
}

func (e *Exchange) Pop(actor string) (*protocol.Message, error) {
	e.init()
	e.lock.Lock()
	defer e.lock.Unlock()
	ac, ok := e.actors[actor]
	if !ok {
		return nil, ErrActorNotFound
	}
	if len(ac.mailbox) == 0 {
		return nil, nil
	}
	m := ac.mailbox[0]
	copy(ac.mailbox[0:], ac.mailbox[1:])
	ac.mailbox[len(ac.mailbox)-1] = nil
	ac.mailbox = ac.mailbox[0 : len(ac.mailbox)-1]
	return m, nil
}

func (e *Exchange) Register(actorID string) error {
	e.init()
	e.lock.Lock()
	defer e.lock.Unlock()
	_, ok := e.actors[actorID]
	if ok {
		return nil
	}
	ac := &actor{}
	e.actors[actorID] = ac
	return nil
}

func (e *Exchange) Export() ExchangeServer {
	return &exchangeServer{e: e}
}

func (e *Exchange) updateCatalog(ctx context.Context, exchangeAddr string) (*protocol.RegisterResponse, error) {
	e.init()
	e.lock.Lock()
	actors := make([]string, len(e.actors))
	e.lock.Unlock()
	if len(actors) == 0 {
		return nil, nil
	}
	return e.catalog.Register(ctx, &protocol.RegisterRequest{
		Actors:   actors,
		Exchange: exchangeAddr,
	})
}

func (e *Exchange) init() {
	if e.initDone {
		return
	}
	e.lock.Lock()
	defer e.lock.Unlock()
	if e.initDone {
		return
	}
	e.actors = make(map[string]*actor)
	e.peers = make(map[string]protocol.ExchangeClient)
	e.initDone = true
}

func (es *exchangeServer) Serve(lst net.Listener, options ...grpc.ServerOption) error {
	es.selfAddr = fmt.Sprintf("grpc://%v", lst.Addr().String())
	srv := grpc.NewServer(options...)
	protocol.RegisterExchangeServer(srv, es)
	return srv.Serve(lst)
}

func (es *exchangeServer) Ping(ctx context.Context, req *protocol.Echo) (*protocol.Echo, error) {
	req.RecvTime = time.Now().UnixNano()
	return req, nil
}

func (es *exchangeServer) Register(ctx context.Context, req *protocol.RegisterRequest) (*protocol.RegisterResponse, error) {
	if req.GetExchange() != "" && req.GetExchange() != es.selfAddr {
		return nil, status.Error(codes.InvalidArgument, "cannot register actors to a different exchange")
	}
	for _, a := range req.Actors {
		err := es.e.Register(a)
		if err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("unable to register actor %v, cause %v", a, err))
		}
	}
	return es.e.updateCatalog(ctx, es.selfAddr)
}

func (es *exchangeServer) Send(ctx context.Context, req *protocol.SendRequest) (*protocol.SendResponse, error) {
	res := &protocol.SendResponse{}
	for _, m := range req.GetMessages() {
		err := es.e.Push(m)
		if err != nil {
			// TODO: think about how to handle errors on batch operations
			continue
		}
		res.Delivered = append(res.Delivered, m.GetIdentity())
	}
	return res, nil
}

func (es *exchangeServer) Recv(ctx context.Context, req *protocol.RecvRequest) (*protocol.RecvResponse, error) {
	const maxItems = 1000
	res := &protocol.RecvResponse{}
	for _, a := range req.GetActors() {
		if len(res.Items) >= maxItems {
			break
		}
		for len(res.Items) < maxItems {
			m, err := es.e.Pop(a)
			if err != nil {
				// TODO: think about how to handle erros on batch operations
				break
			}
			if m == nil {
				// inbox is empty, move to the next actor
				break
			}
			res.Items = append(res.Items, m)
		}
	}
	return res, nil
}
