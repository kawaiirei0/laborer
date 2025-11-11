package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kawaiirei0/laborer"
)

func main() {
	fmt.Println("=== Laborer 函数池使用示例 ===\n")

	// 示例 1: 创建函数池处理数据
	fmt.Println("1. 创建函数池处理数字求和（容量: 10）")

	var sum int64
	sumFunc := func(i interface{}) {
		num := i.(int)
		atomic.AddInt64(&sum, int64(num))
		time.Sleep(10 * time.Millisecond)
	}

	pool, err := laborer.NewPoolWithFunc(10, sumFunc)
	if err != nil {
		panic(err)
	}
	defer pool.Release()

	// 提交参数到函数池
	fmt.Println("2. 提交 100 个数字到函数池")
	var wg sync.WaitGroup
	for i := 1; i <= 100; i++ {
		wg.Add(1)
		num := i
		err := pool.Invoke(num)
		if err != nil {
			fmt.Printf("提交参数失败: %v\n", err)
			wg.Done()
			continue
		}

		// 使用 goroutine 等待任务完成
		go func() {
			defer wg.Done()
			time.Sleep(20 * time.Millisecond)
		}()
	}

	wg.Wait()
	fmt.Printf("所有任务完成，累加结果: %d (预期: 5050)\n\n", sum)

	// 示例 2: 使用函数池处理字符串
	fmt.Println("3. 创建函数池处理字符串转换")

	var results sync.Map
	stringFunc := func(i interface{}) {
		data := i.(map[string]interface{})
		id := data["id"].(int)
		text := data["text"].(string)

		// 模拟处理
		processed := fmt.Sprintf("[处理完成] ID:%d - %s", id, text)
		results.Store(id, processed)
	}

	stringPool, err := laborer.NewPoolWithFunc(5, stringFunc)
	if err != nil {
		panic(err)
	}
	defer stringPool.Release()

	// 提交数据
	fmt.Println("4. 提交 10 条数据进行处理")
	for i := 1; i <= 10; i++ {
		data := map[string]interface{}{
			"id":   i,
			"text": fmt.Sprintf("数据-%d", i),
		}
		err := stringPool.Invoke(data)
		if err != nil {
			fmt.Printf("提交数据失败: %v\n", err)
		}
	}

	// 等待处理完成
	time.Sleep(200 * time.Millisecond)

	fmt.Println("5. 处理结果:")
	results.Range(func(key, value interface{}) bool {
		fmt.Printf("   %v\n", value)
		return true
	})
	fmt.Println()

	// 示例 3: 使用配置选项的函数池
	fmt.Println("6. 创建带配置选项的函数池")

	panicCount := int32(0)
	processFunc := func(i interface{}) {
		num := i.(int)
		if num%10 == 0 {
			panic(fmt.Sprintf("数字 %d 触发 panic", num))
		}
		fmt.Printf("   处理数字: %d\n", num)
	}

	configuredPool, err := laborer.NewPoolWithFunc(
		3,
		processFunc,
		laborer.WithExpiryDuration(3*time.Second),
		laborer.WithPreAlloc(true),
		laborer.WithPanicHandler(func(p interface{}) {
			atomic.AddInt32(&panicCount, 1)
			fmt.Printf("   [Panic 处理器] 捕获: %v\n", p)
		}),
	)
	if err != nil {
		panic(err)
	}
	defer configuredPool.Release()

	fmt.Println("7. 提交包含会 panic 的数据")
	for i := 1; i <= 15; i++ {
		configuredPool.Invoke(i)
		time.Sleep(10 * time.Millisecond)
	}

	time.Sleep(200 * time.Millisecond)
	fmt.Printf("\n捕获的 panic 次数: %d\n\n", panicCount)

	// 示例 4: 非阻塞模式的函数池
	fmt.Println("8. 创建非阻塞模式的函数池（容量: 2）")

	slowFunc := func(i interface{}) {
		num := i.(int)
		fmt.Printf("   处理任务 %d\n", num)
		time.Sleep(500 * time.Millisecond)
	}

	nonblockingPool, err := laborer.NewPoolWithFunc(
		2,
		slowFunc,
		laborer.WithNonblocking(true),
	)
	if err != nil {
		panic(err)
	}
	defer nonblockingPool.Release()

	fmt.Println("9. 快速提交 6 个任务")
	successCount := 0
	for i := 1; i <= 6; i++ {
		err := nonblockingPool.Invoke(i)
		if err != nil {
			fmt.Printf("   任务 %d 提交失败: %v\n", i, err)
		} else {
			successCount++
		}
	}

	fmt.Printf("\n成功提交任务数: %d\n\n", successCount)
	time.Sleep(1 * time.Second)

	// 示例 5: 查询函数池状态
	fmt.Println("10. 查询函数池状态")
	statusPool, _ := laborer.NewPoolWithFunc(8, func(i interface{}) {
		time.Sleep(100 * time.Millisecond)
	})
	defer statusPool.Release()

	// 提交一些任务
	for i := 0; i < 5; i++ {
		statusPool.Invoke(i)
	}

	fmt.Printf("   - 池容量: %d\n", statusPool.Cap())
	fmt.Printf("   - 运行中的 worker: %d\n", statusPool.Running())
	fmt.Printf("   - 空闲的 worker: %d\n", statusPool.Free())
	fmt.Printf("   - 等待的任务: %d\n", statusPool.Waiting())
	fmt.Printf("   - 池是否关闭: %v\n\n", statusPool.IsClosed())

	time.Sleep(200 * time.Millisecond)

	// 示例 6: 带超时的关闭
	fmt.Println("11. 测试带超时的关闭")
	timeoutPool, _ := laborer.NewPoolWithFunc(3, func(i interface{}) {
		time.Sleep(2 * time.Second)
	})

	// 提交长时间运行的任务
	for i := 0; i < 3; i++ {
		timeoutPool.Invoke(i)
	}

	fmt.Println("   - 尝试在 500ms 内关闭池（任务需要 2s）")
	err = timeoutPool.ReleaseTimeout(500 * time.Millisecond)
	if err != nil {
		fmt.Printf("   - 预期的超时错误: %v\n", err)
	}

	fmt.Println("\n=== 示例完成 ===")
}
