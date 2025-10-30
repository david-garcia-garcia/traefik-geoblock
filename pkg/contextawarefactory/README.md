# Context-Aware Factory

A generic, reusable factory pattern for Go that manages shared instances with automatic reference counting and context-aware lifecycle management.

## Features

- **Generic**: Works with any key type (must be `comparable`) and any value type
- **Reference Counting**: Multiple callers can safely share the same instance
- **Context Awareness**: Automatically releases references when contexts are cancelled
- **Thread-Safe**: All operations are protected by mutexes
- **Automatic Cleanup**: Instances are destroyed when the last reference is released
- **Lightweight**: One goroutine per handle for context watching (goroutines are cheap in Go - 2KB stack)

## Use Cases

- **Shared File Handles**: Multiple components writing to the same file
- **Database Connections**: Connection pooling with automatic cleanup
- **Network Clients**: Reusing HTTP clients, gRPC connections, etc.
- **Caching**: Shared cache instances with lifecycle management
- **Any Resource**: Where multiple callers need to share expensive-to-create resources

## Installation

```bash
go get github.com/david-garcia-garcia/traefik-geoblock/pkg/contextawarefactory
```

## Usage

### Basic Example

```go
package main

import (
    "context"
    "github.com/david-garcia-garcia/traefik-geoblock/pkg/contextawarefactory"
)

type DatabaseConnection struct {
    dsn string
    // ...
}

func main() {
    // Create factory with creator and cleanup functions
    factory := contextawarefactory.NewFactory(
        // Creator function
        func(ctx context.Context, dsn string) (*DatabaseConnection, error) {
            return &DatabaseConnection{dsn: dsn}, nil
        },
        // Cleanup function (called when refCount reaches 0)
        func(conn *DatabaseConnection) error {
            return conn.Close()
        },
    )

    ctx := context.Background()

    // Get or create instance
    handle, err := factory.GetOrCreate(ctx, "postgresql://localhost/mydb")
    if err != nil {
        panic(err)
    }
    defer handle.Release() // Always release when done

    // Use the connection
    conn := handle.Value()
    _ = conn // use connection
}
```

### Without Cleanup Function

If your resource doesn't need cleanup, pass `nil`:

```go
factory := contextawarefactory.NewFactory(
    func(ctx context.Context, key string) (*MyResource, error) {
        return &MyResource{id: key}, nil
    },
    nil, // No cleanup needed
)
```

### Struct Keys

Keys can be any comparable type, including structs:

```go
type Config struct {
    Host string
    Port int
}

factory := contextawarefactory.NewFactory(
    func(ctx context.Context, cfg Config) (*Client, error) {
        return NewClient(cfg.Host, cfg.Port)
    },
)

// Same config values will share the same client
handle1, _ := factory.GetOrCreate(ctx, Config{Host: "localhost", Port: 8080})
handle2, _ := factory.GetOrCreate(ctx, Config{Host: "localhost", Port: 8080})
// handle1 and handle2 reference the same client instance
```

### Context Cancellation

The factory automatically monitors contexts and releases references when cancelled:

```go
ctx, cancel := context.WithCancel(context.Background())

handle, _ := factory.GetOrCreate(ctx, "mykey")
// Don't call defer handle.Release() - context will do it

// When context is cancelled, the handle is automatically released
cancel()

// Reference count is decremented, and if it was the last reference,
// the instance is destroyed and cleanup function is called
```

### Monitoring

```go
stats := factory.GetStats()
fmt.Printf("Instances: %d, Total References: %d\n", 
    stats.InstanceCount, stats.TotalRefCount)
```

## API Reference

### Factory

```go
type Factory[K comparable, V any]
```

#### Methods

- `NewFactory[K, V](creator CreatorFunc[K, V]) *Factory[K, V]`  
  Creates a new factory with the given creator function

- `GetOrCreate(ctx context.Context, key K) (*Handle[K, V], error)`  
  Gets or creates an instance for the given key

- `GetStats() Stats`  
  Returns current statistics (instance count, total references)

### Handle

```go
type Handle[K comparable, V any]
```

#### Methods

- `Value() V`  
  Returns the underlying instance value

- `Release() error`  
  Releases the reference (safe to call multiple times)

- `SetCleanup(cleanup CleanupFunc[V])`  
  Sets a cleanup function called when the last reference is released

### Creator Function

```go
type CreatorFunc[K comparable, V any] func(ctx context.Context, key K) (V, error)
```

Function that creates a new instance for a given key.

### Cleanup Function

```go
type CleanupFunc[V any] func(value V) error
```

Optional function called when an instance is destroyed (refCount reaches 0).

## Best Practices

1. **Always call Release()**: Use `defer handle.Release()` even though context cancellation will also release. This ensures prompt cleanup and is safe to call multiple times.

2. **Set Cleanup Early**: Call `SetCleanup()` immediately after getting the first handle, before the instance is shared.

3. **Keep Keys Simple**: Use simple, comparable types as keys. Structs work great for multi-field keys.

4. **Monitor in Production**: Use `GetStats()` to monitor factory health in production environments.

5. **One Factory Per Resource Type**: Create separate factories for different resource types to keep concerns separated.

## Thread Safety

All operations are thread-safe. The factory can be safely used from multiple goroutines concurrently.

## Performance

- **Goroutines**: One lightweight goroutine per handle (2KB stack)
- **Mutexes**: Fine-grained locking minimizes contention
- **Maps**: Standard Go maps with O(1) average lookup

## License

Same as parent project.

