package stage

import (
	"context"
	"errors"
	"runtime"
	"sync"
)

type (
	signal             struct{}
	Inbox[MsgKind any] struct {
		pending leakybuf[MsgKind]

		closeOnce sync.Once

		stop   chan signal
		closed chan signal
		input  chan MsgKind
		output chan MsgKind
	}

	leakybuf[MsgKind any] struct {
		max   int
		items []MsgKind
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
		pending: leakybuf[MsgKind]{
			max: 1000,
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
	case data := <-i.output:
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
		val, empty := i.pending.peek()
		if empty {
			output = nil
		}
		select {
		case output <- val:
		case nval := <-i.input:
			i.pending.push(nval)
		case <-i.stop:
			close(i.closed)
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

func (lb *leakybuf[MsgKind]) peek() (MsgKind, bool) {
	if len(lb.items) == 0 {
		var zero MsgKind
		return zero, false
	}
	return lb.items[0], true
}

func (lb *leakybuf[MsgKind]) pop() {
	if len(lb.items) == 0 {
		return
	}
	// TODO: replace this with a proper ring buffer
	// this will avoid this useless copy operation
	copy(lb.items, lb.items[1:])
	var zero MsgKind
	// allow GC to happen
	lb.items[len(lb.items)-1] = zero
	// shrink the buffer
	lb.items = lb.items[:len(lb.items)-1]
}

func (lb *leakybuf[MsgKind]) push(val MsgKind) {
	if len(lb.items) == lb.max {
		lb.pop()
	}
	lb.items = append(lb.items, val)
}
