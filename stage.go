package stage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

type (
	S struct {
		lock sync.Mutex

		upstream struct {
			u         Upstream
			cancel    context.CancelFunc
			closeOnce sync.Once
		}

		nextReplyPID uint64
		nextActorPID uint64

		actors sync.Map

		discard Identity
	}

	Upstream interface {
		io.Closer
		ID() string
		// RegisterPIDs with the upstream, such that it knows which pids
		// can be hosted by this stage
		RegisterPIDs(context.Context, ...Identity) error
		// Proxy the list of messages via this upstream
		Proxy(context.Context, ...Message) error
		// Fetch the next batch of messages from this upstream
		// the batch might contain messages to PIDs that are no
		// different form the ones that were registered
		Fetch(context.Context) ([]Message, error)
	}

	stageMedia struct {
		id Identity
		s  *S
		na Actor
	}

	NewActor func() Actor
)

var (
	ErrTemplateNotRegistered = errors.New("stage: desired template not available on this stage")
	ErrEmptyIdentity         = errors.New("stage: missing identity")
	ErrMethodNotFound        = errors.New("stage: method not found")
	ErrMediaDisconnected     = errors.New("stage: media disconnected")
	ErrInboxNotFound         = errors.New("stage: inbox not found")
)

func New(upstream Upstream) (*S, error) {
	s := &S{}
	if upstream != nil {
		s.upstream.u = upstream
		var monitorCtx context.Context
		monitorCtx, s.upstream.cancel = context.WithCancel(context.Background())
		go s.monitorUpstream(monitorCtx)
	}
	s.discard = Identity{PID: fmt.Sprintf("%v.discard", s.stageID())}
	return s, nil
}

func (s *S) Discard() Identity {
	return s.discard
}

func (s *S) Close() error {
	var err error
	s.upstream.closeOnce.Do(func() {
		if s.upstream.cancel != nil {
			s.upstream.cancel()
			err = s.upstream.u.Close()
		}
	})
	return err
}

func (s *S) monitorUpstream(ctx context.Context) {
	for {
		batch, err := s.upstream.u.Fetch(ctx)
		log.Trace().Str("stage-id", s.stageID()).Int("batch-size", len(batch)).Send()
		if errors.Is(err, context.Canceled) {
			return
		} else if err != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Millisecond * 500):
				continue
			}
		}
		if len(batch) > 0 {
			for _, m := range batch {
				_ = s.directDispatch(ctx, m)
			}
		}
	}
}

func (s *S) Inject(ctx context.Context, to Identity, method string, data interface{}) error {
	sm := stageMedia{s: s, id: s.discard}
	return sm.Send(ctx, to, method, data)
}

func (s *S) Request(ctx context.Context, out interface{}, ttl time.Duration, to Identity, method string, data interface{}) error {
	ctx, cancel := context.WithTimeout(ctx, ttl)
	defer cancel()

	pid := s.replyPID()
	inbox := s.openInbox(ctx, pid)
	defer s.closeInbox(pid, inbox)
	sm := stageMedia{s: s, id: pid}
	err := sm.Send(ctx, to, method, data)
	if err != nil {
		return err
	}
	val, err := inbox.Next(ctx)
	if err != nil {
		return err
	}
	return json.Unmarshal(val.Content, out)
}

func (s *S) Spawn(ctx context.Context, actor Actor) (Identity, error) {
	pid := s.actorPID()
	out := make(chan error, 1)
	go s.manage(out, ctx, actor, pid)
	err := <-out
	return pid, err
}

func (s *S) manage(initErr chan<- error, ctx context.Context, ac Actor, pid Identity) {
	close(initErr)
	if err := zeroActor(ctx, ac); err != nil {
		initErr <- err
		return
	}
	log := log.Logger.With().Str("pid", pid.PID).Logger()
	inbox := s.openInbox(ctx, pid)
	defer s.closeInbox(pid, inbox)
	for {
		next, err := inbox.Next(ctx)
		if err != nil {
			log.Error().Err(err).Msg("Error while processing actor")
			return
		}
		sm := stageMedia{s: s, id: pid}
		ac.Dispatch(ctx, next, &sm)
		if sm.na != nil {
			log.Error().Msg("Actor has become another one")
			ac = sm.na
		}
	}
}

func (s *S) replyPID() Identity {
	prefix := s.stageID()
	val := atomic.AddUint64(&s.nextReplyPID, 1)
	return Identity{PID: fmt.Sprintf("%v.r.%v", prefix, val)}
}

func (s *S) actorPID() Identity {
	prefix := s.stageID()
	val := atomic.AddUint64(&s.nextActorPID, 1)
	return Identity{PID: fmt.Sprintf("%v.a.%v", prefix, val)}
}

func (s *S) stageID() string {
	prefix := "s"
	if s.upstream.u != nil {
		prefix = s.upstream.u.ID()
	}
	return prefix
}

func (s *S) openInbox(ctx context.Context, pid Identity) *Inbox[Message] {
	s.lock.Lock()
	val, found := s.actors.Load(pid.PID)
	if !found {
		val = NewInbox[Message]()
		go val.(*Inbox[Message]).run(ctx)
		s.actors.Store(pid.PID, val)
	}
	s.lock.Unlock()
	return val.(*Inbox[Message])
}

func (s *S) closeInbox(pid Identity, inbox *Inbox[Message]) {
	s.lock.Lock()
	inbox.Close()
	s.actors.Delete(pid.PID)
	s.lock.Unlock()
}

func (s *S) lookupInbox(id Identity) (*Inbox[Message], bool) {
	val, found := s.actors.Load(id.PID)
	if !found {
		return nil, found
	}
	return val.(*Inbox[Message]), true
}

func (s *S) dispatch(ctx context.Context, msg Message) error {
	_, found := s.lookupInbox(msg.To)
	if !found {
		return s.remoteDispatch(ctx, msg)
	}
	return s.directDispatch(ctx, msg)
}

func (s *S) directDispatch(ctx context.Context, msg Message) error {
	ib, found := s.lookupInbox(msg.To)
	if !found {
		return ErrInboxNotFound
	}
	log.Trace().Str("stage-id", s.stageID()).Str("from", msg.From.PID).Str("to", msg.To.PID).Str("method", msg.Method).Bool("remote", false).Send()
	ib.Push(ctx, msg)
	return nil
}

func (s *S) remoteDispatch(ctx context.Context, msg Message) error {
	if s.upstream.u != nil {
		log.Trace().Str("stage-id", s.stageID()).Str("from", msg.From.PID).Str("to", msg.To.PID).Str("method", msg.Method).Bool("remote", true).Send()
		return s.upstream.u.Proxy(ctx, msg)
	}
	// no upstream proxy, so it is impossible for this inbox to exist at this moment
	return ErrInboxNotFound
}

func (n *stageMedia) Send(ctx context.Context, to Identity, method string, data interface{}) error {
	if to == n.s.discard {
		return nil
	}
	msg := Message{}
	msg.To = to
	var err error
	msg.Method = method
	msg.From = n.id
	msg.Content, err = json.Marshal(data)
	if err != nil {
		return err
	}
	return n.sendMsg(ctx, msg)
}

func (n *stageMedia) sendMsg(ctx context.Context, msg Message) error {
	return n.s.dispatch(ctx, msg)
}

func (n *stageMedia) Become(ac Actor) {
	n.na = ac
}
