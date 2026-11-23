package checkout

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/igoroutine-courses/microservices.ecommerce.cart/internal/db"
)

type postgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *postgresRepository {
	return &postgresRepository{pool: pool}
}

// Get возвращает order_id и true, если checkout уже создан.
// Возвращает пустую строку и false, если checkout не найден.
func (r *postgresRepository) Get(ctx context.Context, requestID string) (string, bool) {
	exec := db.GetExecutor(ctx, r.pool)

	var orderID string
	err := exec.QueryRow(ctx, `
		SELECT order_id
		FROM cart.checkouts
		WHERE request_id = $1
	`, requestID).Scan(&orderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false
		}
		// В продакшене здесь должен быть лог
		return "", false
	}

	return orderID, orderID != ""
}

// Save сохраняет checkout с order_id.
// Использует UPSERT для идемпотентности: если запись уже есть, обновляем order_id.
func (r *postgresRepository) Save(ctx context.Context, requestID, orderID string) error {
	exec := db.GetExecutor(ctx, r.pool)

	_, err := exec.Exec(ctx, `
		INSERT INTO cart.checkouts (request_id, order_id)
		VALUES ($1, $2)
		ON CONFLICT (request_id)
		DO UPDATE SET order_id = EXCLUDED.order_id, updated_at = CURRENT_TIMESTAMP
	`, requestID, orderID)
	if err != nil {
		return fmt.Errorf("save checkout: %w", err)
	}

	return nil
}