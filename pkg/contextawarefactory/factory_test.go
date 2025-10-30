package contextawarefactory

import (
	"context"
	"errors"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

type testResource struct {
	id     string
	closed bool
}

func (r *testResource) Close() error {
	r.closed = true
	return nil
}

func TestFactory_BasicCreateAndRelease(t *testing.T) {
	createCount := 0
	factory := NewFactory(
		func(ctx context.Context, key string) (*testResource, error) {
			createCount++
			return &testResource{id: key}, nil
		},
		nil, // No cleanup needed for this test
	)

	ctx := context.Background()
	handle, err := factory.GetOrCreate(ctx, "test-key")
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Release()

	if createCount != 1 {
		t.Errorf("expected 1 create call, got %d", createCount)
	}

	resource := handle.Value()
	if resource.id != "test-key" {
		t.Errorf("expected id='test-key', got '%s'", resource.id)
	}

	stats := factory.GetStats()
	if stats.InstanceCount != 1 || stats.TotalRefCount != 1 {
		t.Errorf("expected 1 instance with 1 ref, got %d instances with %d refs", stats.InstanceCount, stats.TotalRefCount)
	}
}

func TestFactory_SharedInstance(t *testing.T) {
	createCount := 0
	factory := NewFactory(
		func(ctx context.Context, key string) (*testResource, error) {
			createCount++
			return &testResource{id: key}, nil
		},
		nil, // No cleanup needed
	)

	ctx := context.Background()

	// Create first handle
	handle1, err := factory.GetOrCreate(ctx, "shared-key")
	if err != nil {
		t.Fatal(err)
	}
	defer handle1.Release()

	// Create second handle with same key
	handle2, err := factory.GetOrCreate(ctx, "shared-key")
	if err != nil {
		t.Fatal(err)
	}
	defer handle2.Release()

	// Should only create once
	if createCount != 1 {
		t.Errorf("expected 1 create call for shared key, got %d", createCount)
	}

	// Should reference same instance
	if handle1.Value() != handle2.Value() {
		t.Error("expected handles to share same instance")
	}

	stats := factory.GetStats()
	if stats.InstanceCount != 1 || stats.TotalRefCount != 2 {
		t.Errorf("expected 1 instance with 2 refs, got %d instances with %d refs", stats.InstanceCount, stats.TotalRefCount)
	}
}

func TestFactory_CleanupOnLastRelease(t *testing.T) {
	cleanupCalled := false
	factory := NewFactory(
		func(ctx context.Context, key string) (*testResource, error) {
			return &testResource{id: key}, nil
		},
		func(r *testResource) error {
			cleanupCalled = true
			return r.Close()
		},
	)

	ctx := context.Background()

	handle1, err := factory.GetOrCreate(ctx, "cleanup-key")
	if err != nil {
		t.Fatal(err)
	}

	handle2, err := factory.GetOrCreate(ctx, "cleanup-key")
	if err != nil {
		t.Fatal(err)
	}

	// Release first handle - cleanup should not be called yet
	handle1.Release()
	time.Sleep(50 * time.Millisecond)

	if cleanupCalled {
		t.Error("cleanup called too early")
	}

	stats := factory.GetStats()
	if stats.InstanceCount != 1 || stats.TotalRefCount != 1 {
		t.Errorf("expected 1 instance with 1 ref after first release, got %d instances with %d refs", stats.InstanceCount, stats.TotalRefCount)
	}

	// Release second handle - cleanup should be called now
	handle2.Release()
	time.Sleep(50 * time.Millisecond)

	if !cleanupCalled {
		t.Error("cleanup not called after last release")
	}

	stats = factory.GetStats()
	if stats.InstanceCount != 0 || stats.TotalRefCount != 0 {
		t.Errorf("expected no instances after all releases, got %d instances with %d refs", stats.InstanceCount, stats.TotalRefCount)
	}
}

func TestFactory_ContextCancellation(t *testing.T) {
	factory := NewFactory(
		func(ctx context.Context, key string) (*testResource, error) {
			return &testResource{id: key}, nil
		},
		nil,
	)

	ctx, cancel := context.WithCancel(context.Background())

	handle, err := factory.GetOrCreate(ctx, "context-key")
	if err != nil {
		t.Fatal(err)
	}

	stats := factory.GetStats()
	if stats.TotalRefCount != 1 {
		t.Errorf("expected 1 ref, got %d", stats.TotalRefCount)
	}

	// Cancel context - handle should auto-release
	cancel()
	time.Sleep(100 * time.Millisecond)

	stats = factory.GetStats()
	if stats.InstanceCount != 0 || stats.TotalRefCount != 0 {
		t.Errorf("expected no instances after context cancellation, got %d instances with %d refs", stats.InstanceCount, stats.TotalRefCount)
	}

	// Calling Release again should be safe (no-op)
	err = handle.Release()
	if err != nil {
		t.Errorf("release after context cancellation should not error: %v", err)
	}
}

func TestFactory_MultipleKeys(t *testing.T) {
	factory := NewFactory(
		func(ctx context.Context, key string) (*testResource, error) {
			return &testResource{id: key}, nil
		},
		nil,
	)

	ctx := context.Background()

	handle1, _ := factory.GetOrCreate(ctx, "key1")
	defer handle1.Release()

	handle2, _ := factory.GetOrCreate(ctx, "key2")
	defer handle2.Release()

	handle3, _ := factory.GetOrCreate(ctx, "key3")
	defer handle3.Release()

	stats := factory.GetStats()
	if stats.InstanceCount != 3 || stats.TotalRefCount != 3 {
		t.Errorf("expected 3 instances with 3 refs, got %d instances with %d refs", stats.InstanceCount, stats.TotalRefCount)
	}

	if handle1.Value().id != "key1" {
		t.Errorf("wrong resource for key1")
	}
	if handle2.Value().id != "key2" {
		t.Errorf("wrong resource for key2")
	}
	if handle3.Value().id != "key3" {
		t.Errorf("wrong resource for key3")
	}
}

func TestFactory_CreatorError(t *testing.T) {
	expectedErr := errors.New("creation failed")
	factory := NewFactory(
		func(ctx context.Context, key string) (*testResource, error) {
			return nil, expectedErr
		},
		nil,
	)

	ctx := context.Background()
	handle, err := factory.GetOrCreate(ctx, "error-key")

	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}

	if handle != nil {
		t.Error("expected nil handle on error")
	}

	stats := factory.GetStats()
	if stats.InstanceCount != 0 {
		t.Errorf("expected no instances after creation error, got %d", stats.InstanceCount)
	}
}

