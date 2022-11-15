package stage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"

	nats "github.com/nats-io/nats.go"
)

type (
	S struct {
		lock sync.Mutex
		nc   *nats.Conn

		nextReplyPID uint64
		nextActorPID uint64

		actors sync.Map
	}

	stageMedia struct {
		id Identity
		s  *S
		na Actor
	}

	NewActor func() Actor
)

var (
	discard = Identity{PID: "discard"}
)

var (
	ErrTemplateNotRegistered = errors.New("stage: desired template not available on this stage")
	ErrEmptyIdentity         = errors.New("stage: missing identity")
	ErrMethodNotFound        = errors.New("stage: method not found")
	ErrMediaDisconnected     = errors.New("stage: media disconnected")
	ErrInboxNotFound         = errors.New("stage: inbox not found")
)

func New(natsUpstream string) (*S, error) {
	if natsUpstream == "" {
		natsUpstream = nats.DefaultURL
	}
	s := &S{}
	return s, nil
}

func Discard() Identity {
	return discard
}

func (s *S) Close() error {
	s.nc.Close()
	return nil
}

func (s *S) Inject(ctx context.Context, to Identity, method string, data interface{}) error {
	sm := stageMedia{s: s, id: discard}
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
	val := atomic.AddUint64(&s.nextReplyPID, 1)
	return Identity{PID: fmt.Sprintf("reply.%v", val)}
}

func (s *S) actorPID() Identity {
	val := atomic.AddUint64(&s.nextActorPID, 1)
	return Identity{PID: fmt.Sprintf("actor.%v", val)}
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

func (n *stageMedia) Send(ctx context.Context, to Identity, method string, data interface{}) error {
	if to == discard {
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
	println("Sending: ", string(msg.Content), "to", msg.To.PID, "from", msg.From.PID)
	return n.sendMsg(ctx, msg)
}

func (n *stageMedia) sendMsg(ctx context.Context, msg Message) error {
	dest, found := n.s.lookupInbox(msg.To)
	if !found {
		return ErrInboxNotFound
	}
	dest.Push(ctx, msg)
	return nil
}

func (n *stageMedia) Become(ac Actor) {
	n.na = ac
}
