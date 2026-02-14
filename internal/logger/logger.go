package logger

import (
	"context"
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"
)

func InitLogger() {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	slog.SetDefault(slog.New(handler))
}

func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("app", "uber")
		c.Next()
	}
}

func logBase(ctx context.Context, level slog.Level, msg string, args ...any) {
	l := slog.Default()
	if !l.Enabled(ctx, level) {
		return
	}
	app, ok := ctx.Value("app").(string)
	if !ok {
		if gc, isGin := ctx.(*gin.Context); isGin {
			if val, exists := gc.Get("app"); exists {
				app, ok = val.(string)
			}
		}
	}
	if ok {
		l = l.With("app", app)
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
