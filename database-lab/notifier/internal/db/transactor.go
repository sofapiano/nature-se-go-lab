package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Transactor — интерфейс для работы с транзакциями.
type Transactor interface {
	WithTx(ctx context.Context, fn func(context.Context) error) error
}

// transactorImpl — реализация интерфейса Transactor.
type transactorImpl struct {
	pool *pgxpool.Pool
}

// NewTransactor создаёт новый транзактор.
// Вызывается один раз при старте приложения.
func NewTransactor(pool *pgxpool.Pool) Transactor {
	return &transactorImpl{pool: pool}
}

// WithTx открывает транзакцию, выполняет функцию и делает commit или rollback.
// Если транзакция уже есть в контексте (вложенный вызов), просто выполняет функцию.
func (t *transactorImpl) WithTx(ctx context.Context, fn func(context.Context) error) error {
	// Проверяем, есть ли уже транзакция в контексте (вложенный вызов)
	if _, ok := TxFromContext(ctx); ok {
		// Просто выполняем функцию, не создавая новую транзакцию.
		// Это гарантирует, что вложенный репозиторий не сможет независимо закоммитить часть операции.
		return fn(ctx)
	}

	// Открываем новую транзакцию
	tx, err := t.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	// Создаём новый контекст с транзакцией
	txCtx := ContextWithTx(ctx, tx)

	// Обрабатываем panic: делаем rollback и пробрасываем panic дальше
	defer func() {
		if p := recover(); p != nil {
			// При panic делаем rollback
			_ = tx.Rollback(ctx)
			// Пробрасываем panic дальше, чтобы он был обработан на верхнем уровне
			panic(p)
		}
	}()

	// Выполняем бизнес-логику
	if err := fn(txCtx); err != nil {
		// При ошибке делаем rollback
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("rollback failed: %w (original error: %v)", rbErr, err)
		}
		return err
	}

	// При успехе делаем commit
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
