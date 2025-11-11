package laborer

import "time"

// loopQueue 使用循环队列（FIFO）结构实现 worker 队列
// 适用于大容量场景，提供高效的入队和出队操作
// 内存布局优化：将常用字段和布尔标志放在一起，提高缓存命中率
type loopQueue struct {
	items  []*goWorker
	head   int
	tail   int
	size   int
	isFull bool
	expiry []*goWorker
}

// newWorkerLoopQueue 创建一个新的循环队列
// 预分配固定大小的数组，避免动态扩容
func newWorkerLoopQueue(size int) *loopQueue {
	return &loopQueue{
		items: make([]*goWorker, size),
		size:  size,
	}
}

// len 返回队列中的 worker 数量
func (wq *loopQueue) len() int {
	if wq.isFull {
		return wq.size
	}

	if wq.tail >= wq.head {
		return wq.tail - wq.head
	}

	return wq.size - wq.head + wq.tail
}

// isEmpty 检查队列是否为空
func (wq *loopQueue) isEmpty() bool {
	return wq.head == wq.tail && !wq.isFull
}

// insert 将 worker 插入队列尾部
func (wq *loopQueue) insert(worker *goWorker) error {
	if wq.isFull {
		return ErrPoolOverload
	}

	wq.items[wq.tail] = worker
	wq.tail++

	if wq.tail == wq.size {
		wq.tail = 0
	}

	if wq.tail == wq.head {
		wq.isFull = true
	}

	return nil
}

// detach 从队列头部取出一个 worker
func (wq *loopQueue) detach() *goWorker {
	if wq.isEmpty() {
		return nil
	}

	w := wq.items[wq.head]
	wq.items[wq.head] = nil // 避免内存泄漏
	wq.head++

	if wq.head == wq.size {
		wq.head = 0
	}

	wq.isFull = false

	return w
}

// refresh 清理过期的 worker
// 从队列头部开始检查，移除所有超过 duration 时间未使用的 worker
// 返回被清理的 worker 索引列表
// 优化：减少内存分配，批量处理过期 worker
func (wq *loopQueue) refresh(duration time.Duration) []int {
	if wq.isEmpty() {
		return nil
	}

	expiryTime := time.Now().Add(-duration)

	// 复用 expiry 切片
	if cap(wq.expiry) > 0 {
		wq.expiry = wq.expiry[:0]
	} else {
		wq.expiry = make([]*goWorker, 0, 8)
	}

	var indices []int
	expiredCount := 0

	// 从头部开始检查过期的 worker
	for !wq.isEmpty() {
		w := wq.items[wq.head]
		if w == nil || w.lastUsed.After(expiryTime) {
			break
		}

		if indices == nil {
			// 延迟分配，只在有过期 worker 时才分配
			indices = make([]int, 0, 8)
		}

		indices = append(indices, wq.head)
		wq.expiry = append(wq.expiry, w)
		wq.items[wq.head] = nil
		wq.head++

		if wq.head == wq.size {
			wq.head = 0
		}

		wq.isFull = false
		expiredCount++
	}

	// 关闭过期的 worker（批量处理）
	if expiredCount > 0 {
		for i, w := range wq.expiry {
			w.finish()
			wq.expiry[i] = nil // 清空引用，帮助 GC
		}
	}

	return indices
}

// reset 重置队列，清空所有 worker
func (wq *loopQueue) reset() {
	if wq.isEmpty() {
		return
	}

	// 关闭并清空所有元素
	if wq.head < wq.tail {
		for i := wq.head; i < wq.tail; i++ {
			if wq.items[i] != nil {
				wq.items[i].finish()
			}
			wq.items[i] = nil
		}
	} else {
		for i := wq.head; i < wq.size; i++ {
			if wq.items[i] != nil {
				wq.items[i].finish()
			}
			wq.items[i] = nil
		}
		for i := 0; i < wq.tail; i++ {
			if wq.items[i] != nil {
				wq.items[i].finish()
			}
			wq.items[i] = nil
		}
	}

	wq.head = 0
	wq.tail = 0
	wq.isFull = false
}

