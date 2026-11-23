package binding

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/igoroutine-courses/microservices.ecommerce.notifier/internal/db"
)

type postgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *postgresRepository {
	return &postgresRepository{pool: pool}
}

// Get возвращает chat_id и признак существования.
func (r *postgresRepository) Get(ctx context.Context, userID string) (string, bool) {
	exec := db.GetExecutor(ctx, r.pool)

	var chatID string
	err := exec.QueryRow(ctx, `
		SELECT chat_id
		FROM notifier.user_bindings
		WHERE user_id = $1
	`, userID).Scan(&chatID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false
		}
		// В продакшене здесь должен быть лог
		return "", false
	}

	return chatID, true
}

// Set перезаписывает привязку: повторный BindTelegram заменяет прежний чат.
func (r *postgresRepository) Set(ctx context.Context, userID, chatID string) error {
	exec := db.GetExecutor(ctx, r.pool)

	_, err := exec.Exec(ctx, `
		INSERT INTO notifier.user_bindings (user_id, chat_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id)
		DO UPDATE SET chat_id = EXCLUDED.chat_id
	`, userID, chatID)

	if err != nil {
		return fmt.Errorf("upsert binding: %w", err)
	}

	return nil
}