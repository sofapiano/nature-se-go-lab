package db

import (
	"context"
	"testing"

	"github.com/pashagolub/pgxmock/v6"
	"github.com/stretchr/testify/require"
)

func TestTxFromContext_Empty(t *testing.T) {
	_, ok := TxFromContext(context.Background())
	require.False(t, ok)
}

func TestContextWithTx_RoundTrip_AndExecutor(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	ctx := ContextWithTx(context.Background(), mock)
	tx, ok := TxFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, mock, tx)
	require.Equal(t, mock, GetExecutor(ctx, nil))
}
