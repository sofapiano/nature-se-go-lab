package inbox

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/igoroutine-courses/microservices.ecommerce.notifier/internal/db"
	"github.com/igoroutine-courses/microservices.ecommerce.notifier/internal/entity"
)

type postgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *postgresRepository {
	return &postgresRepository{pool: pool}
}

// Save регистрирует событие. Повтор с тем же event_id и тем же payload — идемпотентный
// no-op; тот же event_id с другим payload — конфликт. Одна атомарная операция:
// ON CONFLICT DO UPDATE ... WHERE payload совпадает (no-op update), иначе 0 строк.
func (r *postgresRepository) Save(ctx context.Context, e entity.InboxEvent) error {
	exec := db.GetExecutor(ctx, r.pool)
	tag, err := exec.Exec(ctx, `
		INSERT INTO notifier.inbox (event_id, order_id, user_id, status, state)
		VALUES ($1, $2, $3, $4, 'pending')
		ON CONFLICT (event_id) DO UPDATE
		SET event_id = EXCLUDED.event_id
		WHERE LOWER(notifier.inbox.order_id) = LOWER(EXCLUDED.order_id)
		  AND LOWER(notifier.inbox.user_id) = LOWER(EXCLUDED.user_id)
		  AND LOWER(notifier.inbox.status) = LOWER(EXCLUDED.status)
	`, e.EventID, e.OrderID, e.UserID, e.Status)
	if err != nil {
		return fmt.Errorf("save inbox event: %w", err)
	}

	// Если RowsAffected == 0, значит запись с таким event_id уже существует,
	// но условия WHERE не совпали (payload другой). Это конфликт.
	if tag.RowsAffected() == 0 {
		return entity.ErrConflict
	}

	return nil
}

// Claim захватывает пакет задач и ставит lease. SKIP LOCKED позволяет
// нескольким воркерам разбирать разные события без блокировок друг друга.
// Просроченный lease (упавший воркер) автоматически возвращается в работу.
func (r *postgresRepository) Claim(ctx context.Context, limit int) ([]entity.InboxEvent, error) {
	exec := db.GetExecutor(ctx, r.pool)
	rows, err := exec.Query(ctx, `
		UPDATE notifier.inbox
		SET state = 'in_progress',
		    lease_until = NOW() + INTERVAL '30 seconds',
		    attempts = attempts + 1,
		    updated_at = NOW()
		WHERE event_id IN (
			SELECT event_id FROM notifier.inbox
			WHERE state <> 'processed'
			  AND (lease_until IS NULL OR lease_until < NOW())
			ORDER BY created_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING event_id, order_id, user_id, status, lease_until, attempts
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("claim inbox events: %w", err)
	}
	defer rows.Close()

	var events []entity.InboxEvent
	for rows.Next() {
		var e entity.InboxEvent
		if err := rows.Scan(&e.EventID, &e.OrderID, &e.UserID, &e.Status, &e.LeaseUntil, &e.Attempts); err != nil {
			return nil, fmt.Errorf("scan inbox event: %w", err)
		}
		events = append(events, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return events, nil
}

// MarkProcessed закрывает задачу. lease_until = $2 — fencing-токен:
// если аренду перехватил другой воркер, наша запись не применится.
func (r *postgresRepository) MarkProcessed(ctx context.Context, eventID string, lease time.Time) error {
	exec := db.GetExecutor(ctx, r.pool)
	_, err := exec.Exec(ctx, `
		UPDATE notifier.inbox
		SET state = 'processed', lease_until = NULL, updated_at = NOW()
		WHERE event_id = $1 AND lease_until = $2 AND state = 'in_progress'
	`, eventID, lease)
	return err
}

// MarkRetry планирует повтор: возвращает задачу в pending с отложенным lease.
func (r *postgresRepository) MarkRetry(ctx context.Context, eventID string, lease time.Time, backoffSec int, errMsg string) error {
	exec := db.GetExecutor(ctx, r.pool)
	_, err := exec.Exec(ctx, `
		UPDATE notifier.inbox
		SET state = 'pending',
		    lease_until = NOW() + make_interval(secs => $2),
		    last_error = $3,
		    updated_at = NOW()
		WHERE event_id = $1 AND lease_until = $4 AND state = 'in_progress'
	`, eventID, backoffSec, errMsg, lease)
	return err
}
