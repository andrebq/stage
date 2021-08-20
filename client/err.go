package client

type (
	Error string
)

const (
	ErrMissingDestination = Error("missing destination")
	ErrMissingSenderID    = Error("missing sender")
)

func (e Error) Error() string { return string(e) }
