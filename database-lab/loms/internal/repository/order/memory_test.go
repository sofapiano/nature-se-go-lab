package order

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
)

func seed(r *repository, ctx context.Context, id, user string) {
	r.Save(ctx, entity.Order{OrderID: id, UserID: user}, "req-"+id, nil)
}

func TestGet_Missing(t *testing.T) {
	r := New()

	_, err := r.Get(context.Background(), "nope")
	require.ErrorIs(t, err, entity.ErrNotFound)
}

func TestSave_ThenGet(t *testing.T) {
	r := New()
	ctx := context.Background()

	o := entity.Order{OrderID: "o1", UserID: "u1", Status: entity.StatusAwaitingPayment}
	r.Save(ctx, o, "req-1", []entity.Item{{SKU: "SKU-1", Count: 1}})

	got, err := r.Get(ctx, "o1")
	require.NoError(t, err)
	require.Equal(t, o, got)
}

func TestGetRequest_Missing(t *testing.T) {
	r := New()

	_, ok := r.GetRequest(context.Background(), "req-unknown")
	require.False(t, ok)
}

func TestGetRequest_Present(t *testing.T) {
	r := New()
	ctx := context.Background()

	items := []entity.Item{{SKU: "SKU-1", Count: 2}}
	r.Save(ctx, entity.Order{OrderID: "o1", UserID: "u1"}, "req-1", items)

	got, ok := r.GetRequest(ctx, "req-1")
	require.True(t, ok)
	require.Equal(t, "u1", got.UserID)
	require.Equal(t, "o1", got.OrderID)
	require.Equal(t, items, got.Items)
}

func TestUpdate_ReplacesState(t *testing.T) {
	r := New()
	ctx := context.Background()

	r.Save(ctx, entity.Order{OrderID: "o1", UserID: "u1", Status: entity.StatusAwaitingPayment}, "req-1", nil)

	updated := entity.Order{OrderID: "o1", UserID: "u1", Status: entity.StatusPaid}
	r.Update(ctx, updated)

	got, _ := r.Get(ctx, "o1")
	require.Equal(t, entity.StatusPaid, got.Status)
}

func TestListByUser_IsolatedByUser(t *testing.T) {
	r := New()
	ctx := context.Background()

	seed(r, ctx, "o1", "u1")
	seed(r, ctx, "o2", "u1")
	seed(r, ctx, "o3", "u2")

	got, err := r.ListByUser(ctx, "u1", "", 10)
	require.NoError(t, err)
	require.Len(t, got, 2)

	for _, o := range got {
		require.Equal(t, "u1", o.UserID)
	}
}

func TestListByUser_EmptyForUnknownUser(t *testing.T) {
	r := New()

	got, err := r.ListByUser(context.Background(), "unknown", "", 10)
	require.NoError(t, err)
	require.Empty(t, got)
}

// TestListByUser_PaginationBoundary — курсор строго больше; сумма страниц =
// исходный список без дублей и пропусков.
func TestListByUser_PaginationBoundary(t *testing.T) {
	r := New()
	ctx := context.Background()

	ids := []string{"a", "b", "c", "d", "e"}
	for _, id := range ids {
		seed(r, ctx, id, "u1")
	}

	var got []string
	cursor := ""
	for {
		page, err := r.ListByUser(ctx, "u1", cursor, 2)
		require.NoError(t, err)
		for _, o := range page {
			got = append(got, o.OrderID)
		}
		if len(page) < 2 {
			break
		}
		cursor = page[len(page)-1].OrderID
	}
	require.Equal(t, ids, got)
}

func TestListByUser_CursorExclusive(t *testing.T) {
	r := New()
	ctx := context.Background()

	seed(r, ctx, "a", "u1")
	seed(r, ctx, "b", "u1")
	seed(r, ctx, "c", "u1")

	got, _ := r.ListByUser(ctx, "u1", "a", 10)
	require.Len(t, got, 2)
	require.Equal(t, "b", got[0].OrderID)
	require.Equal(t, "c", got[1].OrderID)
}
