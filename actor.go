package stage

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
)

type (
	Media interface {
		Send(ctx context.Context, to Identity, method string, content interface{}) error
		Become(Actor)
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
	BaseActor interface {
		Stateful
	}
	Dispatcher interface {
		Dispatch(ctx context.Context, message Message, output Media) error
	}
	Actor interface {
		BaseActor
		Dispatcher
	}

	Stateful interface {
	}

	Zeroer interface {
		Zero(context.Context) error
	}
	baseActor struct {
		template   string
		emptyState func() interface{}
		getState   func() interface{}
	}

	byReflection func(ctx context.Context, msg Message, output Media) error
)

var (
	ErrInvalidMethod = errors.New("stage: an actor method should have the following signature: func(context.Context, Identity, *YourRequestType, Media) error")
)

var (
	ctxType      = reflect.TypeOf((*context.Context)(nil)).Elem()
	mediaType    = reflect.TypeOf((*Media)(nil)).Elem()
	identityType = reflect.TypeOf(Identity{})
	errType      = reflect.TypeOf((*error)(nil)).Elem()
)

func (i Identity) IsZero() bool {
	return i.PID == ""
}

func BasicActor(template string, emptyState func() interface{}, getState func() interface{}) BaseActor {
	return &baseActor{
		template:   template,
		emptyState: emptyState,
		getState:   getState,
	}
}

func (b *baseActor) Template() string {
	return b.template
}

func (b *baseActor) EmptyStatePtr() interface{} {
	if b.emptyState == nil {
		return nil
	}
	return b.emptyState()
}

func (b *baseActor) Hibernate(ctx context.Context) (interface{}, error) {
	if b.getState == nil {
		return nil, nil
	}
	return b.getState(), nil
}

func DispatchByReflection(target BaseActor) Dispatcher {
	return byReflection(func(ctx context.Context, msg Message, output Media) error {
		return ReflectDispatch(ctx, msg, target, output)
	})
}

func ReflectDispatch(ctx context.Context, msg Message, ac BaseActor, output Media) error {
	method, err := findMethod(ac, msg.Method)
	if err != nil {
		println(msg.Method, err.Error())
		return err
	}
	return method(ctx, msg.From, msg.Content, output)
}

func findMethod(ac BaseActor, method string) (func(context.Context, Identity, []byte, Media) error, error) {
	val := reflect.ValueOf(ac)
	mval := val.MethodByName(method)
	if mval == (reflect.Value{}) {
		return nil, ErrMethodNotFound
	}
	mtype := mval.Type()
	if mtype.NumIn() != 4 {
		return nil, ErrInvalidMethod
	}
	if mtype.NumOut() != 1 {
		return nil, ErrInvalidMethod
	}
	if !mtype.In(0).Implements(ctxType) {
		return nil, ErrInvalidMethod
	}
	if mtype.In(1) != identityType {
		return nil, ErrInvalidMethod
	}
	if mtype.In(2).Kind() != reflect.Pointer {
		return nil, ErrInvalidMethod
	}
	if !mtype.In(3).Implements(mediaType) {
		return nil, ErrInvalidMethod
	}
	if !mtype.Out(0).Implements(errType) {
		return nil, ErrInvalidMethod
	}
	return func(ctx context.Context, from Identity, content []byte, output Media) error {
		arg := reflect.New(mtype.In(2).Elem())
		err := json.Unmarshal(content, arg.Interface())
		if err != nil {
			return err
		}
		val := mval.Call([]reflect.Value{
			reflect.ValueOf(ctx),
			reflect.ValueOf(from),
			arg,
			reflect.ValueOf(output),
		})
		if !val[0].IsNil() {
			return val[0].Interface().(error)
		}
		return nil
	}, nil
}

func (d byReflection) Dispatch(ctx context.Context, message Message, output Media) error {
	return d(ctx, message, output)
}
