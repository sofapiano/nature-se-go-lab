package cart

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v6"
	"github.com/stretchr/testify/require"

	"github.com/igoroutine-courses/microservices.ecommerce.cart/internal/entity"
)

type mockTransactor struct {
	txFunc func(ctx context.Context, fn func(context.Context) error) error
}

func (m *mockTransactor) WithTx(ctx context.Context, fn func(context.Context) error) error {
	return m.txFunc(ctx, fn)
}

func TestPostgresRepository_Get(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPostgres(mock, nil)

	rows := pgxmock.NewRows([]string{"sku", "count"}).
		AddRow("SKU-1", 2).
		AddRow("SKU-2", 5)

	mock.ExpectQuery("SELECT sku, count").
		WithArgs("user-1").
		WillReturnRows(rows)

	cart, err := repo.Get(context.Background(), "user-1")
	require.NoError(t, err)
	require.Equal(t, "user-1", cart.UserID)
	require.Len(t, cart.Items, 2)
	require.Equal(t, uint32(2), cart.Items[0].Count)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresRepository_Add_UpdateExisting(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()
	transactor := &mockTransactor{txFunc: func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }}
	repo := NewPostgres(mock, transactor)

	mock.ExpectExec("INSERT INTO cart.carts").WithArgs("user-1").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectQuery("SELECT user_id").WithArgs("user-1").
		WillReturnRows(pgxmock.NewRows([]string{"user_id"}).AddRow("user-1"))
	mock.ExpectQuery("SELECT count").WithArgs("user-1", "SKU-1").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(3))
	mock.ExpectExec("INSERT INTO cart.cart_items").WithArgs("user-1", "SKU-1", uint32(5)).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	require.NoError(t, repo.Add(context.Background(), "user-1", "SKU-1", 2, 10))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresRepository_Add_InsertNew(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()
	transactor := &mockTransactor{txFunc: func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }}
	repo := NewPostgres(mock, transactor)

	mock.ExpectExec("INSERT INTO cart.carts").WithArgs("user-1").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectQuery("SELECT user_id").WithArgs("user-1").
		WillReturnRows(pgxmock.NewRows([]string{"user_id"}).AddRow("user-1"))
	mock.ExpectQuery("SELECT count").WithArgs("user-1", "SKU-1").WillReturnError(pgx.ErrNoRows)
	mock.ExpectExec("INSERT INTO cart.cart_items").WithArgs("user-1", "SKU-1", uint32(2)).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	require.NoError(t, repo.Add(context.Background(), "user-1", "SKU-1", 2, 10))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresRepository_Add_InsufficientStock(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()
	transactor := &mockTransactor{txFunc: func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }}
	repo := NewPostgres(mock, transactor)

	mock.ExpectExec("INSERT INTO cart.carts").WithArgs("user-1").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectQuery("SELECT user_id").WithArgs("user-1").
		WillReturnRows(pgxmock.NewRows([]string{"user_id"}).AddRow("user-1"))
	mock.ExpectQuery("SELECT count").WithArgs("user-1", "SKU-1").WillReturnError(pgx.ErrNoRows)

	err = repo.Add(context.Background(), "user-1", "SKU-1", 11, 10)
	require.ErrorIs(t, err, entity.ErrInsufficientStock)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresRepository_Add_ExceedsMaxItemCount(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()
	transactor := &mockTransactor{txFunc: func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }}
	repo := NewPostgres(mock, transactor)

	mock.ExpectExec("INSERT INTO cart.carts").WithArgs("user-1").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectQuery("SELECT user_id").WithArgs("user-1").
		WillReturnRows(pgxmock.NewRows([]string{"user_id"}).AddRow("user-1"))
	mock.ExpectQuery("SELECT count").WithArgs("user-1", "SKU-1").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(entity.MaxItemCount - 1))

	err = repo.Add(context.Background(), "user-1", "SKU-1", 2, uint64(entity.MaxItemCount)+10)
	require.ErrorIs(t, err, entity.ErrConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresRepository_Set(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	transactor := &mockTransactor{
		txFunc: func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		},
	}
	repo := NewPostgres(mock, transactor)

	mock.ExpectExec("INSERT INTO cart.carts").
		WithArgs("user-1").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	mock.ExpectExec("INSERT INTO cart.cart_items").
		WithArgs("user-1", "SKU-1", uint32(3)).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err = repo.Set(context.Background(), "user-1", "SKU-1", 3, 10)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresRepository_Set_InsufficientStock(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()
	repo := NewPostgres(mock, nil)
	require.ErrorIs(t, repo.Set(context.Background(), "user-1", "SKU-1", 11, 10), entity.ErrInsufficientStock)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresRepository_Set_ExceedsMaxItemCount(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()
	repo := NewPostgres(mock, nil)
	require.ErrorIs(t, repo.Set(context.Background(), "user-1", "SKU-1", entity.MaxItemCount+1, uint64(entity.MaxItemCount)+10), entity.ErrConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresRepository_Delete(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPostgres(mock, nil)

	mock.ExpectExec("DELETE FROM cart.cart_items").
		WithArgs("user-1", "SKU-1").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	err = repo.Delete(context.Background(), "user-1", "SKU-1")
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresRepository_Delete_MissingIsNoop(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPostgres(mock, nil)

	mock.ExpectExec("DELETE FROM cart.cart_items").
		WithArgs("user-1", "SKU-1").
		WillReturnResult(pgxmock.NewResult("DELETE", 0))

	require.NoError(t, repo.Delete(context.Background(), "user-1", "SKU-1"))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresRepository_Clear(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPostgres(mock, nil)

	mock.ExpectExec("DELETE FROM cart.cart_items").
		WithArgs("user-1").
		WillReturnResult(pgxmock.NewResult("DELETE", 2))

	err = repo.Clear(context.Background(), "user-1")
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
