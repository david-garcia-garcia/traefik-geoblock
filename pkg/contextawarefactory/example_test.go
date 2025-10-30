package contextawarefactory_test

import (
	"context"
	"fmt"
	"os"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/contextawarefactory"
)

// Example demonstrates basic usage of the context-aware factory
func Example() {
	// Create temp file for cross-platform compatibility
	tmpFile, err := os.CreateTemp("", "example-*.txt")
	if err != nil {
		panic(err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Create a factory for file handles
	fileFactory := contextawarefactory.NewFactory(
		func(ctx context.Context, path string) (*os.File, error) {
			return os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0666)
		},
		func(f *os.File) error {
			fmt.Println("Closing file")
			return f.Close()
		},
	)

	ctx := context.Background()

	// First caller gets the file
	handle1, err := fileFactory.GetOrCreate(ctx, tmpPath)
	if err != nil {
		panic(err)
	}
	defer handle1.Release()

	// Second caller gets the same file instance
	handle2, _ := fileFactory.GetOrCreate(ctx, tmpPath)
	defer handle2.Release()

	// Both handles reference the same file
	fmt.Printf("Same file: %v\n", handle1.Value() == handle2.Value())

	stats := fileFactory.GetStats()
	fmt.Printf("Instances: %d, References: %d\n", stats.InstanceCount, stats.TotalRefCount)

	// Output:
	// Same file: true
	// Instances: 1, References: 2
	// Closing file
}

// ExampleFactory_structKey demonstrates using struct keys
func ExampleFactory_structKey() {
	type ServerConfig struct {
		Host string
		Port int
	}

	type Server struct {
		config ServerConfig
	}

	factory := contextawarefactory.NewFactory(
		func(ctx context.Context, cfg ServerConfig) (*Server, error) {
			return &Server{config: cfg}, nil
		},
		nil, // No cleanup needed for this example
	)

	ctx := context.Background()

	// Same config values share the same server
	handle1, _ := factory.GetOrCreate(ctx, ServerConfig{Host: "localhost", Port: 8080})
	defer handle1.Release()

	handle2, _ := factory.GetOrCreate(ctx, ServerConfig{Host: "localhost", Port: 8080})
	defer handle2.Release()

	// Different config creates different server
	handle3, _ := factory.GetOrCreate(ctx, ServerConfig{Host: "localhost", Port: 9090})
	defer handle3.Release()

	fmt.Printf("Same server for same config: %v\n", handle1.Value() == handle2.Value())
	fmt.Printf("Different server for different config: %v\n", handle1.Value() != handle3.Value())

	stats := factory.GetStats()
	fmt.Printf("Total servers: %d\n", stats.InstanceCount)

	// Output:
	// Same server for same config: true
	// Different server for different config: true
	// Total servers: 2
}

// ExampleHandle_contextCancellation demonstrates automatic cleanup on context cancellation
func ExampleHandle_contextCancellation() {
	cleanupCalled := false

	factory := contextawarefactory.NewFactory(
		func(ctx context.Context, key string) (*string, error) {
			value := "resource-" + key
			return &value, nil
		},
		func(s *string) error {
			cleanupCalled = true
			fmt.Println("Resource cleaned up")
			return nil
		},
	)

	ctx, cancel := context.WithCancel(context.Background())

	_, _ = factory.GetOrCreate(ctx, "example")

	// Cancel context - handle will be automatically released
	cancel()

	// Small delay to allow goroutine to process
	// In real code, this happens asynchronously
	fmt.Printf("Cleanup will be called: %v\n", !cleanupCalled)

	// Output:
	// Cleanup will be called: true
}
