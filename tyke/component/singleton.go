package component

import "sync"

type Singleton[T any] struct {
	instance *T
	once     sync.Once
}

func (s *Singleton[T]) GetInstance(creator func() *T) *T {
	s.once.Do(func() {
		s.instance = creator()
	})
	return s.instance
}