func TestFactory_GoroutineCleanup(t *testing.T) {
	factory := NewFactory(
		func(ctx context.Context, key string) (*testResource, error) {
			return &testResource{id: key}, nil
		},
		nil,
	)

	ctx := context.Background()
	initial := runtime.NumGoroutine()

	// Create multiple handles
	handles := make([]*Handle[string, *testResource], 10)
	for i := 0; i < 10; i++ {
		h, _ := factory.GetOrCreate(ctx, "goroutine-test")
		handles[i] = h
	}

	afterCreate := runtime.NumGoroutine()
	// Should have created goroutines (one per handle)
	if afterCreate <= initial {
		t.Logf("Warning: expected more goroutines after creating handles")
	}

	// Release all handles
	for _, h := range handles {
		h.Release()
	}

	// Give goroutines time to exit
	time.Sleep(100 * time.Millisecond)

	afterRelease := runtime.NumGoroutine()
	// Goroutines should be cleaned up
	if afterRelease > initial+2 { // Allow small margin
		t.Errorf("goroutines not cleaned up: initial=%d afterCreate=%d afterRelease=%d", initial, afterCreate, afterRelease)
	}
}

func TestFactory_StructKey(t *testing.T) {
	type Config struct {
		Path string
		Size int
	}

	factory := NewFactory(
		func(ctx context.Context, key Config) (*testResource, error) {
			return &testResource{id: key.Path}, nil
		},
		nil,
	)

	ctx := context.Background()

	// Same struct values should share instance
	handle1, _ := factory.GetOrCreate(ctx, Config{Path: "/tmp/file", Size: 1024})
	defer handle1.Release()

	handle2, _ := factory.GetOrCreate(ctx, Config{Path: "/tmp/file", Size: 1024})
	defer handle2.Release()

	if handle1.Value() != handle2.Value() {
		t.Error("expected same struct keys to share instance")
	}

	// Different struct values should create different instances
	handle3, _ := factory.GetOrCreate(ctx, Config{Path: "/tmp/other", Size: 1024})
	defer handle3.Release()

	if handle1.Value() == handle3.Value() {
		t.Error("expected different struct keys to create different instances")
	}

	stats := factory.GetStats()
	if stats.InstanceCount != 2 {
		t.Errorf("expected 2 instances, got %d", stats.InstanceCount)
	}
}

func TestFactory_ConcurrentAccess(t *testing.T) {
	var createCount atomic.Int32
	factory := NewFactory(
		func(ctx context.Context, key string) (*testResource, error) {
			createCount.Add(1)
			time.Sleep(10 * time.Millisecond) // Simulate slow creation
			return &testResource{id: key}, nil
		},
		nil,
	)

	ctx := context.Background()
	const goroutines = 50

	handles := make([]*Handle[string, *testResource], goroutines)
	done := make(chan int, goroutines)

	// Spawn many goroutines trying to get the same key
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			h, err := factory.GetOrCreate(ctx, "concurrent-key")
			if err != nil {
				t.Errorf("goroutine %d: %v", idx, err)
			}
			handles[idx] = h
			done <- idx
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < goroutines; i++ {
		<-done
	}

	// Should only create once despite concurrent access
	if createCount.Load() != 1 {
		t.Errorf("expected 1 create call despite concurrent access, got %d", createCount.Load())
	}

	// All handles should reference the same instance
	firstResource := handles[0].Value()
	for i, h := range handles {
		if h.Value() != firstResource {
			t.Errorf("handle %d references different instance", i)
		}
	}

	// Cleanup
	for _, h := range handles {
		if h != nil {
			h.Release()
		}
	}

	time.Sleep(100 * time.Millisecond)
	stats := factory.GetStats()
	if stats.InstanceCount != 0 {
		t.Errorf("expected all instances cleaned up, got %d", stats.InstanceCount)
	}
}

func TestFactory_MultipleReleasesAreSafe(t *testing.T) {
	factory := NewFactory(
		func(ctx context.Context, key string) (*testResource, error) {
			return &testResource{id: key}, nil
		},
		nil,
	)

	ctx := context.Background()
	handle, _ := factory.GetOrCreate(ctx, "multi-release")

	// Release multiple times - should not panic or error
	err1 := handle.Release()
	err2 := handle.Release()
	err3 := handle.Release()

	if err1 != nil || err2 != nil || err3 != nil {
		t.Error("multiple releases should not error")
	}

	stats := factory.GetStats()
	if stats.InstanceCount != 0 {
		t.Errorf("expected instance cleaned up, got %d instances", stats.InstanceCount)
	}
}
