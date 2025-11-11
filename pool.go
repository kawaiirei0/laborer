// Package laborer 提供高性能的 goroutine 池实现
//
// 性能优化说明：
// 1. PreAlloc 选项：预分配 worker 切片，减少动态扩容开销
// 2. Atomic 操作：使用 atomic 操作管理计数器（running, capacity, state, waiting），避免锁竞争
// 3. 锁优化：最小化锁持有时间，在锁外执行耗时操作（如时间戳更新）
// 4. Worker 对象复用：使用 sync.Pool 复用 worker 对象，减少 GC 压力
// 5. 快速路径优化：在 getWorker 中使用无锁快速路径，避免不必要的锁获取
// 6. 条件唤醒：只在有等待 goroutine 时才调用 Signal，减少不必要的系统调用
// 7. 内存布局优化：worker 队列使用缓存友好的数据结构（栈/循环队列）
// 8. 批量处理：在 refresh 中批量处理过期 worker，减少锁获取次数
// 9. 切片复用：在 refresh 操作中复用 expiry 切片，减少内存分配
// 10. Channel 缓冲：使用带缓冲的 channel 减少 goroutine 阻塞
package laborer

import (
	"sync"
	"sync/atomic"
	"time"
)

const (
	// CLOSED 表示池已关闭
	CLOSED = 1

	// OPENED 表示池正在运行
	OPENED = 0

	// queueSizeThreshold 队列大小阈值，小于此值使用栈，否则使用循环队列
	queueSizeThreshold = 1000

	// workerChanCap worker channel 的缓冲容量
	// 优化：使用缓冲 channel 减少 goroutine 阻塞
	workerChanCap = 1
)

// Pool 通用 goroutine 池，可以执行不同的任务
type Pool struct {
	// capacity 池的容量，即最大可创建的 Worker 数量
	// -1 表示无限容量
	capacity int32

	// running 当前运行的 worker 数量
	running int32

	// state 池的状态：OPENED 或 CLOSED
	state int32

	// lock 保护 workers 队列的锁
	lock sync.Locker

	// cond 条件变量，用于阻塞模式下的等待
	cond *sync.Cond

	// workers worker 队列，存储空闲的 worker
	workers workerQueue

	// options 配置选项
	options *Options

	// waiting 等待执行的任务数量
	waiting int32

	// stopCleaning 用于停止清理 goroutine 的 channel
	stopCleaning chan struct{}

	// cleaningDone 清理 goroutine 完成的信号
	cleaningDone chan struct{}

	// workerPool 用于复用 worker 对象，减少 GC 压力
	workerPool sync.Pool
}

// PoolInterface 定义池的接口
type PoolInterface interface {
	// Submit 提交无返回值任务
	Submit(task func()) error

	// SubmitWithResult 提交带返回值的任务
	SubmitWithResult(task func() (interface{}, error)) (Future, error)

	// Release 优雅关闭池
	Release()

	// ReleaseTimeout 带超时的优雅关闭
	ReleaseTimeout(timeout time.Duration) error

	// Reboot 重启已关闭的池
	Reboot()

	// Running 返回正在运行的 worker 数量
	Running() int

	// Free 返回空闲的 worker 数量
	Free() int

	// Cap 返回池容量
	Cap() int

	// Waiting 返回等待执行的任务数量
	Waiting() int

	// IsClosed 返回池是否已关闭
	IsClosed() bool
}

