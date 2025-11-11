# 性能优化总结

本文档总结了 Laborer goroutine 池中实现的所有性能优化措施。

## 1. PreAlloc 选项 - 预分配 Worker 切片

**位置**: `pool.go`, `pool_func.go`, `worker_stack.go`

**实现**:
- 在 `Options` 中提供 `PreAlloc` 配置选项
- 当 `PreAlloc=true` 时，在创建池时预分配 worker 队列的切片容量
- 避免动态扩容带来的内存分配和拷贝开销

**代码示例**:
```go
if opts.PreAlloc {
    pool.workers = newWorkerStack(size)
} else {
    pool.workers = newWorkerStack(0)
}
```

**性能收益**: 减少内存分配次数，降低 GC 压力

## 2. Atomic 操作优化计数器访问

**位置**: `pool.go`, `pool_func.go`

**实现**:
- 使用 `atomic.LoadInt32` 和 `atomic.AddInt32` 管理关键计数器
- 优化的计数器包括：
  - `running`: 当前运行的 worker 数量
  - `capacity`: 池容量
  - `state`: 池状态（OPENED/CLOSED）
  - `waiting`: 等待执行的任务数量

**代码示例**:
```go
// 使用 atomic 检查池状态，避免不必要的锁
if atomic.LoadInt32(&p.state) == CLOSED {
    return false
}
```

**性能收益**: 避免锁竞争，提高并发性能

## 3. 优化锁的使用范围

**位置**: `pool.go`, `pool_func.go`

**实现**:
- 在 `getWorker()` 中，找到空闲 worker 后立即释放锁
- 在 `putWorker()` 中，在锁外更新时间戳
- 只在有等待 goroutine 时才调用 `Signal()`，减少不必要的系统调用

**代码示例**:
```go
// 更新 worker 的最后使用时间（在锁外执行）
worker.lastUsed = time.Now()

p.lock.Lock()
// ... 队列操作 ...

// 只在有等待的 goroutine 时才唤醒
if atomic.LoadInt32(&p.waiting) > 0 {
    p.cond.Signal()
}
p.lock.Unlock()
```

**性能收益**: 减少锁持有时间，降低锁竞争

## 4. Worker 对象复用

**位置**: `pool.go`, `pool_func.go`

**实现**:
- 使用 `sync.Pool` 复用 worker 对象
- 在 worker 完成任务后，将其放回对象池而不是销毁
- 创建新 worker 时从对象池获取，避免重复分配

**代码示例**:
```go
// 初始化 worker 对象池
pool.workerPool.New = func() interface{} {
    return &goWorker{
        pool: pool,
        task: make(chan func(), workerChanCap),
    }
}

// 从对象池获取 worker
w = p.workerPool.Get().(*goWorker)
```

**性能收益**: 减少内存分配和 GC 压力

## 5. Worker 队列的内存布局优化

**位置**: `worker_stack.go`, `worker_loop_queue.go`

**实现**:
- 将常用字段放在结构体前面，提高缓存命中率
- 使用 LIFO（栈）策略优先使用最近使用的 worker（缓存友好）
- 根据容量大小选择合适的数据结构：
  - 小容量（< 1000）：使用栈
  - 大容量：使用循环队列

**代码示例**:
```go
type workerStack struct {
    items  []*goWorker  // 常用字段放前面
    size   int
    expiry []*goWorker
}
```

**性能收益**: 提高 CPU 缓存命中率，减少内存访问延迟

## 6. 批量处理和切片复用

**位置**: `worker_stack.go`, `worker_loop_queue.go`

**实现**:
- 在 `refresh()` 方法中复用 `expiry` 切片，避免重复分配
- 批量处理过期 worker，减少锁获取次数
- 使用 `copy()` 一次性移动未过期的 worker

**代码示例**:
```go
// 复用 expiry 切片，避免重新分配
if cap(wq.expiry) >= index {
    wq.expiry = wq.expiry[:index]
} else {
    wq.expiry = make([]*goWorker, index)
}

// 批量处理
for i, w := range wq.expiry {
    w.finish()
    wq.expiry[i] = nil // 清空引用，帮助 GC
}
```

**性能收益**: 减少内存分配，降低 GC 压力

## 7. Channel 缓冲优化

**位置**: `pool.go`, `pool_func.go`

**实现**:
- 使用带缓冲的 channel 传递任务
- 缓冲大小设置为 1，平衡内存使用和性能

**代码示例**:
```go
const workerChanCap = 1

task: make(chan func(), workerChanCap)
```

**性能收益**: 减少 goroutine 阻塞，提高吞吐量

## 8. 避免内存泄漏

**位置**: `worker_stack.go`, `worker_loop_queue.go`

**实现**:
- 在移除 worker 时，显式将切片元素设置为 `nil`
- 在 `detach()` 和 `reset()` 方法中清空引用

**代码示例**:
```go
w := wq.items[l-1]
wq.items[l-1] = nil // 避免内存泄漏
wq.items = wq.items[:l-1]
```

**性能收益**: 帮助 GC 及时回收内存，避免内存泄漏

## 9. 延迟分配策略

**位置**: `worker_loop_queue.go`

**实现**:
- 在 `refresh()` 中，只在确实有过期 worker 时才分配索引切片
- 使用延迟分配减少不必要的内存分配

**代码示例**:
```go
if indices == nil {
    // 延迟分配，只在有过期 worker 时才分配
    indices = make([]int, 0, 8)
}
```

**性能收益**: 减少内存分配，提高性能

## 10. 清理 goroutine 优化

**位置**: `pool.go`, `pool_func.go`

**实现**:
- 使用 atomic 操作检查池状态，避免不必要的锁
- 在锁外执行日志记录等耗时操作
- 批量更新 running 计数

**代码示例**:
```go
// 使用 atomic 检查池状态，避免不必要的锁
if atomic.LoadInt32(&p.state) == CLOSED {
    return
}

p.lock.Lock()
expiredWorkers := p.workers.refresh(p.options.ExpiryDuration)
p.lock.Unlock()

// 记录日志（在锁外执行，减少锁持有时间）
if len(expiredWorkers) > 0 && p.options.Logger != nil {
    // ...
}
```

**性能收益**: 减少锁持有时间，提高并发性能

## 性能测试建议

为了验证这些优化的效果，建议进行以下性能测试：

1. **基准测试**: 对比优化前后的吞吐量和延迟
2. **内存分析**: 使用 `go test -bench . -benchmem` 分析内存分配
3. **CPU 分析**: 使用 `pprof` 分析 CPU 热点
4. **压力测试**: 在高并发场景下测试稳定性
5. **对比测试**: 与原生 goroutine 和其他池实现对比

## 总结

通过以上 10 项优化措施，Laborer goroutine 池实现了：
- 更低的内存分配和 GC 压力
- 更少的锁竞争和更高的并发性能
- 更好的 CPU 缓存利用率
- 更稳定的长时间运行表现

这些优化遵循了 Go 语言的最佳实践，充分利用了 Go 的并发特性和内存模型。
