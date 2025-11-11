package laborer

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestPoolSubmit 测试基本的任务提交功能
func TestPoolSubmit(t *testing.T) {
	pool, err := NewPool(10)
	if err != nil {
		t.Fatalf("创建池失败: %v", err)
	}
	defer pool.Release()

	var counter int32
	var wg sync.WaitGroup

	// 提交10个任务
	for i := 0; i < 10; i++ {
		wg.Add(1)
		err := pool.Submit(func() {
			atomic.AddInt32(&counter, 1)
			wg.Done()
		})
		if err != nil {
			t.Errorf("提交任务失败: %v", err)
		}
	}

	wg.Wait()

	if counter != 10 {
		t.Errorf("期望执行10个任务，实际执行了 %d 个", counter)
	}
}

// TestPoolSubmitBlocking 测试阻塞模式
func TestPoolSubmitBlocking(t *testing.T) {
	// 创建容量为2的池，默认阻塞模式
	pool, err := NewPool(2)
	if err != nil {
		t.Fatalf("创建池失败: %v", err)
	}
	defer pool.Release()

	var counter int32
	var wg sync.WaitGroup

	// 提交5个任务，每个任务执行100ms
	for i := 0; i < 5; i++ {
		wg.Add(1)
		err := pool.Submit(func() {
			time.Sleep(100 * time.Millisecond)
			atomic.AddInt32(&counter, 1)
			wg.Done()
		})
		if err != nil {
			t.Errorf("提交任务失败: %v", err)
		}
	}

	wg.Wait()

	if counter != 5 {
		t.Errorf("期望执行5个任务，实际执行了 %d 个", counter)
	}
}

// TestPoolSubmitNonblocking 测试非阻塞模式
func TestPoolSubmitNonblocking(t *testing.T) {
	// 创建容量为2的池，非阻塞模式
	pool, err := NewPool(2, WithNonblocking(true))
	if err != nil {
		t.Fatalf("创建池失败: %v", err)
	}
	defer pool.Release()

	// 提交2个长时间运行的任务占满池
	for i := 0; i < 2; i++ {
		err := pool.Submit(func() {
			time.Sleep(200 * time.Millisecond)
		})
		if err != nil {
			t.Errorf("提交任务失败: %v", err)
		}
	}

	// 等待一下确保worker都在运行
	time.Sleep(50 * time.Millisecond)

	// 尝试提交第3个任务，应该返回错误
	err = pool.Submit(func() {})
	if err != ErrPoolOverload {
		t.Errorf("期望返回 ErrPoolOverload，实际返回: %v", err)
	}
}

// TestPoolSubmitAfterClose 测试关闭后提交任务
func TestPoolSubmitAfterClose(t *testing.T) {
	pool, err := NewPool(5)
	if err != nil {
		t.Fatalf("创建池失败: %v", err)
	}

	pool.Release()

	err = pool.Submit(func() {})
	if err != ErrPoolClosed {
		t.Errorf("期望返回 ErrPoolClosed，实际返回: %v", err)
	}
}

// TestWorkerReuse 测试worker复用
func TestWorkerReuse(t *testing.T) {
	pool, err := NewPool(2)
	if err != nil {
		t.Fatalf("创建池失败: %v", err)
	}
	defer pool.Release()

	var wg sync.WaitGroup

	// 提交10个任务，但只有2个worker
	for i := 0; i < 10; i++ {
		wg.Add(1)
		err := pool.Submit(func() {
			time.Sleep(10 * time.Millisecond)
			wg.Done()
		})
		if err != nil {
			t.Errorf("提交任务失败: %v", err)
		}
	}

	wg.Wait()

	// 验证运行的worker数量不超过容量
	running := pool.Running()
	if running > 2 {
		t.Errorf("运行的worker数量 %d 超过了容量 2", running)
	}
}

// TestSubmitWithResult 测试带返回值的任务提交
func TestSubmitWithResult(t *testing.T) {
	pool, err := NewPool(5)
	if err != nil {
		t.Fatalf("创建池失败: %v", err)
	}
	defer pool.Release()

	// 提交一个返回结果的任务
	future, err := pool.SubmitWithResult(func() (interface{}, error) {
		return 42, nil
	})
	if err != nil {
		t.Fatalf("提交任务失败: %v", err)
	}

	// 获取结果
	result, err := future.Get()
	if err != nil {
		t.Errorf("获取结果失败: %v", err)
	}

	if result != 42 {
		t.Errorf("期望结果为 42，实际为 %v", result)
	}
}

