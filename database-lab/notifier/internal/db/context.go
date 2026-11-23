package db

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ctxKey — неэкспортируемый тип для ключа контекста.
// Это best practice: никто извне не сможет случайно перезаписать нашу транзакцию.
type ctxKey struct{}

// ContextWithTx добавляет транзакцию в контекст.
func ContextWithTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, ctxKey{}, tx)
}

// TxFromContext извлекает транзакцию из контекста.
// Возвращает транзакцию и true, если она есть; иначе nil и false.
func TxFromContext(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(ctxKey{}).(pgx.Tx)
	return tx, ok
}

// Executor — интерфейс для выполнения SQL-запросов.
// Его реализуют и pgxpool.Pool (для работы вне транзакции),
// и pgx.Tx (для работы внутри транзакции).
// Это позволит репозиториям работать в обоих режимах без изменений.
type Executor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// GetExecutor возвращает Executor из контекста (если есть транзакция)
// или пул соединений (если транзакции нет).
// Это ключевой хелпер для репозиториев: они могут работать и внутри транзакции, и вне её.
func GetExecutor(ctx context.Context, pool *pgxpool.Pool) Executor {
	if tx, ok := TxFromContext(ctx); ok {
		return tx
	}
	return pool
}