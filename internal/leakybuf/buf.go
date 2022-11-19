package leakybuf

type (
	B[MsgKind any] struct {
		Max   int
		items []MsgKind
	}
)

func (lb *B[MsgKind]) Peek() (MsgKind, bool) {
	if lb == nil || len(lb.items) == 0 {
		var zero MsgKind
		return zero, false
	}
	return lb.items[0], true
}

func (lb *B[MsgKind]) Pop() {
	if lb == nil || len(lb.items) == 0 {
		return
	}
	// TODO: replace this with a proper ring buffer
	// this will avoid this useless copy operation
	copy(lb.items, lb.items[1:])
	var zero MsgKind
	// allow GC to happen
	lb.items[len(lb.items)-1] = zero
	// shrink the buffer
	lb.items = lb.items[:len(lb.items)-1]
}

func (lb *B[MsgKind]) Push(val MsgKind) {
	if lb == nil {
		*lb = B[MsgKind]{}
	}
	if len(lb.items) == lb.Max {
		lb.Pop()
	}
	lb.items = append(lb.items, val)
}

func (lb *B[MsgKind]) Len() int {
	if lb == nil {
		return 0
	}
	return len(lb.items)
}
