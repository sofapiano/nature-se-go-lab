package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"buf.build/go/protovalidate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	lomsgrpc "github.com/igoroutine-courses/microservices.ecommerce.cart/internal/adapter/loms/grpc"
	"github.com/igoroutine-courses/microservices.ecommerce.cart/internal/config"
	"github.com/igoroutine-courses/microservices.ecommerce.cart/internal/controller/cart"
	"github.com/igoroutine-courses/microservices.ecommerce.cart/internal/db"
	cartrepo "github.com/igoroutine-courses/microservices.ecommerce.cart/internal/repository/cart"
	checkoutrepo "github.com/igoroutine-courses/microservices.ecommerce.cart/internal/repository/checkout"
	cartusecase "github.com/igoroutine-courses/microservices.ecommerce.cart/internal/usecase/cart"
	cartpb "github.com/igoroutine-courses/microservices.ecommerce.pkg/generated/cart/api/cart/v1"
)

func Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.New()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	pool, err := db.NewPool(ctx, db.Config{
		DSN: fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
			cfg.PG.User, cfg.PG.Password, cfg.PG.Host, cfg.PG.Port, cfg.PG.DB),
		MaxConns:        10,
		MinConns:        2,
		MaxConnLifetime: 30 * time.Minute,
	})
	if err != nil {
		return fmt.Errorf("create pool: %w", err)
	}
	defer pool.Close()

	if err := db.RunMigrations(ctx, pool, "cart", "migrations"); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	transactor := db.NewTransactor(pool)
	slog.Info("cart database initialized", slog.Bool("transactor", transactor != nil))

	lomsConn, err := lomsgrpc.Dial(cfg.Clients.LOMSGrpcAddr)
	if err != nil {
		return fmt.Errorf("dial loms: %w", err)
	}
	defer func() { _ = lomsConn.Close() }()

	lomsClient := lomsgrpc.New(lomsConn)
	cartRepo := cartrepo.NewPostgres(pool, transactor)
	checkoutRepo := checkoutrepo.NewPostgres(pool)
	svc := cartusecase.New(cartRepo, checkoutRepo, lomsClient, lomsClient)

	validator, err := protovalidate.New()
	if err != nil {
		return fmt.Errorf("create validator: %w", err)
	}

	server := grpc.NewServer()
	cartpb.RegisterCartServer(server, cart.New(svc, validator))

	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(server, healthSrv)
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	listener, err := net.Listen("tcp", ":"+cfg.GRPC.Port)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	go func() {
		<-ctx.Done()
		server.GracefulStop()
	}()

	slog.Info("cart gRPC server started", "addr", listener.Addr().String())
	if err := server.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		return err
	}
	return nil
}