// loopQueueWithFunc 使用循环队列（FIFO）结构实现函数池 worker 队列
// 适用于大容量场景，提供高效的入队和出队操作
// 内存布局优化：将常用字段和布尔标志放在一起，提高缓存命中率
type loopQueueWithFunc struct {
	items  []*goWorkerWithFunc
	head   int
	tail   int
	size   int
	isFull bool
	expiry []*goWorkerWithFunc
}

// newWorkerLoopQueueWithFunc 创建一个新的函数池循环队列
// 预分配固定大小的数组，避免动态扩容
func newWorkerLoopQueueWithFunc(size int) *loopQueueWithFunc {
	return &loopQueueWithFunc{
		items: make([]*goWorkerWithFunc, size),
		size:  size,
	}
}

// len 返回队列中的 worker 数量
func (wq *loopQueueWithFunc) len() int {
	if wq.isFull {
		return wq.size
	}

	if wq.tail >= wq.head {
		return wq.tail - wq.head
	}

	return wq.size - wq.head + wq.tail
}

// isEmpty 检查队列是否为空
func (wq *loopQueueWithFunc) isEmpty() bool {
	return wq.head == wq.tail && !wq.isFull
}

// insert 将 worker 插入队列尾部
func (wq *loopQueueWithFunc) insert(worker *goWorkerWithFunc) error {
	if wq.isFull {
		return ErrPoolOverload
	}

	wq.items[wq.tail] = worker
	wq.tail++

	if wq.tail == wq.size {
		wq.tail = 0
	}

	if wq.tail == wq.head {
		wq.isFull = true
	}

	return nil
}

// detach 从队列头部取出一个 worker
func (wq *loopQueueWithFunc) detach() *goWorkerWithFunc {
	if wq.isEmpty() {
		return nil
	}

	w := wq.items[wq.head]
	wq.items[wq.head] = nil // 避免内存泄漏
	wq.head++

	if wq.head == wq.size {
		wq.head = 0
	}

	wq.isFull = false

	return w
}

// refresh 清理过期的 worker
// 从队列头部开始检查，移除所有超过 duration 时间未使用的 worker
// 返回被清理的 worker 索引列表
// 优化：减少内存分配，批量处理过期 worker
func (wq *loopQueueWithFunc) refresh(duration time.Duration) []int {
	if wq.isEmpty() {
		return nil
	}

	expiryTime := time.Now().Add(-duration)

	// 复用 expiry 切片
	if cap(wq.expiry) > 0 {
		wq.expiry = wq.expiry[:0]
	} else {
		wq.expiry = make([]*goWorkerWithFunc, 0, 8)
	}

	var indices []int
	expiredCount := 0

	// 从头部开始检查过期的 worker
	for !wq.isEmpty() {
		w := wq.items[wq.head]
		if w == nil || w.lastUsed.After(expiryTime) {
			break
		}

		if indices == nil {
			// 延迟分配，只在有过期 worker 时才分配
			indices = make([]int, 0, 8)
		}

		indices = append(indices, wq.head)
		wq.expiry = append(wq.expiry, w)
		wq.items[wq.head] = nil
		wq.head++

		if wq.head == wq.size {
			wq.head = 0
		}

		wq.isFull = false
		expiredCount++
	}

	// 关闭过期的 worker（批量处理）
	if expiredCount > 0 {
		for i, w := range wq.expiry {
			w.finish()
			wq.expiry[i] = nil // 清空引用，帮助 GC
		}
	}

	return indices
}

// reset 重置队列，清空所有 worker
func (wq *loopQueueWithFunc) reset() {
	if wq.isEmpty() {
		return
	}

	// 关闭并清空所有元素
	if wq.head < wq.tail {
		for i := wq.head; i < wq.tail; i++ {
			if wq.items[i] != nil {
				wq.items[i].finish()
			}
			wq.items[i] = nil
		}
	} else {
		for i := wq.head; i < wq.size; i++ {
			if wq.items[i] != nil {
				wq.items[i].finish()
			}
			wq.items[i] = nil
		}
		for i := 0; i < wq.tail; i++ {
			if wq.items[i] != nil {
				wq.items[i].finish()
			}
			wq.items[i] = nil
		}
	}

	wq.head = 0
	wq.tail = 0
	wq.isFull = false
}