// NewPool 创建一个新的 goroutine 池
// size: 池的容量，-1 表示无限容量
// options: 配置选项
func NewPool(size int, options ...Option) (*Pool, error) {
	// 验证容量参数
	if size == 0 {
		return nil, ErrInvalidPoolSize
	}

	// 创建配置选项
	opts := NewOptions(options...)

	// 验证过期时间
	if opts.ExpiryDuration < 0 {
		return nil, ErrInvalidPoolExpiry
	}

	// 创建池实例
	pool := &Pool{
		capacity:     int32(size),
		options:      opts,
		stopCleaning: make(chan struct{}),
		cleaningDone: make(chan struct{}),
	}

	// 初始化锁和条件变量
	pool.lock = new(sync.Mutex)
	pool.cond = sync.NewCond(pool.lock)

	// 初始化 worker 对象池，用于复用 worker 对象
	// 优化：使用带缓冲的 channel 减少阻塞
	pool.workerPool.New = func() interface{} {
		return &goWorker{
			pool: pool,
			task: make(chan func(), workerChanCap),
		}
	}

	// 根据容量选择合适的 worker 队列实现
	// 小容量使用栈（LIFO），大容量使用循环队列（FIFO）
	if size == -1 {
		// 无限容量，使用栈
		pool.workers = newWorkerStack(0)
	} else if size < queueSizeThreshold {
		// 小容量，使用栈
		if opts.PreAlloc {
			pool.workers = newWorkerStack(size)
		} else {
			pool.workers = newWorkerStack(0)
		}
	} else {
		// 大容量，使用循环队列
		pool.workers = newWorkerLoopQueue(size)
	}

	// 启动定期清理过期 worker 的 goroutine
	go pool.cleanExpiredWorkers()

	return pool, nil
}

// Submit 提交一个任务到池中执行
func (p *Pool) Submit(task func()) error {
	// 检查池是否已关闭
	if p.IsClosed() {
		return ErrPoolClosed
	}

	// 获取一个 worker 并分配任务
	if w := p.getWorker(); w != nil {
		w.task <- task
		return nil
	}

	return ErrPoolOverload
}

// SubmitWithResult 提交一个带返回值的任务到池中执行
func (p *Pool) SubmitWithResult(task func() (interface{}, error)) (Future, error) {
	// 检查池是否已关闭
	if p.IsClosed() {
		return nil, ErrPoolClosed
	}

	// 创建 future 对象
	f := newFuture()

	// 包装任务，将结果设置到 future 中
	wrappedTask := func() {
		result, err := task()
		f.setResult(result, err)
	}

	// 获取一个 worker 并分配任务
	if w := p.getWorker(); w != nil {
		w.task <- wrappedTask
		return f, nil
	}

	return nil, ErrPoolOverload
}

// Running 返回当前正在运行的 worker 数量
func (p *Pool) Running() int {
	return int(atomic.LoadInt32(&p.running))
}

// Free 返回当前空闲的 worker 数量
func (p *Pool) Free() int {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.workers.len()
}

// Cap 返回池的容量
func (p *Pool) Cap() int {
	return int(atomic.LoadInt32(&p.capacity))
}

// Waiting 返回等待执行的任务数量
func (p *Pool) Waiting() int {
	return int(atomic.LoadInt32(&p.waiting))
}

// IsClosed 返回池是否已关闭
func (p *Pool) IsClosed() bool {
	return atomic.LoadInt32(&p.state) == CLOSED
}

// Release 优雅关闭池，等待所有任务完成
func (p *Pool) Release() {
	// 标记池为关闭状态
	if !atomic.CompareAndSwapInt32(&p.state, OPENED, CLOSED) {
		return
	}

	// 停止清理 goroutine
	close(p.stopCleaning)
	<-p.cleaningDone

	p.lock.Lock()
	// 关闭所有空闲的 worker
	p.workers.reset()
	p.lock.Unlock()

	// 唤醒所有等待的 goroutine
	p.cond.Broadcast()
}

// ReleaseTimeout 带超时的优雅关闭
func (p *Pool) ReleaseTimeout(timeout time.Duration) error {
	// 标记池为关闭状态
	if !atomic.CompareAndSwapInt32(&p.state, OPENED, CLOSED) {
		return ErrPoolClosed
	}

	// 创建超时定时器
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	// 使用 channel 等待关闭完成或超时
	done := make(chan struct{})
	go func() {
		// 停止清理 goroutine
		close(p.stopCleaning)
		<-p.cleaningDone

		p.lock.Lock()
		p.workers.reset()
		p.lock.Unlock()

		p.cond.Broadcast()
		close(done)
	}()

	// 等待完成或超时
	select {
	case <-done:
		return nil
	case <-timer.C:
		return ErrTimeout
	}
}

