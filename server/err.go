package server

const (
	ErrServerShutdown = Error("server in shutdown mode")
	ErrServerStopped  = Error("server stopped")
	ErrActorNotFound  = Error("actor not found")
)

type (
	Error string
)

func (e Error) Error() string { return string(e) }
