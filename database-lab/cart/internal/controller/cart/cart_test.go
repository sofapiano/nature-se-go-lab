package cart_test

import (
	"context"
	"errors"
	"testing"

	"buf.build/go/protovalidate"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	. "github.com/igoroutine-courses/microservices.ecommerce.cart/internal/controller/cart"
	mocks "github.com/igoroutine-courses/microservices.ecommerce.cart/internal/controller/cart/mocks"
	"github.com/igoroutine-courses/microservices.ecommerce.cart/internal/entity"
	cartpb "github.com/igoroutine-courses/microservices.ecommerce.pkg/generated/cart/api/cart/v1"
)

const (
	validUser = "01900000-0000-7000-8000-000000000001"
	validReq  = "01900000-0000-7000-8000-000000000002"
	validSKU  = "SKU-1"
)

func newController(t *testing.T) (*Controller, *mocks.MockcartService) {
	t.Helper()
	ctrl := gomock.NewController(t)
	svc := mocks.NewMockcartService(ctrl)

	validator, err := protovalidate.New()
	require.NoError(t, err)

	return New(svc, validator), svc
}

func TestAddItem_MapsToService(t *testing.T) {
	c, svc := newController(t)

	svc.EXPECT().AddItem(gomock.Any(), validUser, validSKU, uint32(2)).Return(nil)

	_, err := c.AddItem(context.Background(), &cartpb.AddItemRequest{
		UserId: validUser, Sku: validSKU, Count: 2,
	})
	require.NoError(t, err)
}

func TestAddItem_InvalidUUID_DoesNotReachService(t *testing.T) {
	c, _ := newController(t)

	// svc.EXPECT() не объявлен — если сервис вызовется, gomock упадёт.

	_, err := c.AddItem(context.Background(), &cartpb.AddItemRequest{
		UserId: "not-a-uuid", Sku: validSKU, Count: 2,
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestAddItem_NotFoundMapsToNotFound(t *testing.T) {
	c, svc := newController(t)
	svc.EXPECT().AddItem(gomock.Any(), validUser, validSKU, uint32(1)).Return(entity.ErrNotFound)

	_, err := c.AddItem(context.Background(), &cartpb.AddItemRequest{
		UserId: validUser, Sku: validSKU, Count: 1,
	})
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestAddItem_InsufficientStockMapsToFailedPrecondition(t *testing.T) {
	c, svc := newController(t)
	svc.EXPECT().AddItem(gomock.Any(), validUser, validSKU, uint32(1)).Return(entity.ErrInsufficientStock)

	_, err := c.AddItem(context.Background(), &cartpb.AddItemRequest{
		UserId: validUser, Sku: validSKU, Count: 1,
	})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

// TestAddItem_PassesStatusErrorVerbatim — если usecase уже вернул gRPC-статус
// (например, от LOMS), контроллер не должен переупаковывать его в Internal.
func TestAddItem_PassesStatusErrorVerbatim(t *testing.T) {
	c, svc := newController(t)
	svc.EXPECT().AddItem(gomock.Any(), validUser, validSKU, uint32(1)).
		Return(status.Error(codes.Unavailable, "loms down"))

	_, err := c.AddItem(context.Background(), &cartpb.AddItemRequest{
		UserId: validUser, Sku: validSKU, Count: 1,
	})
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func TestAddItem_ProductInactiveMapsToFailedPrecondition(t *testing.T) {
	c, svc := newController(t)
	svc.EXPECT().AddItem(gomock.Any(), validUser, validSKU, uint32(1)).Return(entity.ErrProductInactive)

	_, err := c.AddItem(context.Background(), &cartpb.AddItemRequest{
		UserId: validUser, Sku: validSKU, Count: 1,
	})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestAddItem_ConflictMapsToFailedPrecondition(t *testing.T) {
	c, svc := newController(t)
	svc.EXPECT().AddItem(gomock.Any(), validUser, validSKU, uint32(1)).Return(entity.ErrConflict)

	_, err := c.AddItem(context.Background(), &cartpb.AddItemRequest{
		UserId: validUser, Sku: validSKU, Count: 1,
	})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestSetItemQuantity_MapsToService(t *testing.T) {
	c, svc := newController(t)
	svc.EXPECT().SetItemQuantity(gomock.Any(), validUser, validSKU, uint32(5)).Return(nil)

	_, err := c.SetItemQuantity(context.Background(), &cartpb.SetItemQuantityRequest{
		UserId: validUser, Sku: validSKU, Count: 5,
	})
	require.NoError(t, err)
}

func TestDeleteItem_MapsToService(t *testing.T) {
	c, svc := newController(t)
	svc.EXPECT().DeleteItem(gomock.Any(), validUser, validSKU).Return(nil)

	_, err := c.DeleteItem(context.Background(), &cartpb.DeleteItemRequest{
		UserId: validUser, Sku: validSKU,
	})
	require.NoError(t, err)
}

func TestClearCart_MapsToService(t *testing.T) {
	c, svc := newController(t)
	svc.EXPECT().ClearCart(gomock.Any(), validUser).Return(nil)

	_, err := c.ClearCart(context.Background(), &cartpb.ClearCartRequest{
		UserId: validUser,
	})
	require.NoError(t, err)
}

func TestListCart_EmptyIsNotError(t *testing.T) {
	c, svc := newController(t)
	svc.EXPECT().ListCart(gomock.Any(), validUser).Return([]entity.Item(nil), uint32(0), nil)

	resp, err := c.ListCart(context.Background(), &cartpb.ListCartRequest{UserId: validUser})
	require.NoError(t, err)
	require.Empty(t, resp.GetItems())
	require.Zero(t, resp.GetTotalPrice())
}

func TestListCart_MapsItemsAndTotal(t *testing.T) {
	c, svc := newController(t)
	svc.EXPECT().ListCart(gomock.Any(), validUser).Return([]entity.Item{
		{SKU: "SKU-1", Count: 2, Name: "A", Price: 100},
	}, uint32(200), nil)

	resp, err := c.ListCart(context.Background(), &cartpb.ListCartRequest{UserId: validUser})
	require.NoError(t, err)
	require.Equal(t, uint32(200), resp.GetTotalPrice())
	require.Len(t, resp.GetItems(), 1)
	require.Equal(t, "A", resp.GetItems()[0].GetName())
}

func TestCheckout_EmptyCartMapsToFailedPrecondition(t *testing.T) {
	c, svc := newController(t)
	svc.EXPECT().CheckoutCart(gomock.Any(), validUser, validReq).Return("", entity.ErrCartEmpty)

	_, err := c.CheckoutCart(context.Background(), &cartpb.CheckoutCartRequest{
		UserId: validUser, RequestId: validReq,
	})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestCheckout_Success(t *testing.T) {
	c, svc := newController(t)
	svc.EXPECT().CheckoutCart(gomock.Any(), validUser, validReq).Return("order-1", nil)

	resp, err := c.CheckoutCart(context.Background(), &cartpb.CheckoutCartRequest{
		UserId: validUser, RequestId: validReq,
	})
	require.NoError(t, err)
	require.Equal(t, "order-1", resp.GetOrderId())
}

func TestCheckout_UnknownErrorMapsToInternal(t *testing.T) {
	c, svc := newController(t)
	svc.EXPECT().CheckoutCart(gomock.Any(), validUser, validReq).Return("", errors.New("boom"))

	_, err := c.CheckoutCart(context.Background(), &cartpb.CheckoutCartRequest{
		UserId: validUser, RequestId: validReq,
	})
	require.Equal(t, codes.Internal, status.Code(err))
}
