package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"google.golang.org/grpc"

	billingadapter "github.com/igoroutine-courses/microservices.ecommerce.loms/internal/adapter/billing/grpc"
	notifieradapter "github.com/igoroutine-courses/microservices.ecommerce.loms/internal/adapter/notifier/grpc"
	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/config"
	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/controller/interceptor"
	ordercontroller "github.com/igoroutine-courses/microservices.ecommerce.loms/internal/controller/order"
	productcontroller "github.com/igoroutine-courses/microservices.ecommerce.loms/internal/controller/product"
	stockscontroller "github.com/igoroutine-courses/microservices.ecommerce.loms/internal/controller/stocks"
	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/db"
	orderrepo "github.com/igoroutine-courses/microservices.ecommerce.loms/internal/repository/order"
	outboxrepo "github.com/igoroutine-courses/microservices.ecommerce.loms/internal/repository/outbox"
	productrepo "github.com/igoroutine-courses/microservices.ecommerce.loms/internal/repository/product"
	stocksrepo "github.com/igoroutine-courses/microservices.ecommerce.loms/internal/repository/stocks"
	orderusecase "github.com/igoroutine-courses/microservices.ecommerce.loms/internal/usecase/order"
	productusecase "github.com/igoroutine-courses/microservices.ecommerce.loms/internal/usecase/product"
	stocksusecase "github.com/igoroutine-courses/microservices.ecommerce.loms/internal/usecase/stocks"
	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/worker"
	"github.com/igoroutine-courses/microservices.ecommerce.loms/migrations"
	lomspb "github.com/igoroutine-courses/microservices.ecommerce.pkg/generated/loms/api/loms/v1"
	productpb "github.com/igoroutine-courses/microservices.ecommerce.pkg/generated/loms/api/product/v1"
	stockspb "github.com/igoroutine-courses/microservices.ecommerce.pkg/generated/loms/api/stocks/v1"
)

const shutdownTimeout = 10 * time.Second

func Run(ctx context.Context, cfg config.Config) error {
	pool, err := db.NewPool(ctx, db.Config{
		DSN:             fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", cfg.PG.User, cfg.PG.Password, cfg.PG.Host, cfg.PG.Port, cfg.PG.DB),
		MaxConns:        10,
		MinConns:        2,
		MaxConnLifetime: 30 * time.Minute, //nolint:mnd // эти числа являются стандартными константами алгоритмов или конфигурации.
	})
	if err != nil {
		return fmt.Errorf("create pool: %w", err)
	}
	defer pool.Close()

	if err := db.RunMigrations(ctx, pool, "loms", migrations.FS); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	transactor := db.NewTransactor(pool)
	slog.Info("loms database initialized", slog.Bool("transactor", transactor != nil))

	validation, err := interceptor.Validation()
	if err != nil {
		return fmt.Errorf("create validation interceptor: %w", err)
	}

	billingConn, err := billingadapter.Dial(cfg.Clients.BillingGrpcAddr)
	if err != nil {
		return fmt.Errorf("dial billing: %w", err)
	}
	defer func() { _ = billingConn.Close() }()
	billingClient := billingadapter.New(billingConn)

	productRepository := productrepo.NewPostgres(pool)
	productService := productusecase.New(productRepository)
	productController := productcontroller.New(productService)

	stocksRepository := stocksrepo.NewPostgres(pool, transactor)
	stocksService := stocksusecase.New(stocksRepository, productRepository)
	stocksController := stockscontroller.New(stocksService)

	notifierConn, err := notifieradapter.Dial(cfg.Clients.NotifierGrpcAddr)
	if err != nil {
		return fmt.Errorf("dial notifier: %w", err)
	}
	defer func() { _ = notifierConn.Close() }()
	notifierClient := notifieradapter.New(notifierConn)

	// Инициализируем репозитории для заказов и outbox
	outboxRepo := outboxrepo.NewPostgres(pool)
	orderRepository := orderrepo.NewPostgres(pool) // <-- ВОТ ЭТОЙ СТРОКИ НЕ ХВАТАЛО!

	orderService := orderusecase.New(
		orderRepository,
		productRepository,
		stocksRepository,
		billingClient,
		notifierClient,
		transactor,
		outboxRepo,
	)
	orderController := ordercontroller.New(orderService)

	server := grpc.NewServer(grpc.ChainUnaryInterceptor(validation))
	productpb.RegisterProductServiceServer(server, productController)
	stockspb.RegisterStocksServer(server, stocksController)
	lomspb.RegisterLomsServer(server, orderController)

	listener, err := net.Listen("tcp", ":"+cfg.GRPC.Port)
	if err != nil {
		return fmt.Errorf("listen on port %s: %w", cfg.GRPC.Port, err)
	}

	// Запускаем фоновый worker для outbox
	outboxWorker := worker.NewOutboxWorker(outboxRepo, notifierClient, 2*time.Second, 10)
	go outboxWorker.Start(ctx)

	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()

	slog.Info("loms started", slog.String("port", cfg.GRPC.Port))

	select {
	case err := <-serveErr:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	slog.Info("loms is shutting down")
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
