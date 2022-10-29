package ubank

import "github.com/andrebq/stage"

func Register(s *stage.S) {
	s.Register(Account{}.Template(), func() stage.Actor {
		return Account{}
	})
}
