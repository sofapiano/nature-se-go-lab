package stocks

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/controller/converter"
	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
	stockspb "github.com/igoroutine-courses/microservices.ecommerce.pkg/generated/loms/api/stocks/v1"
)

type stocksService interface {
	Get(ctx context.Context, sku string) (uint64, error)
	Set(ctx context.Context, sku string, count uint64) error
	Receive(ctx context.Context, receipt entity.StockReceipt) error
}

type controller struct {
	service stocksService
}

var _ stockspb.StocksServer = (*controller)(nil)

func New(service stocksService) *controller {
	return &controller{
		service: service,
	}
}

func (c *controller) GetStock(ctx context.Context, req *stockspb.GetStockRequest) (*stockspb.GetStockResponse, error) {
	count, err := c.service.Get(ctx, req.GetSku())
	if err != nil {
		return nil, converter.Error(err)
	}

	return &stockspb.GetStockResponse{Count: count}, nil
}

func (c *controller) SetStock(ctx context.Context, req *stockspb.SetStockRequest) (*emptypb.Empty, error) {
	if req.Count == nil {
		return nil, status.Error(codes.InvalidArgument, "count is required")
	}
	if err := c.service.Set(ctx, req.GetSku(), req.GetCount()); err != nil {
		return nil, converter.Error(err)
	}

	return &emptypb.Empty{}, nil
}

func (c *controller) ReceiveStock(ctx context.Context, req *stockspb.ReceiveStockRequest) (*emptypb.Empty, error) {
	err := c.service.Receive(ctx, entity.StockReceipt{
		OperationID: req.GetOperationId(),
		SKU:         req.GetSku(),
		Count:       req.GetCount(),
	})
	if err != nil {
		return nil, converter.Error(err)
	}

	return &emptypb.Empty{}, nil
}
