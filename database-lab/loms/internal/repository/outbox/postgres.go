package outbox

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/db"
	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
)

type postgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *postgresRepository {
	return &postgresRepository{pool: pool}
}

// Save сохраняет событие в outbox. Вызывается внутри бизнес-транзакции.
func (r *postgresRepository) Save(ctx context.Context, event entity.OutboxEvent) error {
	exec := db.GetExecutor(ctx, r.pool)
	_, err := exec.Exec(ctx, `
		INSERT INTO loms.outbox (event_id, event_type, order_id, user_id, status, payload, status_sent)
		VALUES ($1, $2, $3, $4, $5, $6, 'pending')
	`, event.EventID, event.EventType, event.OrderID, event.UserID, event.Status, event.Payload)

	if err != nil {
		return fmt.Errorf("save outbox event: %w", err)
	}
	return nil
}

// FetchAndLock забирает пакет неподтверждённых событий и блокирует их для текущего воркера.
func (r *postgresRepository) FetchAndLock(ctx context.Context, limit int) ([]entity.OutboxEvent, error) {
	exec := db.GetExecutor(ctx, r.pool)

	// SKIP LOCKED гарантирует, что если другой воркер уже взял это событие, мы его пропустим.
	rows, err := exec.Query(ctx, `
		UPDATE loms.outbox
		SET lease_until = NOW() + INTERVAL '30 seconds', attempts = attempts + 1
		WHERE event_id IN (
			SELECT event_id FROM loms.outbox
			WHERE status_sent = 'pending' AND (lease_until IS NULL OR lease_until < NOW())
			ORDER BY created_at ASC
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING event_id, event_type, order_id, user_id, status, payload::text, status_sent, COALESCE(lease_until, NOW()), attempts, COALESCE(last_error, '')
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch and lock outbox events: %w", err)
	}
	defer rows.Close()

	var events []entity.OutboxEvent
	for rows.Next() {
		var e entity.OutboxEvent
		if err := rows.Scan(&e.EventID, &e.EventType, &e.OrderID, &e.UserID, &e.Status, &e.Payload, &e.StatusSent, &e.LeaseUntil, &e.Attempts, &e.LastError); err != nil {
			return nil, fmt.Errorf("scan outbox event: %w", err)
		}
		events = append(events, e)
	}
	return events, nil
}

// MarkSent помечает событие как успешно отправленное.
func (r *postgresRepository) MarkSent(ctx context.Context, eventID string) error {
	exec := db.GetExecutor(ctx, r.pool)
	_, err := exec.Exec(ctx, `
		UPDATE loms.outbox SET status_sent = 'sent', updated_at = NOW() WHERE event_id = $1
	`, eventID)
	return err
}

// MarkFailed сбрасывает блокировку, чтобы воркер мог попробовать снова через некоторое время.
func (r *postgresRepository) MarkFailed(ctx context.Context, eventID, errMsg string) error {
	exec := db.GetExecutor(ctx, r.pool)
	_, err := exec.Exec(ctx, `
		UPDATE loms.outbox
		SET status_sent = 'pending', last_error = $2, lease_until = NOW() + INTERVAL '1 minute', updated_at = NOW()
		WHERE event_id = $1
	`, eventID, errMsg)
	return err
}
