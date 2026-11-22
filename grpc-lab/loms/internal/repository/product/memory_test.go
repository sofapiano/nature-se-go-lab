package product

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
)

func TestCreateIsAtomicUnderConcurrency(t *testing.T) {
	repo := New()
	ctx := context.Background()

	const workers = 12

	errs := make([]error, workers)

	var wg sync.WaitGroup
	for i := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = repo.Create(ctx, entity.Product{SKU: "SAME", Name: "n", Price: uint32(100 + i), Active: true})
		}()
	}
	wg.Wait()

	winners := 0
	for _, err := range errs {
		switch {
		case err == nil:
			winners++
		case !errors.Is(err, entity.ErrConflict):
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if winners != 1 {
		t.Fatalf("want exactly 1 successful Create, got %d", winners)
	}
}

func TestListReturnsSortedPagesAfterCursor(t *testing.T) {
	repo := New()
	ctx := context.Background()

	for _, sku := range []string{"C", "A", "B"} {
		if err := repo.Create(ctx, entity.Product{SKU: sku, Name: sku, Price: 1}); err != nil {
			t.Fatal(err)
		}
	}

	cases := []struct {
		name  string
		after string
		limit int
		want  []string
	}{
		{"from the start", "", 2, []string{"A", "B"}},
		{"after existing cursor", "A", 10, []string{"B", "C"}},
		{"after missing cursor", "AA", 10, []string{"B", "C"}},
		{"after the last", "C", 10, nil},
		{"zero limit", "", 0, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := repo.List(ctx, tc.after, tc.limit)
			if err != nil {
				t.Fatal(err)
			}

			if len(got) != len(tc.want) {
				t.Fatalf("want %v, got %v", tc.want, got)
			}

			for i, p := range got {
				if p.SKU != tc.want[i] {
					t.Fatalf("position %d: want %s, got %s", i, tc.want[i], p.SKU)
				}
			}
		})
	}
}

func TestGetAndUpdateUnknownProduct(t *testing.T) {
	repo := New()
	ctx := context.Background()

	if _, err := repo.Get(ctx, "NOPE"); !errors.Is(err, entity.ErrNotFound) {
		t.Fatalf("Get: want ErrNotFound, got %v", err)
	}

	if err := repo.Update(ctx, entity.Product{SKU: "NOPE"}); !errors.Is(err, entity.ErrNotFound) {
		t.Fatalf("Update: want ErrNotFound, got %v", err)
	}
}
