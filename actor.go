package stage

import "context"

type (
	Media interface {
		Send(ctx context.Context, to Identity, method string, content interface{}) error
	}
	RawMedia interface {
		Media
		SendMsg(ctx context.Context, msg Message) error
	}
	Identity struct {
		PID string
	}
	PersitentState struct {
		Template string
		PID      string
		Content  interface{}
	}
	Message struct {
		From    Identity
		To      Identity
		Method  string
		Content []byte
	}
	Actor interface {
		Template() string
		Init(ctx context.Context, state interface{}) error
		Hibernate(ctx context.Context) (state interface{}, err error)
		Dispatch(ctx context.Context, message Message, output Media) (self Actor, err error)
	}
)
