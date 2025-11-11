package laborer

import (
	"sync"
	"time"
)

// Future 表示一个异步计算的结果。
//
// Future 模式允许提交带返回值的任务，并在稍后获取执行结果。
// 这对于需要等待任务完成并获取返回值的场景非常有用。
//
// Future 是线程安全的，可以从多个 goroutine 中调用。
// 结果只会被设置一次，多次调用 Get 会返回相同的结果。
//
// 示例:
//
//	future, err := pool.SubmitWithResult(func() (interface{}, error) {
//	    result := heavyComputation()
//	    return result, nil
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// 阻塞等待结果
//	result, err := future.Get()
//	if err != nil {
//	    log.Printf("Task failed: %v", err)
//	} else {
//	    log.Printf("Result: %v", result)
//	}
type Future interface {
	// Get 阻塞等待并获取任务执行结果。
	//
	// 此方法会一直阻塞直到任务完成。
	// 如果任务已经完成，会立即返回结果。
	// 多次调用 Get 会返回相同的结果。
	//
	// 返回:
	//  - interface{}: 任务的返回值
	//  - error: 任务执行过程中的错误，如果没有错误则为 nil
	//
	// 示例:
	//  result, err := future.Get()
	//  if err != nil {
	//      log.Printf("Task failed: %v", err)
	//  }
	Get() (interface{}, error)

	// GetWithTimeout 带超时地等待并获取任务执行结果。
	//
	// 此方法会等待指定的时间，如果任务在超时前完成则返回结果，
	// 否则返回 ErrTimeout 错误。
	//
	// 参数:
	//  - timeout: 等待超时时间
	//
	// 返回:
	//  - interface{}: 任务的返回值
	//  - error: 任务执行错误或 ErrTimeout
	//
	// 示例:
	//  result, err := future.GetWithTimeout(5 * time.Second)
	//  if errors.Is(err, laborer.ErrTimeout) {
	//      log.Println("Task timed out")
	//  } else if err != nil {
	//      log.Printf("Task failed: %v", err)
	//  }
	GetWithTimeout(timeout time.Duration) (interface{}, error)

	// IsDone 检查任务是否已完成。
	//
	// 此方法不会阻塞，立即返回任务的完成状态。
	// 可以用于轮询任务状态或实现非阻塞的结果获取。
	//
	// 返回:
	//  - bool: true 表示任务已完成，false 表示任务仍在执行
	//
	// 示例:
	//  if future.IsDone() {
	//      result, err := future.Get()
	//      // 处理结果
	//  } else {
	//      // 任务仍在执行，继续其他工作
	//  }
	IsDone() bool
}

// future 是 Future 接口的内部实现。
//
// 使用 channel 和 sync.Once 确保线程安全和结果的唯一性。
type future struct {
	// result 存储任务执行的返回值
	result interface{}

	// err 存储任务执行过程中的错误
	err error

	// done 是一个无缓冲 channel，用于通知任务完成
	// 关闭此 channel 表示任务已完成
	done chan struct{}

	// once 确保结果只被设置一次
	// 防止多次设置结果导致的竞态条件
	once sync.Once
}

// newFuture 创建一个新的 future 实例。
//
// 此函数由池内部调用，用户不应直接调用。
//
// 返回:
//   - *future: 新创建的 future 实例
func newFuture() *future {
	return &future{
		done: make(chan struct{}),
	}
}

// Get 实现 Future.Get 接口。
//
// 阻塞等待任务完成并返回结果。
// 如果任务已完成，立即返回；否则阻塞直到任务完成。
//
// 返回:
//   - interface{}: 任务的返回值
//   - error: 任务执行错误，如果没有错误则为 nil
func (f *future) Get() (interface{}, error) {
	<-f.done
	return f.result, f.err
}

// GetWithTimeout 实现 Future.GetWithTimeout 接口。
//
// 在指定时间内等待任务完成。
// 如果任务在超时前完成，返回结果；否则返回 ErrTimeout。
//
// 参数:
//   - timeout: 等待超时时间
//
// 返回:
//   - interface{}: 任务的返回值（超时时为 nil）
//   - error: 任务执行错误或 ErrTimeout
func (f *future) GetWithTimeout(timeout time.Duration) (interface{}, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-f.done:
		return f.result, f.err
	case <-timer.C:
		return nil, ErrTimeout
	}
}

// IsDone 实现 Future.IsDone 接口。
//
// 非阻塞地检查任务是否已完成。
// 使用 select 的 default 分支实现非阻塞检查。
//
// 返回:
//   - bool: true 表示任务已完成，false 表示任务仍在执行
func (f *future) IsDone() bool {
	select {
	case <-f.done:
		return true
	default:
		return false
	}
}

// setResult 设置任务执行结果（内部方法）。
//
// 此方法由池内部调用，用于设置任务的执行结果。
// 使用 sync.Once 确保结果只被设置一次，即使多次调用也是安全的。
// 设置结果后会关闭 done channel，通知所有等待的 goroutine。
//
// 参数:
//   - result: 任务的返回值
//   - err: 任务执行过程中的错误
func (f *future) setResult(result interface{}, err error) {
	f.once.Do(func() {
		f.result = result
		f.err = err
		close(f.done)
	})
}
