package server

import (
	"fmt"
	"sync/atomic"

	"github.com/andrebq/stage/internal/protocol"
	"github.com/rs/zerolog"
)

type (
	recoveredPanic struct {
		cause interface{}
	}

	actor struct {
		inbox                  chan *protocol.Message
		teardown               chan struct{}
		failedDeliveriesInARow int32
		callback               ActorCallback
		log                    zerolog.Logger
	}
)

func (e recoveredPanic) Error() string {
	if e.cause == nil {
		return ""
	}
	switch e.cause.(type) {
	case error:
		return e.Error()
	default:
		return fmt.Sprintf("%v", e.cause)
	}
}

func (e recoveredPanic) Unwrap() error {
	if e.cause == nil {
		return nil
	}
	if err, ok := e.cause.(error); ok {
		return err
	}
	return fmt.Errorf("%v", e.cause)
}

func newActor(actorID string, callback ActorCallback, exchangeLog zerolog.Logger) *actor {
	return &actor{
		inbox:                  make(chan *protocol.Message, 1),
		teardown:               make(chan struct{}),
		failedDeliveriesInARow: 0,
		callback:               callback,
		log:                    exchangeLog.With().Str("actorID", actorID).Logger(),
	}
}

func (a *actor) stop() {
	close(a.teardown)
}

func (a *actor) act() {
	done := make(chan struct{}, 1)
	inbox := a.inbox
	for {
		select {
		case msg := <-inbox:
			go a.handleMsg(msg, done)
			// this prevents any new message from being processed
			// while the old one is still being processed
			//
			// but still leaves the actor available to receive the
			// teardown message
			inbox = nil
		case <-done:
			inbox = a.inbox
		case <-a.teardown:
			return
		}
	}
}

func (a *actor) deliver(msg *protocol.Message) bool {
	select {
	case a.inbox <- msg:
		return true
	default:
		atomic.AddInt32(&a.failedDeliveriesInARow, 1)
		return false
	}
}

func (a *actor) handleMsg(msg *protocol.Message, done chan struct{}) {
	err := dontPanic(func() {
		a.callback(msg)
	})
	if err != nil {
		a.log.Error().Err(err).Bool("panic", true).Msg("actor recovered from a panic")
	}
	select {
	case done <- struct{}{}:
	default:
	}
}

func dontPanic(fn func()) (err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = recoveredPanic{cause: rec}
		}
	}()
	fn()
	return
}
