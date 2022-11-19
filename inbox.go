package stage

import (
	"context"
	"errors"
	"runtime"
	"sync"

	"github.com/andrebq/stage/internal/leakybuf"
)

type (
	signal             struct{}
	Inbox[MsgKind any] struct {
		pending leakybuf.B[MsgKind]

		closeOnce sync.Once

		stop   chan signal
		closed chan signal
		input  chan MsgKind
		output chan MsgKind
	}
)

var (
	errClosedInbox = errors.New("stage: closed inbox")
)

func NewInbox[MsgKind any]() *Inbox[MsgKind] {
	i := &Inbox[MsgKind]{
		stop:   make(chan signal),
		closed: make(chan signal),
		input:  make(chan MsgKind, runtime.NumCPU()),
		output: make(chan MsgKind),
		pending: leakybuf.B[MsgKind]{
			Max: 1000,
		},
	}
	return i
}

func (i *Inbox[MsgKind]) Next(ctx context.Context) (MsgKind, error) {
	var zero MsgKind
	select {
	case <-ctx.Done():
		return zero, ctx.Err()
	case <-i.closed:
		return zero, errClosedInbox
	case data, open := <-i.output:
		if !open {
			return zero, errClosedInbox
		}
		return data, nil
	}
}

func (i *Inbox[MsgKind]) Push(ctx context.Context, val MsgKind) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case i.input <- val:
		return nil
	case <-i.closed:
		return nil
	case <-i.stop:
		return nil
	}
}

func (i *Inbox[MsgKind]) run(ctx context.Context) error {
	defer close(i.closed)
	for {
		output := i.output
		val, foundData := i.pending.Peek()
		if !foundData {
			output = nil
		}
		select {
		case output <- val:
			i.pending.Pop()
		case nval := <-i.input:
			i.pending.Push(nval)
		case <-i.stop:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (i *Inbox[MsgKind]) Close() error {
	select {
	case <-i.stop:
		return nil
	case <-i.closed:
		return nil
	default:
		i.closeOnce.Do(i.doStop)
		return nil
	}
}

func (i *Inbox[MsgKind]) doStop() {
	close(i.stop)
}
