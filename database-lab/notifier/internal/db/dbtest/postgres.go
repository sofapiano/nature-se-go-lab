// Package dbtest поднимает временный PostgreSQL для тестов репозиториев.
package dbtest

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/igoroutine-courses/microservices.ecommerce.notifier/internal/db"
)

// NewPool запускает Postgres, применяет миграции сервиса и возвращает пул.
// Контейнер и пул закрываются автоматически после теста.
func NewPool(t *testing.T, serviceName, migrationsDir string) *pgxpool.Pool {
	t.Helper()

	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("ecommerce"),
		tcpostgres.WithUsername("ecommerce_user"),
		tcpostgres.WithPassword("12345"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, container.Terminate(context.Background())) })

	connString, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, connString)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	require.NoError(t, db.RunMigrations(ctx, pool, serviceName, migrationsDir))
	return pool
}