// TestSubmitWithResultError 测试带错误返回的任务
func TestSubmitWithResultError(t *testing.T) {
	pool, err := NewPool(5)
	if err != nil {
		t.Fatalf("创建池失败: %v", err)
	}
	defer pool.Release()

	// 提交一个返回错误的任务
	future, err := pool.SubmitWithResult(func() (interface{}, error) {
		return nil, ErrPoolOverload
	})
	if err != nil {
		t.Fatalf("提交任务失败: %v", err)
	}

	// 获取结果
	result, err := future.Get()
	if err != ErrPoolOverload {
		t.Errorf("期望错误为 ErrPoolOverload，实际为 %v", err)
	}

	if result != nil {
		t.Errorf("期望结果为 nil，实际为 %v", result)
	}
}

// TestFutureGetWithTimeout 测试带超时的结果获取
func TestFutureGetWithTimeout(t *testing.T) {
	pool, err := NewPool(5)
	if err != nil {
		t.Fatalf("创建池失败: %v", err)
	}
	defer pool.Release()

	// 提交一个长时间运行的任务
	future, err := pool.SubmitWithResult(func() (interface{}, error) {
		time.Sleep(200 * time.Millisecond)
		return "done", nil
	})
	if err != nil {
		t.Fatalf("提交任务失败: %v", err)
	}

	// 使用短超时获取结果，应该超时
	result, err := future.GetWithTimeout(50 * time.Millisecond)
	if err != ErrTimeout {
		t.Errorf("期望错误为 ErrTimeout，实际为 %v", err)
	}
	if result != nil {
		t.Errorf("超时时期望结果为 nil，实际为 %v", result)
	}

	// 使用足够长的超时获取结果，应该成功
	result, err = future.GetWithTimeout(300 * time.Millisecond)
	if err != nil {
		t.Errorf("获取结果失败: %v", err)
	}
	if result != "done" {
		t.Errorf("期望结果为 'done'，实际为 %v", result)
	}
}

// TestFutureIsDone 测试任务完成状态检查
func TestFutureIsDone(t *testing.T) {
	pool, err := NewPool(5)
	if err != nil {
		t.Fatalf("创建池失败: %v", err)
	}
	defer pool.Release()

	// 提交一个任务
	future, err := pool.SubmitWithResult(func() (interface{}, error) {
		time.Sleep(100 * time.Millisecond)
		return "result", nil
	})
	if err != nil {
		t.Fatalf("提交任务失败: %v", err)
	}

	// 任务刚提交，应该未完成
	if future.IsDone() {
		t.Error("任务刚提交就显示已完成")
	}

	// 等待任务完成
	result, err := future.Get()
	if err != nil {
		t.Errorf("获取结果失败: %v", err)
	}
	if result != "result" {
		t.Errorf("期望结果为 'result'，实际为 %v", result)
	}

	// 任务完成后，应该显示已完成
	if !future.IsDone() {
		t.Error("任务完成后显示未完成")
	}
}

// TestSubmitWithResultAfterClose 测试关闭后提交带返回值的任务
func TestSubmitWithResultAfterClose(t *testing.T) {
	pool, err := NewPool(5)
	if err != nil {
		t.Fatalf("创建池失败: %v", err)
	}

	pool.Release()

	future, err := pool.SubmitWithResult(func() (interface{}, error) {
		return nil, nil
	})
	if err != ErrPoolClosed {
		t.Errorf("期望返回 ErrPoolClosed，实际返回: %v", err)
	}
	if future != nil {
		t.Error("池关闭后提交任务应该返回 nil future")
	}
}

// TestMultipleFutureGet 测试多次调用 Get 方法
func TestMultipleFutureGet(t *testing.T) {
	pool, err := NewPool(5)
	if err != nil {
		t.Fatalf("创建池失败: %v", err)
	}
	defer pool.Release()

	future, err := pool.SubmitWithResult(func() (interface{}, error) {
		return 100, nil
	})
	if err != nil {
		t.Fatalf("提交任务失败: %v", err)
	}

	// 第一次获取
	result1, err1 := future.Get()
	if err1 != nil {
		t.Errorf("第一次获取结果失败: %v", err1)
	}

	// 第二次获取，应该返回相同的结果
	result2, err2 := future.Get()
	if err2 != nil {
		t.Errorf("第二次获取结果失败: %v", err2)
	}

	if result1 != result2 {
		t.Errorf("多次获取结果不一致: %v vs %v", result1, result2)
	}
}
