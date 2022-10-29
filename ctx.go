package stage

import "context"

type (
	ctxKey byte

	nopMedia struct{}
)

const (
	mediaKey = ctxKey(1)
)

func withMedia(ctx context.Context, media Media) context.Context {
	return context.WithValue(ctx, mediaKey, media)
}

func MediaFromContext(ctx context.Context) Media {
	val := ctx.Value(mediaKey)
	if val == nil {
		return nopMedia{}
	}
	return val.(Media)
}

func (n nopMedia) Send(ctx context.Context, to Identity, method string, data interface{}) error {
	return ErrMediaDisconnected
}
