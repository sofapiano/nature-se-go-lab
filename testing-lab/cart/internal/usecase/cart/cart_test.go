package cart_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/igoroutine-courses/microservices.ecommerce.cart/internal/entity"
	. "github.com/igoroutine-courses/microservices.ecommerce.cart/internal/usecase/cart"
	mocks "github.com/igoroutine-courses/microservices.ecommerce.cart/internal/usecase/cart/mocks"
)

// activeProduct — типовой ответ каталога: активный товар. Используется там,
// где каталог не является предметом проверки.
var activeProduct = entity.Product{SKU: "SKU-1", Name: "Widget", Price: 100, Active: true}

type deps struct {
	svc      *Service
	carts    *mocks.MockcartRepository
	checkout *mocks.MockcheckoutRepository
	catalog  *mocks.MockcatalogClient
	loms     *mocks.MocklomsClient
}

func setup(t *testing.T) deps {
	t.Helper()
	ctrl := gomock.NewController(t) // Fail() при неудовлетворённых ожиданиях

	d := deps{
		carts:    mocks.NewMockcartRepository(ctrl),
		checkout: mocks.NewMockcheckoutRepository(ctrl),
		catalog:  mocks.NewMockcatalogClient(ctrl),
		loms:     mocks.NewMocklomsClient(ctrl),
	}
	d.svc = New(d.carts, d.checkout, d.catalog, d.loms)
	return d
}

// ---------- AddItem ----------

func TestAddItem_SumsCountAndPassesStock(t *testing.T) {
	d := setup(t)

	d.catalog.EXPECT().GetProduct(gomock.Any(), "SKU-1").Return(activeProduct, nil)
	d.loms.EXPECT().GetStock(gomock.Any(), "SKU-1").Return(uint64(10), nil)
	// available пробрасывается в репозиторий: он под одним Lock проверит
	// «existing + count ≤ available» и запишет. Неверная available сломает
	// атомарность проверки остатка внутри репозитория.
	d.carts.EXPECT().Add(gomock.Any(), "user-1", "SKU-1", uint32(2), uint64(10)).Return(nil)

	require.NoError(t, d.svc.AddItem(context.Background(), "user-1", "SKU-1", 2))
}

func TestAddItem_NormalizesUserAndSKU(t *testing.T) {
	d := setup(t)

	d.catalog.EXPECT().GetProduct(gomock.Any(), "SKU-1").Return(activeProduct, nil)
	d.loms.EXPECT().GetStock(gomock.Any(), "SKU-1").Return(uint64(5), nil)
	d.carts.EXPECT().Add(gomock.Any(), "user-1", "SKU-1", uint32(1), uint64(5)).Return(nil)

	require.NoError(t, d.svc.AddItem(context.Background(), "USER-1", "sku-1", 1))
}

func TestAddItem_UnknownProduct_DoesNotTouchRepository(t *testing.T) {
	d := setup(t)

	d.catalog.EXPECT().GetProduct(gomock.Any(), "SKU-X").Return(entity.Product{}, entity.ErrNotFound)
	// carts.Add не ожидается — при вызове gomock упадёт с "unexpected call".

	err := d.svc.AddItem(context.Background(), "user-1", "SKU-X", 1)
	require.ErrorIs(t, err, entity.ErrNotFound)
}

func TestAddItem_InactiveProduct_DoesNotTouchRepository(t *testing.T) {
	d := setup(t)

	product := activeProduct
	product.Active = false
	d.catalog.EXPECT().GetProduct(gomock.Any(), "SKU-1").Return(product, nil)

	err := d.svc.AddItem(context.Background(), "user-1", "SKU-1", 1)
	require.ErrorIs(t, err, entity.ErrProductInactive)
}

func TestAddItem_StockUnavailable_Propagates(t *testing.T) {
	d := setup(t)

	d.catalog.EXPECT().GetProduct(gomock.Any(), "SKU-1").Return(activeProduct, nil)
	d.loms.EXPECT().GetStock(gomock.Any(), "SKU-1").Return(uint64(1), nil)
	// Проверку available+count делает репозиторий под Lock; usecase
	// пробрасывает его ошибку как есть.
	d.carts.EXPECT().Add(gomock.Any(), "user-1", "SKU-1", uint32(5), uint64(1)).
		Return(entity.ErrInsufficientStock)

	err := d.svc.AddItem(context.Background(), "user-1", "SKU-1", 5)
	require.ErrorIs(t, err, entity.ErrInsufficientStock)
}

func TestAddItem_DependencyError_IsNotSwallowed(t *testing.T) {
	d := setup(t)

	sentinel := errors.New("network down")
	d.catalog.EXPECT().GetProduct(gomock.Any(), "SKU-1").Return(entity.Product{}, sentinel)

	err := d.svc.AddItem(context.Background(), "user-1", "SKU-1", 1)
	require.ErrorIs(t, err, sentinel)
}

// ---------- SetItemQuantity ----------

