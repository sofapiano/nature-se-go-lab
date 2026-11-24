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
	ctrl := gomock.NewController(t)
	d := deps{
		carts:    mocks.NewMockcartRepository(ctrl),
		checkout: mocks.NewMockcheckoutRepository(ctrl),
		catalog:  mocks.NewMockcatalogClient(ctrl),
		loms:     mocks.NewMocklomsClient(ctrl),
	}
	d.svc = New(d.carts, d.checkout, d.catalog, d.loms)
	return d
}

func TestAddItem_Success(t *testing.T) {
	d := setup(t)
	d.catalog.EXPECT().GetProduct(gomock.Any(), "SKU-1").Return(activeProduct, nil)
	d.loms.EXPECT().GetStock(gomock.Any(), "SKU-1").Return(uint64(10), nil)
	d.carts.EXPECT().Add(gomock.Any(), "user-1", "SKU-1", uint32(2), uint64(10)).Return(nil)
	require.NoError(t, d.svc.AddItem(context.Background(), "user-1", "SKU-1", 2))
}

func TestAddItem_InactiveProduct(t *testing.T) {
	d := setup(t)
	d.catalog.EXPECT().GetProduct(gomock.Any(), "SKU-1").Return(entity.Product{Active: false}, nil)
	require.ErrorIs(t, d.svc.AddItem(context.Background(), "user-1", "SKU-1", 1), entity.ErrProductInactive)
}

func TestAddItem_CatalogError(t *testing.T) {
	d := setup(t)
	errTest := errors.New("db down")
	d.catalog.EXPECT().GetProduct(gomock.Any(), "SKU-1").Return(entity.Product{}, errTest)
	require.ErrorIs(t, d.svc.AddItem(context.Background(), "user-1", "SKU-1", 1), errTest)
}

func TestAddItem_StockError(t *testing.T) {
	d := setup(t)
	d.catalog.EXPECT().GetProduct(gomock.Any(), "SKU-1").Return(activeProduct, nil)
	d.loms.EXPECT().GetStock(gomock.Any(), "SKU-1").Return(uint64(1), nil)
	d.carts.EXPECT().Add(gomock.Any(), "user-1", "SKU-1", uint32(5), uint64(1)).Return(entity.ErrInsufficientStock)
	require.ErrorIs(t, d.svc.AddItem(context.Background(), "user-1", "SKU-1", 5), entity.ErrInsufficientStock)
}

func TestSetItemQuantity_Success(t *testing.T) {
	d := setup(t)
	d.catalog.EXPECT().GetProduct(gomock.Any(), "SKU-1").Return(activeProduct, nil)
	d.loms.EXPECT().GetStock(gomock.Any(), "SKU-1").Return(uint64(10), nil)
	d.carts.EXPECT().Set(gomock.Any(), "user-1", "SKU-1", uint32(3), uint64(10)).Return(nil)
	require.NoError(t, d.svc.SetItemQuantity(context.Background(), "user-1", "SKU-1", 3))
}

func TestDeleteItem_Success(t *testing.T) {
	d := setup(t)
	d.carts.EXPECT().Delete(gomock.Any(), "user-1", "SKU-1").Return(nil)
	require.NoError(t, d.svc.DeleteItem(context.Background(), "user-1", "SKU-1"))
}

func TestClearCart_Success(t *testing.T) {
	d := setup(t)
	d.carts.EXPECT().Clear(gomock.Any(), "user-1").Return(nil)
	require.NoError(t, d.svc.ClearCart(context.Background(), "user-1"))
}

func TestListCart_Success(t *testing.T) {
	d := setup(t)
	d.carts.EXPECT().Get(gomock.Any(), "user-1").Return(entity.Cart{Items: []entity.Item{{SKU: "SKU-1", Count: 2}}}, nil)
	d.catalog.EXPECT().GetProduct(gomock.Any(), "SKU-1").Return(entity.Product{Name: "A", Price: 100, Active: true}, nil)
	items, total, err := d.svc.ListCart(context.Background(), "user-1")
	require.NoError(t, err)
	require.Equal(t, uint32(200), total)
	require.Len(t, items, 1)
}

