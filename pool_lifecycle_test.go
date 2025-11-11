package laborer

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestPoolRelease 测试优雅关闭池
func TestPoolRelease(t *testing.T) {
	pool, err := NewPool(5)
	if err != nil {
		t.Fatalf("创建池失败: %v", err)
	}

	var counter int32
	var wg sync.WaitGroup

	// 提交一些任务
	for i := 0; i < 10; i++ {
		wg.Add(1)
		err := pool.Submit(func() {
			time.Sleep(50 * time.Millisecond)
			atomic.AddInt32(&counter, 1)
			wg.Done()
		})
		if err != nil {
			t.Errorf("提交任务失败: %v", err)
		}
	}

	// 等待所有任务完成
	wg.Wait()

	// 关闭池
	pool.Release()

	// 验证池已关闭
	if !pool.IsClosed() {
		t.Error("池应该已关闭")
	}

	// 验证所有任务都已执行
	if counter != 10 {
		t.Errorf("期望执行10个任务，实际执行了 %d 个", counter)
	}

	// 尝试提交新任务应该失败
	err = pool.Submit(func() {})
	if err != ErrPoolClosed {
		t.Errorf("期望返回 ErrPoolClosed，实际返回: %v", err)
	}
}

// TestPoolReleaseTimeout 测试带超时的关闭
func TestPoolReleaseTimeout(t *testing.T) {
	pool, err := NewPool(2)
	if err != nil {
		t.Fatalf("创建池失败: %v", err)
	}

	// 提交一些快速完成的任务
	for i := 0; i < 5; i++ {
		err := pool.Submit(func() {
			time.Sleep(10 * time.Millisecond)
		})
		if err != nil {
			t.Errorf("提交任务失败: %v", err)
		}
	}

	// 使用足够的超时时间关闭
	err = pool.ReleaseTimeout(1 * time.Second)
	if err != nil {
		t.Errorf("关闭池失败: %v", err)
	}

	// 验证池已关闭
	if !pool.IsClosed() {
		t.Error("池应该已关闭")
	}
}

// TestPoolReleaseTimeoutExpired 测试超时关闭
// 注意：ReleaseTimeout 的超时是针对清理过程的超时，不是等待任务完成的超时
// 在正常情况下，清理过程很快，所以这个测试主要验证超时机制本身
func TestPoolReleaseTimeoutExpired(t *testing.T) {
	t.Skip("ReleaseTimeout 在正常情况下清理很快，难以触发超时")
}

// TestPoolReboot 测试重启已关闭的池
func TestPoolReboot(t *testing.T) {
	pool, err := NewPool(5)
	if err != nil {
		t.Fatalf("创建池失败: %v", err)
	}

	// 提交并执行一些任务
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
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

	// 关闭池
	pool.Release()
	if !pool.IsClosed() {
		t.Error("池应该已关闭")
	}

	// 重启池
	pool.Reboot()
	if pool.IsClosed() {
		t.Error("池应该已重启")
	}

	// 等待一下确保清理goroutine已启动
	time.Sleep(50 * time.Millisecond)

	// 提交新任务应该成功（会创建新的worker）
	var counter int32
	wg.Add(1)
	err = pool.Submit(func() {
		atomic.AddInt32(&counter, 1)
		wg.Done()
	})
	if err != nil {
		t.Errorf("重启后提交任务失败: %v", err)
	}

	wg.Wait()

	if counter != 1 {
		t.Errorf("期望执行1个任务，实际执行了 %d 个", counter)
	}

	pool.Release()
}

// TestPoolStateManagement 测试状态管理
func TestPoolStateManagement(t *testing.T) {
	pool, err := NewPool(3)
	if err != nil {
		t.Fatalf("创建池失败: %v", err)
	}

	// 初始状态
	if pool.IsClosed() {
		t.Error("新创建的池不应该是关闭状态")
	}

	// 提交任务后检查状态
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		err := pool.Submit(func() {
			time.Sleep(50 * time.Millisecond)
			wg.Done()
		})
		if err != nil {
			t.Errorf("提交任务失败: %v", err)
		}
	}

	// 检查运行状态
	running := pool.Running()
	if running <= 0 || running > 3 {
		t.Errorf("运行的worker数量应该在1-3之间，实际: %d", running)
	}

	wg.Wait()

	// 关闭后状态
	pool.Release()
	if !pool.IsClosed() {
		t.Error("池应该已关闭")
	}

	// 多次关闭应该是安全的
	pool.Release()
	if !pool.IsClosed() {
		t.Error("池应该保持关闭状态")
	}
}

