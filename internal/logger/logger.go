package logger

import (
	"context"
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"
)

type ctxKey string

const logInfoKey ctxKey = "gin_metadata"

func InitLogger() {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	slog.SetDefault(slog.New(handler))
}

func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		attrs := []any{
			slog.String("ip", c.ClientIP()),
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
		}
		ctx := context.WithValue(c.Request.Context(), logInfoKey, attrs)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func logBase(ctx context.Context, level slog.Level, msg string, args ...any) {
	l := slog.Default()
	if !l.Enabled(ctx, level) {
		return
	}
	if attrs, ok := ctx.Value(logInfoKey).([]any); ok {
		args = append(args, attrs...)
	}
	l.Log(ctx, level, msg, args...)
}

func Debug(ctx context.Context, msg string, args ...any) {
	logBase(ctx, slog.LevelDebug, msg, args...)
}

func Info(ctx context.Context, msg string, args ...any) {
	logBase(ctx, slog.LevelInfo, msg, args...)
}

func Warn(ctx context.Context, msg string, args ...any) {
	logBase(ctx, slog.LevelWarn, msg, args...)
}

func Error(ctx context.Context, msg string, args ...any) {
	logBase(ctx, slog.LevelError, msg, args...)
}