func TestListCart_SkipsNotFound(t *testing.T) {
	d := setup(t)
	d.carts.EXPECT().Get(gomock.Any(), "user-1").Return(entity.Cart{Items: []entity.Item{{SKU: "SKU-X", Count: 1}}}, nil)
	d.catalog.EXPECT().GetProduct(gomock.Any(), "SKU-X").Return(entity.Product{}, entity.ErrNotFound)
	items, total, err := d.svc.ListCart(context.Background(), "user-1")
	require.NoError(t, err)
	require.Empty(t, items)
	require.Zero(t, total)
}

func TestCheckoutCart_Empty(t *testing.T) {
	d := setup(t)
	d.checkout.EXPECT().Get(gomock.Any(), "req-1").Return("", false)
	d.carts.EXPECT().Get(gomock.Any(), "user-1").Return(entity.Cart{}, nil)
	_, err := d.svc.CheckoutCart(context.Background(), "user-1", "req-1")
	require.ErrorIs(t, err, entity.ErrCartEmpty)
}

func TestCheckoutCart_Replay(t *testing.T) {
	d := setup(t)
	d.checkout.EXPECT().Get(gomock.Any(), "req-1").Return("order-1", true)
	orderID, err := d.svc.CheckoutCart(context.Background(), "user-1", "req-1")
	require.NoError(t, err)
	require.Equal(t, "order-1", orderID)
}

func TestCheckoutCart_LomsError(t *testing.T) {
	d := setup(t)
	d.checkout.EXPECT().Get(gomock.Any(), "req-1").Return("", false)
	d.carts.EXPECT().Get(gomock.Any(), "user-1").Return(entity.Cart{Items: []entity.Item{{SKU: "SKU-1", Count: 1}}}, nil)
	errTest := errors.New("loms down")
	d.loms.EXPECT().CreateOrder(gomock.Any(), "user-1", "req-1", gomock.Any()).Return("", errTest)
	_, err := d.svc.CheckoutCart(context.Background(), "user-1", "req-1")
	require.ErrorIs(t, err, errTest)
}

func TestCheckoutCart_Success(t *testing.T) {
	d := setup(t)
	d.checkout.EXPECT().Get(gomock.Any(), "req-1").Return("", false)
	d.carts.EXPECT().Get(gomock.Any(), "user-1").Return(entity.Cart{Items: []entity.Item{{SKU: "SKU-1", Count: 1}}}, nil)
	d.loms.EXPECT().CreateOrder(gomock.Any(), "user-1", "req-1", gomock.Any()).Return("order-1", nil)
	gomock.InOrder(
		d.checkout.EXPECT().Save(gomock.Any(), "req-1", "order-1").Return(nil),
		d.carts.EXPECT().Clear(gomock.Any(), "user-1").Return(nil),
	)
	orderID, err := d.svc.CheckoutCart(context.Background(), "user-1", "req-1")
	require.NoError(t, err)
	require.Equal(t, "order-1", orderID)
}

func TestCheckoutCart_Concurrent(t *testing.T) {
	d := setup(t)
	d.checkout.EXPECT().Get(gomock.Any(), "req-1").Return("", false).Times(1)
	d.checkout.EXPECT().Get(gomock.Any(), "req-1").Return("order-1", true).Times(3)
	d.carts.EXPECT().Get(gomock.Any(), "user-1").Return(entity.Cart{Items: []entity.Item{{SKU: "SKU-1", Count: 1}}}, nil).Times(1)
	d.loms.EXPECT().CreateOrder(gomock.Any(), "user-1", "req-1", gomock.Any()).Return("order-1", nil).Times(1)
	d.checkout.EXPECT().Save(gomock.Any(), "req-1", "order-1").Return(nil).Times(1)
	d.carts.EXPECT().Clear(gomock.Any(), "user-1").Return(nil).Times(1)

	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, _ = d.svc.CheckoutCart(context.Background(), "user-1", "req-1")
		}()
	}
	close(start)
	wg.Wait()
}
