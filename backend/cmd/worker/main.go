package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/duclamdev/application-chat/backend/internal/bootstrap"
	"github.com/duclamdev/application-chat/backend/internal/config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("Không đọc được cấu hình", "error", err)
		os.Exit(1)
	}

	worker, err := bootstrap.NewWorker(cfg)
	if err != nil {
		slog.Error("Không khởi tạo được worker", "error", err)
		os.Exit(1)
	}

	if err := worker.Run(ctx); err != nil {
		slog.Error("Worker dừng với lỗi", "error", err)
		os.Exit(1)
	}
}
