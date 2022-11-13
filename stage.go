package stage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	nats "github.com/nats-io/nats.go"
)

type (
	S struct {
		lock sync.Mutex
		nc   *nats.Conn

		nextReplyPID uint64

		actorTemplate map[string]NewActor

		actors sync.Map
	}

	stageMedia struct {
		nc *nats.Conn
		id Identity
		s  *S
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
)

func New(natsUpstream string) (*S, error) {
	if natsUpstream == "" {
		natsUpstream = nats.DefaultURL
	}
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		return nil, err
	}
	s := &S{
		actorTemplate: make(map[string]NewActor),
	}
	s.nc = nc
	return s, nil
}

func Discard() Identity {
	return discard
}

func (s *S) Close() error {
	s.nc.Close()
	return nil
}

func (s *S) Register(name string, fn NewActor) {
	s.lock.Lock()
	s.actorTemplate[name] = fn
	s.lock.Unlock()
}

func (s *S) Inject(ctx context.Context, to Identity, method string, data interface{}) error {
	nm := stageMedia{
		id: Discard(),
		nc: s.nc,
		s:  s,
	}
	return nm.Send(ctx, to, method, data)
}

func (s *S) Request(ctx context.Context, out interface{}, ttl time.Duration, to Identity, method string, data interface{}) error {
	ctx, cancel := context.WithTimeout(ctx, ttl)
	defer cancel()
	nm := stageMedia{
		id: s.replyPID(),
		nc: s.nc,
		s:  s,
	}
	sub, err := s.nc.SubscribeSync(fmt.Sprintf("pids.%v", nm.id.PID))
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()
	err = nm.Send(ctx, to, method, data)
	if err != nil {
		return err
	}
	msg, err := sub.NextMsgWithContext(ctx)
	if err != nil {
		return err
	}
	err = json.Unmarshal(msg.Data, out)
	return err
}

func (s *S) Spawn(ctx context.Context, template string, id Identity) error {
	if id.PID == "" {
		return ErrEmptyIdentity
	}
	s.lock.Lock()
	fn := s.actorTemplate[template]
	s.lock.Unlock()
	if fn == nil {
		return ErrTemplateNotRegistered
	}
	ac := fn()
	if ac == nil {
		return ErrTemplateNotRegistered
	}

	initErr := make(chan error, 1)

	go s.manage(initErr, ctx, ac, id)
	err := <-initErr
	return err
}

func (s *S) manage(initErr chan<- error, ctx context.Context, ac Actor, id Identity) {
	inbox := NewInbox[*nats.Msg]()
	s.actors.Store(id.PID, inbox)
	defer func() {
		inbox.Close()
		s.actors.Delete(id.PID)
	}()

	out := make(chan *nats.Msg, 1000)
	topic := fmt.Sprintf("pids.%v", id.PID)
	subs, err := s.nc.ChanSubscribe(topic, out)
	if err != nil {
		initErr <- err
		return
	}
	err = s.init(ctx, ac, id, nil)
	if err != nil {
		initErr <- err
		return
	}
	media := stageMedia{s.nc, id, s}
	defer subs.Unsubscribe()
	close(initErr)
	for {
		msg, err := inbox.Next(ctx)
		if err != nil {
			return
		}
		s.dispatch(ctx, ac, id, msg, media)
	}
}

func (s *S) dispatch(ctx context.Context, ac Actor, id Identity, msg *nats.Msg, output Media) error {
	smsg := Message{}
	smsg.To = id
	smsg.From = Identity{PID: msg.Header.Get("Sender")}
	if smsg.From.PID == "" {
		return nil
	}
	smsg.Content = msg.Data
	smsg.Method = msg.Header.Get("Method")
	if ds, ok := ac.(Dispatcher); ok {
		return ds.Dispatch(ctx, smsg, output)
	}
	return ReflectDispatch(ctx, smsg, ac, output)
}

func (s *S) init(ctx context.Context, ac Actor, id Identity, snapshot []byte) error {
	if init, ok := ac.(Initializer); ok {
		st := ac.EmptyStatePtr()
		if len(snapshot) > 0 {
			err := json.Unmarshal(snapshot, st)
			if err != nil {
				return nil
			}
		}
		init.Init(ctx, st)
	}
	return nil
}

func (s *S) replyPID() Identity {
	val := atomic.AddUint64(&s.nextReplyPID, 1)
	return Identity{PID: fmt.Sprintf("reply.%v", val)}
}

func (n stageMedia) Send(ctx context.Context, to Identity, method string, data interface{}) error {
	if to == discard {
		return nil
	}
	msg := Message{}
	msg.To = to
	var err error
	msg.Method = method
	msg.Content, err = json.Marshal(data)
	if err != nil {
		return err
	}
	return n.SendMsg(ctx, msg)
}

func (n stageMedia) SendMsg(ctx context.Context, msg Message) error {
	msg.From = n.id
	m := &nats.Msg{}
	m.Subject = fmt.Sprintf("pids.%v", msg.To.PID)
	m.Header = make(nats.Header)
	m.Data = msg.Content
	m.Header.Add("Method", msg.Method)
	m.Header.Add("Sender", n.id.PID)
	dest, found := n.s.actors.Load(msg.To.PID)
	if !found {
		// probably a bug or remote actor,
		// for now, just drop the message
		// TODO: setup remote actors here
		// return n.nc.PublishMsg(m)
		return nil
	}
	dest.(*Inbox[*nats.Msg]).Push(ctx, m)
	return nil
}
