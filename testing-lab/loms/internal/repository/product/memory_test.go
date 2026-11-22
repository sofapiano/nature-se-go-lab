package product

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
)

func newProduct(sku, name string, price uint32) entity.Product {
	return entity.Product{SKU: sku, Name: name, Price: price, Active: true}
}

func TestCreate_NewProduct(t *testing.T) {
	r := New()
	ctx := context.Background()

	p := newProduct("SKU-1", "Widget", 100)
	require.NoError(t, r.Create(ctx, p))

	got, err := r.Get(ctx, "SKU-1")
	require.NoError(t, err)
	require.Equal(t, p, got)
}

// TestCreate_DuplicateReturnsConflictAndDoesNotOverwrite — ключевое:
// повторное создание не должно «затирать» уже существующий товар (цена и
// название могли использоваться в оформленных заказах).
func TestCreate_DuplicateReturnsConflictAndDoesNotOverwrite(t *testing.T) {
	r := New()
	ctx := context.Background()

	original := newProduct("SKU-1", "Original", 100)
	require.NoError(t, r.Create(ctx, original))

	err := r.Create(ctx, newProduct("SKU-1", "Changed", 999))
	require.ErrorIs(t, err, entity.ErrConflict)

	got, _ := r.Get(ctx, "SKU-1")
	require.Equal(t, original, got)
}

func TestGet_Missing(t *testing.T) {
	r := New()

	_, err := r.Get(context.Background(), "SKU-X")
	require.ErrorIs(t, err, entity.ErrNotFound)
}

func TestUpdate_Existing(t *testing.T) {
	r := New()
	ctx := context.Background()

	require.NoError(t, r.Create(ctx, newProduct("SKU-1", "Old", 100)))

	updated := entity.Product{SKU: "SKU-1", Name: "New", Price: 200, Active: false}
	require.NoError(t, r.Update(ctx, updated))

	got, _ := r.Get(ctx, "SKU-1")
	require.Equal(t, updated, got)
}

func TestUpdate_Missing(t *testing.T) {
	r := New()

	err := r.Update(context.Background(), entity.Product{SKU: "SKU-X", Name: "W", Price: 100})
	require.ErrorIs(t, err, entity.ErrNotFound)
}

// TestList_PaginationBoundaries — идём страницами по 2 и проверяем, что
// сумма страниц = исходный список без дублей и пропусков. Именно этого
// требует TASK.md: «не пропускает и не дублирует элементы на границе».
func TestList_PaginationBoundaries(t *testing.T) {
	r := New()
	ctx := context.Background()

	for _, sku := range []string{"A", "B", "C", "D", "E"} {
		require.NoError(t, r.Create(ctx, newProduct(sku, "n-"+sku, 1)))
	}

	var got []string
	cursor := ""
	for {
		page, err := r.List(ctx, cursor, 2)
		require.NoError(t, err)
		for _, p := range page {
			got = append(got, p.SKU)
		}
		if len(page) < 2 {
			break
		}
		cursor = page[len(page)-1].SKU
	}

	require.Equal(t, []string{"A", "B", "C", "D", "E"}, got)
}

func TestList_CursorIsExclusive(t *testing.T) {
	r := New()
	ctx := context.Background()

	for _, sku := range []string{"A", "B", "C"} {
		require.NoError(t, r.Create(ctx, newProduct(sku, sku, 1)))
	}

	page, err := r.List(ctx, "A", 10)
	require.NoError(t, err)
	require.Len(t, page, 2)
	require.Equal(t, "B", page[0].SKU)
	require.Equal(t, "C", page[1].SKU)
}

// TestList_CursorNotPresent — курсор может отсутствовать (например, товар
// удалили): возвращаем всё, что строго больше по бинарному поиску.
func TestList_CursorNotPresent(t *testing.T) {
	r := New()
	ctx := context.Background()

	for _, sku := range []string{"A", "C", "E"} {
		require.NoError(t, r.Create(ctx, newProduct(sku, sku, 1)))
	}

	page, err := r.List(ctx, "B", 10)
	require.NoError(t, err)
	require.Len(t, page, 2)
	require.Equal(t, "C", page[0].SKU)
	require.Equal(t, "E", page[1].SKU)
}

// TestCreate_ConcurrentSameSKU_OneWins — проверка атомарности проверки
// «уже есть» + вставки под Lock.
func TestCreate_ConcurrentSameSKU_OneWins(t *testing.T) {
	r := New()
	ctx := context.Background()

	const n = 10
	errs := make([]error, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			<-start
			errs[i] = r.Create(ctx, newProduct("SKU-1", "W", 100))
		}()
	}
	close(start)
	wg.Wait()

	wins := 0
	for _, err := range errs {
		if err == nil {
			wins++
		} else {
			require.ErrorIs(t, err, entity.ErrConflict)
		}
	}
	require.Equal(t, 1, wins)
}
