package laborer

import (
	"sync"
	"sync/atomic"
	"time"
)

// goWorkerWithFunc 表示执行固定函数的 worker
type goWorkerWithFunc struct {
	// 所属的池
	pool *PoolWithFunc

	// 参数 channel
	args chan interface{}

	// 最后使用时间（用于超时回收）
	lastUsed time.Time

	// 回收标志
	recycled int32
}

// PoolWithFunc 函数池，用于执行相同类型的任务
// 相比通用池，函数池减少了函数指针的传递，提高了性能
type PoolWithFunc struct {
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
	workers workerQueueWithFunc

	// poolFunc 池中所有 worker 执行的固定函数
	poolFunc func(interface{})

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

// PoolWithFuncInterface 定义函数池的接口
type PoolWithFuncInterface interface {
	// Invoke 提交参数到固定函数执行
	Invoke(args interface{}) error

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

// NewPoolWithFunc 创建一个新的函数池
// size: 池的容量，-1 表示无限容量
// pf: 池中所有 worker 执行的固定函数
// options: 配置选项
func NewPoolWithFunc(size int, pf func(interface{}), options ...Option) (*PoolWithFunc, error) {
	// 验证容量参数
	if size == 0 {
		return nil, ErrInvalidPoolSize
	}

	// 验证函数参数
	if pf == nil {
		return nil, ErrInvalidPoolFunc
	}

	// 创建配置选项
	opts := NewOptions(options...)

	// 验证过期时间
	if opts.ExpiryDuration < 0 {
		return nil, ErrInvalidPoolExpiry
	}

	// 创建池实例
	pool := &PoolWithFunc{
		capacity:     int32(size),
		poolFunc:     pf,
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
		return &goWorkerWithFunc{
			pool: pool,
			args: make(chan interface{}, workerChanCap),
		}
	}

	// 根据容量选择合适的 worker 队列实现
	if size == -1 {
		// 无限容量，使用栈
		pool.workers = newWorkerStackWithFunc(0)
	} else if size < queueSizeThreshold {
		// 小容量，使用栈
		if opts.PreAlloc {
			pool.workers = newWorkerStackWithFunc(size)
		} else {
			pool.workers = newWorkerStackWithFunc(0)
		}
	} else {
		// 大容量，使用循环队列
		pool.workers = newWorkerLoopQueueWithFunc(size)
	}

	// 启动定期清理过期 worker 的 goroutine
	go pool.cleanExpiredWorkers()

	return pool, nil
}

// Invoke 提交参数到固定函数执行
func (p *PoolWithFunc) Invoke(args interface{}) error {
	// 检查池是否已关闭
	if p.IsClosed() {
		return ErrPoolClosed
	}

	// 获取一个 worker 并分配参数
	if w := p.getWorker(); w != nil {
		w.args <- args
		return nil
	}

	return ErrPoolOverload
}

// Running 返回当前正在运行的 worker 数量
func (p *PoolWithFunc) Running() int {
	return int(atomic.LoadInt32(&p.running))
}

// Free 返回当前空闲的 worker 数量
func (p *PoolWithFunc) Free() int {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.workers.len()
}

// Cap 返回池的容量
func (p *PoolWithFunc) Cap() int {
	return int(atomic.LoadInt32(&p.capacity))
}

// Waiting 返回等待执行的任务数量
func (p *PoolWithFunc) Waiting() int {
	return int(atomic.LoadInt32(&p.waiting))
}

// IsClosed 返回池是否已关闭
func (p *PoolWithFunc) IsClosed() bool {
	return atomic.LoadInt32(&p.state) == CLOSED
}

// Release 优雅关闭池，等待所有任务完成
func (p *PoolWithFunc) Release() {
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
func (p *PoolWithFunc) ReleaseTimeout(timeout time.Duration) error {
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
func (p *PoolWithFunc) Reboot() {
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
func (p *PoolWithFunc) getWorker() *goWorkerWithFunc {
	var w *goWorkerWithFunc

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
		w = p.workerPool.Get().(*goWorkerWithFunc)

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
func (p *PoolWithFunc) putWorker(worker *goWorkerWithFunc) bool {
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
func (p *PoolWithFunc) cleanExpiredWorkers() {
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

// run 启动 worker 的主循环，处理参数执行
// 包含 panic 恢复机制，确保单个任务的 panic 不会导致整个池崩溃
func (w *goWorkerWithFunc) run() {
	go func() {
		defer func() {
			// 减少运行中的 worker 计数
			atomic.AddInt32(&w.pool.running, -1)

			// 处理 panic
			if p := recover(); p != nil {
				if w.pool.options.PanicHandler != nil {
					w.pool.options.PanicHandler(p)
				} else if w.pool.options.Logger != nil {
					w.pool.options.Logger.Printf("worker exits from panic: %v", p)
				}
			}

			// 通知池 worker 已退出
			w.pool.cond.Signal()
		}()

		// 主循环：持续接收和执行参数
		for args := range w.args {
			if args == nil {
				// nil 参数表示 worker 应该退出
				return
			}

			// 执行固定函数
			w.pool.poolFunc(args)

			// 任务完成后，将 worker 放回池中以供复用
			if ok := w.pool.putWorker(w); !ok {
				// 如果放回失败（池已关闭），退出循环
				return
			}
		}
	}()
}

// updateLastUsed 更新 worker 的最后使用时间
// 用于超时回收机制
func (w *goWorkerWithFunc) updateLastUsed() {
	w.lastUsed = time.Now()
}

// isRecycled 检查 worker 是否已被回收
func (w *goWorkerWithFunc) isRecycled() bool {
	return atomic.LoadInt32(&w.recycled) == 1
}

// recycle 标记 worker 为已回收状态
func (w *goWorkerWithFunc) recycle() {
	atomic.StoreInt32(&w.recycled, 1)
}

// finish 结束 worker，关闭参数 channel
func (w *goWorkerWithFunc) finish() {
	w.recycle()
	close(w.args)
}
