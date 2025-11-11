package laborer

import "time"

// Options 定义了 goroutine 池的配置选项。
//
// 通过函数式选项模式，可以灵活地配置池的行为。
// 所有选项都有合理的默认值，可以根据实际需求进行调整。
//
// 示例:
//
//	pool, err := laborer.NewPool(
//	    100,
//	    laborer.WithExpiryDuration(30 * time.Second),
//	    laborer.WithPreAlloc(true),
//	    laborer.WithNonblocking(false),
//	)
//
// 默认配置常量
const (
	// DefaultCleanIntervalTime 默认清理间隔时间
	DefaultCleanIntervalTime = 1 * time.Second

	// DefaultExpiryDuration 默认 Worker 空闲超时时间
	DefaultExpiryDuration = 10 * time.Second
)

// Options 定义池的配置选项。
//
// 所有字段都有合理的默认值，可以通过 With* 函数进行自定义配置。
type Options struct {
	// ExpiryDuration 定义 Worker 的空闲超时时间。
	// 当 Worker 空闲时间超过此值时，将被回收以释放资源。
	// 默认值: 10 秒
	ExpiryDuration time.Duration

	// PreAlloc 指定是否预分配 worker 切片。
	// 启用后会在池创建时预先分配内存，适合容量固定的场景。
	// 默认值: false
	PreAlloc bool

	// MaxBlockingTasks 定义最大阻塞任务数量（当前未使用）。
	// 保留用于未来扩展。
	MaxBlockingTasks int

	// Nonblocking 指定池是否使用非阻塞模式。
	// 在非阻塞模式下，当池满时 Submit 会立即返回 ErrPoolOverload 错误。
	// 在阻塞模式下，Submit 会等待直到有可用的 worker。
	// 默认值: false（阻塞模式）
	Nonblocking bool

	// PanicHandler 定义任务执行时发生 panic 的处理函数。
	// 如果未设置，panic 会被记录到日志中。
	// 默认值: nil
	PanicHandler func(interface{})

	// Logger 定义日志记录器接口。
	// 用于记录池的运行状态和错误信息。
	// 默认值: 空日志记录器（不输出）
	Logger Logger
}

// Option 定义函数式选项类型。
//
// 使用函数式选项模式可以灵活地配置池的行为，
// 同时保持 API 的简洁性和向后兼容性。
type Option func(*Options)

// NewOptions 创建带有默认值的 Options 实例。
//
// 参数:
//   - opts: 可变数量的选项函数
//
// 返回:
//   - *Options: 配置好的选项实例
//
// 示例:
//
//	opts := laborer.NewOptions(
//	    laborer.WithExpiryDuration(30 * time.Second),
//	    laborer.WithPreAlloc(true),
//	)
func NewOptions(opts ...Option) *Options {
	options := &Options{
		ExpiryDuration: DefaultExpiryDuration,
		PreAlloc:       false,
		Nonblocking:    false,
		Logger:         newDefaultLogger(),
	}

	// 应用所有选项
	for _, opt := range opts {
		opt(options)
	}

	return options
}

// WithExpiryDuration 设置 Worker 的空闲超时时间。
//
// Worker 空闲时间超过此值后将被回收以释放资源。
// 较短的超时时间可以更快地释放资源，但可能导致频繁的 worker 创建/销毁。
// 较长的超时时间可以保持更多的 worker 可用，但会占用更多内存。
//
// 参数:
//   - duration: 超时时间，必须为正数
//
// 返回:
//   - Option: 配置选项函数
//
// 示例:
//
//	pool, _ := laborer.NewPool(10, laborer.WithExpiryDuration(30 * time.Second))
func WithExpiryDuration(duration time.Duration) Option {
	return func(opts *Options) {
		opts.ExpiryDuration = duration
	}
}

// WithPreAlloc 设置是否预分配 worker 切片。
//
// 启用预分配会在池创建时立即分配所有 worker 的内存空间，
// 这可以减少运行时的内存分配开销，但会增加初始内存使用。
// 适合容量固定且已知的场景。
//
// 参数:
//   - preAlloc: true 表示启用预分配，false 表示按需分配
//
// 返回:
//   - Option: 配置选项函数
//
// 示例:
//
//	pool, _ := laborer.NewPool(100, laborer.WithPreAlloc(true))
func WithPreAlloc(preAlloc bool) Option {
	return func(opts *Options) {
		opts.PreAlloc = preAlloc
	}
}

// WithMaxBlockingTasks 设置最大阻塞任务数量。
//
// 此选项当前保留用于未来扩展，暂未实现具体功能。
//
// 参数:
//   - maxBlockingTasks: 最大阻塞任务数量
//
// 返回:
//   - Option: 配置选项函数
func WithMaxBlockingTasks(maxBlockingTasks int) Option {
	return func(opts *Options) {
		opts.MaxBlockingTasks = maxBlockingTasks
	}
}

// WithNonblocking 设置池的阻塞模式。
//
// 在非阻塞模式下，当池满时 Submit 会立即返回 ErrPoolOverload 错误。
// 在阻塞模式下，Submit 会等待直到有可用的 worker。
//
// 非阻塞模式适合对延迟敏感的应用，可以快速失败并采取备用策略。
// 阻塞模式适合可以容忍等待的应用，确保所有任务最终都会被执行。
//
// 参数:
//   - nonblocking: true 表示非阻塞模式，false 表示阻塞模式
//
// 返回:
//   - Option: 配置选项函数
//
// 示例:
//
//	// 非阻塞模式
//	pool, _ := laborer.NewPool(10, laborer.WithNonblocking(true))
//	if err := pool.Submit(task); err == laborer.ErrPoolOverload {
//	    // 处理过载情况
//	}
func WithNonblocking(nonblocking bool) Option {
	return func(opts *Options) {
		opts.Nonblocking = nonblocking
	}
}

// WithPanicHandler 设置任务执行时的 panic 处理函数。
//
// 当任务执行过程中发生 panic 时，会调用此处理函数。
// 如果未设置，panic 会被记录到日志中。
// 处理函数可以用于记录错误、发送告警或执行清理操作。
//
// 参数:
//   - panicHandler: panic 处理函数，接收 panic 的值作为参数
//
// 返回:
//   - Option: 配置选项函数
//
// 示例:
//
//	pool, _ := laborer.NewPool(10, laborer.WithPanicHandler(func(err interface{}) {
//	    log.Printf("Task panicked: %v", err)
//	    metrics.RecordPanic(err)
//	}))
func WithPanicHandler(panicHandler func(interface{})) Option {
	return func(opts *Options) {
		opts.PanicHandler = panicHandler
	}
}

// WithLogger 设置自定义日志记录器。
//
// 日志记录器用于记录池的运行状态、错误信息和调试信息。
// 必须实现 Logger 接口。
//
// 参数:
//   - logger: 实现了 Logger 接口的日志记录器
//
// 返回:
//   - Option: 配置选项函数
//
// 示例:
//
//	type MyLogger struct{}
//	func (l *MyLogger) Printf(format string, args ...interface{}) {
//	    log.Printf(format, args...)
//	}
//
//	pool, _ := laborer.NewPool(10, laborer.WithLogger(&MyLogger{}))
func WithLogger(logger Logger) Option {
	return func(opts *Options) {
		opts.Logger = logger
	}
}
