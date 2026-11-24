package cart

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/igoroutine-courses/microservices.ecommerce.cart/internal/db"
	"github.com/igoroutine-courses/microservices.ecommerce.cart/internal/entity"
)

type postgresRepository struct {
	executor   db.Executor
	transactor db.Transactor
}

func NewPostgres(executor db.Executor, transactor db.Transactor) *postgresRepository {
	return &postgresRepository{
		executor:   executor,
		transactor: transactor,
	}
}

// Get возвращает корзину пользователя со всеми позициями.
func (r *postgresRepository) Get(ctx context.Context, userID string) (entity.Cart, error) {
	exec := db.GetExecutor(ctx, r.executor)

	rows, err := exec.Query(ctx, `
		SELECT sku, count
		FROM cart.cart_items
		WHERE user_id = $1
		ORDER BY sku
	`, userID)
	if err != nil {
		return entity.Cart{}, fmt.Errorf("query cart items: %w", err)
	}
	defer rows.Close()

	cart := entity.Cart{
		UserID: userID,
		Items:  make([]entity.Item, 0),
	}

	for rows.Next() {
		var item entity.Item
		if err := rows.Scan(&item.SKU, &item.Count); err != nil {
			return entity.Cart{}, fmt.Errorf("scan cart item: %w", err)
		}
		cart.Items = append(cart.Items, item)
	}

	if err := rows.Err(); err != nil {
		return entity.Cart{}, fmt.Errorf("rows iteration: %w", err)
	}

	return cart, nil
}

// Add добавляет товар в корзину или увеличивает количество.
func (r *postgresRepository) Add(ctx context.Context, userID, sku string, count uint32, available uint64) error {
	return r.transactor.WithTx(ctx, func(txCtx context.Context) error {
		exec := db.GetExecutor(txCtx, r.executor)

		if _, err := exec.Exec(txCtx, `
			INSERT INTO cart.carts (user_id)
			VALUES ($1)
			ON CONFLICT (user_id) DO NOTHING
		`, userID); err != nil {
			return fmt.Errorf("materialize cart: %w", err)
		}

		// Блокируем строку корзины, чтобы параллельные Add одного пользователя
		// не потеряли инкремент и не превысили остаток (как mutex в memory).
		var locked string
		if err := exec.QueryRow(txCtx, `
			SELECT user_id
			FROM cart.carts
			WHERE user_id = $1
			FOR UPDATE
		`, userID).Scan(&locked); err != nil {
			return fmt.Errorf("lock cart: %w", err)
		}

		var current uint32
		err := exec.QueryRow(txCtx, `
			SELECT count
			FROM cart.cart_items
			WHERE user_id = $1 AND sku = $2
		`, userID, sku).Scan(&current)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("get cart item: %w", err)
		}

		next := current + count
		if next > entity.MaxItemCount {
			return entity.ErrConflict
		}
		if uint64(next) > available {
			return entity.ErrInsufficientStock
		}

		if _, err := exec.Exec(txCtx, `
			INSERT INTO cart.cart_items (user_id, sku, count)
			VALUES ($1, $2, $3)
			ON CONFLICT (user_id, sku)
			DO UPDATE SET count = EXCLUDED.count, updated_at = CURRENT_TIMESTAMP
		`, userID, sku, next); err != nil {
			return fmt.Errorf("upsert cart item: %w", err)
		}

		return nil
	})
}

// Set устанавливает точное количество товара в корзине.
func (r *postgresRepository) Set(ctx context.Context, userID, sku string, count uint32, available uint64) error {
	if count > entity.MaxItemCount {
		return entity.ErrConflict
	}
	if uint64(count) > available {
		return entity.ErrInsufficientStock
	}

	return r.transactor.WithTx(ctx, func(txCtx context.Context) error {
		exec := db.GetExecutor(txCtx, r.executor)

		if _, err := exec.Exec(txCtx, `
			INSERT INTO cart.carts (user_id)
			VALUES ($1)
			ON CONFLICT (user_id) DO NOTHING
		`, userID); err != nil {
			return fmt.Errorf("materialize cart: %w", err)
		}

		if _, err := exec.Exec(txCtx, `
			INSERT INTO cart.cart_items (user_id, sku, count)
			VALUES ($1, $2, $3)
			ON CONFLICT (user_id, sku)
			DO UPDATE SET count = EXCLUDED.count, updated_at = CURRENT_TIMESTAMP
		`, userID, sku, count); err != nil {
			return fmt.Errorf("upsert cart item: %w", err)
		}

		return nil
	})
}

// Delete удаляет позицию из корзины.
func (r *postgresRepository) Delete(ctx context.Context, userID, sku string) error {
	exec := db.GetExecutor(ctx, r.executor)

	_, err := exec.Exec(ctx, `
		DELETE FROM cart.cart_items
		WHERE user_id = $1 AND sku = $2
	`, userID, sku)
	if err != nil {
		return fmt.Errorf("delete cart item: %w", err)
	}

	return nil
}

// Clear удаляет все позиции из корзины.
func (r *postgresRepository) Clear(ctx context.Context, userID string) error {
	exec := db.GetExecutor(ctx, r.executor)

	_, err := exec.Exec(ctx, `
		DELETE FROM cart.cart_items
		WHERE user_id = $1
	`, userID)
	if err != nil {
		return fmt.Errorf("clear cart: %w", err)
	}

	return nil
}
