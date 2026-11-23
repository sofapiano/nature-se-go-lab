package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/app"
	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/config"
)

func main() {
	if err := run(); err != nil {
		slog.Error("loms failed", slog.Any("error", err))
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	return app.Run(ctx, cfg)
}
