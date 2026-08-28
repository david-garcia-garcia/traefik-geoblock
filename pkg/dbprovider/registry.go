package dbprovider

import "sync"

// InstanceLock serializes create-once for a caller-owned map.
// The typed map must stay in the caller's package. Yaegi v0.16.1 panics on
// *reclaim.Table[*T] named in another package; same-package Table[T] is fine.
// A map[string]any here plus a type-assert in the caller does load on
// traefik:v3.7.11 — do not use it anyway; keep T next to the table.
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
