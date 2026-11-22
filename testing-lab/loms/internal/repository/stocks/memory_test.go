package stocks

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
)

func TestGet_UnknownSKUReturnsZero(t *testing.T) {
	r := New()

	got, err := r.Get(context.Background(), "SKU-X")
	require.NoError(t, err)
	require.Zero(t, got)
}

func TestSet_ReplacesBalance(t *testing.T) {
	r := New()
	ctx := context.Background()

	require.NoError(t, r.Set(ctx, "SKU-1", 10))
	require.NoError(t, r.Set(ctx, "SKU-1", 3))

	got, _ := r.Get(ctx, "SKU-1")
	require.Equal(t, uint64(3), got)
}

func TestReceive_AppliesOnce(t *testing.T) {
	r := New()
	ctx := context.Background()

	require.NoError(t, r.Receive(ctx, entity.StockReceipt{SKU: "SKU-1", Count: 5, OperationID: "op-1"}))

	got, _ := r.Get(ctx, "SKU-1")
	require.Equal(t, uint64(5), got)
}

// TestReceive_SameOperationIDIdempotent — ключевое требование TASK.md:
// повтор с теми же данными не увеличивает остаток повторно.
func TestReceive_SameOperationIDIdempotent(t *testing.T) {
	r := New()
	ctx := context.Background()

	receipt := entity.StockReceipt{SKU: "SKU-1", Count: 5, OperationID: "op-1"}
	require.NoError(t, r.Receive(ctx, receipt))
	require.NoError(t, r.Receive(ctx, receipt))
	require.NoError(t, r.Receive(ctx, receipt))

	got, _ := r.Get(ctx, "SKU-1")
	require.Equal(t, uint64(5), got) // не 15
}

// TestReceive_SameOperationIDDifferentData_Conflict — ErrConflict, состояние
// не меняется.
func TestReceive_SameOperationIDDifferentData_Conflict(t *testing.T) {
	r := New()
	ctx := context.Background()

	require.NoError(t, r.Receive(ctx, entity.StockReceipt{SKU: "SKU-1", Count: 5, OperationID: "op-1"}))

	err := r.Receive(ctx, entity.StockReceipt{SKU: "SKU-1", Count: 7, OperationID: "op-1"})
	require.ErrorIs(t, err, entity.ErrConflict)

	got, _ := r.Get(ctx, "SKU-1")
	require.Equal(t, uint64(5), got)
}

// TestReceive_OverflowReturnsErrState — не применяем, ключ НЕ запоминаем:
// после исправления остатка операцию можно повторить с тем же ключом.
func TestReceive_OverflowReturnsErrState(t *testing.T) {
	r := New()
	ctx := context.Background()

	require.NoError(t, r.Set(ctx, "SKU-1", entity.MaxStock))

	err := r.Receive(ctx, entity.StockReceipt{SKU: "SKU-1", Count: 1, OperationID: "op-1"})
	require.ErrorIs(t, err, entity.ErrState)

	got, _ := r.Get(ctx, "SKU-1")
	require.EqualValues(t, entity.MaxStock, got)

	// Ключ не зафиксирован — можно повторить после уменьшения остатка.
	require.NoError(t, r.Set(ctx, "SKU-1", 0))
	require.NoError(t, r.Receive(ctx, entity.StockReceipt{SKU: "SKU-1", Count: 1, OperationID: "op-1"}))
	got, _ = r.Get(ctx, "SKU-1")
	require.Equal(t, uint64(1), got)
}

// TestReceive_ConcurrentSameOperationID — 20 параллельных повторов с одним
// ключом дают одну запись. Проверяет атомарность ключа+записи под Lock.
func TestReceive_ConcurrentSameOperationID(t *testing.T) {
	r := New()
	ctx := context.Background()

	const n = 20
	receipt := entity.StockReceipt{SKU: "SKU-1", Count: 3, OperationID: "op-1"}
	errs := make([]error, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			<-start
			errs[i] = r.Receive(ctx, receipt)
		}()
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		require.NoError(t, err, "goroutine %d", i)
	}
	got, _ := r.Get(ctx, "SKU-1")
	require.Equal(t, uint64(3), got) // не 60
}

// ---------- Reserve / Release ----------

func TestReserve_SucceedsWhenAllAvailable(t *testing.T) {
	r := New()
	ctx := context.Background()

	require.NoError(t, r.Set(ctx, "A", 10))
	require.NoError(t, r.Set(ctx, "B", 5))

	require.NoError(t, r.Reserve(ctx, []entity.Item{
		{SKU: "A", Count: 3},
		{SKU: "B", Count: 2},
	}))

	a, _ := r.Get(ctx, "A")
	b, _ := r.Get(ctx, "B")
	require.Equal(t, uint64(7), a)
	require.Equal(t, uint64(3), b)
}

// TestReserve_AtomicOnPartialFailure — если хотя бы одной позиции не хватает,
// не списывается НИ ОДНА. Это требование TASK.md: «не резервируются остальные
// позиции».
func TestReserve_AtomicOnPartialFailure(t *testing.T) {
	r := New()
	ctx := context.Background()

	require.NoError(t, r.Set(ctx, "A", 10))
	require.NoError(t, r.Set(ctx, "B", 1))

	err := r.Reserve(ctx, []entity.Item{
		{SKU: "A", Count: 3},
		{SKU: "B", Count: 5}, // не хватает
	})
	require.ErrorIs(t, err, entity.ErrStock)

	a, _ := r.Get(ctx, "A")
	require.Equal(t, uint64(10), a) // A не тронут
}

// TestReserve_ConcurrentLastUnit — ровно один из N претендентов получает
// последнюю единицу.
func TestReserve_ConcurrentLastUnit(t *testing.T) {
	r := New()
	ctx := context.Background()

	require.NoError(t, r.Set(ctx, "SKU-1", 1))

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
			errs[i] = r.Reserve(ctx, []entity.Item{{SKU: "SKU-1", Count: 1}})
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

	got, _ := r.Get(ctx, "SKU-1")
	require.Zero(t, got)
}

func TestRelease_AddsBack(t *testing.T) {
	r := New()
	ctx := context.Background()

	require.NoError(t, r.Set(ctx, "SKU-1", 5))
	require.NoError(t, r.Reserve(ctx, []entity.Item{{SKU: "SKU-1", Count: 3}}))
	require.NoError(t, r.Release(ctx, []entity.Item{{SKU: "SKU-1", Count: 3}}))

	got, _ := r.Get(ctx, "SKU-1")
	require.Equal(t, uint64(5), got)
}
