package cart

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/igoroutine-courses/microservices.ecommerce.cart/internal/db"
	"github.com/igoroutine-courses/microservices.ecommerce.cart/internal/entity"
)

type postgresRepository struct {
	executor   db.Executor
	transactor db.Transactor
}

func NewPostgres(pool *pgxpool.Pool, transactor db.Transactor) *postgresRepository {
	return &postgresRepository{
		pool:       pool,
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
// Использует транзакцию для конкурентной безопасности.
func (r *postgresRepository) Add(ctx context.Context, userID, sku string, count uint32, _ uint64) error {
	return r.transactor.WithTx(ctx, func(txCtx context.Context) error {
		exec := db.GetExecutor(txCtx, r.executor)

		// Материализуем корзину, если её нет
		_, err := exec.Exec(txCtx, `
			INSERT INTO cart.carts (user_id)
			VALUES ($1)
			ON CONFLICT (user_id) DO NOTHING
		`, userID)
		if err != nil {
			return fmt.Errorf("materialize cart: %w", err)
		}

		// Пытаемся увеличить количество существующей позиции
		tag, err := exec.Exec(txCtx, `
			UPDATE cart.cart_items
			SET count = count + $3, updated_at = CURRENT_TIMESTAMP
			WHERE user_id = $1 AND sku = $2
		`, userID, sku, count)
		if err != nil {
			return fmt.Errorf("update cart item: %w", err)
		}

		// Если позиция не найдена, создаём новую
		if tag.RowsAffected() == 0 {
			_, err = exec.Exec(txCtx, `
				INSERT INTO cart.cart_items (user_id, sku, count)
				VALUES ($1, $2, $3)
			`, userID, sku, count)
			if err != nil {
				return fmt.Errorf("insert cart item: %w", err)
			}
		}

		return nil
	})
}

// Set устанавливает точное количество товара в корзине.
func (r *postgresRepository) Set(ctx context.Context, userID, sku string, count uint32, _ uint64) error {
	return r.transactor.WithTx(ctx, func(txCtx context.Context) error {
		exec := db.GetExecutor(txCtx, r.executor)

		// Материализуем корзину, если её нет
		_, err := exec.Exec(txCtx, `
			INSERT INTO cart.carts (user_id)
			VALUES ($1)
			ON CONFLICT (user_id) DO NOTHING
		`, userID)
		if err != nil {
			return fmt.Errorf("materialize cart: %w", err)
		}

		// Обновляем или вставляем позицию
		_, err = exec.Exec(txCtx, `
			INSERT INTO cart.cart_items (user_id, sku, count)
			VALUES ($1, $2, $3)
			ON CONFLICT (user_id, sku)
			DO UPDATE SET count = EXCLUDED.count, updated_at = CURRENT_TIMESTAMP
		`, userID, sku, count)
		if err != nil {
			return fmt.Errorf("upsert cart item: %w", err)
		}

		return nil
	})
}

// Delete удаляет позицию из корзины.
func (r *postgresRepository) Delete(ctx context.Context, userID, sku string) error {
	exec := db.GetExecutor(ctx, r.executor)

	tag, err := exec.Exec(ctx, `
		DELETE FROM cart.cart_items
		WHERE user_id = $1 AND sku = $2
	`, userID, sku)
	if err != nil {
		return fmt.Errorf("delete cart item: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("cart item %q for user %q: %w", sku, userID, entity.ErrNotFound)
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
