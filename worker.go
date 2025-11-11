package laborer

import (
	"sync/atomic"
	"time"
)

// goWorker 表示一个执行任务的 worker
type goWorker struct {
	// 所属的池
	pool *Pool

	// 任务 channel
	task chan func()

	// 最后使用时间（用于超时回收）
	lastUsed time.Time

	// 回收标志
	recycled int32
}

// run 启动 worker 的主循环，处理任务执行
// 包含 panic 恢复机制，确保单个任务的 panic 不会导致整个池崩溃
func (w *goWorker) run() {
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

		// 主循环：持续接收和执行任务
		for task := range w.task {
			if task == nil {
				// nil 任务表示 worker 应该退出
				return
			}

			// 执行任务
			task()

			// 任务完成后，将 worker 放回池中以供复用
			if ok := w.pool.putWorker(w); !ok {
				// 如果放回失败（池已关闭），退出循环
				return
			}
		}
	}()
}

// isRecycled 检查 worker 是否已被回收
func (w *goWorker) isRecycled() bool {
	return atomic.LoadInt32(&w.recycled) == 1
}

// recycle 标记 worker 为已回收状态
func (w *goWorker) recycle() {
	atomic.StoreInt32(&w.recycled, 1)
}

// finish 结束 worker，关闭任务 channel
func (w *goWorker) finish() {
	w.recycle()
	close(w.task)
}
