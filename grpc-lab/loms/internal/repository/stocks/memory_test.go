package stocks

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
)

func TestGetReturnsZeroForUnknownSKU(t *testing.T) {
	got, err := New().Get(context.Background(), "NOPE")
	if err != nil || got != 0 {
		t.Fatalf("want 0 without error, got %d, %v", got, err)
	}
}

func TestSetReplacesBalance(t *testing.T) {
	repo, ctx := New(), context.Background()

	for _, count := range []uint64{10, 5, 5, 0} {
		if err := repo.Set(ctx, "A", count); err != nil {
			t.Fatal(err)
		}

		if got, _ := repo.Get(ctx, "A"); got != count {
			t.Fatalf("want %d, got %d", count, got)
		}
	}
}

func TestReceiveIsIdempotent(t *testing.T) {
	repo, ctx := New(), context.Background()
	receipt := entity.StockReceipt{OperationID: "op-1", SKU: "A", Count: 7}

	for range 3 {
		if err := repo.Receive(ctx, receipt); err != nil {
			t.Fatal(err)
		}
	}

	if got, _ := repo.Get(ctx, "A"); got != 7 {
		t.Fatalf("receipt must be applied once: want 7, got %d", got)
	}
}

func TestReceiveWithSameKeyAndDifferentDataConflicts(t *testing.T) {
	repo, ctx := New(), context.Background()

	if err := repo.Receive(ctx, entity.StockReceipt{OperationID: "op-1", SKU: "A", Count: 7}); err != nil {
		t.Fatal(err)
	}

	for _, other := range []entity.StockReceipt{
		{OperationID: "op-1", SKU: "A", Count: 8},
		{OperationID: "op-1", SKU: "B", Count: 7},
	} {
		if err := repo.Receive(ctx, other); !errors.Is(err, entity.ErrConflict) {
			t.Fatalf("%+v: want ErrConflict, got %v", other, err)
		}
	}

	if got, _ := repo.Get(ctx, "A"); got != 7 {
		t.Fatalf("want 7, got %d", got)
	}
}

func TestReceiveOverflowKeepsBalanceAndDoesNotRememberKey(t *testing.T) {
	repo, ctx := New(), context.Background()

	if err := repo.Set(ctx, "A", entity.MaxStock); err != nil {
		t.Fatal(err)
	}

	receipt := entity.StockReceipt{OperationID: "op-1", SKU: "A", Count: 1}
	if err := repo.Receive(ctx, receipt); !errors.Is(err, entity.ErrState) {
		t.Fatalf("want ErrState, got %v", err)
	}

	if got, _ := repo.Get(ctx, "A"); got != entity.MaxStock {
		t.Fatalf("balance must not change, got %d", got)
	}

	// Ключ не запомнился: после освобождения места та же операция проходит.
	if err := repo.Set(ctx, "A", 0); err != nil {
		t.Fatal(err)
	}

	if err := repo.Receive(ctx, receipt); err != nil {
		t.Fatal(err)
	}

	if got, _ := repo.Get(ctx, "A"); got != 1 {
		t.Fatalf("want 1, got %d", got)
	}
}

func TestConcurrentReceiptIsAppliedOnce(t *testing.T) {
	repo, ctx := New(), context.Background()
	receipt := entity.StockReceipt{OperationID: "op-1", SKU: "A", Count: 7}

	const workers = 12

	errs := make([]error, workers)

	var wg sync.WaitGroup
	for i := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = repo.Receive(ctx, receipt)
		}()
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			t.Fatalf("every retry must succeed, got %v", err)
		}
	}

	if got, _ := repo.Get(ctx, "A"); got != 7 {
		t.Fatalf("want 7, got %d", got)
	}
}
