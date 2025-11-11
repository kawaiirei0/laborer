# Laborer API Documentation

This document provides comprehensive API documentation for the Laborer goroutine pool library.

## Table of Contents

- [Pool Creation](#pool-creation)
- [Task Submission](#task-submission)
- [Pool Management](#pool-management)
- [Status Monitoring](#status-monitoring)
- [Configuration Options](#configuration-options)
- [Future Interface](#future-interface)
- [Error Handling](#error-handling)
- [Logger Interface](#logger-interface)

## Pool Creation

### NewPool

```go
func NewPool(size int, options ...Option) (*Pool, error)
```

Creates a new goroutine pool with the specified capacity.

**Parameters:**
- `size`: Pool capacity (maximum number of workers)
  - Positive integer: Fixed capacity
  - `-1`: Unlimited capacity
  - `0`: Invalid, returns `ErrInvalidPoolSize`
- `options`: Variable number of configuration options

**Returns:**
- `*Pool`: The created pool instance
- `error`: Error if creation fails

**Example:**

```go
// Fixed capacity pool
pool, err := laborer.NewPool(100)
if err != nil {
    log.Fatal(err)
}
defer pool.Release()

// Unlimited capacity pool
pool, err := laborer.NewPool(-1)

// Pool with options
pool, err := laborer.NewPool(
    100,
    laborer.WithExpiryDuration(30 * time.Second),
    laborer.WithPreAlloc(true),
    laborer.WithNonblocking(false),
)
```

### NewPoolWithFunc

```go
func NewPoolWithFunc(size int, pf func(interface{}), options ...Option) (*PoolWithFunc, error)
```

Creates a new function pool that executes the same function with different parameters.

**Parameters:**
- `size`: Pool capacity
- `pf`: The function to be executed by all workers
- `options`: Configuration options

**Returns:**
- `*PoolWithFunc`: The created function pool instance
- `error`: Error if creation fails

**Example:**

```go
pool, err := laborer.NewPoolWithFunc(10, func(data interface{}) {
    // Process data
    result := processData(data)
    fmt.Println(result)
})
if err != nil {
    log.Fatal(err)
}
defer pool.Release()

// Submit parameters
for i := 0; i < 100; i++ {
    pool.Invoke(i)
}
```

## Task Submission

### Submit

```go
func (p *Pool) Submit(task func()) error
```

Submits a task without return value to the pool for execution.

**Parameters:**
- `task`: The function to be executed

**Returns:**
- `error`: 
  - `nil`: Task submitted successfully
  - `ErrPoolClosed`: Pool has been closed
  - `ErrPoolOverload`: Pool is overloaded (non-blocking mode only)

**Behavior:**
- **Blocking mode** (default): Waits until a worker is available
- **Non-blocking mode**: Returns `ErrPoolOverload` immediately if pool is full

**Example:**

```go
err := pool.Submit(func() {
    fmt.Println("Task executing")
    // Do work
})
if err != nil {
    if errors.Is(err, laborer.ErrPoolOverload) {
        // Handle overload
    } else if errors.Is(err, laborer.ErrPoolClosed) {
        // Handle closed pool
    }
}
```

### SubmitWithResult

```go
func (p *Pool) SubmitWithResult(task func() (interface{}, error)) (Future, error)
```

Submits a task with return value to the pool for execution.

**Parameters:**
- `task`: The function to be executed, returns a result and an error

**Returns:**
- `Future`: A Future object to retrieve the result
- `error`: Submission error (same as Submit)

**Example:**

```go
future, err := pool.SubmitWithResult(func() (interface{}, error) {
    result := heavyComputation()
    if result == nil {
        return nil, errors.New("computation failed")
    }
    return result, nil
})
if err != nil {
    log.Fatal(err)
}

// Get result
result, err := future.Get()
if err != nil {
    log.Printf("Task failed: %v", err)
} else {
    log.Printf("Result: %v", result)
}
```

### Invoke (PoolWithFunc)

```go
func (p *PoolWithFunc) Invoke(args interface{}) error
```

Submits parameters to the fixed function for execution.

**Parameters:**
- `args`: Parameters to be passed to the pool function

**Returns:**
- `error`: Same as Submit

**Example:**

```go
pool, _ := laborer.NewPoolWithFunc(10, func(data interface{}) {
    processData(data)
})

for i := 0; i < 100; i++ {
    if err := pool.Invoke(i); err != nil {
        log.Printf("Failed to invoke: %v", err)
    }
}
```

## Pool Management

### Release

```go
func (p *Pool) Release()
```

Gracefully shuts down the pool, waiting for all running tasks to complete.

**Behavior:**
- Stops accepting new tasks
- Waits for all running tasks to complete
- Releases all worker resources
- Wakes up all waiting goroutines

**Example:**

```go
pool, _ := laborer.NewPool(10)
defer pool.Release()

// Use pool...
```

### ReleaseTimeout

```go
func (p *Pool) ReleaseTimeout(timeout time.Duration) error
```

Gracefully shuts down the pool with a timeout.

**Parameters:**
- `timeout`: Maximum time to wait for shutdown

**Returns:**
- `error`:
  - `nil`: Shutdown completed successfully
  - `ErrTimeout`: Shutdown timed out
  - `ErrPoolClosed`: Pool already closed

**Example:**

```go
// Try graceful shutdown with timeout
if err := pool.ReleaseTimeout(30 * time.Second); err != nil {
    if errors.Is(err, laborer.ErrTimeout) {
        log.Println("Graceful shutdown timed out, forcing shutdown")
        pool.Release()
    }
}
```

### Reboot

```go
func (p *Pool) Reboot()
```

Restarts a closed pool.

**Behavior:**
- Changes pool state from CLOSED to OPENED
- Restarts the worker cleanup goroutine
- Pool can accept new tasks again

**Example:**

```go
pool.Release()
// Maintenance or configuration changes
time.Sleep(time.Second)
pool.Reboot()
// Pool is ready to use again
```

## Status Monitoring

### Running

```go
func (p *Pool) Running() int
```

Returns the number of currently running workers.

**Returns:**
- `int`: Number of workers currently executing tasks

**Example:**

```go
running := pool.Running()
fmt.Printf("Running workers: %d\n", running)
```

### Free

```go
func (p *Pool) Free() int
```

Returns the number of idle workers available in the pool.

**Returns:**
- `int`: Number of idle workers

**Example:**

```go
free := pool.Free()
fmt.Printf("Free workers: %d\n", free)
```

### Cap

```go
func (p *Pool) Cap() int
```

Returns the pool capacity.

**Returns:**
- `int`: Pool capacity (-1 for unlimited)

**Example:**

```go
capacity := pool.Cap()
fmt.Printf("Pool capacity: %d\n", capacity)
```

### Waiting

```go
func (p *Pool) Waiting() int
```

Returns the number of tasks waiting to be executed.

**Returns:**
- `int`: Number of waiting tasks (only in blocking mode)

**Example:**

```go
waiting := pool.Waiting()
if waiting > pool.Cap() {
    log.Println("Pool is overloaded")
}
```

### IsClosed

```go
func (p *Pool) IsClosed() bool
```

Checks if the pool is closed.

**Returns:**
- `bool`: `true` if pool is closed, `false` otherwise

**Example:**

```go
if pool.IsClosed() {
    log.Println("Pool is closed, rebooting...")
    pool.Reboot()
}
```

## Configuration Options

### WithExpiryDuration

```go
func WithExpiryDuration(duration time.Duration) Option
```

Sets the worker idle timeout duration.

**Parameters:**
- `duration`: Timeout duration (must be positive)

**Default:** 10 seconds

**Example:**

```go
pool, _ := laborer.NewPool(10,
    laborer.WithExpiryDuration(30 * time.Second))
```

### WithPreAlloc

```go
func WithPreAlloc(preAlloc bool) Option
```

Sets whether to pre-allocate the worker slice.

**Parameters:**
- `preAlloc`: `true` to enable pre-allocation

**Default:** `false`

**Example:**

```go
pool, _ := laborer.NewPool(100,
    laborer.WithPreAlloc(true))
```

### WithNonblocking

```go
func WithNonblocking(nonblocking bool) Option
```

Sets the pool to non-blocking mode.

**Parameters:**
- `nonblocking`: `true` for non-blocking mode

**Default:** `false` (blocking mode)

**Example:**

```go
pool, _ := laborer.NewPool(10,
    laborer.WithNonblocking(true))
```

### WithPanicHandler

```go
func WithPanicHandler(panicHandler func(interface{})) Option
```

Sets a custom panic handler for task execution.

**Parameters:**
- `panicHandler`: Function to handle panics

**Default:** `nil` (panics are logged)

**Example:**

```go
pool, _ := laborer.NewPool(10,
    laborer.WithPanicHandler(func(err interface{}) {
        log.Printf("Task panicked: %v", err)
        metrics.RecordPanic(err)
    }))
```

### WithLogger

```go
func WithLogger(logger Logger) Option
```

Sets a custom logger for the pool.

**Parameters:**
- `logger`: Logger implementation

**Default:** Empty logger (no output)

**Example:**

```go
import "log"

pool, _ := laborer.NewPool(10,
    laborer.WithLogger(log.Default()))
```

## Future Interface

### Get

```go
func (f Future) Get() (interface{}, error)
```

Blocks until the task completes and returns the result.

**Returns:**
- `interface{}`: Task result
- `error`: Task error or `nil`

**Example:**

```go
future, _ := pool.SubmitWithResult(task)
result, err := future.Get()
```

### GetWithTimeout

```go
func (f Future) GetWithTimeout(timeout time.Duration) (interface{}, error)
```

Waits for the task to complete with a timeout.

**Parameters:**
- `timeout`: Maximum wait time

**Returns:**
- `interface{}`: Task result (nil if timeout)
- `error`: Task error or `ErrTimeout`

**Example:**

```go
result, err := future.GetWithTimeout(5 * time.Second)
if errors.Is(err, laborer.ErrTimeout) {
    log.Println("Task timed out")
}
```

### IsDone

```go
func (f Future) IsDone() bool
```

Checks if the task has completed (non-blocking).

**Returns:**
- `bool`: `true` if completed, `false` otherwise

**Example:**

```go
if future.IsDone() {
    result, _ := future.Get()
    // Process result
}
```

## Error Handling

### Error Types

- **ErrPoolClosed**: Pool has been closed
- **ErrPoolOverload**: Pool is overloaded (non-blocking mode)
- **ErrInvalidPoolSize**: Invalid pool size (0)
- **ErrInvalidPoolExpiry**: Invalid expiry duration (negative)
- **ErrInvalidPoolFunc**: Invalid pool function (nil)
- **ErrTimeout**: Operation timed out

### Error Checking

```go
import "errors"

if err := pool.Submit(task); err != nil {
    if errors.Is(err, laborer.ErrPoolClosed) {
        // Handle closed pool
    } else if errors.Is(err, laborer.ErrPoolOverload) {
        // Handle overload
    }
}
```

## Logger Interface

### Interface Definition

```go
type Logger interface {
    Printf(format string, args ...interface{})
}
```

### Implementation Example

```go
type MyLogger struct {
    logger *zap.Logger
}

func (l *MyLogger) Printf(format string, args ...interface{}) {
    l.logger.Sugar().Infof(format, args...)
}

pool, _ := laborer.NewPool(10,
    laborer.WithLogger(&MyLogger{logger: zapLogger}))
```

### Using Standard Library Logger

```go
import "log"

pool, _ := laborer.NewPool(10,
    laborer.WithLogger(log.Default()))
```

## Complete Example

```go
package main

import (
    "fmt"
    "log"
    "time"
    
    "github.com/kawaiirei0/laborer"
)

func main() {
    // Create pool with options
    pool, err := laborer.NewPool(
        100,
        laborer.WithExpiryDuration(30 * time.Second),
        laborer.WithPreAlloc(true),
        laborer.WithNonblocking(false),
        laborer.WithPanicHandler(func(err interface{}) {
            log.Printf("Task panicked: %v", err)
        }),
        laborer.WithLogger(log.Default()),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer pool.Release()
    
    // Submit tasks
    for i := 0; i < 1000; i++ {
        i := i
        if err := pool.Submit(func() {
            fmt.Printf("Task %d executing\n", i)
            time.Sleep(100 * time.Millisecond)
        }); err != nil {
            log.Printf("Failed to submit task: %v", err)
        }
    }
    
    // Submit task with result
    future, err := pool.SubmitWithResult(func() (interface{}, error) {
        time.Sleep(time.Second)
        return "computation result", nil
    })
    if err != nil {
        log.Fatal(err)
    }
    
    // Monitor pool status
    go func() {
        ticker := time.NewTicker(5 * time.Second)
        defer ticker.Stop()
        for range ticker.C {
            log.Printf("Pool stats - Running: %d, Free: %d, Waiting: %d",
                pool.Running(), pool.Free(), pool.Waiting())
        }
    }()
    
    // Get result with timeout
    result, err := future.GetWithTimeout(10 * time.Second)
    if err != nil {
        log.Printf("Failed to get result: %v", err)
    } else {
        log.Printf("Result: %v", result)
    }
    
    // Graceful shutdown
    if err := pool.ReleaseTimeout(30 * time.Second); err != nil {
        log.Printf("Graceful shutdown failed: %v", err)
        pool.Release()
    }
}
```
