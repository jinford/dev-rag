package logger

import (
	"log/slog"
	"os"
)

// Config はロガーの設定
type Config struct {
	Level  slog.Level
	Format string // "json" or "text"
}

// DefaultConfig はデフォルトのロガー設定
func DefaultConfig() Config {
	return Config{
		Level:  slog.LevelInfo,
		Format: "json",
	}
}

// New は新しいロガーを作成し、デフォルトロガーとして設定します
func New(cfg Config) *slog.Logger {
	var handler slog.Handler

	opts := &slog.HandlerOptions{
		Level: cfg.Level,
	}

	switch cfg.Format {
	case "text":
		handler = slog.NewTextHandler(os.Stdout, opts)
	default: // "json"
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger
}