func TestSetItemQuantity_ReplacesCount(t *testing.T) {
	d := setup(t)

	d.catalog.EXPECT().GetProduct(gomock.Any(), "SKU-1").Return(activeProduct, nil)
	d.loms.EXPECT().GetStock(gomock.Any(), "SKU-1").Return(uint64(10), nil)
	d.carts.EXPECT().Set(gomock.Any(), "user-1", "SKU-1", uint32(3), uint64(10)).Return(nil)

	require.NoError(t, d.svc.SetItemQuantity(context.Background(), "user-1", "SKU-1", 3))
}

// ---------- DeleteItem / ClearCart ----------

func TestDeleteItem_NormalizesAndCallsRepository(t *testing.T) {
	d := setup(t)

	d.carts.EXPECT().Delete(gomock.Any(), "user-1", "SKU-1").Return(nil)

	require.NoError(t, d.svc.DeleteItem(context.Background(), "USER-1", "sku-1"))
}

func TestDeleteItem_MissingItem_IsNoop(t *testing.T) {
	d := setup(t)

	d.carts.EXPECT().Delete(gomock.Any(), "user-1", "SKU-X").Return(nil)

	require.NoError(t, d.svc.DeleteItem(context.Background(), "user-1", "SKU-X"))
}

func TestClearCart_CallsRepository(t *testing.T) {
	d := setup(t)

	d.carts.EXPECT().Clear(gomock.Any(), "user-1").Return(nil)

	require.NoError(t, d.svc.ClearCart(context.Background(), "user-1"))
}

// ---------- ListCart ----------

func TestListCart_JoinsWithCatalogAndSumsTotal(t *testing.T) {
	d := setup(t)

	cart := entity.Cart{
		UserID: "user-1",
		Items: []entity.Item{
			{SKU: "SKU-1", Count: 2},
			{SKU: "SKU-2", Count: 3},
		},
	}
	d.carts.EXPECT().Get(gomock.Any(), "user-1").Return(cart, nil)
	d.catalog.EXPECT().GetProduct(gomock.Any(), "SKU-1").
		Return(entity.Product{SKU: "SKU-1", Name: "A", Price: 100, Active: true}, nil)
	d.catalog.EXPECT().GetProduct(gomock.Any(), "SKU-2").
		Return(entity.Product{SKU: "SKU-2", Name: "B", Price: 250, Active: true}, nil)

	items, total, err := d.svc.ListCart(context.Background(), "user-1")
	require.NoError(t, err)
	require.Equal(t, uint32(950), total) // 2*100 + 3*250
	require.Len(t, items, 2)
	require.Equal(t, "A", items[0].Name)
	require.Equal(t, uint32(100), items[0].Price)
}

func TestListCart_SkipsRemovedSKUs(t *testing.T) {
	d := setup(t)

	cart := entity.Cart{
		UserID: "user-1",
		Items: []entity.Item{
			{SKU: "SKU-1", Count: 1},
			{SKU: "SKU-X", Count: 5},
		},
	}
	d.carts.EXPECT().Get(gomock.Any(), "user-1").Return(cart, nil)
	d.catalog.EXPECT().GetProduct(gomock.Any(), "SKU-1").Return(activeProduct, nil)
	d.catalog.EXPECT().GetProduct(gomock.Any(), "SKU-X").
		Return(entity.Product{}, entity.ErrNotFound)

	items, total, err := d.svc.ListCart(context.Background(), "user-1")
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "SKU-1", items[0].SKU)
	require.Equal(t, uint32(100), total)
}

func TestListCart_NonNotFoundError_Propagates(t *testing.T) {
	d := setup(t)

	d.carts.EXPECT().Get(gomock.Any(), "user-1").Return(entity.Cart{
		UserID: "user-1",
		Items:  []entity.Item{{SKU: "SKU-1", Count: 1}},
	}, nil)

	sentinel := errors.New("loms unavailable")
	d.catalog.EXPECT().GetProduct(gomock.Any(), "SKU-1").Return(entity.Product{}, sentinel)

	_, _, err := d.svc.ListCart(context.Background(), "user-1")
	// Ключевое: недоступность каталога НЕ должна превращаться в пустую корзину.
	require.ErrorIs(t, err, sentinel)
}

// ---------- CheckoutCart ----------

func TestCheckout_EmptyCart_ReturnsErrCartEmpty(t *testing.T) {
	d := setup(t)

	d.checkout.EXPECT().Get(gomock.Any(), "req-1").Return("", false)
	d.carts.EXPECT().Get(gomock.Any(), "user-1").Return(entity.Cart{UserID: "user-1"}, nil)
	// loms.CreateOrder НЕ ожидается — при вызове gomock упадёт.

	_, err := d.svc.CheckoutCart(context.Background(), "user-1", "req-1")
	require.ErrorIs(t, err, entity.ErrCartEmpty)
}

func TestCheckout_Replay_ReturnsSameOrderID_WithoutNewOrder(t *testing.T) {
	d := setup(t)

	d.checkout.EXPECT().Get(gomock.Any(), "req-1").Return("order-1", true)
	// carts.Get / loms.CreateOrder / carts.Clear НЕ ожидаются.

	orderID, err := d.svc.CheckoutCart(context.Background(), "user-1", "req-1")
	require.NoError(t, err)
	require.Equal(t, "order-1", orderID)
}

