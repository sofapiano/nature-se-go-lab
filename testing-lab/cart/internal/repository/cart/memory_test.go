package cart

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/igoroutine-courses/microservices.ecommerce.cart/internal/entity"
)

func TestAdd_NewItem(t *testing.T) {
	r := New()
	ctx := context.Background()

	require.NoError(t, r.Add(ctx, "u1", "SKU-1", 2, 10))

	cart, err := r.Get(ctx, "u1")
	require.NoError(t, err)
	require.Len(t, cart.Items, 1)
	require.Equal(t, "SKU-1", cart.Items[0].SKU)
	require.Equal(t, uint32(2), cart.Items[0].Count)
}

func TestAdd_SumsExistingCount(t *testing.T) {
	r := New()
	ctx := context.Background()

	require.NoError(t, r.Add(ctx, "u1", "SKU-1", 2, 10))
	require.NoError(t, r.Add(ctx, "u1", "SKU-1", 3, 10))

	cart, _ := r.Get(ctx, "u1")
	require.Equal(t, uint32(5), cart.Items[0].Count)
}

func TestAdd_RespectsAvailable(t *testing.T) {
	r := New()
	ctx := context.Background()

	err := r.Add(ctx, "u1", "SKU-1", 5, 3)
	require.ErrorIs(t, err, entity.ErrInsufficientStock)

	cart, _ := r.Get(ctx, "u1")
	require.Empty(t, cart.Items)
}

func TestAdd_RespectsMaxItemCount(t *testing.T) {
	r := New()
	ctx := context.Background()

	err := r.Add(ctx, "u1", "SKU-1", entity.MaxItemCount+1, 1_000_000)
	require.ErrorIs(t, err, entity.ErrConflict)
}

func TestAdd_AvailableIncludesExisting(t *testing.T) {
	r := New()
	ctx := context.Background()

	require.NoError(t, r.Add(ctx, "u1", "SKU-1", 4, 10))
	require.NoError(t, r.Add(ctx, "u1", "SKU-1", 6, 10))

	cart, _ := r.Get(ctx, "u1")
	require.Equal(t, uint32(10), cart.Items[0].Count)
}

func TestAdd_OverflowAcrossAdds(t *testing.T) {
	r := New()
	ctx := context.Background()

	require.NoError(t, r.Add(ctx, "u1", "SKU-1", 8, 10))
	err := r.Add(ctx, "u1", "SKU-1", 5, 10) // 13 > 10
	require.ErrorIs(t, err, entity.ErrInsufficientStock)

	cart, _ := r.Get(ctx, "u1")
	require.Equal(t, uint32(8), cart.Items[0].Count)
}

func TestSet_ReplacesCount(t *testing.T) {
	r := New()
	ctx := context.Background()

	require.NoError(t, r.Add(ctx, "u1", "SKU-1", 4, 10))
	require.NoError(t, r.Set(ctx, "u1", "SKU-1", 2, 10))

	cart, _ := r.Get(ctx, "u1")
	require.Equal(t, uint32(2), cart.Items[0].Count)
}

func TestSet_ExceedsAvailable(t *testing.T) {
	r := New()
	ctx := context.Background()

	require.NoError(t, r.Add(ctx, "u1", "SKU-1", 2, 10))
	err := r.Set(ctx, "u1", "SKU-1", 11, 10)
	require.ErrorIs(t, err, entity.ErrInsufficientStock)

	cart, _ := r.Get(ctx, "u1")
	require.Equal(t, uint32(2), cart.Items[0].Count)
}

func TestSet_ExceedsMax(t *testing.T) {
	r := New()
	ctx := context.Background()

	err := r.Set(ctx, "u1", "SKU-1", entity.MaxItemCount+1, 1_000_000)
	require.ErrorIs(t, err, entity.ErrConflict)
}

func TestDelete_RemovesItem(t *testing.T) {
	r := New()
	ctx := context.Background()

	require.NoError(t, r.Add(ctx, "u1", "SKU-1", 1, 10))
	require.NoError(t, r.Delete(ctx, "u1", "SKU-1"))

	cart, _ := r.Get(ctx, "u1")
	require.Empty(t, cart.Items)
}

func TestDelete_MissingIsNoop(t *testing.T) {
	r := New()
	ctx := context.Background()

	require.NoError(t, r.Delete(ctx, "u1", "SKU-X"))
	require.NoError(t, r.Delete(ctx, "no-such-user", "SKU-1"))
}

func TestClear_RemovesAllItems(t *testing.T) {
	r := New()
	ctx := context.Background()

	require.NoError(t, r.Add(ctx, "u1", "SKU-1", 1, 10))
	require.NoError(t, r.Add(ctx, "u1", "SKU-2", 1, 10))
	require.NoError(t, r.Clear(ctx, "u1"))

	cart, _ := r.Get(ctx, "u1")
	require.Empty(t, cart.Items)
}

func TestClear_EmptyIsNoop(t *testing.T) {
	r := New()
	require.NoError(t, r.Clear(context.Background(), "u1"))
}

func TestUsersAreIndependent(t *testing.T) {
	r := New()
	ctx := context.Background()

	require.NoError(t, r.Add(ctx, "u1", "SKU-1", 1, 10))
	require.NoError(t, r.Add(ctx, "u2", "SKU-2", 5, 10))

	c1, _ := r.Get(ctx, "u1")
	c2, _ := r.Get(ctx, "u2")
	require.Equal(t, "SKU-1", c1.Items[0].SKU)
	require.Equal(t, "SKU-2", c2.Items[0].SKU)
}

// TestGet_ReturnsSnapshot — Get отдаёт копию, мутация результата не
// должна менять состояние хранилища.
func TestGet_ReturnsSnapshot(t *testing.T) {
	r := New()
	ctx := context.Background()

	require.NoError(t, r.Add(ctx, "u1", "SKU-1", 2, 10))
	cart1, _ := r.Get(ctx, "u1")
	cart1.Items[0].Count = 999

	cart2, _ := r.Get(ctx, "u1")
	require.Equal(t, uint32(2), cart2.Items[0].Count)
}

func TestConcurrentAdds_SameSKU(t *testing.T) {
	r := New()
	ctx := context.Background()

	const n = 20
	errs := make([]error, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			<-start
			errs[i] = r.Add(ctx, "u1", "SKU-1", 1, 100)
		}()
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		require.NoError(t, err, "goroutine %d", i)
	}
	cart, _ := r.Get(ctx, "u1")
	require.Equal(t, uint32(n), cart.Items[0].Count)
}

// TestConcurrentAdds_LastStock — ровно один из N претендентов получает
// последнюю единицу; проверяет атомарность проверки available+запись.
func TestConcurrentAdds_LastStock(t *testing.T) {
	r := New()
	ctx := context.Background()

	const n = 8
	errs := make([]error, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			<-start
			errs[i] = r.Add(ctx, "u1", "SKU-1", 1, 1)
		}()
	}
	close(start)
	wg.Wait()

	wins := 0
	for _, err := range errs {
		if err == nil {
			wins++
		}
	}
	require.Equal(t, 1, wins)

	cart, _ := r.Get(ctx, "u1")
	require.Equal(t, uint32(1), cart.Items[0].Count)
}
