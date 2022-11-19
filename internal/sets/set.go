package sets

type (
	S[K comparable] struct {
		v map[K]struct{}
	}
)

func (s *S[K]) Add(keys ...K) {
	if s == nil {
		*s = S[K]{}
	}
	if s.v == nil {
		s.v = map[K]struct{}{}
	}
	for _, k := range keys {
		s.v[k] = struct{}{}
	}
}

func (s *S[K]) Has(k K) bool {
	if s == nil {
		return false
	}
	_, found := s.v[k]
	return found
}
