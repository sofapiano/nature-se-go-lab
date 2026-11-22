package stocks_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	. "github.com/igoroutine-courses/microservices.ecommerce.loms/internal/controller/stocks"
	mocks "github.com/igoroutine-courses/microservices.ecommerce.loms/internal/controller/stocks/mocks"
	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
	stockspb "github.com/igoroutine-courses/microservices.ecommerce.pkg/generated/loms/api/stocks/v1"
)

func setup(t *testing.T) (*Controller, *mocks.MockstocksService) {
	t.Helper()
	ctrl := gomock.NewController(t)
	svc := mocks.NewMockstocksService(ctrl)
	return New(svc), svc
}

func TestGetStock_Success(t *testing.T) {
	c, svc := setup(t)
	svc.EXPECT().Get(gomock.Any(), "SKU-1").Return(uint64(42), nil)

	resp, err := c.GetStock(context.Background(), &stockspb.GetStockRequest{Sku: "SKU-1"})
	require.NoError(t, err)
	require.Equal(t, uint64(42), resp.GetCount())
}

func TestGetStock_PropagatesError(t *testing.T) {
	c, svc := setup(t)
	svc.EXPECT().Get(gomock.Any(), "SKU-X").Return(uint64(0), entity.ErrNotFound)

	_, err := c.GetStock(context.Background(), &stockspb.GetStockRequest{Sku: "SKU-X"})
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestSetStock_Success(t *testing.T) {
	c, svc := setup(t)
	svc.EXPECT().Set(gomock.Any(), "SKU-1", uint64(10)).Return(nil)

	_, err := c.SetStock(context.Background(), &stockspb.SetStockRequest{Sku: "SKU-1", Count: 10})
	require.NoError(t, err)
}

func TestSetStock_InvalidCountMapsToInvalidArgument(t *testing.T) {
	c, svc := setup(t)
	svc.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(entity.ErrInvalid)

	_, err := c.SetStock(context.Background(), &stockspb.SetStockRequest{Sku: "SKU-1", Count: 0})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestReceiveStock_MapsRequest(t *testing.T) {
	c, svc := setup(t)

	svc.EXPECT().Receive(gomock.Any(), entity.StockReceipt{
		SKU:         "SKU-1",
		Count:       5,
		OperationID: "01900000-0000-7000-8000-000000000001",
	}).Return(nil)

	_, err := c.ReceiveStock(context.Background(), &stockspb.ReceiveStockRequest{
		Sku:         "SKU-1",
		Count:       5,
		OperationId: "01900000-0000-7000-8000-000000000001",
	})
	require.NoError(t, err)
}

func TestReceiveStock_PropagatesConflict(t *testing.T) {
	c, svc := setup(t)
	svc.EXPECT().Receive(gomock.Any(), gomock.Any()).Return(entity.ErrConflict)

	_, err := c.ReceiveStock(context.Background(), &stockspb.ReceiveStockRequest{Sku: "SKU-1"})
	require.Equal(t, codes.AlreadyExists, status.Code(err))
}
