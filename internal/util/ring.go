// Copyright (c) 2026 Cielavenir
// SPDX-License-Identifier: GPL-3.0-only

// Package util provides small, standalone utilities used across logwatch.
package util

import (
	"sync"
	"sync/atomic"
)

// RingBuffer is a fixed-capacity lock-free ring buffer. It overwrites old
// entries when full rather than blocking.
type RingBuffer[T any] struct {
	data []T
	head atomic.Uint64 // written to by producers only
	tail atomic.Uint64 // written to by the consumer only
	cap  uint64
}

// NewRingBuffer creates a RingBuffer with the given capacity.
func NewRingBuffer[T any](cap int) *RingBuffer[T] {
	if cap < 1 {
		cap = 1
	}
	return &RingBuffer[T]{
		data: make([]T, cap),
		cap:  uint64(cap),
	}
}

// Push stores value at the next slot, overwriting the oldest entry if the
// buffer is full. It never blocks.
func (r *RingBuffer[T]) Push(v T) {
	idx := r.head.Add(1) - 1
	r.data[idx%r.cap] = v
}

// Pop reads and removes the oldest buffered value. It returns ok=false if
// the buffer is empty.
func (r *RingBuffer[T]) Pop() (T, bool) {
	if r.tail.Load() >= r.head.Load() {
		var zero T
		return zero, false
	}
	idx := r.tail.Add(1) - 1
	v := r.data[idx%r.cap]
	return v, true
}

// Len returns the number of buffered items.
func (r *RingBuffer[T]) Len() int {
	n := int(r.head.Load() - r.tail.Load())
	return n
}

// Drain repeatedly calls Pop until the buffer is empty, calling fn on each item.
func (r *RingBuffer[T]) Drain(fn func(T)) {
	for {
		v, ok := r.Pop()
		if !ok {
			return
		}
		fn(v)
	}
}

// DropOldest removes and discards the oldest entry, returning true if
// something was actually dropped.
func (r *RingBuffer[T]) DropOldest() bool {
	if r.tail.Load() >= r.head.Load() {
		return false
	}
	r.tail.Add(1)
	return true
}

// ── Int utilities ─────────────────────────────────────────────────────────────

// Clamp returns v clamped to the [lo, hi] range.
func Clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Min returns the smaller of a or b.
func Min[T ~int | ~int64 | ~uint64](a, b T) T {
	if a < b {
		return a
	}
	return b
}

// Max returns the larger of a or b.
func Max[T ~int | ~int64 | ~uint64](a, b T) T {
	if a > b {
		return a
	}
	return b
}

// SafeClose is a convenience to close a channel and recover if it was already closed.
func SafeClose(ch chan struct{}) {
	defer func() { _ = recover() }()
	close(ch)
}

// ── Thread-safe counter ───────────────────────────────────────────────────────

// Counter is a simple atomic counter.
type Counter struct{ n atomic.Int64 }

func (c *Counter) Add(v int64) { c.n.Add(v) }
func (c *Counter) Get() int64  { return c.n.Load() }

// ── OnceFn caches the result of calling fn exactly once. ─────────────────────

type OnceFn[T any] struct {
	once sync.Once
	fn   func() T
	val  T
}

// NewOnceFn creates a OnceFn that calls fn on first access.
func NewOnceFn[T any](fn func() T) *OnceFn[T] {
	return &OnceFn[T]{fn: fn}
}

// Get calls fn if this is the first call, then returns the cached result.
func (o *OnceFn[T]) Get() T {
	o.once.Do(func() { o.val = o.fn() })
	return o.val
}
