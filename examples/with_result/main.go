package main

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/kawaiirei0/laborer"
)

func main() {
	fmt.Println("=== Laborer 带返回值任务示例 ===\n")

	// 示例 1: 基本的 Future 使用
	fmt.Println("1. 创建池并提交带返回值的任务")
	pool, err := laborer.NewPool(5)
	if err != nil {
		panic(err)
	}
	defer pool.Release()

	// 提交计算任务
	future, err := pool.SubmitWithResult(func() (interface{}, error) {
		time.Sleep(100 * time.Millisecond)
		result := 42 * 2
		return result, nil
	})
	if err != nil {
		panic(err)
	}

	fmt.Println("2. 等待任务完成并获取结果")
	result, err := future.Get()
	if err != nil {
		fmt.Printf("获取结果失败: %v\n", err)
	} else {
		fmt.Printf("计算结果: %v\n\n", result)
	}

	// 示例 2: 多个 Future 并发执行
	fmt.Println("3. 提交多个带返回值的任务")
	futures := make([]laborer.Future, 10)

	for i := 0; i < 10; i++ {
		num := i
		f, err := pool.SubmitWithResult(func() (interface{}, error) {
			time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
			return num * num, nil
		})
		if err != nil {
			fmt.Printf("提交任务 %d 失败: %v\n", i, err)
			continue
		}
		futures[i] = f
	}

	fmt.Println("4. 获取所有任务的结果")
	for i, f := range futures {
		if f == nil {
			continue
		}
		result, err := f.Get()
		if err != nil {
			fmt.Printf("   任务 %d 失败: %v\n", i, err)
		} else {
			fmt.Printf("   任务 %d 结果: %v\n", i, result)
		}
	}
	fmt.Println()

	// 示例 3: 处理错误的任务
	fmt.Println("5. 提交会返回错误的任务")
	errorFuture, err := pool.SubmitWithResult(func() (interface{}, error) {
		time.Sleep(50 * time.Millisecond)
		return nil, errors.New("模拟的错误")
	})
	if err != nil {
		panic(err)
	}

	result, err = errorFuture.Get()
	if err != nil {
		fmt.Printf("预期的错误: %v\n\n", err)
	}

	// 示例 4: 使用超时获取结果
	fmt.Println("6. 使用超时获取结果")

	// 提交一个长时间运行的任务
	slowFuture, err := pool.SubmitWithResult(func() (interface{}, error) {
		time.Sleep(2 * time.Second)
		return "慢任务完成", nil
	})
	if err != nil {
		panic(err)
	}

	// 尝试在短时间内获取结果
	fmt.Println("   - 尝试在 500ms 内获取结果（任务需要 2s）")
	result, err = slowFuture.GetWithTimeout(500 * time.Millisecond)
	if err != nil {
		fmt.Printf("   - 预期的超时错误: %v\n", err)
	}

	// 检查任务是否完成
	fmt.Printf("   - 任务是否完成: %v\n\n", slowFuture.IsDone())

	// 示例 5: 复杂数据类型的返回值
	fmt.Println("7. 处理复杂数据类型的返回值")

	type UserData struct {
		ID   int
		Name string
		Age  int
	}

	userFuture, err := pool.SubmitWithResult(func() (interface{}, error) {
		time.Sleep(100 * time.Millisecond)
		user := UserData{
			ID:   1001,
			Name: "张三",
			Age:  25,
		}
		return user, nil
	})
	if err != nil {
		panic(err)
	}

	result, err = userFuture.Get()
	if err != nil {
		fmt.Printf("获取用户数据失败: %v\n", err)
	} else {
		user := result.(UserData)
		fmt.Printf("用户数据: ID=%d, Name=%s, Age=%d\n\n", user.ID, user.Name, user.Age)
	}

	// 示例 6: 批量处理并收集结果
	fmt.Println("8. 批量处理数据并收集结果")

	type Task struct {
		ID   int
		Data string
	}

	type Result struct {
		TaskID    int
		Processed string
		Success   bool
	}

	tasks := []Task{
		{ID: 1, Data: "数据A"},
		{ID: 2, Data: "数据B"},
		{ID: 3, Data: "数据C"},
		{ID: 4, Data: "数据D"},
		{ID: 5, Data: "数据E"},
	}

	resultFutures := make([]laborer.Future, len(tasks))

	// 提交所有任务
	for i, task := range tasks {
		t := task
		f, err := pool.SubmitWithResult(func() (interface{}, error) {
			time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)

			// 模拟处理
			processed := fmt.Sprintf("[已处理] %s", t.Data)

			return Result{
				TaskID:    t.ID,
				Processed: processed,
				Success:   true,
			}, nil
		})
		if err != nil {
			fmt.Printf("提交任务 %d 失败: %v\n", task.ID, err)
			continue
		}
		resultFutures[i] = f
	}

	// 收集所有结果
	fmt.Println("9. 收集处理结果:")
	for i, f := range resultFutures {
		if f == nil {
			continue
		}

		result, err := f.Get()
		if err != nil {
			fmt.Printf("   任务 %d 失败: %v\n", i+1, err)
			continue
		}

		r := result.(Result)
		fmt.Printf("   任务 %d: %s (成功: %v)\n", r.TaskID, r.Processed, r.Success)
	}
	fmt.Println()

	// 示例 7: 检查任务完成状态
	fmt.Println("10. 检查任务完成状态")

	checkFuture, err := pool.SubmitWithResult(func() (interface{}, error) {
		time.Sleep(200 * time.Millisecond)
		return "任务完成", nil
	})
	if err != nil {
		panic(err)
	}

	// 轮询检查任务状态
	fmt.Println("   - 轮询检查任务状态:")
	for i := 0; i < 5; i++ {
		isDone := checkFuture.IsDone()
		fmt.Printf("   - 第 %d 次检查: 任务完成=%v\n", i+1, isDone)
		if isDone {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	result, _ = checkFuture.Get()
	fmt.Printf("   - 最终结果: %v\n\n", result)

	// 示例 8: 组合使用配置选项
	fmt.Println("11. 使用配置选项的池处理带返回值任务")

	configuredPool, err := laborer.NewPool(
		3,
		laborer.WithExpiryDuration(5*time.Second),
		laborer.WithPanicHandler(func(p interface{}) {
			fmt.Printf("   [Panic 处理器] 捕获: %v\n", p)
		}),
	)
	if err != nil {
		panic(err)
	}
	defer configuredPool.Release()

	// 提交会 panic 的任务
	_, err = configuredPool.SubmitWithResult(func() (interface{}, error) {
		panic("测试 panic")
	})
	if err != nil {
		panic(err)
	}

	// 等待一下让 panic 被处理
	time.Sleep(100 * time.Millisecond)

	// 提交正常任务验证池仍然工作
	normalFuture, err := configuredPool.SubmitWithResult(func() (interface{}, error) {
		return "池仍然正常工作", nil
	})
	if err != nil {
		panic(err)
	}

	result, err = normalFuture.Get()
	if err != nil {
		fmt.Printf("获取结果失败: %v\n", err)
	} else {
		fmt.Printf("   - %v\n", result)
	}

	// 注意：panic 的任务的 future 会一直阻塞，因为结果永远不会被设置
	// 在实际应用中，应该使用 GetWithTimeout 来避免永久阻塞
	fmt.Println("   - 注意: panic 的任务不会返回结果")

	fmt.Println("\n=== 示例完成 ===")
}
