package binding

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/igoroutine-courses/microservices.ecommerce.notifier/internal/db/dbtest"
)

func TestPostgresSetGetAndOverwrite(t *testing.T) {
	r := NewPostgres(dbtest.NewPool(t, "notifier", "../../../migrations"))
	ctx := context.Background()

	chat, ok := r.Get(ctx, "u1")
	require.False(t, ok)
	require.Empty(t, chat)

	require.NoError(t, r.Set(ctx, "u1", "1001"))
	chat, ok = r.Get(ctx, "u1")
	require.True(t, ok)
	require.Equal(t, "1001", chat)

	require.NoError(t, r.Set(ctx, "u1", "-1002003"))
	chat, ok = r.Get(ctx, "u1")
	require.True(t, ok)
	require.Equal(t, "-1002003", chat)
}