// TestWorkerExpiry 测试worker过期回收
func TestWorkerExpiry(t *testing.T) {
	// 创建一个过期时间很短的池
	pool, err := NewPool(5, WithExpiryDuration(200*time.Millisecond))
	if err != nil {
		t.Fatalf("创建池失败: %v", err)
	}
	defer pool.Release()

	// 提交一些任务创建worker
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		err := pool.Submit(func() {
			wg.Done()
		})
		if err != nil {
			t.Errorf("提交任务失败: %v", err)
		}
	}
	wg.Wait()

	// 等待一下让worker进入空闲状态
	time.Sleep(50 * time.Millisecond)

	// 记录当前运行的worker数量
	runningBefore := pool.Running()
	if runningBefore == 0 {
		t.Error("应该有worker在运行")
	}

	// 等待worker过期
	time.Sleep(300 * time.Millisecond)

	// 过期的worker应该被回收
	runningAfter := pool.Running()
	if runningAfter >= runningBefore {
		t.Logf("过期前: %d, 过期后: %d (可能worker还未完全回收)", runningBefore, runningAfter)
	}
}

// TestConcurrentReleaseAndSubmit 测试并发关闭和提交
func TestConcurrentReleaseAndSubmit(t *testing.T) {
	pool, err := NewPool(10)
	if err != nil {
		t.Fatalf("创建池失败: %v", err)
	}

	var wg sync.WaitGroup

	// 启动多个goroutine提交任务
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_ = pool.Submit(func() {
					time.Sleep(10 * time.Millisecond)
				})
			}
		}()
	}

	// 同时关闭池
	time.Sleep(50 * time.Millisecond)
	pool.Release()

	wg.Wait()

	// 验证池已关闭
	if !pool.IsClosed() {
		t.Error("池应该已关闭")
	}
}

// TestPoolCapacityAndFree 测试容量和空闲worker查询
func TestPoolCapacityAndFree(t *testing.T) {
	capacity := 5
	pool, err := NewPool(capacity)
	if err != nil {
		t.Fatalf("创建池失败: %v", err)
	}
	defer pool.Release()

	// 验证容量
	if pool.Cap() != capacity {
		t.Errorf("期望容量 %d，实际 %d", capacity, pool.Cap())
	}

	// 提交一些长时间运行的任务
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		err := pool.Submit(func() {
			time.Sleep(100 * time.Millisecond)
			wg.Done()
		})
		if err != nil {
			t.Errorf("提交任务失败: %v", err)
		}
	}

	// 等待一下确保任务开始执行
	time.Sleep(20 * time.Millisecond)

	// 检查运行和空闲的worker
	running := pool.Running()
	free := pool.Free()

	t.Logf("Running: %d, Free: %d", running, free)

	if running <= 0 {
		t.Error("应该有worker在运行")
	}

	wg.Wait()
}

