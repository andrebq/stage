package stage

import "context"

func zeroActor(ctx context.Context, ac BaseActor) error {
	if ac, ok := ac.(Zeroer); ok {
		return ac.Zero(ctx)
	}
	return nil
}
