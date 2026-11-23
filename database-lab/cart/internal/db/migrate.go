package db

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // регистрирует драйвер "pgx" для database/sql
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

// RunMigrations безопасно применяет миграции с использованием блокировки сессии.
// migrations — файловая система с .sql файлами (обычно embed.FS сервиса).
func RunMigrations(ctx context.Context, pool *pgxpool.Pool, serviceName string, migrations fs.FS) error {
	lockID := generateStableLockID(serviceName)

	// В goose v3.30 таймауты передаются как uint64 (секунды): 5 попыток по 10 секунд.
	sessionLocker, err := lock.NewPostgresSessionLocker(
		lock.WithLockTimeout(5, 10),   //nolint:mnd // эти числа являются стандартными константами алгоритмов или конфигурации.
		lock.WithUnlockTimeout(5, 10), //nolint:mnd // эти числа являются стандартными константами алгоритмов или конфигурации.
		lock.WithLockID(lockID),
	)
	if err != nil {
		return fmt.Errorf("create session locker: %w", err)
	}

	// goose.NewProvider в v3.30 принимает *sql.DB, а не *pgxpool.Pool.
	sqlDB, err := sql.Open("pgx", pool.Config().ConnString())
	if err != nil {
		return fmt.Errorf("open sql db for migrations: %w", err)
	}
	defer sqlDB.Close()

	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		sqlDB,
		migrations,
		goose.WithTableName(serviceName+"_goose_db_version"),
		goose.WithSessionLocker(sessionLocker),
	)
	if err != nil {
		return fmt.Errorf("create goose provider: %w", err)
	}

	// GetDBVersion создаёт таблицу версий ПОД session-блокировкой.
	currentVersion, err := provider.GetDBVersion(ctx)
	if err != nil {
		return fmt.Errorf("get current db version: %w", err)
	}

	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	fmt.Printf("[%s] Migrations applied successfully. Current version: %d. Applied %d migrations.\n",
		serviceName, currentVersion, len(results))
	return nil
}

// generateStableLockID создаёт детерминированный int64 из имени сервиса.
func generateStableLockID(serviceName string) int64 {
	var id int64
	for _, c := range serviceName {
		id = id*31 + int64(c) //nolint:mnd // эти числа являются стандартными константами алгоритмов или конфигурации.
	}
	if id < 0 {
		id = -id
	}
	return id
}
