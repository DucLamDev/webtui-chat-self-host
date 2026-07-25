package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/duclamdev/application-chat/backend/internal/config"
	"github.com/duclamdev/application-chat/backend/internal/platform/database"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if len(os.Args) < 2 || (os.Args[1] != "up" && os.Args[1] != "down") {
		slog.Error("Lệnh migrate phải là: go run ./cmd/migrate up|down [steps]")
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("Không đọc được cấu hình", "error", err)
		os.Exit(1)
	}

	db, err := database.NewPostgres(ctx, cfg.Database)
	if err != nil {
		slog.Error("Không kết nối được cơ sở dữ liệu", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	runner := database.NewMigrationRunner(db, cfg.Database.MigrationsPath)
	var migrationErr error
	if os.Args[1] == "up" {
		migrationErr = runner.Up(ctx)
	} else {
		steps := 1
		if len(os.Args) > 2 {
			steps, err = strconv.Atoi(os.Args[2])
			if err != nil || steps <= 0 {
				slog.Error("Số bước rollback phải là số nguyên dương")
				os.Exit(1)
			}
		}
		migrationErr = runner.Down(ctx, steps)
	}
	if migrationErr != nil {
		slog.Error("Chạy migration thất bại", "error", migrationErr)
		os.Exit(1)
	}

	slog.Info("Chạy migration thành công", "command", os.Args[1])
}
