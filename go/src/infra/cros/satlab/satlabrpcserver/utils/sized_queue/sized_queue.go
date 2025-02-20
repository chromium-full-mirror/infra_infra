// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package sized_queue

import (
	"sync"

	"go.chromium.org/infra/cros/satlab/satlabrpcserver/utils"
)

// SizedQueue the queue uses the standard `List`.
// The main difference between `List` is that is capped
// size, and drops the oldest item when pushing more than its
// allowed capacity.
type SizedQueue[T any] struct {
	mu       sync.Mutex
	data     []T
	size     int
	capacity int
}

// New create a new queue with capacity
func New[T any](capacity int) SizedQueue[T] {
	return SizedQueue[T]{
		mu:       sync.Mutex{},
		data:     make([]T, capacity*2),
		size:     0,
		capacity: capacity,
	}
}

// Push the new item to the queue. If the size hits the capacity,
// It will drop the oldest item.
func (s *SizedQueue[T]) Push(data T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.size > (s.capacity - 1) {
		_, _ = s.innerPop()
	}
	s.data[s.size] = data
	s.size += 1
}

func (s *SizedQueue[T]) innerPop() (T, error) {
	var res T
	if s.size == 0 {
		return res, utils.EmptyQueue
	}
	s.size -= 1
	res = s.data[0]
	s.data = s.data[1:]
	// if the length of queue less or equal capacity
	// and then extend the length of the queue
	if len(s.data) == s.capacity {
		s.extend()
	}
	return res, nil
}

// Pop the oldest item.
func (s *SizedQueue[T]) Pop() (T, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.innerPop()
}

// Data Get the whole data
func (s *SizedQueue[T]) Data() []T {
	return s.data[0:s.size]
}

// Size get the size
func (s *SizedQueue[T]) Size() int {
	return s.size
}

// Clear reset the queue
func (s *SizedQueue[T]) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = make([]T, s.capacity*2)
	s.size = 0
}

// extend the data space.
func (s *SizedQueue[T]) extend() {
	moreSpace := make([]T, s.capacity)
	s.data = append(s.data, moreSpace...)
}