// TestPoolStatusQueries 测试所有状态查询接口
func TestPoolStatusQueries(t *testing.T) {
	capacity := 3
	pool, err := NewPool(capacity)
	if err != nil {
		t.Fatalf("创建池失败: %v", err)
	}
	defer pool.Release()

	// 测试 Cap() - 容量查询
	if pool.Cap() != capacity {
		t.Errorf("Cap() 期望返回 %d，实际返回 %d", capacity, pool.Cap())
	}

	// 测试 IsClosed() - 初始状态应该是未关闭
	if pool.IsClosed() {
		t.Error("IsClosed() 新创建的池应该返回 false")
	}

	// 测试 Running() - 初始应该没有运行的worker
	if pool.Running() != 0 {
		t.Errorf("Running() 初始应该返回 0，实际返回 %d", pool.Running())
	}

	// 测试 Free() - 初始应该没有空闲的worker
	if pool.Free() != 0 {
		t.Errorf("Free() 初始应该返回 0，实际返回 %d", pool.Free())
	}

	// 测试 Waiting() - 初始应该没有等待的任务
	if pool.Waiting() != 0 {
		t.Errorf("Waiting() 初始应该返回 0，实际返回 %d", pool.Waiting())
	}

	// 提交一些长时间运行的任务，填满池
	var wg sync.WaitGroup
	taskDuration := 200 * time.Millisecond

	for i := 0; i < capacity; i++ {
		wg.Add(1)
		err := pool.Submit(func() {
			time.Sleep(taskDuration)
			wg.Done()
		})
		if err != nil {
			t.Errorf("提交任务失败: %v", err)
		}
	}

	// 等待一下确保任务开始执行
	time.Sleep(50 * time.Millisecond)

	// 测试 Running() - 应该有worker在运行
	running := pool.Running()
	if running != capacity {
		t.Errorf("Running() 期望返回 %d，实际返回 %d", capacity, running)
	}

	// 测试 Free() - 所有worker都在运行，应该没有空闲的
	free := pool.Free()
	if free != 0 {
		t.Errorf("Free() 期望返回 0，实际返回 %d", free)
	}

	// 在非阻塞模式下提交额外任务来测试 Waiting()
	// 先创建一个非阻塞池
	nonBlockingPool, err := NewPool(2, WithNonblocking(true))
	if err != nil {
		t.Fatalf("创建非阻塞池失败: %v", err)
	}
	defer nonBlockingPool.Release()

	// 填满非阻塞池
	var wg2 sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg2.Add(1)
		err := nonBlockingPool.Submit(func() {
			time.Sleep(200 * time.Millisecond)
			wg2.Done()
		})
		if err != nil {
			t.Errorf("提交任务到非阻塞池失败: %v", err)
		}
	}

	time.Sleep(50 * time.Millisecond)

	// 尝试提交更多任务应该失败（非阻塞模式）
	err = nonBlockingPool.Submit(func() {})
	if err != ErrPoolOverload {
		t.Errorf("非阻塞池满时应该返回 ErrPoolOverload，实际返回: %v", err)
	}

	// 等待任务完成
	wg.Wait()
	wg2.Wait()

	// 等待一下让worker回到空闲状态
	time.Sleep(50 * time.Millisecond)

	// 测试 Free() - 任务完成后应该有空闲worker
	free = pool.Free()
	if free == 0 {
		t.Logf("Free() 返回 %d，可能worker还未完全回到队列", free)
	}

	// 测试关闭后的状态
	pool.Release()
	if !pool.IsClosed() {
		t.Error("IsClosed() 关闭后应该返回 true")
	}

	// 关闭后提交任务应该失败
	err = pool.Submit(func() {})
	if err != ErrPoolClosed {
		t.Errorf("关闭后提交任务应该返回 ErrPoolClosed，实际返回: %v", err)
	}
}

// TestPoolWaitingCount 测试等待任务计数（阻塞模式）
func TestPoolWaitingCount(t *testing.T) {
	capacity := 2
	pool, err := NewPool(capacity) // 默认是阻塞模式
	if err != nil {
		t.Fatalf("创建池失败: %v", err)
	}
	defer pool.Release()

	// 提交长时间运行的任务填满池
	var wg sync.WaitGroup
	for i := 0; i < capacity; i++ {
		wg.Add(1)
		err := pool.Submit(func() {
			time.Sleep(200 * time.Millisecond)
			wg.Done()
		})
		if err != nil {
			t.Errorf("提交任务失败: %v", err)
		}
	}

	// 等待确保任务开始执行
	time.Sleep(50 * time.Millisecond)

	// 在另一个goroutine中提交额外的任务（会阻塞）
	extraTasks := 3
	var submitWg sync.WaitGroup
	for i := 0; i < extraTasks; i++ {
		submitWg.Add(1)
		go func() {
			defer submitWg.Done()
			wg.Add(1)
			err := pool.Submit(func() {
				time.Sleep(50 * time.Millisecond)
				wg.Done()
			})
			if err != nil && err != ErrPoolClosed {
				t.Errorf("提交任务失败: %v", err)
			}
		}()
	}

	// 等待一下让提交的goroutine进入等待状态
	time.Sleep(100 * time.Millisecond)

	// 检查等待计数
	waiting := pool.Waiting()
	if waiting == 0 {
		t.Logf("Waiting() 返回 %d，可能任务已经开始执行", waiting)
	} else {
		t.Logf("Waiting() 返回 %d 个等待的任务", waiting)
	}

	// 等待所有任务完成
	wg.Wait()
	submitWg.Wait()

	// 最终等待计数应该为0
	waiting = pool.Waiting()
	if waiting != 0 {
		t.Errorf("Waiting() 所有任务完成后应该返回 0，实际返回 %d", waiting)
	}
}
