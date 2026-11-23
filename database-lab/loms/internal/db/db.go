package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config хранит параметры для подключения к PostgreSQL.
// Мы выносим их в отдельную структуру, чтобы не передавать 5 разных строк в функцию.
type Config struct {
	DSN             string        // Строка подключения (например, "postgres://user:pass@localhost:5432/db")
	MaxConns        int32         // Максимальное количество соединений в пуле
	MinConns        int32         // Минимальное количество соединений
	MaxConnLifetime time.Duration // Максимальное время жизни соединения
}

// NewPool создаёт и настраивает пул соединений с базой данных.
// Пул соединений (Connection Pool) — это заранее созданный набор подключений к БД,
// которые можно переиспользовать, вместо того чтобы создавать новое подключение для каждого запроса.
func NewPool(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}

	// Настраиваем параметры пула для устойчивости под нагрузкой
	poolConfig.MaxConns = cfg.MaxConns
	poolConfig.MinConns = cfg.MinConns
	poolConfig.MaxConnLifetime = cfg.MaxConnLifetime

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create pgxpool: %w", err)
	}

	// Проверяем, что база данных действительно доступна
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}
