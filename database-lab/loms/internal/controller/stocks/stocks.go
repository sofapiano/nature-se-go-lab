package stocks

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/controller/converter"
	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
	stockspb "github.com/igoroutine-courses/microservices.ecommerce.pkg/generated/loms/api/stocks/v1"
)

type Controller struct {
	service stocksService
}

var _ stockspb.StocksServer = (*Controller)(nil)

func New(service stocksService) *Controller {
	return &Controller{
		service: service,
	}
}

func (c *Controller) GetStock(ctx context.Context, req *stockspb.GetStockRequest) (*stockspb.GetStockResponse, error) {
	count, err := c.service.Get(ctx, req.GetSku())
	if err != nil {
		return nil, converter.Error(err)
	}

	return &stockspb.GetStockResponse{Count: count}, nil
}

func (c *Controller) SetStock(ctx context.Context, req *stockspb.SetStockRequest) (*emptypb.Empty, error) {
	if err := c.service.Set(ctx, req.GetSku(), req.GetCount()); err != nil {
		return nil, converter.Error(err)
	}

	return &emptypb.Empty{}, nil
}

func (c *Controller) ReceiveStock(ctx context.Context, req *stockspb.ReceiveStockRequest) (*emptypb.Empty, error) {
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
