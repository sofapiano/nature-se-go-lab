package stocks

import (
	"context"
	"errors"
	"testing"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
	productrepo "github.com/igoroutine-courses/microservices.ecommerce.loms/internal/repository/product"
	stocksrepo "github.com/igoroutine-courses/microservices.ecommerce.loms/internal/repository/stocks"
)

// newService собирает сервис на настоящих in-memory репозиториях; в каталоге
// уже есть товар с SKU "MUG".
func newService(t *testing.T) *service {
	t.Helper()

	products := productrepo.New()
	if err := products.Create(context.Background(), entity.Product{SKU: "MUG", Name: "Mug", Price: 100, Active: true}); err != nil {
		t.Fatal(err)
	}

	return New(stocksrepo.New(), products)
}

func TestGetReturnsZeroForUnknownProduct(t *testing.T) {
	got, err := newService(t).Get(context.Background(), "unknown")
	if err != nil || got != 0 {
		t.Fatalf("want 0 without error, got %d, %v", got, err)
	}
}

func TestSetAndReceiveRequireExistingProduct(t *testing.T) {
	svc, ctx := newService(t), context.Background()

	if err := svc.Set(ctx, "UNKNOWN", 1); !errors.Is(err, entity.ErrNotFound) {
		t.Fatalf("Set: want ErrNotFound, got %v", err)
	}

	err := svc.Receive(ctx, entity.StockReceipt{OperationID: "op", SKU: "UNKNOWN", Count: 1})
	if !errors.Is(err, entity.ErrNotFound) {
		t.Fatalf("Receive: want ErrNotFound, got %v", err)
	}
}

func TestSKUCaseDoesNotMatter(t *testing.T) {
	svc, ctx := newService(t), context.Background()

	if err := svc.Set(ctx, "mug", 5); err != nil {
		t.Fatal(err)
	}

	if got, _ := svc.Get(ctx, "Mug"); got != 5 {
		t.Fatalf("want 5, got %d", got)
	}
}

func TestReceiveIgnoresCaseOfSKUAndOperationID(t *testing.T) {
	svc, ctx := newService(t), context.Background()

	if err := svc.Set(ctx, "MUG", 5); err != nil {
		t.Fatal(err)
	}

	first := entity.StockReceipt{OperationID: "0190AAAA-0000-7000-8000-000000000001", SKU: "MUG", Count: 7}
	again := entity.StockReceipt{OperationID: "0190aaaa-0000-7000-8000-000000000001", SKU: "mug", Count: 7}

	for _, receipt := range []entity.StockReceipt{first, again} {
		if err := svc.Receive(ctx, receipt); err != nil {
			t.Fatal(err)
		}
	}

	if got, _ := svc.Get(ctx, "MUG"); got != 12 {
		t.Fatalf("want 12, got %d", got)
	}

	different := again
	different.Count = 8
	if err := svc.Receive(ctx, different); !errors.Is(err, entity.ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
}

func TestReceiveOverflowIsStateError(t *testing.T) {
	svc, ctx := newService(t), context.Background()

	if err := svc.Set(ctx, "MUG", entity.MaxStock); err != nil {
		t.Fatal(err)
	}

	err := svc.Receive(ctx, entity.StockReceipt{OperationID: "op", SKU: "MUG", Count: 1})
	if !errors.Is(err, entity.ErrState) {
		t.Fatalf("want ErrState, got %v", err)
	}
}

func TestInvalidValuesAreRejected(t *testing.T) {
	svc, ctx := newService(t), context.Background()

	if err := svc.Set(ctx, "MUG", entity.MaxStock+1); !errors.Is(err, entity.ErrInvalid) {
		t.Errorf("Set above max: want ErrInvalid, got %v", err)
	}

	invalid := []entity.StockReceipt{
		{OperationID: "", SKU: "MUG", Count: 1},
		{OperationID: "op", SKU: "MUG", Count: 0},
		{OperationID: "op", SKU: "MUG", Count: entity.MaxStock + 1},
	}

	for _, receipt := range invalid {
		if err := svc.Receive(ctx, receipt); !errors.Is(err, entity.ErrInvalid) {
			t.Errorf("Receive(%+v): want ErrInvalid, got %v", receipt, err)
		}
	}
}
