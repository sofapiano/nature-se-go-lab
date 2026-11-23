package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/igoroutine-courses/microservices.ecommerce.notifier/internal/adapter/telegram"
	"github.com/igoroutine-courses/microservices.ecommerce.notifier/internal/config"
	"github.com/igoroutine-courses/microservices.ecommerce.notifier/internal/controller/interceptor"
	notificationcontroller "github.com/igoroutine-courses/microservices.ecommerce.notifier/internal/controller/notification"
	"github.com/igoroutine-courses/microservices.ecommerce.notifier/internal/db"
	"github.com/igoroutine-courses/microservices.ecommerce.notifier/internal/repository/binding"
	inboxrepo "github.com/igoroutine-courses/microservices.ecommerce.notifier/internal/repository/inbox"
	notificationusecase "github.com/igoroutine-courses/microservices.ecommerce.notifier/internal/usecase/notification"
	"github.com/igoroutine-courses/microservices.ecommerce.notifier/internal/worker"
	"github.com/igoroutine-courses/microservices.ecommerce.notifier/migrations"
	notifierpb "github.com/igoroutine-courses/microservices.ecommerce.pkg/generated/notifier/api/v1"
)

const (
	shutdownTimeout = 10 * time.Second
	telegramTimeout = 5 * time.Second
)

func Run(ctx context.Context, cfg *config.Config) error {
	pool, err := db.NewPool(ctx, db.Config{
		DSN: fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
			cfg.PG.User, cfg.PG.Password, cfg.PG.Host, cfg.PG.Port, cfg.PG.DB),
		MaxConns:        10,
		MinConns:        2,
		MaxConnLifetime: 30 * time.Minute, //nolint:mnd // эти числа являются стандартными константами алгоритмов или конфигурации.
	})
	if err != nil {
		return fmt.Errorf("create pool: %w", err)
	}
	defer pool.Close()

	if err := db.RunMigrations(ctx, pool, "notifier", migrations.FS); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	transactor := db.NewTransactor(pool)
	slog.Info("notifier database initialized", slog.Bool("transactor", transactor != nil))

	validation, err := interceptor.Validation()
	if err != nil {
		return fmt.Errorf("create validation interceptor: %w", err)
	}

	telegramClient := telegram.New(cfg.Telegram.BaseURL, cfg.Telegram.Token, &http.Client{
		Timeout: telegramTimeout,
	})

	repo := binding.NewPostgres(pool)
	inboxRepo := inboxrepo.NewPostgres(pool)
	svc := notificationusecase.New(telegramClient, repo, inboxRepo)
	ctrl := notificationcontroller.New(svc)

	server := grpc.NewServer(grpc.ChainUnaryInterceptor(validation))
	notifierpb.RegisterNotifierServer(server, ctrl)

	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(server, healthSrv)
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	listener, err := net.Listen("tcp", ":"+cfg.GRPC.Port)
	if err != nil {
		return fmt.Errorf("listen on port %s: %w", cfg.GRPC.Port, err)
	}

	// Фоновый воркер доставки: забирает задачи из inbox и шлёт в Telegram.
	inboxWorker := worker.NewInboxWorker(inboxRepo, svc, 2*time.Second, 10)
	go inboxWorker.Start(ctx)

	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()

	slog.Info("notifier started", slog.String("port", cfg.GRPC.Port))

	select {
	case err := <-serveErr:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	slog.Info("notifier is shutting down")
	stopGracefully(server)
	return nil
}

func stopGracefully(server *grpc.Server) {
	stopped := make(chan struct{})
	go func() {
		server.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
	case <-time.After(shutdownTimeout):
		server.Stop()
	}
}
