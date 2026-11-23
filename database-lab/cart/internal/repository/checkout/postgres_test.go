package checkout

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v6"
	"github.com/stretchr/testify/require"
)

func TestPostgresRepository_Get_Found(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPostgres(mock)

	rows := pgxmock.NewRows([]string{"order_id"}).AddRow("order-123")
	mock.ExpectQuery("SELECT order_id").
		WithArgs("req-1").
		WillReturnRows(rows)

	orderID, found := repo.Get(context.Background(), "req-1")
	require.True(t, found)
	require.Equal(t, "order-123", orderID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresRepository_Get_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPostgres(mock)

	mock.ExpectQuery("SELECT order_id").
		WithArgs("req-1").
		WillReturnError(pgx.ErrNoRows)

	orderID, found := repo.Get(context.Background(), "req-1")
	require.False(t, found)
	require.Empty(t, orderID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresRepository_Save(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPostgres(mock)

	mock.ExpectExec("INSERT INTO cart.checkouts").
		WithArgs("req-1", "order-123").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err = repo.Save(context.Background(), "req-1", "order-123")
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}