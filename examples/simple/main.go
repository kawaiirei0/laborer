package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kawaiirei0/laborer"
)

func main() {
	fmt.Println("=== Laborer 基础使用示例 ===\n")

	// 示例 1: 创建固定容量的池
	fmt.Println("1. 创建固定容量的池（容量: 10）")
	pool, err := laborer.NewPool(10)
	if err != nil {
		panic(err)
	}
	defer pool.Release()

	// 提交简单任务
	var sum int32
	var wg sync.WaitGroup

	fmt.Println("2. 提交 100 个任务到池中执行")
	for i := 0; i < 100; i++ {
		wg.Add(1)
		num := i
		err := pool.Submit(func() {
			defer wg.Done()
			atomic.AddInt32(&sum, int32(num))
			time.Sleep(10 * time.Millisecond)
		})
		if err != nil {
			fmt.Printf("提交任务失败: %v\n", err)
			wg.Done()
		}
	}

	// 等待所有任务完成
	wg.Wait()
	fmt.Printf("所有任务完成，累加结果: %d\n\n", sum)

	// 示例 2: 查询池状态
	fmt.Println("3. 查询池状态信息")
	fmt.Printf("   - 池容量: %d\n", pool.Cap())
	fmt.Printf("   - 运行中的 worker: %d\n", pool.Running())
	fmt.Printf("   - 空闲的 worker: %d\n", pool.Free())
	fmt.Printf("   - 等待的任务: %d\n", pool.Waiting())
	fmt.Printf("   - 池是否关闭: %v\n\n", pool.IsClosed())

	// 示例 3: 使用配置选项
	fmt.Println("4. 创建带配置选项的池")
	poolWithOptions, err := laborer.NewPool(
		5,
		laborer.WithExpiryDuration(5*time.Second),
		laborer.WithPreAlloc(true),
		laborer.WithPanicHandler(func(p interface{}) {
			fmt.Printf("捕获到 panic: %v\n", p)
		}),
	)
	if err != nil {
		panic(err)
	}
	defer poolWithOptions.Release()

	// 提交会 panic 的任务
	fmt.Println("5. 提交一个会 panic 的任务（测试 panic 处理）")
	err = poolWithOptions.Submit(func() {
		panic("这是一个测试 panic")
	})
	if err != nil {
		fmt.Printf("提交任务失败: %v\n", err)
	}

	time.Sleep(100 * time.Millisecond)
	fmt.Println("池仍然正常运行，panic 已被处理\n")

	// 示例 4: 非阻塞模式
	fmt.Println("6. 创建非阻塞模式的池（容量: 2）")
	nonblockingPool, err := laborer.NewPool(
		2,
		laborer.WithNonblocking(true),
	)
	if err != nil {
		panic(err)
	}
	defer nonblockingPool.Release()

	// 提交超过容量的任务
	fmt.Println("7. 提交 5 个长时间运行的任务")
	successCount := 0
	failCount := 0

	for i := 0; i < 5; i++ {
		err := nonblockingPool.Submit(func() {
			time.Sleep(1 * time.Second)
		})
		if err != nil {
			failCount++
			fmt.Printf("   任务 %d 提交失败: %v\n", i+1, err)
		} else {
			successCount++
			fmt.Printf("   任务 %d 提交成功\n", i+1)
		}
	}

	fmt.Printf("\n成功提交: %d, 失败: %d\n\n", successCount, failCount)

	// 示例 5: 池的关闭和重启
	fmt.Println("8. 测试池的关闭和重启")
	testPool, _ := laborer.NewPool(5)

	fmt.Println("   - 提交任务到池")
	testPool.Submit(func() {
		fmt.Println("   - 任务执行中...")
		time.Sleep(100 * time.Millisecond)
	})

	time.Sleep(200 * time.Millisecond)

	fmt.Println("   - 关闭池")
	testPool.Release()
	fmt.Printf("   - 池是否关闭: %v\n", testPool.IsClosed())

	fmt.Println("   - 尝试提交任务到已关闭的池")
	err = testPool.Submit(func() {
		fmt.Println("这个任务不会执行")
	})
	if err != nil {
		fmt.Printf("   - 预期的错误: %v\n", err)
	}

	fmt.Println("   - 重启池")
	testPool.Reboot()
	fmt.Printf("   - 池是否关闭: %v\n", testPool.IsClosed())

	fmt.Println("   - 提交任务到重启后的池")
	err = testPool.Submit(func() {
		fmt.Println("   - 重启后的任务执行成功！")
	})
	if err != nil {
		fmt.Printf("提交失败: %v\n", err)
	}

	time.Sleep(100 * time.Millisecond)
	testPool.Release()

	fmt.Println("\n=== 示例完成 ===")
}
