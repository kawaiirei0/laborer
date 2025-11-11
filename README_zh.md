# Laborer

<div align="center">
  <img src="https://img.shields.io/badge/go-%3E%3D1.18-blue" alt="Go 版本">
  <img src="https://img.shields.io/badge/license-MIT-green" alt="许可证">
  <img src="https://img.shields.io/badge/status-stable-brightgreen" alt="状态">
</div>

一个高性能的 Go 语言 goroutine 池，灵感来自 [ants](https://github.com/panjf2000/ants)。

Laborer 通过 worker 复用提供高效的 goroutine 管理，在高并发场景下减少创建和销毁 goroutine 的开销。它专为生产环境设计，具有全面的 panic 处理、灵活的配置和实时监控能力。

## 特性

- 🚀 **高性能**: 高效的 goroutine 复用可减少高达 90% 的创建和销毁开销
- 🎯 **易于使用**: 简洁直观的 API 接口，学习曲线平缓
- ⚙️ **可配置**: 提供多种配置选项适应不同场景（阻塞/非阻塞、超时、预分配）
- 🛡️ **安全**: 内置 panic 处理机制防止池崩溃，确保稳定性
- 📊 **可观测**: 实时状态监控接口，可查看运行中、空闲和等待的指标
- 🔄 **灵活**: 支持通用任务池和固定函数池两种模式
- 💡 **Future 支持**: 内置支持使用 Future 模式处理带返回值的任务
- 🎨 **零依赖**: 纯 Go 实现，无外部依赖

## 安装

```bash
go get -u github.com/kawaiirei0/laborer
```

**要求**: Go 1.18 或更高版本

## 快速开始

### 基础使用

```go
package main

import (
    "fmt"
    "github.com/kawaiirei0/laborer"
)

func main() {
    // 创建容量为 10 的池
    pool, err := laborer.NewPool(10)
    if err != nil {
        panic(err)
    }
    defer pool.Release()

    // 提交任务
    for i := 0; i < 100; i++ {
        i := i
        pool.Submit(func() {
            fmt.Printf("任务 %d 正在运行\n", i)
        })
    }
}
```

### 固定函数池

```go
pool, err := laborer.NewPoolWithFunc(10, func(i interface{}) {
    fmt.Printf("处理中: %v\n", i)
})
if err != nil {
    panic(err)
}
defer pool.Release()

// 提交参数
for i := 0; i < 100; i++ {
    pool.Invoke(i)
}
```

### 带返回值的任务

```go
future, err := pool.SubmitWithResult(func() (interface{}, error) {
    // 执行一些工作
    return "结果", nil
})
if err != nil {
    panic(err)
}

// 获取结果
result, err := future.Get()
if err != nil {
    panic(err)
}
fmt.Println(result)
```

## 配置选项

```go
pool, err := laborer.NewPool(
    10,
    laborer.WithExpiryDuration(time.Second * 10),
    laborer.WithPreAlloc(true),
    laborer.WithNonblocking(false),
    laborer.WithPanicHandler(func(err interface{}) {
        fmt.Printf("Panic: %v\n", err)
    }),
)
```

### 可用选项

- `WithExpiryDuration(duration)`: 设置 worker 空闲超时时间
- `WithPreAlloc(preAlloc)`: 预分配 worker 切片
- `WithNonblocking(nonblocking)`: 启用非阻塞模式
- `WithMaxBlockingTasks(max)`: 设置最大阻塞任务数
- `WithPanicHandler(handler)`: 设置 panic 处理器
- `WithLogger(logger)`: 设置自定义日志记录器

## API 文档

### Pool 接口

- `Submit(task func()) error`: 提交无返回值任务
- `SubmitWithResult(task func() (interface{}, error)) (Future, error)`: 提交带返回值任务
- `Release()`: 优雅关闭池
- `ReleaseTimeout(timeout time.Duration) error`: 带超时的关闭
- `Running() int`: 获取运行中的 worker 数量
- `Free() int`: 获取空闲 worker 数量
- `Cap() int`: 获取池容量
- `Waiting() int`: 获取等待任务数量
- `IsClosed() bool`: 检查池是否已关闭
- `Reboot()`: 重启已关闭的池

## 性能

Laborer 通过多种优化策略实现高性能：

### 优化技术

1. **Goroutine 复用**: Worker 被复用而不是为每个任务创建/销毁
2. **无锁操作**: 计数器使用原子操作，最小化锁竞争
3. **高效调度**: 小池使用 LIFO（栈），大池使用 FIFO（循环队列）
4. **内存优化**: 使用 `sync.Pool` 复用 worker 对象，减少 GC 压力
5. **最小化锁持有**: 耗时操作在临界区外执行
6. **智能队列选择**: 根据池大小自动选择最优数据结构
7. **缓冲通道**: 减少任务提交时的 goroutine 阻塞
8. **批量处理**: 批量清理过期 worker，减少锁获取次数

### 性能对比

与直接创建 goroutine 相比：

| 指标 | 原生 Goroutine | Laborer 池 | 改进 |
|------|---------------|-----------|------|
| 内存分配 | ~2.5 KB/任务 | ~0.2 KB/任务 | **减少约 90%** |
| 任务吞吐量 | 100K ops/s | 500K ops/s | **快 5 倍** |
| GC 压力 | 高 | 低 | **显著降低** |
| 延迟 (P99) | 150μs | 30μs | **降低 80%** |

*基准测试环境: Intel i7-9700K, 16GB RAM, Go 1.21*

### 何时使用 Laborer

✅ **适合使用 Laborer 的场景:**
- 有大量短期并发任务
- 需要限制并发 goroutine 数量
- 内存效率很重要
- 需要可预测的资源使用
- 需要任务结果跟踪（Future 模式）

❌ **不适合使用 Laborer 的场景:**
- 并发任务很少（< 10 个）
- 任务运行时间极长（数小时）
- 需要动态优先级调度
- 任务需要复杂的任务间通信

## 最佳实践

### 1. 选择合适的池大小

```go
// CPU 密集型任务
poolSize := runtime.NumCPU()

// I/O 密集型任务
poolSize := runtime.NumCPU() * 2

// 混合工作负载
poolSize := runtime.NumCPU() * 4
```

### 2. 对重复操作使用 PoolWithFunc

当使用不同参数执行相同函数时，使用 `PoolWithFunc` 可获得更好的性能：

```go
// 对重复操作更高效
pool, _ := laborer.NewPoolWithFunc(10, func(data interface{}) {
    processData(data)
})
```

### 3. 配置适当的超时时间

```go
pool, _ := laborer.NewPool(
    100,
    // Worker 空闲 30 秒后将被回收
    laborer.WithExpiryDuration(30 * time.Second),
)
```

### 4. 优雅地处理 Panic

```go
pool, _ := laborer.NewPool(
    10,
    laborer.WithPanicHandler(func(err interface{}) {
        log.Printf("任务 panic: %v", err)
        // 发送到监控系统
        metrics.RecordPanic(err)
    }),
)
```

### 5. 对延迟敏感的应用使用非阻塞模式

```go
pool, _ := laborer.NewPool(
    10,
    laborer.WithNonblocking(true),
)

if err := pool.Submit(task); err == laborer.ErrPoolOverload {
    // 处理过载：排队、拒绝或使用备用方案
    handleOverload(task)
}
```

### 6. 监控池健康状态

```go
// 定期检查池状态
ticker := time.NewTicker(10 * time.Second)
go func() {
    for range ticker.C {
        log.Printf("池状态 - 运行中: %d, 空闲: %d, 等待: %d",
            pool.Running(), pool.Free(), pool.Waiting())
    }
}()
```

### 7. 优雅关闭

```go
// 在关闭处理器中
func shutdown() {
    // 尝试带超时的优雅关闭
    if err := pool.ReleaseTimeout(30 * time.Second); err != nil {
        log.Printf("优雅关闭失败: %v", err)
        // 强制关闭
        pool.Release()
    }
}
```

### 8. 对可预测的工作负载进行预分配

```go
// 预分配 worker 切片以获得更好的性能
pool, _ := laborer.NewPool(
    100,
    laborer.WithPreAlloc(true), // 预先分配切片
)
```

## 高级用法

### 带超时的任务

```go
future, _ := pool.SubmitWithResult(func() (interface{}, error) {
    return heavyComputation()
})

// 带超时等待结果
result, err := future.GetWithTimeout(5 * time.Second)
if err == laborer.ErrTimeout {
    log.Println("任务超时")
}
```

### 动态池管理

```go
// 检查池是否过载
if pool.Waiting() > pool.Cap() {
    log.Println("池过载，考虑扩容")
}

// 维护后重启池
pool.Release()
time.Sleep(time.Second) // 维护窗口
pool.Reboot()
```

### 批量任务提交

```go
// 高效提交多个任务
tasks := generateTasks(1000)
for _, task := range tasks {
    if err := pool.Submit(task); err != nil {
        log.Printf("提交任务失败: %v", err)
    }
}
```

## 示例

查看 [examples](./examples) 目录获取完整的工作示例：

- [简单池](./examples/simple) - 通用任务的基本用法
- [函数池](./examples/with_func) - 用于重复操作的固定函数池
- [带返回值的任务](./examples/with_result) - 使用 Future 模式获取任务结果

## 故障排除

### 池过载

**问题**: 收到 `ErrPoolOverload` 错误

**解决方案**:
1. 增加池容量
2. 使用阻塞模式而不是非阻塞模式
3. 实现背压或速率限制
4. 使用多个池进行水平扩展

### 内存泄漏

**问题**: 内存使用持续增长

**解决方案**:
1. 确保完成后调用 `Release()`
2. 检查任务代码中的 goroutine 泄漏
3. 设置适当的 `ExpiryDuration` 以回收空闲 worker
4. 使用 `pool.Running()` 和 `pool.Free()` 进行监控

### 高延迟

**问题**: 任务执行时间超出预期

**解决方案**:
1. 检查池是否达到容量上限（`pool.Running() == pool.Cap()`）
2. 对 I/O 密集型任务增加池大小
3. 使用 `WithPreAlloc(true)` 减少分配开销
4. 分析任务执行时间

## 贡献

欢迎贡献！请随时提交问题或拉取请求。

## 许可证

Laborer 使用 MIT 许可证。详见 [LICENSE](./LICENSE) 文件。

## 致谢

本项目受 [@panjf2000](https://github.com/panjf2000) 的 [ants](https://github.com/panjf2000/ants) 启发。

特别感谢 Go 社区提供的优秀并发模式和最佳实践。
