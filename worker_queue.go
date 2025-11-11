package laborer

import "time"

// workerQueue 定义了 worker 队列的接口
// 用于管理空闲的 worker，支持高效的插入和获取操作
type workerQueue interface {
	// len 返回队列中的 worker 数量
	len() int

	// isEmpty 检查队列是否为空
	isEmpty() bool

	// insert 将 worker 插入队列
	insert(worker *goWorker) error

	// detach 从队列中取出一个 worker
	detach() *goWorker

	// refresh 清理过期的 worker，返回被清理的 worker 索引列表
	refresh(duration time.Duration) []int

	// reset 重置队列
	reset()
}

// workerQueueWithFunc 定义了函数池 worker 队列的接口
// 用于管理空闲的 goWorkerWithFunc，支持高效的插入和获取操作
type workerQueueWithFunc interface {
	// len 返回队列中的 worker 数量
	len() int

	// isEmpty 检查队列是否为空
	isEmpty() bool

	// insert 将 worker 插入队列
	insert(worker *goWorkerWithFunc) error

	// detach 从队列中取出一个 worker
	detach() *goWorkerWithFunc

	// refresh 清理过期的 worker，返回被清理的 worker 索引列表
	refresh(duration time.Duration) []int

	// reset 重置队列
	reset()
}
