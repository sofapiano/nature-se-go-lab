package order

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
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *postgresRepository {
	return &postgresRepository{pool: pool}
}

func (r *postgresRepository) Get(ctx context.Context, orderID string) (entity.Order, error) {
	exec := db.GetExecutor(ctx, r.pool)

	var o entity.Order
	err := exec.QueryRow(ctx, `
		SELECT order_id, user_id, status, total_amount_minor, currency, created_at, updated_at
		FROM loms.orders WHERE order_id = $1
	`, orderID).Scan(&o.OrderID, &o.UserID, &o.Status, &o.TotalAmountMinor, &o.Currency, &o.CreatedAt, &o.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Order{}, fmt.Errorf("order %q: %w", orderID, entity.ErrNotFound)
		}
		return entity.Order{}, fmt.Errorf("get order: %w", err)
	}

	// Позиции
	rows, err := exec.Query(ctx, `SELECT sku, count, name, price_minor FROM loms.order_items WHERE order_id = $1`, orderID)
	if err != nil {
		return entity.Order{}, fmt.Errorf("get order items: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item entity.Item
		if err := rows.Scan(&item.SKU, &item.Count, &item.Name, &item.Price); err != nil {
			return entity.Order{}, fmt.Errorf("scan order item: %w", err)
		}
		o.Items = append(o.Items, item)
	}

	// Платежи
	pRows, err := exec.Query(ctx, `
		SELECT payment_id, external_id, amount_minor, currency, status, antifraud_risk_score, antifraud_model_version, antifraud_decline_reason
		FROM loms.payments WHERE order_id = $1
	`, orderID)
	if err != nil {
		return entity.Order{}, fmt.Errorf("get payments: %w", err)
	}
	defer pRows.Close()
	for pRows.Next() {
		var p entity.PaymentSummary
		var riskScore *float64
		var modelVersion, declineReason *string
		err := pRows.Scan(&p.PaymentID, &p.ExternalID, &p.AmountMinor, &p.Currency, &p.Status, &riskScore, &modelVersion, &declineReason)
		if err != nil {
			return entity.Order{}, fmt.Errorf("scan payment: %w", err)
		}
		if riskScore != nil {
			p.Antifraud = &entity.Antifraud{RiskScore: *riskScore, ModelVersion: *modelVersion, DeclineReason: *declineReason}
		}
		o.Payments = append(o.Payments, p)
	}

	// История
	hRows, err := exec.Query(ctx, `SELECT status, occurred_at FROM loms.order_status_history WHERE order_id = $1 ORDER BY occurred_at ASC`, orderID)
	if err != nil {
		return entity.Order{}, fmt.Errorf("get history: %w", err)
	}
	defer hRows.Close()
	for hRows.Next() {
		var h entity.Transition
		if err := hRows.Scan(&h.Status, &h.OccurredAt); err != nil {
			return entity.Order{}, fmt.Errorf("scan history: %w", err)
		}
		o.History = append(o.History, h)
	}

	return o, nil
}

func (r *postgresRepository) GetRequest(ctx context.Context, requestID string) (entity.OrderRequest, bool) {
	exec := db.GetExecutor(ctx, r.pool)

	var req entity.OrderRequest
	err := exec.QueryRow(ctx, `SELECT request_id, user_id, order_id FROM loms.orders WHERE request_id = $1`, requestID).
		Scan(&req.RequestID, &req.UserID, &req.OrderID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.OrderRequest{}, false
		}
		return entity.OrderRequest{}, false
	}

	rows, err := exec.Query(ctx, `SELECT sku, count FROM loms.order_items WHERE order_id = $1`, req.OrderID)
	if err != nil {
		return entity.OrderRequest{}, false
	}
	defer rows.Close()
	for rows.Next() {
		var item entity.Item
		if err := rows.Scan(&item.SKU, &item.Count); err != nil {
			return entity.OrderRequest{}, false
		}
		req.Items = append(req.Items, item)
	}

	return req, true
}

func (r *postgresRepository) Save(ctx context.Context, order entity.Order, requestID string, items []entity.Item) {
	exec := db.GetExecutor(ctx, r.pool)

	_, err := exec.Exec(ctx, `
		INSERT INTO loms.orders (order_id, request_id, user_id, status, total_amount_minor, currency, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, order.OrderID, requestID, order.UserID, order.Status, order.TotalAmountMinor, order.Currency, order.CreatedAt, order.UpdatedAt)
	if err != nil {
		return // Ошибка транзакции откатит всё
	}

	for _, item := range items {
		_, _ = exec.Exec(ctx, `
			INSERT INTO loms.order_items (order_id, sku, count, name, price_minor)
			VALUES ($1, $2, $3, $4, $5)
		`, order.OrderID, item.SKU, item.Count, item.Name, item.Price)
	}

	for _, h := range order.History {
		_, _ = exec.Exec(ctx, `
			INSERT INTO loms.order_status_history (order_id, status, occurred_at)
			VALUES ($1, $2, $3)
		`, order.OrderID, h.Status, h.OccurredAt)
	}
}

func (r *postgresRepository) Update(ctx context.Context, order entity.Order) {
	exec := db.GetExecutor(ctx, r.pool)

	_, _ = exec.Exec(ctx, `
		UPDATE loms.orders SET status = $2, total_amount_minor = $3, currency = $4, updated_at = $5 WHERE order_id = $1
	`, order.OrderID, order.Status, order.TotalAmountMinor, order.Currency, order.UpdatedAt)

	for _, p := range order.Payments {
		var riskScore *float64
		var modelVersion, declineReason *string
		if p.Antifraud != nil {
			riskScore = &p.Antifraud.RiskScore
			modelVersion = &p.Antifraud.ModelVersion
			declineReason = &p.Antifraud.DeclineReason
		}

		_, _ = exec.Exec(ctx, `
			INSERT INTO loms.payments (payment_id, order_id, external_id, amount_minor, currency, status, antifraud_risk_score, antifraud_model_version, antifraud_decline_reason)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (external_id) DO UPDATE
			SET status = EXCLUDED.status, antifraud_risk_score = EXCLUDED.antifraud_risk_score,
			    antifraud_model_version = EXCLUDED.antifraud_model_version, antifraud_decline_reason = EXCLUDED.antifraud_decline_reason,
			    updated_at = CURRENT_TIMESTAMP
		`, p.PaymentID, order.OrderID, p.ExternalID, p.AmountMinor, p.Currency, p.Status, riskScore, modelVersion, declineReason)
	}

	if len(order.History) > 0 {
		last := order.History[len(order.History)-1]
		_, _ = exec.Exec(ctx, `
			INSERT INTO loms.order_status_history (order_id, status, occurred_at)
			VALUES ($1, $2, $3) ON CONFLICT DO NOTHING
		`, order.OrderID, last.Status, last.OccurredAt)
	}
}

func (r *postgresRepository) ListByUser(ctx context.Context, userID, afterOrderID string, limit int) ([]entity.Order, error) {
	exec := db.GetExecutor(ctx, r.pool)

	rows, err := exec.Query(ctx, `
		SELECT order_id, user_id, status, total_amount_minor, currency, created_at, updated_at
		FROM loms.orders WHERE user_id = $1 AND order_id > $2 ORDER BY order_id ASC LIMIT $3
	`, userID, afterOrderID, limit)
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}
	defer rows.Close()

	var orders []entity.Order
	for rows.Next() {
		var o entity.Order
		if err := rows.Scan(&o.OrderID, &o.UserID, &o.Status, &o.TotalAmountMinor, &o.Currency, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan order: %w", err)
		}
		orders = append(orders, o)
	}
	return orders, nil
}
