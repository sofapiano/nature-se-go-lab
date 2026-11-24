package outbox

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v6"
	"github.com/stretchr/testify/require"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/db"
	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
)

func newCtx(t *testing.T) (context.Context, pgxmock.PgxPoolIface) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(mock.Close)
	return db.ContextWithTx(context.Background(), mock), mock
}

func TestPostgresSave(t *testing.T) {
	ctx, mock := newCtx(t)
	ev := entity.OutboxEvent{
		EventID: "e-1", EventType: "order_status_changed", OrderID: "o-1",
		UserID: "u-1", Status: "awaiting_payment", Payload: `{"order_id":"o-1"}`,
	}
	mock.ExpectExec("INSERT INTO loms.outbox").
		WithArgs(ev.EventID, ev.EventType, ev.OrderID, ev.UserID, ev.Status, ev.Payload).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	require.NoError(t, NewPostgres(nil).Save(ctx, ev))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresSave_Error(t *testing.T) {
	ctx, mock := newCtx(t)
	ev := entity.OutboxEvent{EventID: "e-1", EventType: "order_status_changed", OrderID: "o-1",
		UserID: "u-1", Status: "awaiting_payment", Payload: `{"order_id":"o-1"}`}
	mock.ExpectExec("INSERT INTO loms.outbox").
		WithArgs(ev.EventID, ev.EventType, ev.OrderID, ev.UserID, ev.Status, ev.Payload).
		WillReturnError(errors.New("db down"))
	err := NewPostgres(nil).Save(ctx, ev)
	require.ErrorContains(t, err, "save outbox event")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresFetchAndLock(t *testing.T) {
	ctx, mock := newCtx(t)
	lease := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery("UPDATE loms.outbox").WithArgs(10).
		WillReturnRows(pgxmock.NewRows([]string{
			"event_id", "event_type", "order_id", "user_id", "status",
			"payload", "status_sent", "lease_until", "attempts", "last_error"}).
			AddRow("e-1", "order_status_changed", "o-1", "u-1", "awaiting_payment",
				`{"order_id":"o-1"}`, "pending", lease, 1, ""))

	events, err := NewPostgres(nil).FetchAndLock(ctx, 10)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "e-1", events[0].EventID)
	require.Equal(t, 1, events[0].Attempts)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresFetchAndLock_QueryError(t *testing.T) {
	ctx, mock := newCtx(t)
	mock.ExpectQuery("UPDATE loms.outbox").WithArgs(10).WillReturnError(errors.New("db down"))
	_, err := NewPostgres(nil).FetchAndLock(ctx, 10)
	require.ErrorContains(t, err, "fetch and lock outbox events")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresMarkSent(t *testing.T) {
	ctx, mock := newCtx(t)
	mock.ExpectExec("UPDATE loms.outbox SET status_sent").
		WithArgs("e-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	require.NoError(t, NewPostgres(nil).MarkSent(ctx, "e-1"))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresMarkFailed(t *testing.T) {
	ctx, mock := newCtx(t)
	mock.ExpectExec("UPDATE loms.outbox").
		WithArgs("e-1", "boom").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	require.NoError(t, NewPostgres(nil).MarkFailed(ctx, "e-1", "boom"))
	require.NoError(t, mock.ExpectationsWereMet())
}
