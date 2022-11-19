package ubank

import (
	"context"
	"sync"
	"time"

	"github.com/andrebq/stage"
	"github.com/andrebq/stage/internal/leakybuf"
)

type (
	pipe struct {
		sync.Mutex
		left, right string
		leftBuf     leakybuf.B[stage.Message]
		rightBuf    leakybuf.B[stage.Message]
	}
	fakeUpstream struct {
		p  *pipe
		id string
	}
)

func pipeUpstream(left, right string) (stage.Upstream, stage.Upstream) {
	p := &pipe{
		left:  left,
		right: right,
	}
	return &fakeUpstream{
			p:  p,
			id: left,
		}, &fakeUpstream{
			p:  p,
			id: right,
		}
}

func (f *fakeUpstream) Close() error {
	return nil
}

func (f *fakeUpstream) ID() string {
	return f.id
}

func (f *fakeUpstream) Proxy(_ context.Context, msgs ...stage.Message) error {
	f.p.Lock()
	defer f.p.Unlock()
	if f.id == f.p.left {
		for _, m := range msgs {
			f.p.rightBuf.Push(m)
		}
	} else {
		for _, m := range msgs {
			f.p.leftBuf.Push(m)
		}
	}
	return nil
}

func (f *fakeUpstream) RegisterPIDs(_ context.Context, pids ...stage.Identity) error {
	return nil
}

func (f *fakeUpstream) Fetch(ctx context.Context) ([]stage.Message, error) {
	var buf *leakybuf.B[stage.Message]
	if f.id == f.p.right {
		buf = &f.p.rightBuf
	} else {
		buf = &f.p.leftBuf
	}
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		f.p.Lock()
		m, found := buf.Peek()
		if found {
			buf.Pop()
		}
		f.p.Unlock()
		if !found {
			time.Sleep(time.Millisecond)
			continue
		}
		return []stage.Message{m}, nil
	}
}
