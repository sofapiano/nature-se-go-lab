package stocks

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/db"
	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/db/dbtest"
	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
)

func newPostgresRepository(t *testing.T) *postgresRepository {
	t.Helper()
	pool := dbtest.NewPool(t, "loms", "../../../migrations")
	return NewPostgres(pool, db.NewTransactor(pool))
}

func TestPostgresSetAndGet(t *testing.T) {
	r := newPostgresRepository(t)
	ctx := context.Background()

	count, err := r.Get(ctx, "SKU-1")
	require.NoError(t, err)
	require.Zero(t, count)

	require.NoError(t, r.Set(ctx, "SKU-1", 7))
	count, err = r.Get(ctx, "SKU-1")
	require.NoError(t, err)
	require.Equal(t, uint64(7), count)
}

func TestPostgresReceiveIsIdempotent(t *testing.T) {
	r := newPostgresRepository(t)
	ctx := context.Background()
	receipt := entity.StockReceipt{OperationID: "op-1", SKU: "SKU-1", Count: 10}

	require.NoError(t, r.Receive(ctx, receipt))
	require.NoError(t, r.Receive(ctx, receipt))

	count, err := r.Get(ctx, "SKU-1")
	require.NoError(t, err)
	require.Equal(t, uint64(10), count)

	conflicting := receipt
	conflicting.Count = 11
	require.ErrorIs(t, r.Receive(ctx, conflicting), entity.ErrConflict)

	count, err = r.Get(ctx, "SKU-1")
	require.NoError(t, err)
	require.Equal(t, uint64(10), count)
}

func TestPostgresReserveAndRelease(t *testing.T) {
	r := newPostgresRepository(t)
	ctx := context.Background()
	require.NoError(t, r.Receive(ctx, entity.StockReceipt{
		OperationID: "op-1", SKU: "SKU-1", Count: 10,
	}))

	items := []entity.Item{{SKU: "SKU-1", Count: 4}}
	require.NoError(t, r.Reserve(ctx, items))

	count, err := r.Get(ctx, "SKU-1")
	require.NoError(t, err)
	require.Equal(t, uint64(6), count)

	require.NoError(t, r.Release(ctx, items))
	count, err = r.Get(ctx, "SKU-1")
	require.NoError(t, err)
	require.Equal(t, uint64(10), count)

	require.ErrorIs(t, r.Reserve(ctx, []entity.Item{{SKU: "SKU-1", Count: 11}}), entity.ErrStock)
	count, err = r.Get(ctx, "SKU-1")
	require.NoError(t, err)
	require.Equal(t, uint64(10), count)
}
