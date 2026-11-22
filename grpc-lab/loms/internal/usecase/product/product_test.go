package product_test

import (
	"context"
	"errors"
	"testing"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
	productrepo "github.com/igoroutine-courses/microservices.ecommerce.loms/internal/repository/product"
	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/usecase/product"
)

func newService() interface {
	Create(context.Context, entity.Product) (string, error)
	Get(context.Context, string) (entity.Product, error)
	Update(context.Context, entity.Product) error
	List(context.Context, string, int) (entity.ProductPage, error)
} {
	return product.New(productrepo.New())
}

func TestCreateCanonicalizesSKUAndActivatesProduct(t *testing.T) {
	svc, ctx := newService(), context.Background()

	sku, err := svc.Create(ctx, entity.Product{SKU: " mug-1 ", Name: "Mug", Price: 15000})
	if err != nil {
		t.Fatal(err)
	}

	if sku != "MUG-1" {
		t.Fatalf("want canonical SKU MUG-1, got %q", sku)
	}

	got, err := svc.Get(ctx, "mug-1")
	if err != nil {
		t.Fatal(err)
	}

	want := entity.Product{SKU: "MUG-1", Name: "Mug", Price: 15000, Active: true}
	if got != want {
		t.Fatalf("want %+v, got %+v", want, got)
	}
}

func TestCreateDuplicateIgnoresCase(t *testing.T) {
	svc, ctx := newService(), context.Background()

	if _, err := svc.Create(ctx, entity.Product{SKU: "abc", Name: "n", Price: 1}); err != nil {
		t.Fatal(err)
	}

	_, err := svc.Create(ctx, entity.Product{SKU: "ABC", Name: "n", Price: 1})
	if !errors.Is(err, entity.ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
}

func TestCreateAndUpdateRejectInvalidProducts(t *testing.T) {
	svc, ctx := newService(), context.Background()

	if _, err := svc.Create(ctx, entity.Product{SKU: "OK", Name: "n", Price: 1}); err != nil {
		t.Fatal(err)
	}

	invalid := []entity.Product{
		{SKU: "", Name: "n", Price: 1},
		{SKU: "OK", Name: "", Price: 1},
		{SKU: "OK", Name: "   ", Price: 1},
		{SKU: "OK", Name: "n", Price: 0},
		{SKU: "OK", Name: "n", Price: entity.MaxProductPrice + 1},
	}

	for _, p := range invalid {
		if _, err := svc.Create(ctx, p); !errors.Is(err, entity.ErrInvalid) {
			t.Errorf("Create(%+v): want ErrInvalid, got %v", p, err)
		}

		if err := svc.Update(ctx, p); !errors.Is(err, entity.ErrInvalid) {
			t.Errorf("Update(%+v): want ErrInvalid, got %v", p, err)
		}
	}
}

func TestUpdateReplacesFieldsAndFailsForUnknownSKU(t *testing.T) {
	svc, ctx := newService(), context.Background()

	if _, err := svc.Create(ctx, entity.Product{SKU: "mug", Name: "Mug", Price: 100}); err != nil {
		t.Fatal(err)
	}

	if err := svc.Update(ctx, entity.Product{SKU: "MUG", Name: "New", Price: 200, Active: false}); err != nil {
		t.Fatal(err)
	}

	got, err := svc.Get(ctx, "mug")
	if err != nil {
		t.Fatal(err)
	}

	want := entity.Product{SKU: "MUG", Name: "New", Price: 200, Active: false}
	if got != want {
		t.Fatalf("want %+v, got %+v", want, got)
	}

	err = svc.Update(ctx, entity.Product{SKU: "UNKNOWN", Name: "n", Price: 1})
	if !errors.Is(err, entity.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestListPaginatesWithCaseInsensitiveCursor(t *testing.T) {
	svc, ctx := newService(), context.Background()

	for _, sku := range []string{"z-c", "Z-A", "z-b"} {
		if _, err := svc.Create(ctx, entity.Product{SKU: sku, Name: "n", Price: 1}); err != nil {
			t.Fatal(err)
		}
	}

	cursor := "z-" // курсор в нижнем регистре, как присылает клиент
	for _, wantSKU := range []string{"Z-A", "Z-B", "Z-C"} {
		page, err := svc.List(ctx, cursor, 1)
		if err != nil {
			t.Fatal(err)
		}

		if len(page.Products) != 1 || page.Products[0].SKU != wantSKU {
			t.Fatalf("want page [%s], got %+v", wantSKU, page.Products)
		}

		if wantSKU == "Z-C" {
			if page.NextAfterSKU != "" {
				t.Fatalf("last page must have empty cursor, got %q", page.NextAfterSKU)
			}

			continue
		}

		if page.NextAfterSKU != wantSKU {
			t.Fatalf("want next cursor %s, got %q", wantSKU, page.NextAfterSKU)
		}

		cursor = page.NextAfterSKU
	}
}

func TestListRejectsNonPositiveLimit(t *testing.T) {
	_, err := newService().List(context.Background(), "", 0)
	if !errors.Is(err, entity.ErrInvalid) {
		t.Fatalf("want ErrInvalid, got %v", err)
	}
}
