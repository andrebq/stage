package upstream

import (
	"context"
	"net"
	"sync"

	"github.com/andrebq/stage"
	"github.com/andrebq/stage/internal/leakybuf"
	"github.com/andrebq/stage/internal/sets"
	"github.com/andrebq/stage/protocol"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type (
	server struct {
		protocol.UnimplementedUpstreamServer
		sync.Mutex

		remStages map[string]*leakybuf.B[stage.Message]
		pidMap    map[string]sets.S[string]
	}
)

func NewUpstreamServer() protocol.UpstreamServer {
	return &server{
		remStages: make(map[string]*leakybuf.B[stage.Message]),
	}
}

func NewServer(us protocol.UpstreamServer) *grpc.Server {
	s := grpc.NewServer()
	protocol.RegisterUpstreamServer(s, us)
	return s
}

func Serve(ctx context.Context, lst net.Listener, us protocol.UpstreamServer) error {
	s := NewServer(NewUpstreamServer())
	log.Info().Str("addr", lst.Addr().String()).Msg("Starting upstream server")
	return s.Serve(lst)
}

func ListenAndServe(ctx context.Context, addr string, us protocol.UpstreamServer) error {
	lst, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s := grpc.NewServer()
	log.Info().Str("addr", addr).Msg("Starting upstream server")
	protocol.RegisterUpstreamServer(s, us)
	return s.Serve(lst)
}

func (s *server) RegisterPIDs(ctx context.Context, pids *protocol.PIDSet) (*protocol.Reply, error) {
	sid, err := getStage(ctx)
	if err != nil {
		return nil, err
	}
	s.mapPids(sid, pids)
	return &protocol.Reply{}, nil
}

func (s *server) Proxy(ctx context.Context, pr *protocol.ProxyRequest) (*protocol.ProxyReply, error) {
	_, err := getStage(ctx)
	if err != nil {
		return nil, err
	}
	var proxied int
	s.Lock()
	defer s.Unlock()
	for _, v := range pr.GetMessages() {
		targetsid, found := s.sidForPid(v.To)
		if !found {
			continue
		}
		s.enqueue(targetsid, v)
		proxied++
	}
	return &protocol.ProxyReply{Total: int32(proxied)}, nil
}

func (s *server) Fetch(ctx context.Context, req *protocol.FetchRequest) (*protocol.FetchReply, error) {
	sid, err := getStage(ctx)
	if err != nil {
		return nil, err
	}
	s.Lock()
	defer s.Unlock()
	buf, ok := s.remStages[sid]
	if !ok {
		return &protocol.FetchReply{}, nil
	}
	if req.MaxSize > 500 || req.MaxSize <= 0 {
		req.MaxSize = 500
	}
	fr := &protocol.FetchReply{}
	for buf.Len() > 0 && req.MaxSize > 0 {
		req.MaxSize--
		m, _ := buf.Peek()
		buf.Pop()
		fr.Batch = append(fr.Batch, &protocol.StageMessage{
			From:    m.From.PID,
			To:      m.To.PID,
			Method:  m.Method,
			Content: m.Content,
		})
	}
	return fr, nil
}

func (s *server) mapPids(sid string, pids *protocol.PIDSet) {
	s.Lock()
	defer s.Unlock()
	if s.pidMap == nil {
		s.pidMap = make(map[string]sets.S[string])
	}
	stagePids := s.pidMap[sid]
	for _, v := range pids.GetPids() {
		stagePids.Add(v)
	}
	s.pidMap[sid] = stagePids
}

func (s *server) sidForPid(pid string) (string, bool) {
	for k, v := range s.pidMap {
		if v.Has(pid) {
			return k, true
		}
	}
	return "", false
}

func (s *server) enqueue(sid string, msg *protocol.StageMessage) {
	msgs := s.remStages[sid]
	if msgs == nil {
		msgs = &leakybuf.B[stage.Message]{}
		s.remStages[sid] = msgs
	}
	msgs.Push(stage.Message{
		From:    stage.Identity{PID: msg.GetFrom()},
		To:      stage.Identity{PID: msg.GetTo()},
		Method:  msg.GetMethod(),
		Content: msg.GetContent(),
	})
}

func getStage(ctx context.Context) (string, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	vals := md.Get("StageID")
	if len(vals) == 0 || len(vals) > 1 {
		return "", status.Errorf(codes.FailedPrecondition, "call is missing (or has invalid) StageID header")
	}
	return vals[0], nil
}
