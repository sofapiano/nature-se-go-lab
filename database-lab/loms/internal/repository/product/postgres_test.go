package product

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/db/dbtest"
	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
)

func TestPostgresCRUDAndList(t *testing.T) {
	r := NewPostgres(dbtest.NewPool(t, "loms", "../../../migrations"))
	ctx := context.Background()

	original := newProduct("SKU-1", "Widget", 100)
	require.NoError(t, r.Create(ctx, original))

	got, err := r.Get(ctx, "SKU-1")
	require.NoError(t, err)
	require.Equal(t, original, got)

	updated := entity.Product{SKU: "SKU-1", Name: "New", Price: 200, Active: false}
	require.NoError(t, r.Update(ctx, updated))
	got, err = r.Get(ctx, "SKU-1")
	require.NoError(t, err)
	require.Equal(t, updated, got)

	require.NoError(t, r.Create(ctx, newProduct("SKU-2", "Gadget", 300)))
	list, err := r.List(ctx, "SKU-1", 10)
	require.NoError(t, err)
	require.Equal(t, []entity.Product{newProduct("SKU-2", "Gadget", 300)}, list)
}

func TestPostgresCreateDuplicateReturnsConflict(t *testing.T) {
	r := NewPostgres(dbtest.NewPool(t, "loms", "../../../migrations"))
	ctx := context.Background()

	original := newProduct("SKU-1", "Original", 100)
	require.NoError(t, r.Create(ctx, original))

	err := r.Create(ctx, newProduct("SKU-1", "Changed", 999))
	require.ErrorIs(t, err, entity.ErrConflict)

	got, err := r.Get(ctx, "SKU-1")
	require.NoError(t, err)
	require.Equal(t, original, got)
}

func TestPostgresGetMissing(t *testing.T) {
	r := NewPostgres(dbtest.NewPool(t, "loms", "../../../migrations"))

	_, err := r.Get(context.Background(), "SKU-X")
	require.ErrorIs(t, err, entity.ErrNotFound)
}
