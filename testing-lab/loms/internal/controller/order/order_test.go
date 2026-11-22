package order_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	. "github.com/igoroutine-courses/microservices.ecommerce.loms/internal/controller/order"
	mocks "github.com/igoroutine-courses/microservices.ecommerce.loms/internal/controller/order/mocks"
	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
	lomspb "github.com/igoroutine-courses/microservices.ecommerce.pkg/generated/loms/api/loms/v1"
)

const (
	userID    = "01900000-0000-7000-8000-000000000001"
	requestID = "01900000-0000-7000-8000-000000000002"
	orderID   = "01900000-0000-7000-8000-000000000003"
)

func setup(t *testing.T) (*Controller, *mocks.MockorderService) {
	t.Helper()
	ctrl := gomock.NewController(t)
	svc := mocks.NewMockorderService(ctrl)
	return New(svc), svc
}

func TestCreateOrder_MapsItems(t *testing.T) {
	c, svc := setup(t)

	svc.EXPECT().CreateOrder(gomock.Any(), userID, requestID, []entity.Item{
		{SKU: "SKU-1", Count: 2},
		{SKU: "SKU-2", Count: 1},
	}).Return("order-1", nil)

	resp, err := c.CreateOrder(context.Background(), &lomspb.CreateOrderRequest{
		UserId:    userID,
		RequestId: requestID,
		Items: []*lomspb.ItemInput{
			{Sku: "SKU-1", Count: 2},
			{Sku: "SKU-2", Count: 1},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "order-1", resp.GetOrderId())
}

func TestCreateOrder_StockMapsToFailedPrecondition(t *testing.T) {
	c, svc := setup(t)
	svc.EXPECT().CreateOrder(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("", entity.ErrStock)

	_, err := c.CreateOrder(context.Background(), &lomspb.CreateOrderRequest{
		UserId: userID, RequestId: requestID,
	})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestGetOrder_Success(t *testing.T) {
	c, svc := setup(t)

	svc.EXPECT().GetOrder(gomock.Any(), orderID).Return(entity.Order{
		OrderID: orderID, UserID: userID, Status: entity.StatusAwaitingPayment,
		Currency: "RUB",
	}, nil)

	resp, err := c.GetOrder(context.Background(), &lomspb.GetOrderRequest{OrderId: orderID})
	require.NoError(t, err)
	require.Equal(t, orderID, resp.GetOrder().GetOrderId())
	require.Equal(t, lomspb.OrderStatus_ORDER_STATUS_AWAITING_PAYMENT, resp.GetOrder().GetStatus())
}

func TestGetOrder_NotFound(t *testing.T) {
	c, svc := setup(t)
	svc.EXPECT().GetOrder(gomock.Any(), gomock.Any()).Return(entity.Order{}, entity.ErrNotFound)

	_, err := c.GetOrder(context.Background(), &lomspb.GetOrderRequest{OrderId: orderID})
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestListOrders_MapsPage(t *testing.T) {
	c, svc := setup(t)

	svc.EXPECT().ListOrders(gomock.Any(), userID, "", uint32(2)).
		Return(entity.OrderPage{
			Orders: []entity.Order{
				{OrderID: "a", UserID: userID, Status: entity.StatusPaid},
				{OrderID: "b", UserID: userID, Status: entity.StatusCancelled},
			},
			NextAfterOrderID: "b",
		}, nil)

	resp, err := c.ListOrders(context.Background(), &lomspb.ListOrdersRequest{
		UserId: userID, Limit: 2,
	})
	require.NoError(t, err)
	require.Len(t, resp.GetOrders(), 2)
	require.Equal(t, "b", resp.GetNextAfterOrderId())
}

func TestPayOrder_Success(t *testing.T) {
	c, svc := setup(t)
	svc.EXPECT().PayOrder(gomock.Any(), orderID).Return(nil)

	_, err := c.PayOrder(context.Background(), &lomspb.PayOrderRequest{OrderId: orderID})
	require.NoError(t, err)
}

func TestPayOrder_DeclinedMapsToFailedPrecondition(t *testing.T) {
	c, svc := setup(t)
	svc.EXPECT().PayOrder(gomock.Any(), orderID).Return(entity.ErrDeclined)

	_, err := c.PayOrder(context.Background(), &lomspb.PayOrderRequest{OrderId: orderID})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestCancelOrder_Success(t *testing.T) {
	c, svc := setup(t)
	svc.EXPECT().CancelOrder(gomock.Any(), orderID).Return(nil)

	_, err := c.CancelOrder(context.Background(), &lomspb.CancelOrderRequest{OrderId: orderID})
	require.NoError(t, err)
}

func TestCancelOrder_StateMapsToFailedPrecondition(t *testing.T) {
	c, svc := setup(t)
	svc.EXPECT().CancelOrder(gomock.Any(), orderID).Return(entity.ErrState)

	_, err := c.CancelOrder(context.Background(), &lomspb.CancelOrderRequest{OrderId: orderID})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}
