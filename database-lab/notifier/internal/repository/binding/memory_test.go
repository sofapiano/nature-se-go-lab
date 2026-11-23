package binding

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGet_Missing(t *testing.T) {
	r := New()

	chat, ok := r.Get(context.Background(), "unknown")
	require.False(t, ok)
	require.Empty(t, chat)
}

func TestSet_ThenGet(t *testing.T) {
	r := New()
	ctx := context.Background()

	r.Set(ctx, "u1", "1001")
	chat, ok := r.Get(ctx, "u1")
	require.True(t, ok)
	require.Equal(t, "1001", chat)
}

// TestSet_Overwrites — повторная привязка заменяет прежний чат (TASK.md).
func TestSet_Overwrites(t *testing.T) {
	r := New()
	ctx := context.Background()

	r.Set(ctx, "u1", "1001")
	r.Set(ctx, "u1", "-1002003")

	chat, ok := r.Get(ctx, "u1")
	require.True(t, ok)
	require.Equal(t, "-1002003", chat)
}

// TestSet_IsolatedByUser — привязки разных пользователей независимы.
func TestSet_IsolatedByUser(t *testing.T) {
	r := New()
	ctx := context.Background()

	r.Set(ctx, "u1", "1001")
	r.Set(ctx, "u2", "2002")

	chat1, _ := r.Get(ctx, "u1")
	chat2, _ := r.Get(ctx, "u2")
	require.Equal(t, "1001", chat1)
	require.Equal(t, "2002", chat2)
}
