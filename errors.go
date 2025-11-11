package laborer

import "errors"

// 错误定义
//
// Laborer 定义了以下错误类型，用于表示不同的错误情况。
// 所有错误都是预定义的 sentinel 错误，可以使用 errors.Is 进行比较。
var (
	// ErrPoolClosed 表示池已经被关闭。
	//
	// 当尝试向已关闭的池提交任务时返回此错误。
	// 可以使用 Reboot() 方法重启池。
	//
	// 示例:
	//  if err := pool.Submit(task); errors.Is(err, laborer.ErrPoolClosed) {
	//      pool.Reboot()
	//      pool.Submit(task)
	//  }
	ErrPoolClosed = errors.New("pool has been closed")

	// ErrPoolOverload 表示池已过载（仅在非阻塞模式下返回）。
	//
	// 当池的所有 worker 都在忙碌且达到容量上限时，
	// 在非阻塞模式下提交任务会返回此错误。
	//
	// 处理建议:
	//  - 增加池容量
	//  - 实现任务队列或重试机制
	//  - 使用阻塞模式
	//  - 拒绝请求并返回错误给客户端
	//
	// 示例:
	//  if err := pool.Submit(task); errors.Is(err, laborer.ErrPoolOverload) {
	//      // 将任务放入队列稍后重试
	//      taskQueue.Push(task)
	//  }
	ErrPoolOverload = errors.New("pool is overloaded")

	// ErrInvalidPoolSize 表示提供的池大小无效。
	//
	// 当创建池时提供的容量为 0 时返回此错误。
	// 有效的容量值为正整数或 -1（表示无限容量）。
	//
	// 示例:
	//  pool, err := laborer.NewPool(0)  // 返回 ErrInvalidPoolSize
	//  pool, err := laborer.NewPool(-1) // OK，无限容量
	//  pool, err := laborer.NewPool(10) // OK
	ErrInvalidPoolSize = errors.New("invalid pool size")

	// ErrInvalidPoolExpiry 表示提供的过期时间无效。
	//
	// 当 ExpiryDuration 配置为负数时返回此错误。
	// 过期时间必须为非负数。
	//
	// 示例:
	//  pool, err := laborer.NewPool(10,
	//      laborer.WithExpiryDuration(-1 * time.Second)) // 返回 ErrInvalidPoolExpiry
	ErrInvalidPoolExpiry = errors.New("invalid pool expiry")

	// ErrInvalidPoolFunc 表示提供的池函数无效。
	//
	// 当创建 PoolWithFunc 时提供的函数为 nil 时返回此错误。
	//
	// 示例:
	//  pool, err := laborer.NewPoolWithFunc(10, nil) // 返回 ErrInvalidPoolFunc
	ErrInvalidPoolFunc = errors.New("invalid pool function")

	// ErrTimeout 表示操作超时。
	//
	// 在以下情况下返回此错误:
	//  - ReleaseTimeout: 池关闭超时
	//  - Future.GetWithTimeout: 等待任务结果超时
	//
	// 示例:
	//  if err := pool.ReleaseTimeout(5 * time.Second); errors.Is(err, laborer.ErrTimeout) {
	//      // 强制关闭
	//      pool.Release()
	//  }
	ErrTimeout = errors.New("operation timeout")
)
