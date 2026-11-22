package grpc

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/igoroutine-courses/microservices.ecommerce.cart/internal/entity"
	lomspb "github.com/igoroutine-courses/microservices.ecommerce.pkg/generated/loms/api/loms/v1"
	product "github.com/igoroutine-courses/microservices.ecommerce.pkg/generated/loms/api/product/v1"
	stocks "github.com/igoroutine-courses/microservices.ecommerce.pkg/generated/loms/api/stocks/v1"
)

type Client struct {
	products product.ProductServiceClient
	stocks   stocks.StocksClient
	loms     lomspb.LomsClient
}

func Dial(address string) (*grpc.ClientConn, error) {
	return grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
}

func New(conn *grpc.ClientConn) *Client {
	return &Client{
		products: product.NewProductServiceClient(conn),
		stocks:   stocks.NewStocksClient(conn),
		loms:     lomspb.NewLomsClient(conn),
	}
}

func (c *Client) GetProduct(ctx context.Context, sku string) (entity.Product, error) {
	resp, err := c.products.GetProduct(ctx, &product.GetProductRequest{Sku: sku})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return entity.Product{}, entity.ErrNotFound
		}
		return entity.Product{}, fmt.Errorf("get product: %w", err)
	}
	return entity.Product{
		SKU:    resp.GetSku(),
		Name:   resp.GetName(),
		Price:  resp.GetPrice(),
		Active: resp.GetActive(),
	}, nil
}

func (c *Client) GetStock(ctx context.Context, sku string) (uint64, error) {
	resp, err := c.stocks.GetStock(ctx, &stocks.GetStockRequest{Sku: sku})
	if err != nil {
		return 0, fmt.Errorf("get stock: %w", err)
	}
	return resp.GetCount(), nil
}

// CreateOrder не маппит ошибки: LOMS уже возвращает нужные gRPC-коды
// (FailedPrecondition на нехватку остатка, AlreadyExists на конфликт
// request_id), а gateway и cart-controller пропускают status-ошибки как есть.
func (c *Client) CreateOrder(ctx context.Context, userID, requestID string, items []entity.Item) (string, error) {
	req := &lomspb.CreateOrderRequest{
		UserId:    userID,
		RequestId: requestID,
		Items:     make([]*lomspb.ItemInput, 0, len(items)),
	}
	for _, item := range items {
		req.Items = append(req.Items, &lomspb.ItemInput{
			Sku:   item.SKU,
			Count: item.Count,
		})
	}

	resp, err := c.loms.CreateOrder(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.GetOrderId(), nil
}
