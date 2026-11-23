package db

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ctxKey — неэкспортируемый тип для ключа контекста.
type ctxKey struct{}

// ContextWithTx добавляет транзакцию в контекст.
func ContextWithTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, ctxKey{}, tx)
}

// TxFromContext извлекает транзакцию из контекста.
func TxFromContext(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(ctxKey{}).(pgx.Tx)
	return tx, ok
}

// Executor — интерфейс для выполнения SQL-запросов.
// Его реализуют и *pgxpool.Pool, и pgx.Tx, и pgxmock.PgxPoolIface.
type Executor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// GetExecutor возвращает Executor из контекста (если есть транзакция)
// или переданный executor (если транзакции нет).
func GetExecutor(ctx context.Context, exec Executor) Executor {
	if tx, ok := TxFromContext(ctx); ok {
		return tx
	}
	return exec
}