package db

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pashagolub/pgxmock/v6"
	"github.com/stretchr/testify/require"
)

func TestWithTx_Nested_RunsFnWithoutNewTx(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	ctx := ContextWithTx(context.Background(), mock)
	called := false
	err = NewTransactor(nil).WithTx(ctx, func(context.Context) error {
		called = true
		return nil
	})
	require.NoError(t, err)
	require.True(t, called)
	require.NoError(t, mock.ExpectationsWereMet()) // Begin не ожидался
}

func TestWithTx_BeginError(t *testing.T) {
	cfg, err := pgxpool.ParseConfig("postgres://u:p@127.0.0.1:1/db?sslmode=disable")
	require.NoError(t, err)
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	require.NoError(t, err)
	defer pool.Close()

	err = NewTransactor(pool).WithTx(context.Background(), func(context.Context) error {
		return errors.New("must not run")
	})
	require.ErrorContains(t, err, "begin transaction")
}
