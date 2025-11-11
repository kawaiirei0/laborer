package laborer

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestNewPoolWithFunc 测试创建函数池
func TestNewPoolWithFunc(t *testing.T) {
	// 测试正常创建
	pf := func(i interface{}) {}
	pool, err := NewPoolWithFunc(5, pf)
	if err != nil {
		t.Fatalf("创建函数池失败: %v", err)
	}
	defer pool.Release()

	if pool.Cap() != 5 {
		t.Errorf("期望容量 5，实际 %d", pool.Cap())
	}

	// 测试无效容量
	_, err = NewPoolWithFunc(0, pf)
	if err != ErrInvalidPoolSize {
		t.Errorf("期望返回 ErrInvalidPoolSize，实际返回: %v", err)
	}

	// 测试无效函数
	_, err = NewPoolWithFunc(5, nil)
	if err != ErrInvalidPoolFunc {
		t.Errorf("期望返回 ErrInvalidPoolFunc，实际返回: %v", err)
	}
}

// TestPoolWithFuncInvoke 测试函数池的Invoke方法
func TestPoolWithFuncInvoke(t *testing.T) {
	var counter int32
	pf := func(i interface{}) {
		atomic.AddInt32(&counter, i.(int32))
	}

	pool, err := NewPoolWithFunc(5, pf)
	if err != nil {
		t.Fatalf("创建函数池失败: %v", err)
	}
	defer pool.Release()

	var wg sync.WaitGroup
	for i := int32(1); i <= 10; i++ {
		wg.Add(1)
		val := i
		go func() {
			defer wg.Done()
			err := pool.Invoke(val)
			if err != nil {
				t.Errorf("Invoke失败: %v", err)
			}
		}()
	}

	wg.Wait()

	// 等待一下确保所有任务完成
	time.Sleep(100 * time.Millisecond)

	// 验证结果：1+2+3+...+10 = 55
	if counter != 55 {
		t.Errorf("期望counter为55，实际为 %d", counter)
	}
}

// TestPoolWithFuncNonblocking 测试非阻塞模式
func TestPoolWithFuncNonblocking(t *testing.T) {
	pf := func(i interface{}) {
		time.Sleep(100 * time.Millisecond)
	}

	pool, err := NewPoolWithFunc(2, pf, WithNonblocking(true))
	if err != nil {
		t.Fatalf("创建函数池失败: %v", err)
	}
	defer pool.Release()

	// 填满池
	for i := 0; i < 2; i++ {
		err := pool.Invoke(i)
		if err != nil {
			t.Errorf("Invoke失败: %v", err)
		}
	}

	// 等待一下确保worker开始执行
	time.Sleep(20 * time.Millisecond)

	// 再次提交应该失败（非阻塞模式）
	err = pool.Invoke(3)
	if err != ErrPoolOverload {
		t.Errorf("期望返回 ErrPoolOverload，实际返回: %v", err)
	}
}

// TestPoolWithFuncRelease 测试函数池关闭
func TestPoolWithFuncRelease(t *testing.T) {
	var counter int32
	done := make(chan struct{}, 10)
	pf := func(i interface{}) {
		atomic.AddInt32(&counter, 1)
		time.Sleep(50 * time.Millisecond)
		done <- struct{}{}
	}

	pool, err := NewPoolWithFunc(5, pf)
	if err != nil {
		t.Fatalf("创建函数池失败: %v", err)
	}

	// 提交10个任务
	for i := 0; i < 10; i++ {
		err := pool.Invoke(i)
		if err != nil {
			t.Errorf("Invoke失败: %v", err)
		}
	}

	// 等待所有任务执行完成
	for i := 0; i < 10; i++ {
		<-done
	}

	pool.Release()

	// 验证池已关闭
	if !pool.IsClosed() {
		t.Error("池应该已关闭")
	}

	// 关闭后提交应该失败
	err = pool.Invoke(nil)
	if err != ErrPoolClosed {
		t.Errorf("期望返回 ErrPoolClosed，实际返回: %v", err)
	}

	// 验证所有任务都已执行
	if counter != 10 {
		t.Errorf("期望执行10个任务，实际执行了 %d 个", counter)
	}
}

// TestPoolWithFuncReboot 测试函数池重启
func TestPoolWithFuncReboot(t *testing.T) {
	var counter int32
	pf := func(i interface{}) {
		atomic.AddInt32(&counter, 1)
	}

	pool, err := NewPoolWithFunc(5, pf)
	if err != nil {
		t.Fatalf("创建函数池失败: %v", err)
	}

	// 提交一些任务
	for i := 0; i < 5; i++ {
		err := pool.Invoke(i)
		if err != nil {
			t.Errorf("Invoke失败: %v", err)
		}
	}

	time.Sleep(100 * time.Millisecond)

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

	// 提交新任务应该成功
	err = pool.Invoke(100)
	if err != nil {
		t.Errorf("重启后Invoke失败: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if counter != 6 {
		t.Errorf("期望执行6个任务，实际执行了 %d 个", counter)
	}

	pool.Release()
}

// TestPoolWithFuncStatusQueries 测试函数池状态查询
func TestPoolWithFuncStatusQueries(t *testing.T) {
	capacity := 3
	started := make(chan struct{}, capacity)
	pf := func(i interface{}) {
		started <- struct{}{}
		time.Sleep(100 * time.Millisecond)
	}

	pool, err := NewPoolWithFunc(capacity, pf)
	if err != nil {
		t.Fatalf("创建函数池失败: %v", err)
	}
	defer pool.Release()

	// 测试容量
	if pool.Cap() != capacity {
		t.Errorf("Cap() 期望返回 %d，实际返回 %d", capacity, pool.Cap())
	}

	// 测试初始状态
	if pool.IsClosed() {
		t.Error("新创建的池不应该是关闭状态")
	}

	if pool.Running() != 0 {
		t.Errorf("初始Running()应该返回0，实际返回 %d", pool.Running())
	}

	// 提交任务
	var wg sync.WaitGroup
	for i := 0; i < capacity; i++ {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			_ = pool.Invoke(val)
		}(i)
	}

	// 等待所有任务开始执行
	for i := 0; i < capacity; i++ {
		<-started
	}

	// 检查运行状态
	running := pool.Running()
	if running != capacity {
		t.Errorf("Running() 期望返回 %d，实际返回 %d", capacity, running)
	}

	wg.Wait()
}
