package laborer

import "time"

// workerStack 使用栈（LIFO）结构实现 worker 队列
// 适用于小容量场景（< 1000），优先使用最近使用的 worker（缓存友好）
// 内存布局优化：将常用字段放在前面，提高缓存命中率
type workerStack struct {
	items  []*goWorker
	size   int
	expiry []*goWorker
}

// newWorkerStack 创建一个新的 worker 栈
// 如果 size > 0，预分配切片容量以减少后续的内存分配
func newWorkerStack(size int) *workerStack {
	if size > 0 {
		return &workerStack{
			items: make([]*goWorker, 0, size),
			size:  size,
		}
	}
	return &workerStack{
		items: make([]*goWorker, 0, 32), // 默认初始容量
		size:  size,
	}
}

// len 返回栈中的 worker 数量
func (wq *workerStack) len() int {
	return len(wq.items)
}

// isEmpty 检查栈是否为空
func (wq *workerStack) isEmpty() bool {
	return len(wq.items) == 0
}

// insert 将 worker 压入栈顶
func (wq *workerStack) insert(worker *goWorker) error {
	wq.items = append(wq.items, worker)
	return nil
}

// detach 从栈顶弹出一个 worker
func (wq *workerStack) detach() *goWorker {
	l := len(wq.items)
	if l == 0 {
		return nil
	}

	w := wq.items[l-1]
	wq.items[l-1] = nil // 避免内存泄漏
	wq.items = wq.items[:l-1]

	return w
}

// refresh 清理过期的 worker
// 遍历栈中的所有 worker，将超过 duration 时间未使用的 worker 标记为过期
// 返回被清理的 worker 在原栈中的索引列表
// 优化：减少内存分配，复用 expiry 切片，使用更高效的算法
func (wq *workerStack) refresh(duration time.Duration) []int {
	n := len(wq.items)
	if n == 0 {
		return nil
	}

	expiryTime := time.Now().Add(-duration)
	index := 0

	// 找到第一个未过期的 worker
	for index < n && wq.items[index].lastUsed.Before(expiryTime) {
		index++
	}

	// 如果有过期的 worker
	if index > 0 {
		// 复用 expiry 切片，避免重新分配
		if cap(wq.expiry) >= index {
			wq.expiry = wq.expiry[:index]
		} else {
			wq.expiry = make([]*goWorker, index)
		}
		copy(wq.expiry, wq.items[:index])

		// 移动未过期的 worker 到前面（优化：使用 copy 一次性完成）
		m := copy(wq.items, wq.items[index:])

		// 清空尾部引用，避免内存泄漏（优化：批量清空）
		for i := m; i < n; i++ {
			wq.items[i] = nil
		}
		wq.items = wq.items[:m]

		// 关闭过期的 worker（在返回前执行，减少持锁时间）
		for i, w := range wq.expiry {
			w.finish()
			// 直接使用索引，避免额外的切片分配
			wq.expiry[i] = nil
		}

		// 返回过期 worker 的索引列表（优化：预分配固定大小）
		indices := make([]int, index)
		for i := range indices {
			indices[i] = i
		}
		return indices
	}

	return nil
}

// reset 重置栈，清空所有 worker
func (wq *workerStack) reset() {
	// 关闭所有 worker
	for _, w := range wq.items {
		if w != nil {
			w.finish()
		}
	}

	for i := range wq.items {
		wq.items[i] = nil
	}
	wq.items = wq.items[:0]
}

// workerStackWithFunc 使用栈（LIFO）结构实现函数池 worker 队列
// 适用于小容量场景（< 1000），优先使用最近使用的 worker（缓存友好）
// 内存布局优化：将常用字段放在前面，提高缓存命中率
type workerStackWithFunc struct {
	items  []*goWorkerWithFunc
	size   int
	expiry []*goWorkerWithFunc
}

// newWorkerStackWithFunc 创建一个新的函数池 worker 栈
// 如果 size > 0，预分配切片容量以减少后续的内存分配
func newWorkerStackWithFunc(size int) *workerStackWithFunc {
	if size > 0 {
		return &workerStackWithFunc{
			items: make([]*goWorkerWithFunc, 0, size),
			size:  size,
		}
	}
	return &workerStackWithFunc{
		items: make([]*goWorkerWithFunc, 0, 32), // 默认初始容量
		size:  size,
	}
}

// len 返回栈中的 worker 数量
func (wq *workerStackWithFunc) len() int {
	return len(wq.items)
}

// isEmpty 检查栈是否为空
func (wq *workerStackWithFunc) isEmpty() bool {
	return len(wq.items) == 0
}

// insert 将 worker 压入栈顶
func (wq *workerStackWithFunc) insert(worker *goWorkerWithFunc) error {
	wq.items = append(wq.items, worker)
	return nil
}

// detach 从栈顶弹出一个 worker
func (wq *workerStackWithFunc) detach() *goWorkerWithFunc {
	l := len(wq.items)
	if l == 0 {
		return nil
	}

	w := wq.items[l-1]
	wq.items[l-1] = nil // 避免内存泄漏
	wq.items = wq.items[:l-1]

	return w
}

// refresh 清理过期的 worker
// 遍历栈中的所有 worker，将超过 duration 时间未使用的 worker 标记为过期
// 返回被清理的 worker 在原栈中的索引列表
// 优化：减少内存分配，复用 expiry 切片，使用更高效的算法
func (wq *workerStackWithFunc) refresh(duration time.Duration) []int {
	n := len(wq.items)
	if n == 0 {
		return nil
	}

	expiryTime := time.Now().Add(-duration)
	index := 0

	// 找到第一个未过期的 worker
	for index < n && wq.items[index].lastUsed.Before(expiryTime) {
		index++
	}

	// 如果有过期的 worker
	if index > 0 {
		// 复用 expiry 切片，避免重新分配
		if cap(wq.expiry) >= index {
			wq.expiry = wq.expiry[:index]
		} else {
			wq.expiry = make([]*goWorkerWithFunc, index)
		}
		copy(wq.expiry, wq.items[:index])

		// 移动未过期的 worker 到前面（优化：使用 copy 一次性完成）
		m := copy(wq.items, wq.items[index:])

		// 清空尾部引用，避免内存泄漏（优化：批量清空）
		for i := m; i < n; i++ {
			wq.items[i] = nil
		}
		wq.items = wq.items[:m]

		// 关闭过期的 worker（在返回前执行，减少持锁时间）
		for i, w := range wq.expiry {
			w.finish()
			// 直接使用索引，避免额外的切片分配
			wq.expiry[i] = nil
		}

		// 返回过期 worker 的索引列表（优化：预分配固定大小）
		indices := make([]int, index)
		for i := range indices {
			indices[i] = i
		}
		return indices
	}

	return nil
}

// reset 重置栈，清空所有 worker
func (wq *workerStackWithFunc) reset() {
	// 关闭所有 worker
	for _, w := range wq.items {
		if w != nil {
			w.finish()
		}
	}

	for i := range wq.items {
		wq.items[i] = nil
	}
	wq.items = wq.items[:0]
}
