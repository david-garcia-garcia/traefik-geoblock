package dbprovider

import "sync"

// InstanceLock serializes create-once for a caller-owned map.
// Yaegi v0.16.1 panics on *reclaim.Table[*T] named in another package.
// The process table is non-generic any; callers assert (pkg/reclaim).
type InstanceLock struct {
	mu sync.RWMutex
}

// NewInstanceLock returns an unlocked InstanceLock.
func NewInstanceLock() *InstanceLock {
	return &InstanceLock{}
}

// LoadOrStore runs store unless has is already true. has is checked again under the write lock.
func (l *InstanceLock) LoadOrStore(has func() bool, store func() error) error {
	l.mu.RLock()
	if has() {
		l.mu.RUnlock()
		return nil
	}
	l.mu.RUnlock()

	l.mu.Lock()
	defer l.mu.Unlock()
	if has() {
		return nil
	}
	return store()
}

// Reset runs fn under the write lock (tests / shutdown).
func (l *InstanceLock) Reset(fn func()) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fn()
}
