package order

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v6"
	"github.com/stretchr/testify/require"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/db"
	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
)

var pgTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// newCtx кладёт pgxmock в контекст как транзакцию: репозиторий возьмёт его
// через db.GetExecutor, поэтому реальный пул не нужен (передаём nil).
func newCtx(t *testing.T) (context.Context, pgxmock.PgxPoolIface) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(mock.Close)
	return db.ContextWithTx(context.Background(), mock), mock
}

func TestPostgresGet_NotFound(t *testing.T) {
	ctx, mock := newCtx(t)
	mock.ExpectQuery("SELECT order_id, user_id, status").
		WithArgs("o-1").
		WillReturnError(pgx.ErrNoRows)

	_, err := NewPostgres(nil).Get(ctx, "o-1")
	require.ErrorIs(t, err, entity.ErrNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresGet_QueryError(t *testing.T) {
	ctx, mock := newCtx(t)
	mock.ExpectQuery("SELECT order_id, user_id, status").
		WithArgs("o-1").
		WillReturnError(errors.New("db down"))

	_, err := NewPostgres(nil).Get(ctx, "o-1")
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresGet_Full(t *testing.T) {
	ctx, mock := newCtx(t)
	mock.ExpectQuery("SELECT order_id, user_id, status").WithArgs("o-1").
		WillReturnRows(pgxmock.NewRows(
			[]string{"order_id", "user_id", "status", "total_amount_minor", "currency", "created_at", "updated_at"}).
			AddRow("o-1", "u-1", "ORDER_STATUS_AWAITING_PAYMENT", 200, "RUB", pgTime, pgTime))
	mock.ExpectQuery("SELECT sku, count, name, price_minor FROM loms.order_items").WithArgs("o-1").
		WillReturnRows(pgxmock.NewRows([]string{"sku", "count", "name", "price_minor"}).
			AddRow("SKU-1", 2, "Widget", 100))
	mock.ExpectQuery("SELECT payment_id, external_id, amount_minor").WithArgs("o-1").
		WillReturnRows(pgxmock.NewRows([]string{
			"payment_id", "external_id", "amount_minor", "currency", "status",
			"antifraud_risk_score", "antifraud_model_version", "antifraud_decline_reason"}).
			AddRow("p-1", "ext-1", 200, "RUB", "PAYMENT_STATUS_PENDING", nil, nil, nil))
	mock.ExpectQuery("SELECT status, occurred_at FROM loms.order_status_history").WithArgs("o-1").
		WillReturnRows(pgxmock.NewRows([]string{"status", "occurred_at"}).
			AddRow("ORDER_STATUS_AWAITING_PAYMENT", pgTime))

	o, err := NewPostgres(nil).Get(ctx, "o-1")
	require.NoError(t, err)
	require.Equal(t, "o-1", o.OrderID)
	require.Equal(t, entity.StatusAwaitingPayment, o.Status)
	require.Equal(t, uint32(200), o.TotalAmountMinor)
	require.Len(t, o.Items, 1)
	require.Equal(t, "Widget", o.Items[0].Name)
	require.Len(t, o.Payments, 1)
	require.Nil(t, o.Payments[0].Antifraud)
	require.Len(t, o.History, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresGet_PaymentWithAntifraud(t *testing.T) {
	ctx, mock := newCtx(t)
	mock.ExpectQuery("SELECT order_id, user_id, status").WithArgs("o-1").
		WillReturnRows(pgxmock.NewRows(
			[]string{"order_id", "user_id", "status", "total_amount_minor", "currency", "created_at", "updated_at"}).
			AddRow("o-1", "u-1", "ORDER_STATUS_FAILED", 100, "RUB", pgTime, pgTime))
	mock.ExpectQuery("SELECT sku, count, name, price_minor FROM loms.order_items").WithArgs("o-1").
		WillReturnRows(pgxmock.NewRows([]string{"sku", "count", "name", "price_minor"}))
	mock.ExpectQuery("SELECT payment_id, external_id, amount_minor").WithArgs("o-1").
		WillReturnRows(pgxmock.NewRows([]string{
			"payment_id", "external_id", "amount_minor", "currency", "status",
			"antifraud_risk_score", "antifraud_model_version", "antifraud_decline_reason"}).
			AddRow("p-1", "ext-1", 100, "RUB", "PAYMENT_STATUS_DECLINED", 0.9, "v1", "simulated_decline"))
	mock.ExpectQuery("SELECT status, occurred_at FROM loms.order_status_history").WithArgs("o-1").
		WillReturnRows(pgxmock.NewRows([]string{"status", "occurred_at"}))

	o, err := NewPostgres(nil).Get(ctx, "o-1")
	require.NoError(t, err)
	require.NotNil(t, o.Payments[0].Antifraud)
	require.Equal(t, "simulated_decline", o.Payments[0].Antifraud.DeclineReason)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresGetRequest_NotFound(t *testing.T) {
	ctx, mock := newCtx(t)
	mock.ExpectQuery("SELECT request_id, user_id, order_id FROM loms.orders").
		WithArgs("r-1").
		WillReturnError(pgx.ErrNoRows)

	_, ok := NewPostgres(nil).GetRequest(ctx, "r-1")
	require.False(t, ok)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresGetRequest_Found(t *testing.T) {
	ctx, mock := newCtx(t)
	mock.ExpectQuery("SELECT request_id, user_id, order_id FROM loms.orders").WithArgs("r-1").
		WillReturnRows(pgxmock.NewRows([]string{"request_id", "user_id", "order_id"}).
			AddRow("r-1", "u-1", "o-1"))
	mock.ExpectQuery("SELECT sku, count FROM loms.order_items").WithArgs("o-1").
		WillReturnRows(pgxmock.NewRows([]string{"sku", "count"}).AddRow("SKU-1", 2))

	req, ok := NewPostgres(nil).GetRequest(ctx, "r-1")
	require.True(t, ok)
	require.Equal(t, "o-1", req.OrderID)
	require.True(t, req.Matches("u-1", []entity.Item{{SKU: "SKU-1", Count: 2}}))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresSave(t *testing.T) {
	ctx, mock := newCtx(t)
	o := entity.Order{
		OrderID: "o-1", UserID: "u-1", Status: entity.StatusAwaitingPayment,
		TotalAmountMinor: 200, Currency: "RUB", CreatedAt: pgTime, UpdatedAt: pgTime,
		History: []entity.Transition{{Status: entity.StatusAwaitingPayment, OccurredAt: pgTime}},
	}
	items := []entity.Item{{SKU: "SKU-1", Count: 2, Name: "Widget", Price: 100}}

	mock.ExpectExec("INSERT INTO loms.orders").
		WithArgs("o-1", "r-1", "u-1", o.Status, uint32(200), "RUB", pgTime, pgTime).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("INSERT INTO loms.order_items").
		WithArgs("o-1", "SKU-1", uint32(2), "Widget", uint32(100)).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("INSERT INTO loms.order_status_history").
		WithArgs("o-1", o.Status, pgTime).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	NewPostgres(nil).Save(ctx, o, "r-1", items)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresSave_OrderInsertError_StopsEarly(t *testing.T) {
	ctx, mock := newCtx(t)
	o := entity.Order{OrderID: "o-1", Status: entity.StatusAwaitingPayment, CreatedAt: pgTime, UpdatedAt: pgTime}
	mock.ExpectExec("INSERT INTO loms.orders").
		WithArgs("o-1", "r-1", "", o.Status, uint32(0), "", pgTime, pgTime).
		WillReturnError(errors.New("tx aborted"))
	NewPostgres(nil).Save(ctx, o, "r-1", []entity.Item{{SKU: "SKU-1", Count: 1}})
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresUpdate(t *testing.T) {
	ctx, mock := newCtx(t)
	o := entity.Order{OrderID: "o-1", Status: entity.StatusPaid, TotalAmountMinor: 200, Currency: "RUB", UpdatedAt: pgTime,
		Payments: []entity.PaymentSummary{{PaymentID: "p-1", ExternalID: "ext-1", AmountMinor: 200, Currency: "RUB",
			Status:    entity.PaymentStatusSucceeded,
			Antifraud: &entity.Antifraud{RiskScore: 0.1, ModelVersion: "v1", DeclineReason: ""}}},
		History: []entity.Transition{{Status: entity.StatusPaid, OccurredAt: pgTime}}}
	mock.ExpectExec("UPDATE loms.orders SET status").
		WithArgs("o-1", o.Status, uint32(200), "RUB", pgTime).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectExec("INSERT INTO loms.payments").
		WithArgs("p-1", "o-1", "ext-1", uint64(200), "RUB", o.Payments[0].Status,
			&o.Payments[0].Antifraud.RiskScore, &o.Payments[0].Antifraud.ModelVersion, &o.Payments[0].Antifraud.DeclineReason).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("INSERT INTO loms.order_status_history").
		WithArgs("o-1", entity.StatusPaid, pgTime).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	NewPostgres(nil).Update(ctx, o)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresListByUser(t *testing.T) {
	ctx, mock := newCtx(t)
	mock.ExpectQuery("SELECT order_id, user_id, status").
		WithArgs("u-1", "", 3).
		WillReturnRows(pgxmock.NewRows(
			[]string{"order_id", "user_id", "status", "total_amount_minor", "currency", "created_at", "updated_at"}).
			AddRow("o-1", "u-1", "ORDER_STATUS_PAID", 100, "RUB", pgTime, pgTime).
			AddRow("o-2", "u-1", "ORDER_STATUS_PAID", 200, "RUB", pgTime, pgTime))

	orders, err := NewPostgres(nil).ListByUser(ctx, "u-1", "", 3)
	require.NoError(t, err)
	require.Len(t, orders, 2)
	require.Equal(t, "o-1", orders[0].OrderID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresListByUser_QueryError(t *testing.T) {
	ctx, mock := newCtx(t)
	mock.ExpectQuery("SELECT order_id, user_id, status").
		WithArgs("u-1", "", 3).
		WillReturnError(errors.New("db down"))

	_, err := NewPostgres(nil).ListByUser(ctx, "u-1", "", 3)
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
