package inbox

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v6"
	"github.com/stretchr/testify/require"

	"github.com/igoroutine-courses/microservices.ecommerce.notifier/internal/db"
	"github.com/igoroutine-courses/microservices.ecommerce.notifier/internal/entity"
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
	ev := entity.InboxEvent{EventID: "e-1", OrderID: "o-1", UserID: "u-1", Status: "paid"}
	mock.ExpectExec("INSERT INTO notifier.inbox").
		WithArgs(ev.EventID, ev.OrderID, ev.UserID, ev.Status).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	require.NoError(t, NewPostgres(nil).Save(ctx, ev))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresSave_Error(t *testing.T) {
	ctx, mock := newCtx(t)
	ev := entity.InboxEvent{EventID: "e-1", OrderID: "o-1", UserID: "u-1", Status: "paid"}
	mock.ExpectExec("INSERT INTO notifier.inbox").
		WithArgs(ev.EventID, ev.OrderID, ev.UserID, ev.Status).
		WillReturnError(errors.New("db down"))
	err := NewPostgres(nil).Save(ctx, ev)
	require.ErrorContains(t, err, "save inbox event")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresClaim(t *testing.T) {
	ctx, mock := newCtx(t)
	lease := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery("UPDATE notifier.inbox").WithArgs(5).
		WillReturnRows(pgxmock.NewRows(
			[]string{"event_id", "order_id", "user_id", "status", "lease_until", "attempts"}).
			AddRow("e-1", "o-1", "u-1", "paid", lease, 2))

	events, err := NewPostgres(nil).Claim(ctx, 5)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, 2, events[0].Attempts)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresClaim_QueryError(t *testing.T) {
	ctx, mock := newCtx(t)
	mock.ExpectQuery("UPDATE notifier.inbox").WithArgs(5).WillReturnError(errors.New("db down"))
	_, err := NewPostgres(nil).Claim(ctx, 5)
	require.ErrorContains(t, err, "claim inbox events")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresMarkProcessed(t *testing.T) {
	ctx, mock := newCtx(t)
	lease := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectExec("UPDATE notifier.inbox").
		WithArgs("e-1", lease).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	require.NoError(t, NewPostgres(nil).MarkProcessed(ctx, "e-1", lease))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresMarkRetry(t *testing.T) {
	ctx, mock := newCtx(t)
	lease := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectExec("UPDATE notifier.inbox").
		WithArgs("e-1", 10, "telegram down", lease).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	require.NoError(t, NewPostgres(nil).MarkRetry(ctx, "e-1", lease, 10, "telegram down"))
	require.NoError(t, mock.ExpectationsWereMet())
}
