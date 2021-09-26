package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/andrebq/stage/internal/protocol"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type (
	Catalog struct {
		lock     sync.RWMutex
		entries  map[string]*url.URL
		initDone bool
	}

	catalogServer struct {
		protocol.UnimplementedCatalogServer
		c *Catalog
	}

	Server interface {
		Serve(lst net.Listener, opts ...grpc.ServerOption) error
	}

	CatalogServer interface {
		Server
		protocol.CatalogServer
	}
)

func (c *Catalog) AddPair(actorID string, endpoint *url.URL) error {
	if endpoint.Scheme != "grpc" {
		return errors.New("catalog requires exchange to be a grpc:// url")
	}
	c.init()
	c.lock.Lock()
	c.entries[actorID] = endpoint
	c.lock.Unlock()
	return nil
}

func (c *Catalog) GetEndpoint(actorID string) (*url.URL, bool) {
	c.init()
	c.lock.RLock()
	u, ok := c.GetEndpoint(actorID)
	c.lock.RUnlock()
	return u, ok
}

func (c *Catalog) Endpoints(output []string, actors []string) []string {
	c.init()
	c.lock.RLock()
	for _, v := range actors {
		ep, ok := c.entries[v]
		if !ok {
			output = append(output, "")
			continue
		}
		output = append(output, ep.String())
	}
	return output
}

func (c *Catalog) Export() CatalogServer {
	cs := catalogServer{}
	cs.c = c
	return cs
}

func (c *Catalog) init() {
	if c.initDone {
		return
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.initDone {
		return
	}
	c.entries = make(map[string]*url.URL)
	c.initDone = true
}

func (cs catalogServer) Register(ctx context.Context, req *protocol.RegisterRequest) (*protocol.RegisterResponse, error) {
	u, err := url.Parse(req.Exchange)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("endpoint %v cannot be parsed as a valid exchange, cause %v", req.Exchange, err))
	}
	for _, actor := range req.GetActors() {
		err = cs.c.AddPair(actor, u)
		if err != nil {
			return nil, status.Error(codes.Unknown, fmt.Sprintf("unable to bind actor %v to exchange %v, cause %v", actor, req.Exchange, err))
		}
	}
	return &protocol.RegisterResponse{Active: true}, nil
}

func (cs catalogServer) Address(ctx context.Context, req *protocol.AddressList) (*protocol.AddressList, error) {
	items := make([]string, len(req.GetAddresses()))
	for i := range req.Addresses {
		items[i] = req.GetAddresses()[i].Actor
	}
	endpoints := cs.c.Endpoints(nil, items)
	for i := range req.Addresses {
		req.GetAddresses()[i].Endpoint = endpoints[i]
	}
	return req, nil
}

func (cs catalogServer) Ping(ctx context.Context, req *protocol.Echo) (*protocol.Echo, error) {
	req.RecvTime = time.Now().UnixNano()
	return req, nil
}

func (cs catalogServer) Serve(lst net.Listener, opts ...grpc.ServerOption) error {
	server := grpc.NewServer(opts...)
	protocol.RegisterCatalogServer(server, cs)
	return server.Serve(lst)
}