func TestCheckout_Failure_KeepsCartAndDoesNotMarkDone(t *testing.T) {
	d := setup(t)

	d.checkout.EXPECT().Get(gomock.Any(), "req-1").Return("", false)
	cart := entity.Cart{
		UserID: "user-1",
		Items:  []entity.Item{{SKU: "SKU-1", Count: 2}},
	}
	d.carts.EXPECT().Get(gomock.Any(), "user-1").Return(cart, nil)

	lomsErr := errors.New("stock conflict")
	d.loms.EXPECT().CreateOrder(gomock.Any(), "user-1", "req-1", gomock.Any()).
		Return("", lomsErr)
	// Save и Clear НЕ ожидаются: при ошибке ключ не фиксируется и корзина
	// не теряется.

	_, err := d.svc.CheckoutCart(context.Background(), "user-1", "req-1")
	require.ErrorIs(t, err, lomsErr)
}

func TestCheckout_Success_SavesKeyAndClearsCart(t *testing.T) {
	d := setup(t)

	d.checkout.EXPECT().Get(gomock.Any(), "req-1").Return("", false)
	cart := entity.Cart{
		UserID: "user-1",
		Items: []entity.Item{
			{SKU: "SKU-1", Count: 1},
			{SKU: "SKU-2", Count: 2},
		},
	}
	d.carts.EXPECT().Get(gomock.Any(), "user-1").Return(cart, nil)

	// В LOMS уходят позиции из корзины (без name/price — их фиксирует LOMS
	// на момент оформления, чтобы заказ не менялся вслед за каталогом).
	expectedItems := []entity.Item{
		{SKU: "SKU-1", Count: 1},
		{SKU: "SKU-2", Count: 2},
	}
	d.loms.EXPECT().CreateOrder(gomock.Any(), "user-1", "req-1", expectedItems).
		Return("order-1", nil)

	// Порядок: сначала фиксируем ключ, затем чистим корзину. Если Clear
	// упадёт — ключ уже сохранён, повтор вернёт тот же order_id.
	gomock.InOrder(
		d.checkout.EXPECT().Save(gomock.Any(), "req-1", "order-1"),
		d.carts.EXPECT().Clear(gomock.Any(), "user-1").Return(nil),
	)

	orderID, err := d.svc.CheckoutCart(context.Background(), "user-1", "req-1")
	require.NoError(t, err)
	require.Equal(t, "order-1", orderID)
}

// ---------- Concurrency ----------

// TestCheckout_ConcurrentSameKey_CreatesOneOrder — 8 параллельных checkout с
// одним request_id должны создать ровно один заказ. stateMx сериализует
// Get/Save: первый видит «не сохранено», остальные — «уже есть».
func TestCheckout_ConcurrentSameKey_CreatesOneOrder(t *testing.T) {
	d := setup(t)

	cart := entity.Cart{
		UserID: "user-1",
		Items:  []entity.Item{{SKU: "SKU-1", Count: 1}},
	}

	d.checkout.EXPECT().Get(gomock.Any(), "req-1").Return("", false).Times(1)
	d.checkout.EXPECT().Get(gomock.Any(), "req-1").Return("order-1", true).Times(7)
	d.carts.EXPECT().Get(gomock.Any(), "user-1").Return(cart, nil).Times(1)
	d.loms.EXPECT().CreateOrder(gomock.Any(), "user-1", "req-1", gomock.Any()).
		Return("order-1", nil).Times(1)
	d.checkout.EXPECT().Save(gomock.Any(), "req-1", "order-1").Times(1)
	d.carts.EXPECT().Clear(gomock.Any(), "user-1").Return(nil).Times(1)

	const n = 8
	orderIDs := make([]string, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			<-start
			orderIDs[i], errs[i] = d.svc.CheckoutCart(context.Background(), "user-1", "req-1")
		}()
	}
	close(start)
	wg.Wait()

	for i := 0; i < n; i++ {
		require.NoError(t, errs[i], "goroutine %d", i)
		require.Equal(t, "order-1", orderIDs[i], "goroutine %d", i)
	}
}

func TestAddItem_Concurrent_AllSucceed(t *testing.T) {
	d := setup(t)

	const n = 8
	d.catalog.EXPECT().GetProduct(gomock.Any(), "SKU-1").Return(activeProduct, nil).Times(n)
	d.loms.EXPECT().GetStock(gomock.Any(), "SKU-1").Return(uint64(100), nil).Times(n)
	d.carts.EXPECT().Add(gomock.Any(), "user-1", "SKU-1", uint32(1), uint64(100)).
		Return(nil).Times(n)

	errs := make([]error, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			<-start
			errs[i] = d.svc.AddItem(context.Background(), "user-1", "SKU-1", 1)
		}()
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		require.NoError(t, err, "goroutine %d", i)
	}
}
