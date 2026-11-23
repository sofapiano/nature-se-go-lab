package stocks

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/db"
	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
)

type postgresRepository struct {
	pool       *pgxpool.Pool
	transactor db.Transactor
}

func NewPostgres(pool *pgxpool.Pool, transactor db.Transactor) *postgresRepository {
	return &postgresRepository{
		pool:       pool,
		transactor: transactor,
	}
}

// Get возвращает остаток; у товара, которому остаток ещё не задавали, он нулевой.
func (r *postgresRepository) Get(ctx context.Context, sku string) (uint64, error) {
	exec := db.GetExecutor(ctx, r.pool)

	var count uint64
	err := exec.QueryRow(ctx, `
		SELECT count
		FROM loms.stocks
		WHERE sku = $1
	`, sku).Scan(&count)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil // остаток по умолчанию нулевой
		}
		return 0, fmt.Errorf("get stock: %w", err)
	}

	return count, nil
}

// Set заменяет остаток.
func (r *postgresRepository) Set(ctx context.Context, sku string, count uint64) error {
	exec := db.GetExecutor(ctx, r.pool)

	_, err := exec.Exec(ctx, `
		INSERT INTO loms.stocks (sku, count)
		VALUES ($1, $2)
		ON CONFLICT (sku)
		DO UPDATE SET count = EXCLUDED.count
	`, sku, count)

	if err != nil {
		return fmt.Errorf("set stock: %w", err)
	}

	return nil
}

// Receive применяет поступление ровно один раз (идемпотентность через operation_id).
func (r *postgresRepository) Receive(ctx context.Context, receipt entity.StockReceipt) error {
	return r.transactor.WithTx(ctx, func(txCtx context.Context) error {
		exec := db.GetExecutor(txCtx, r.pool)

		// 1. Пытаемся создать запись о поступлении
		tag, err := exec.Exec(txCtx, `
			INSERT INTO loms.stock_receipts (operation_id, sku, count)
			VALUES ($1, $2, $3)
			ON CONFLICT (operation_id) DO NOTHING
		`, receipt.OperationID, receipt.SKU, receipt.Count)

		if err != nil {
			return fmt.Errorf("create stock receipt: %w", err)
		}

		// 2. Если запись уже существовала, проверяем идемпотентность
		if tag.RowsAffected() == 0 {
			var existingSKU string
			var existingCount uint64

			err := exec.QueryRow(txCtx, `
				SELECT sku, count
				FROM loms.stock_receipts
				WHERE operation_id = $1
			`, receipt.OperationID).Scan(&existingSKU, &existingCount)

			if err != nil {
				return fmt.Errorf("get existing receipt: %w", err)
			}

			// Проверяем, что данные совпадают
			if existingSKU != receipt.SKU || existingCount != receipt.Count {
				return fmt.Errorf("operation %s was already used with different data: %w",
					receipt.OperationID, entity.ErrConflict)
			}

			// Идентичное поступление уже применено — успех
			return nil
		}

		// 3. Материализуем строку остатка, если её нет
		_, err = exec.Exec(txCtx, `
			INSERT INTO loms.stocks (sku, count)
			VALUES ($1, 0)
			ON CONFLICT (sku) DO NOTHING
		`, receipt.SKU)

		if err != nil {
			return fmt.Errorf("materialize stock: %w", err)
		}

		// 4. Блокируем строку и обновляем остаток
		var currentCount uint64
		err = exec.QueryRow(txCtx, `
			SELECT count
			FROM loms.stocks
			WHERE sku = $1
			FOR UPDATE
		`, receipt.SKU).Scan(&currentCount)

		if err != nil {
			return fmt.Errorf("lock stock: %w", err)
		}

		// 5. Проверяем overflow
		newCount := currentCount + receipt.Count
		if newCount > entity.MaxStock {
			return fmt.Errorf("stock of %s would exceed %d: %w",
				receipt.SKU, entity.MaxStock, entity.ErrState)
		}

		// 6. Обновляем остаток
		_, err = exec.Exec(txCtx, `
			UPDATE loms.stocks
			SET count = $2
			WHERE sku = $1
		`, receipt.SKU, newCount)

		if err != nil {
			return fmt.Errorf("update stock: %w", err)
		}

		return nil
	})
}

// Reserve списывает count по каждому SKU атомарно.
func (r *postgresRepository) Reserve(ctx context.Context, items []entity.Item) error {
	return r.transactor.WithTx(ctx, func(txCtx context.Context) error {
		exec := db.GetExecutor(txCtx, r.pool)

		// Сначала проверяем все позиции с блокировкой
		for _, item := range items {
			// Материализуем строку, если её нет
			_, err := exec.Exec(txCtx, `
				INSERT INTO loms.stocks (sku, count)
				VALUES ($1, 0)
				ON CONFLICT (sku) DO NOTHING
			`, item.SKU)

			if err != nil {
				return fmt.Errorf("materialize stock for %s: %w", item.SKU, err)
			}

			// Блокируем и проверяем остаток
			var currentCount uint64
			err = exec.QueryRow(txCtx, `
				SELECT count
				FROM loms.stocks
				WHERE sku = $1
				FOR UPDATE
			`, item.SKU).Scan(&currentCount)

			if err != nil {
				return fmt.Errorf("lock stock for %s: %w", item.SKU, err)
			}

			if currentCount < uint64(item.Count) {
				return fmt.Errorf("insufficient stock for %s: %w", item.SKU, entity.ErrStock)
			}
		}

		// Затем списываем все позиции
		for _, item := range items {
			_, err := exec.Exec(txCtx, `
				UPDATE loms.stocks
				SET count = count - $2
				WHERE sku = $1 AND count >= $2
			`, item.SKU, item.Count)

			if err != nil {
				return fmt.Errorf("reserve stock for %s: %w", item.SKU, err)
			}
		}

		return nil
	})
}

// Release возвращает ранее зарезервированное.
func (r *postgresRepository) Release(ctx context.Context, items []entity.Item) error {
	return r.transactor.WithTx(ctx, func(txCtx context.Context) error {
		exec := db.GetExecutor(txCtx, r.pool)

		for _, item := range items {
			_, err := exec.Exec(txCtx, `
				UPDATE loms.stocks
				SET count = count + $2
				WHERE sku = $1
			`, item.SKU, item.Count)

			if err != nil {
				return fmt.Errorf("release stock for %s: %w", item.SKU, err)
			}
		}

		return nil
	})
}