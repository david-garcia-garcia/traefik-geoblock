package contextawarefactory

import (
	"context"
	"sync"
)

// Factory is a generic context-aware factory that manages shared instances with reference counting.
// Instances are shared based on a key, and automatically cleaned up when all references are released.
//
// Type Parameters:
//   - K: Key type (must be comparable for use as map key)
//   - V: Value type (the instances being managed)
//
// Features:
//   - Reference counting: Multiple callers can share the same instance
//   - Context awareness: When a caller's context is cancelled, its reference is auto-released
//   - Automatic cleanup: Instance is destroyed when last reference is released
//   - Thread-safe: All operations are protected by mutexes
//
// Example:
//
//	type Config struct {
//	    Path string
//	    Size int
//	}
//
//	factory := NewFactory(
//	    func(ctx context.Context, key Config) (*MyResource, error) {
//	        return &MyResource{path: key.Path, size: key.Size}, nil
//	    },
//	    func(resource *MyResource) error {
//	        return resource.Close()
//	    },
//	)
//
//	handle, err := factory.GetOrCreate(ctx, Config{Path: "/tmp/file", Size: 1024})
//	if err != nil {
//	    // handle error
//	}
//	defer handle.Release()
//
//	resource := handle.Value()
type Factory[K comparable, V any] struct {
	mu        sync.Mutex
	instances map[K]*instance[K, V]
	creator   CreatorFunc[K, V]
	cleanup   CleanupFunc[V] // Optional cleanup function set at factory creation
}

// CreatorFunc is a function that creates a new instance for a given key.
// It receives the context and key, and returns the instance or an error.
type CreatorFunc[K comparable, V any] func(ctx context.Context, key K) (V, error)

// CleanupFunc is an optional function called when an instance is being destroyed (refCount reaches 0).
// It can be used to close files, release resources, etc.
type CleanupFunc[V any] func(value V) error

// instance represents a shared instance with reference counting
type instance[K comparable, V any] struct {
	mu       sync.Mutex
	key      K
	value    V
	refCount int
}

// Handle represents a reference to a shared instance.
// It must be released when no longer needed to decrement the reference count.
type Handle[K comparable, V any] struct {
	mu       sync.Mutex
	factory  *Factory[K, V]
	instance *instance[K, V]
	released bool
	cancel   context.CancelFunc // Cancels the context watcher goroutine
}

// NewFactory creates a new context-aware factory with the given creator function and optional cleanup function.
// The creator function is called when a new instance needs to be created for a key.
// The cleanup function is called when an instance is destroyed (refCount reaches 0). Pass nil for no cleanup.
func NewFactory[K comparable, V any](creator CreatorFunc[K, V], cleanup CleanupFunc[V]) *Factory[K, V] {
	return &Factory[K, V]{
		instances: make(map[K]*instance[K, V]),
		creator:   creator,
		cleanup:   cleanup,
	}
}

// GetOrCreate returns a handle to an instance for the given key.
// If an instance already exists for the key, it increments the reference count and returns a handle to it.
// If no instance exists, it creates one using the creator function.
//
// The returned handle automatically watches the provided context. When the context is cancelled,
// the handle is automatically released and the reference count is decremented.
//
// The caller must call handle.Release() when done (typically via defer), even though context
// cancellation will also release it. This ensures prompt cleanup and is safe to call multiple times.
func (f *Factory[K, V]) GetOrCreate(ctx context.Context, key K) (*Handle[K, V], error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	inst, exists := f.instances[key]
	if exists {
		// Increment reference count for existing instance
		inst.mu.Lock()
		inst.refCount++
		inst.mu.Unlock()

		handle := &Handle[K, V]{
			factory:  f,
			instance: inst,
			released: false,
		}

		// Start context watcher
		handle.startContextWatcher(ctx)

		return handle, nil
	}

	// Create new instance
	value, err := f.creator(ctx, key)
	if err != nil {
		return nil, err
	}

	inst = &instance[K, V]{
		key:      key,
		value:    value,
		refCount: 1,
	}

	f.instances[key] = inst

	handle := &Handle[K, V]{
		factory:  f,
		instance: inst,
		released: false,
	}

	// Start context watcher
	handle.startContextWatcher(ctx)

	return handle, nil
}

// Value returns the underlying instance value.
// The value remains valid as long as the handle hasn't been released.
func (h *Handle[K, V]) Value() V {
	return h.instance.value
}

// Release decrements the reference count for the instance.
// When the last reference is released, the instance is destroyed and removed from the factory.
// It is safe to call Release multiple times - subsequent calls are no-ops.
func (h *Handle[K, V]) Release() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.released {
		return nil
	}
	h.released = true

	// Stop the context watcher goroutine
	if h.cancel != nil {
		h.cancel()
	}

	// Decrement reference count through factory
	return h.factory.release(h.instance)
}

// release decrements the reference count and cleans up if needed
func (f *Factory[K, V]) release(inst *instance[K, V]) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	inst.mu.Lock()
	inst.refCount--
	refCount := inst.refCount
	key := inst.key
	value := inst.value
	inst.mu.Unlock()

	// If no more references, clean up and remove from factory
	if refCount <= 0 {
		// Remove from factory map
		delete(f.instances, key)

		// Call factory's cleanup function if set
		if f.cleanup != nil {
			return f.cleanup(value)
		}
	}

	return nil
}

// startContextWatcher starts a lightweight goroutine that watches the caller's context
// and auto-releases the handle when the context is cancelled.
func (h *Handle[K, V]) startContextWatcher(ctx context.Context) {
	// Create a cancellable context for this watcher
	watcherCtx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel

	go func() {
		select {
		case <-ctx.Done():
			// Caller's context was cancelled - auto-release this handle
			_ = h.Release()

		case <-watcherCtx.Done():
			// Handle was explicitly released - watcher can exit
			return
		}
	}()
}

// Stats returns statistics about the factory's current state (for debugging/monitoring)
type Stats struct {
	InstanceCount int
	TotalRefCount int
}

// GetStats returns current statistics about the factory
func (f *Factory[K, V]) GetStats() Stats {
	f.mu.Lock()
	defer f.mu.Unlock()

	stats := Stats{
		InstanceCount: len(f.instances),
	}

	for _, inst := range f.instances {
		inst.mu.Lock()
		stats.TotalRefCount += inst.refCount
		inst.mu.Unlock()
	}

	return stats
}

// ForceCleanupAll forcefully cleans up all instances in the factory.
// This is intended for testing/shutdown scenarios where you need to ensure
// all resources are released immediately, regardless of context state.
// WARNING: This bypasses the normal reference counting and context awareness.
func (f *Factory[K, V]) ForceCleanupAll() {
	f.mu.Lock()
	defer f.mu.Unlock()

	for key, inst := range f.instances {
		inst.mu.Lock()
		value := inst.value
		inst.refCount = 0 // Reset ref count
		inst.mu.Unlock()

		// Call cleanup if set
		if f.cleanup != nil {
			_ = f.cleanup(value)
		}

		// Remove from map
		delete(f.instances, key)
	}
}
