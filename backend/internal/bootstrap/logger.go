package bootstrap

import (
	"log/slog"
	"os"
	"strings"

	"github.com/duclamdev/application-chat/backend/internal/config"
)

func configureLogger(cfg *config.Config) {
	level := slog.LevelInfo
	switch strings.ToLower(cfg.App.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if strings.EqualFold(cfg.App.LogFormat, "json") {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler).With(
		"app", cfg.App.Name,
		"env", cfg.App.Env,
		"service", cfg.App.ServiceName,
	))
}
