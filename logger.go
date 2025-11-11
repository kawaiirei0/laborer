package laborer

// Logger 定义日志记录接口。
//
// 实现此接口可以自定义池的日志输出行为。
// 池会使用此接口记录运行状态、错误信息和调试信息。
//
// 标准库的 log.Logger 实现了此接口，可以直接使用。
//
// 示例:
//
//	import "log"
//
//	pool, _ := laborer.NewPool(10,
//	    laborer.WithLogger(log.Default()))
//
// 自定义实现示例:
//
//	type MyLogger struct {
//	    logger *zap.Logger
//	}
//
//	func (l *MyLogger) Printf(format string, args ...interface{}) {
//	    l.logger.Sugar().Infof(format, args...)
//	}
//
//	pool, _ := laborer.NewPool(10,
//	    laborer.WithLogger(&MyLogger{logger: zapLogger}))
type Logger interface {
	// Printf 格式化输出日志。
	//
	// 此方法的签名与标准库 log.Logger.Printf 相同，
	// 便于直接使用标准库的日志记录器。
	//
	// 参数:
	//  - format: 格式化字符串，使用 fmt 包的格式化语法
	//  - args: 格式化参数
	//
	// 示例:
	//  logger.Printf("Worker %d expired", workerID)
	Printf(format string, args ...interface{})
}

// defaultLogger 是默认的空日志实现。
//
// 此实现不输出任何日志，用于在用户未指定日志记录器时使用。
// 这样可以避免不必要的日志输出，同时保持代码的简洁性。
type defaultLogger struct{}

// Printf 实现 Logger.Printf 接口。
//
// 此实现为空操作，不输出任何日志。
func (l *defaultLogger) Printf(format string, args ...interface{}) {
	// 空实现，不输出日志
	// 用户可以通过 WithLogger 选项提供自定义的日志记录器
}

// newDefaultLogger 创建默认的空日志记录器。
//
// 此函数由池内部调用，用户不应直接调用。
//
// 返回:
//   - Logger: 默认的空日志记录器实例
func newDefaultLogger() Logger {
	return &defaultLogger{}
}
