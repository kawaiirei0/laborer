# Laborer 示例代码

本目录包含 Laborer goroutine 池的使用示例。

## 示例列表

### 1. simple - 基础使用示例

展示 Laborer 的基本功能：

- 创建固定容量的池
- 提交任务到池中执行
- 查询池的状态信息
- 使用配置选项（过期时间、预分配、panic 处理）
- 非阻塞模式的使用
- 池的关闭和重启

**运行方式：**

```bash
cd examples/simple
go run main.go
```

### 2. with_func - 函数池使用示例

展示 PoolWithFunc 的使用，适用于执行相同类型的任务：

- 创建函数池处理数据
- 提交参数到固定函数执行
- 处理不同类型的数据（数字、字符串、结构体）
- 使用配置选项的函数池
- 非阻塞模式的函数池
- 查询函数池状态
- 带超时的关闭

**运行方式：**

```bash
cd examples/with_func
go run main.go
```

### 3. with_result - 带返回值任务示例

展示如何使用 Future 机制处理带返回值的任务：

- 提交带返回值的任务
- 使用 Future 获取任务结果
- 多个 Future 并发执行
- 处理任务执行错误
- 使用超时获取结果
- 处理复杂数据类型的返回值
- 批量处理并收集结果
- 检查任务完成状态

**运行方式：**

```bash
cd examples/with_result
go run main.go
```

## 配置选项说明

所有示例都展示了以下配置选项的使用：

- `WithExpiryDuration(duration)` - 设置 worker 空闲超时时间
- `WithPreAlloc(bool)` - 是否预分配 worker 切片
- `WithNonblocking(bool)` - 设置非阻塞模式
- `WithPanicHandler(func)` - 设置 panic 处理函数
- `WithLogger(logger)` - 设置日志记录器

## 构建示例

如果想构建可执行文件：

```bash
# 构建 simple 示例
cd examples/simple
go build -o simple.exe .

# 构建 with_func 示例
cd examples/with_func
go build -o with_func.exe .

# 构建 with_result 示例
cd examples/with_result
go build -o with_result.exe .
```

## 注意事项

1. 所有示例都使用 `replace` 指令引用本地的 laborer 包
2. 运行示例前确保已经在项目根目录执行 `go mod tidy`
3. 示例中的延时和容量设置仅用于演示，实际使用时应根据具体场景调整