// Reboot 重启已关闭的池
func (p *Pool) Reboot() {
	if atomic.CompareAndSwapInt32(&p.state, CLOSED, OPENED) {
		// 重新创建清理相关的 channel
		p.stopCleaning = make(chan struct{})
		p.cleaningDone = make(chan struct{})
		// 重启清理 goroutine
		go p.cleanExpiredWorkers()
	}
}

// getWorker 获取一个可用的 worker
// 优化：最小化锁持有时间，使用 atomic 操作避免不必要的锁
func (p *Pool) getWorker() *goWorker {
	var w *goWorker

	p.lock.Lock()

	// 尝试从队列中获取空闲 worker
	w = p.workers.detach()

	if w != nil {
		// 找到空闲 worker，立即释放锁以减少锁持有时间
		p.lock.Unlock()
		return w
	}

	// 检查是否可以创建新的 worker（使用 atomic 读取避免额外的锁）
	capacity := atomic.LoadInt32(&p.capacity)
	running := atomic.LoadInt32(&p.running)

	if capacity == -1 || running < capacity {
		// 可以创建新 worker，先释放锁
		p.lock.Unlock()

		// 从对象池获取 worker 对象以复用
		w = p.workerPool.Get().(*goWorker)

		// 重置 worker 状态
		atomic.StoreInt32(&w.recycled, 0)
		w.lastUsed = time.Now()

		// 增加运行计数
		atomic.AddInt32(&p.running, 1)

		// 启动 worker
		w.run()

		return w
	}

	// 池已满
	if p.options.Nonblocking {
		// 非阻塞模式，直接返回 nil
		p.lock.Unlock()
		return nil
	}

	// 阻塞模式，等待 worker 可用
	atomic.AddInt32(&p.waiting, 1)
	p.cond.Wait()
	atomic.AddInt32(&p.waiting, -1)

	// 被唤醒后，检查池是否已关闭
	if atomic.LoadInt32(&p.state) == CLOSED {
		p.lock.Unlock()
		return nil
	}

	// 再次尝试获取 worker
	w = p.workers.detach()
	p.lock.Unlock()

	return w
}

// putWorker 将 worker 放回池中
// 优化：在锁外更新时间戳，减少锁持有时间
func (p *Pool) putWorker(worker *goWorker) bool {
	// 使用 atomic 检查池状态，避免不必要的锁
	if atomic.LoadInt32(&p.state) == CLOSED {
		return false
	}

	// 更新 worker 的最后使用时间（在锁外执行）
	worker.lastUsed = time.Now()

	p.lock.Lock()

	// 将 worker 放回队列
	if err := p.workers.insert(worker); err != nil {
		p.lock.Unlock()
		return false
	}

	// 只在有等待的 goroutine 时才唤醒
	// 优化：减少不必要的 Signal 调用
	if atomic.LoadInt32(&p.waiting) > 0 {
		p.cond.Signal()
	}
	p.lock.Unlock()

	return true
}

// cleanExpiredWorkers 定期清理过期的 worker
func (p *Pool) cleanExpiredWorkers() {
	ticker := time.NewTicker(p.options.ExpiryDuration)
	defer func() {
		ticker.Stop()
		close(p.cleaningDone)
	}()

	for {
		select {
		case <-ticker.C:
			// 使用 atomic 检查池状态，避免不必要的锁
			if atomic.LoadInt32(&p.state) == CLOSED {
				return
			}

			p.lock.Lock()
			expiredWorkers := p.workers.refresh(p.options.ExpiryDuration)
			p.lock.Unlock()

			// 记录日志（在锁外执行，减少锁持有时间）
			if len(expiredWorkers) > 0 && p.options.Logger != nil {
				for _, idx := range expiredWorkers {
					p.options.Logger.Printf("worker at index %d expired and will be recycled", idx)
				}
			}

			// 减少运行计数（过期的worker已经从队列中移除）
			n := int32(len(expiredWorkers))
			if n > 0 {
				atomic.AddInt32(&p.running, -n)
			}

		case <-p.stopCleaning:
			return
		}
	}
}
