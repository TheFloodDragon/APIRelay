package logger

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type ctxKey struct{}

// RequestIDKey 是 gin.Context 中存放 request_id 的键名。
const RequestIDKey = "request_id"

var global *zap.Logger

// Init 初始化全局 logger。
// level: debug/info/warn/error；format: json/console；dir 非空时同时写文件。
func Init(level, format, dir string) error {
	lvl := zapcore.InfoLevel
	_ = lvl.UnmarshalText([]byte(level))

	encCfg := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.TimeEncoderOfLayout(time.RFC3339),
		EncodeDuration: zapcore.MillisDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	var encoder zapcore.Encoder
	if format == "json" {
		encoder = zapcore.NewJSONEncoder(encCfg)
	} else {
		encoder = zapcore.NewConsoleEncoder(encCfg)
	}

	cores := []zapcore.Core{
		zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), lvl),
	}

	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		f, err := os.OpenFile(filepath.Join(dir, "apirelay.log"),
			os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(f), lvl))
	}

	global = zap.New(zapcore.NewTee(cores...), zap.AddCaller(), zap.AddCallerSkip(0))
	return nil
}

// L 返回全局 logger（未初始化时返回 Nop）。
func L() *zap.Logger {
	if global == nil {
		return zap.NewNop()
	}
	return global
}

// WithContext 把带字段的 logger 存入 context。
func WithContext(ctx context.Context, l *zap.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext 从 context 取出 logger，没有则返回全局 logger。
func FromContext(ctx context.Context) *zap.Logger {
	if ctx != nil {
		if l, ok := ctx.Value(ctxKey{}).(*zap.Logger); ok && l != nil {
			return l
		}
	}
	return L()
}

// Sync 刷新缓冲（程序退出前调用）。
func Sync() {
	if global != nil {
		_ = global.Sync()
	}
}
