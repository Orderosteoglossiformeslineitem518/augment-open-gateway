package logger

import (
	"context"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	globalLogger *zap.SugaredLogger
	once         sync.Once
	enabled      = true
)

// ContextKey 用于在 context 中存储追踪ID
type ContextKey string

const (
	TraceIDKey ContextKey = "trace_id"
)

// Config 日志配置
type Config struct {
	Enabled bool   // 是否启用日志（默认 true，只打印到控制台不写文件）
	Level   string // 日志级别：debug/info/warn/error
	Format  string // 日志格式：json/text
}

// customTimeEncoder 自定义时间格式编码器（东八区，格式：2006-01-02 15:04:05）
func customTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.In(time.FixedZone("CST", 8*3600)).Format("2006-01-02 15:04:05"))
}

// Init 初始化日志
func Init(cfg *Config) {
	once.Do(func() {
		if cfg == nil {
			cfg = &Config{
				Enabled: true,
				Level:   "info",
				Format:  "text",
			}
		}

		enabled = cfg.Enabled

		if !enabled {
			// 禁用日志时使用 nop logger
			globalLogger = zap.NewNop().Sugar()
			return
		}

		// 解析日志级别
		var level zapcore.Level
		switch cfg.Level {
		case "debug":
			level = zapcore.DebugLevel
		case "info":
			level = zapcore.InfoLevel
		case "warn":
			level = zapcore.WarnLevel
		case "error":
			level = zapcore.ErrorLevel
		default:
			level = zapcore.InfoLevel
		}

		// 配置编码器
		var encoder zapcore.Encoder
		encoderConfig := zapcore.EncoderConfig{
			TimeKey:        "time",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "", // 不显示调用位置
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     customTimeEncoder, // 使用自定义时间格式
			EncodeDuration: zapcore.SecondsDurationEncoder,
		}

		if cfg.Format == "text" {
			encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
			encoderConfig.ConsoleSeparator = " " // 使用单个空格作为分隔符
			encoder = zapcore.NewConsoleEncoder(encoderConfig)
		} else {
			encoder = zapcore.NewJSONEncoder(encoderConfig)
		}

		// 创建 core
		core := zapcore.NewCore(
			encoder,
			zapcore.AddSync(os.Stdout),
			level,
		)

		// 创建 logger（不添加 caller 信息）
		logger := zap.New(core)
		globalLogger = logger.Sugar()
	})
}

// GetLogger 获取全局 logger
func GetLogger() *zap.SugaredLogger {
	if globalLogger == nil {
		Init(nil)
	}
	return globalLogger
}

// WithTraceID 添加追踪ID到日志
func WithTraceID(traceID string) *zap.SugaredLogger {
	if !enabled || globalLogger == nil {
		return zap.NewNop().Sugar()
	}
	return globalLogger.With("trace_id", traceID)
}

// WithContext 从 context 中获取追踪ID并添加到日志
func WithContext(ctx context.Context) *zap.SugaredLogger {
	if !enabled || globalLogger == nil {
		return zap.NewNop().Sugar()
	}
	if ctx == nil {
		return globalLogger
	}
	if traceID, ok := ctx.Value(TraceIDKey).(string); ok && traceID != "" {
		return globalLogger.With("trace_id", traceID)
	}
	return globalLogger
}

// WithFields 添加自定义字段
func WithFields(fields ...interface{}) *zap.SugaredLogger {
	if !enabled || globalLogger == nil {
		return zap.NewNop().Sugar()
	}
	return globalLogger.With(fields...)
}

// Debug 调试日志
func Debug(args ...interface{}) {
	if enabled && globalLogger != nil {
		globalLogger.Debug(args...)
	}
}

// Debugf 格式化调试日志
func Debugf(template string, args ...interface{}) {
	if enabled && globalLogger != nil {
		globalLogger.Debugf(template, args...)
	}
}

// Info 信息日志
func Info(args ...interface{}) {
	if enabled && globalLogger != nil {
		globalLogger.Info(args...)
	}
}

// Infof 格式化信息日志
func Infof(template string, args ...interface{}) {
	if enabled && globalLogger != nil {
		globalLogger.Infof(template, args...)
	}
}

// Warn 警告日志
func Warn(args ...interface{}) {
	if enabled && globalLogger != nil {
		globalLogger.Warn(args...)
	}
}

// Warnf 格式化警告日志
func Warnf(template string, args ...interface{}) {
	if enabled && globalLogger != nil {
		globalLogger.Warnf(template, args...)
	}
}

// Error 错误日志
func Error(args ...interface{}) {
	if enabled && globalLogger != nil {
		globalLogger.Error(args...)
	}
}

// Errorf 格式化错误日志
func Errorf(template string, args ...interface{}) {
	if enabled && globalLogger != nil {
		globalLogger.Errorf(template, args...)
	}
}

// Fatal 致命错误日志（会退出程序）
func Fatal(args ...interface{}) {
	if enabled && globalLogger != nil {
		globalLogger.Fatal(args...)
	}
}

// Fatalf 格式化致命错误日志（会退出程序）
func Fatalf(template string, args ...interface{}) {
	if enabled && globalLogger != nil {
		globalLogger.Fatalf(template, args...)
	}
}

// Sync 刷新日志缓冲
func Sync() error {
	if globalLogger != nil {
		return globalLogger.Sync()
	}
	return nil
}
