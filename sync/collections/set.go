package collections

import (
	"context"
	"sync"
)

type Set[T comparable] struct {
	lock sync.Mutex
	set  map[T]bool
	ch   chan T
}

func NewSet[T comparable](cap int) *Set[T] {
	s := &Set[T]{
		set: make(map[T]bool),
		ch:  make(chan T, cap),
	}
	return s
}

func (s *Set[T]) Push(i T) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if _, has := s.set[i]; has {
		return
	}
	if len(s.ch) == cap(s.ch) {
		return
	}
	s.ch <- i
}

func (s *Set[T]) Poll(ctx context.Context) (*T, error) {
	ch := s.Channel()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case v, ok := <-ch:
		if !ok {
			return nil, nil
		}
		s.Remove(v)
		return &v, nil
	}
}

func (s *Set[T]) Channel() chan T {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.ch
}

// Remove removes an element from the set.
func (s *Set[T]) Remove(i T) {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.set, i)
}

func (s *Set[T]) Empty() {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.empty()
}

func (s *Set[T]) empty() {
	for len(s.ch) > 0 {
		select {
		case <-s.ch:
		default:
		}
	}

	s.set = map[T]bool{}
}

// Len returns the number of elements in the set.
func (s *Set[T]) Len() int {
	return len(s.set)
}

// Cap returns the number of elements in the set.
func (s *Set[T]) Cap() int {
	return cap(s.Channel())
}

func (s *Set[T]) Expand(ns int) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if cap(s.ch) < ns {
		return
	}
	ch := s.ch
	s.ch = make(chan T, ns)

	for len(ch) > 0 {
		select {
		case v := <-ch:
			s.ch <- v
		default:
		}
	}
	close(ch)
}
