package ubank

import "github.com/andrebq/stage"

func Register(s *stage.S) {
	s.Register(accountActorName, func() stage.Actor {
		return NewAccount()
	})
}
