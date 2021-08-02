package xmpp

// stack is a basic stack data structure
type stack struct {
	stack []interface{}
}

func (s *stack) Push(ele interface{}) {
	s.stack = append(s.stack, ele)
}

func (s *stack) Pop() interface{} {
	n := len(s.stack) - 1
	if n < 0 {
		return nil
	}
	v := s.stack[n]
	s.stack = s.stack[:n]
	return v
}

func (s *stack) Peek() interface{} {
	n := len(s.stack) - 1
	if n < 0 {
		return nil
	}
	return s.stack[n]
}
