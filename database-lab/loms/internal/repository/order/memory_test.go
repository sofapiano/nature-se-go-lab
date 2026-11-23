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

func TestMemoryRepository_Get_NotFound(t *testing.T) {
	repo := order.NewMemory()
	ctx := context.Background()

	_, err := repo.Get(ctx, "non-existent-order")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != entity.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMemoryRepository_SaveAndGetRequest(t *testing.T) {
	repo := order.NewMemory()
	ctx := context.Background()

	req := entity.OrderRequest{
		RequestID: "req-1",
		UserID:    "user-1",
		OrderID:   "order-1",
		Items:     []entity.Item{{SKU: "SKU-1", Count: 2}},
	}
	orderEnt := entity.Order{
		OrderID:         "order-1",
		UserID:          "user-1",
		Status:          "new",
		TotalAmountMinor: 200,
		Currency:        "RUB",
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		History:         []entity.Transition{{Status: "new", OccurredAt: time.Now()}},
	}

	repo.Save(ctx, orderEnt, req.RequestID, req.Items)

	gotReq, found := repo.GetRequest(ctx, "req-1")
	if !found {
		t.Fatal("expected request to be found")
	}
	if gotReq.RequestID != "req-1" || gotReq.UserID != "user-1" || gotReq.OrderID != "order-1" {
		t.Errorf("unexpected request: %+v", gotReq)
	}
	if len(gotReq.Items) != 1 || gotReq.Items[0].SKU != "SKU-1" {
		t.Errorf("unexpected request items: %+v", gotReq.Items)
	}
}

func TestMemoryRepository_Update(t *testing.T) {
	repo := order.NewMemory()
	ctx := context.Background()

	ord := entity.Order{
		OrderID:          "order-1",
		UserID:           "user-1",
		Status:           "new",
		TotalAmountMinor: 100,
		Currency:         "RUB",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	repo.Save(ctx, ord, "req-1", nil)

	ord.Status = "paid"
	ord.UpdatedAt = time.Now()
	ord.Payments = []entity.PaymentSummary{
		{PaymentID: "p1", ExternalID: "ext1", AmountMinor: 100, Currency: "RUB", Status: "succeeded"},
	}
	ord.History = append(ord.History, entity.Transition{Status: "paid", OccurredAt: time.Now()})

	repo.Update(ctx, ord)

	got, err := repo.Get(ctx, "order-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != "paid" {
		t.Errorf("expected status paid, got %s", got.Status)
	}
	if len(got.Payments) != 1 || got.Payments[0].ExternalID != "ext1" {
		t.Errorf("unexpected payments: %+v", got.Payments)
	}
}

func TestMemoryRepository_ListByUser(t *testing.T) {
	repo := order.NewMemory()
	ctx := context.Background()

	now := time.Now()
	repo.Save(ctx, entity.Order{OrderID: "ord-1", UserID: "user-1", Status: "new", TotalAmountMinor: 100, Currency: "RUB", CreatedAt: now, UpdatedAt: now}, "req-1", nil)
	repo.Save(ctx, entity.Order{OrderID: "ord-2", UserID: "user-1", Status: "paid", TotalAmountMinor: 200, Currency: "RUB", CreatedAt: now, UpdatedAt: now}, "req-2", nil)
	repo.Save(ctx, entity.Order{OrderID: "ord-3", UserID: "user-2", Status: "new", TotalAmountMinor: 300, Currency: "RUB", CreatedAt: now, UpdatedAt: now}, "req-3", nil)

	// Test pagination and filtering
	list, err := repo.ListByUser(ctx, "user-1", "", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 orders, got %d", len(list))
	}

	// Test cursor
	list, err = repo.ListByUser(ctx, "user-1", "ord-1", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 1 || list[0].OrderID != "ord-2" {
		t.Fatalf("expected ord-2, got %+v", list)
	}
}
