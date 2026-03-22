package main

import (
	"sync"
	"time"
)

// IdleTimer tracks user activity and fires a callback when the configured
// duration elapses without a Reset call. It uses time.AfterFunc internally
// and protects timer state with a mutex.
type IdleTimer struct {
	mu       sync.Mutex
	timer    *time.Timer
	duration time.Duration
	onIdle   func()
}

// NewIdleTimer creates and starts an idle timer that calls onIdle after the
// given duration of inactivity.
func NewIdleTimer(duration time.Duration, onIdle func()) *IdleTimer {
	it := &IdleTimer{
		duration: duration,
		onIdle:   onIdle,
	}
	it.mu.Lock()
	it.timer = time.AfterFunc(duration, onIdle)
	it.mu.Unlock()
	return it
}

// Reset restarts the idle timer. Call this on every user-initiated action to
// defer the idle lock.
func (it *IdleTimer) Reset() {
	it.mu.Lock()
	defer it.mu.Unlock()
	if it.timer != nil {
		it.timer.Stop()
		it.timer = time.AfterFunc(it.duration, it.onIdle)
	}
}

// Stop cancels the idle timer. After Stop, Reset is a no-op.
func (it *IdleTimer) Stop() {
	it.mu.Lock()
	defer it.mu.Unlock()
	if it.timer != nil {
		it.timer.Stop()
		it.timer = nil
	}
}
