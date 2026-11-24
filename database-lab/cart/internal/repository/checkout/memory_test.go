package checkout

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGet_NotFound(t *testing.T) {
	r := New()

	orderID, ok := r.Get(context.Background(), "req-unknown")
	require.False(t, ok)
	require.Empty(t, orderID)
}

func TestSave_AndGet(t *testing.T) {
	r := New()
	ctx := context.Background()

	err := r.Save(ctx, "req-1", "order-1")
	require.NoError(t, err)

	got, ok := r.Get(ctx, "req-1")
	require.True(t, ok)
	require.Equal(t, "order-1", got)
}

func TestSave_IsolatedByRequestID(t *testing.T) {
	r := New()
	ctx := context.Background()

	require.NoError(t, r.Save(ctx, "req-1", "order-1"))
	require.NoError(t, r.Save(ctx, "req-2", "order-2"))

	got1, _ := r.Get(ctx, "req-1")
	got2, _ := r.Get(ctx, "req-2")
	require.Equal(t, "order-1", got1)
	require.Equal(t, "order-2", got2)
}
