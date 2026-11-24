package cart

import (
	"context"
	"testing"

	"github.com/pashagolub/pgxmock/v6"
	"github.com/stretchr/testify/require"
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

	transactor := &mockTransactor{
		txFunc: func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		},
	}
	repo := NewPostgres(mock, transactor)

	mock.ExpectExec("INSERT INTO cart.carts").
		WithArgs("user-1").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	mock.ExpectExec("UPDATE cart.cart_items").
		WithArgs("user-1", "SKU-1", uint32(2)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err = repo.Add(context.Background(), "user-1", "SKU-1", 2, 10)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresRepository_Add_InsertNew(t *testing.T) {
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

	// 0 затронутых строк эмулирует отсутствие товара, триггеря INSERT
	mock.ExpectExec("UPDATE cart.cart_items").
		WithArgs("user-1", "SKU-1", uint32(2)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	mock.ExpectExec("INSERT INTO cart.cart_items").
		WithArgs("user-1", "SKU-1", uint32(2)).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err = repo.Add(context.Background(), "user-1", "SKU-1", 2, 10)
	require.NoError(t, err)
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
