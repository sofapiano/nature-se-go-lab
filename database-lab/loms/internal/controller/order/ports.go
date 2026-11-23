package order

import (
	"context"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
)

//go:generate mockgen -source=ports.go -destination=mocks/ports_mock.go -package=mocks

type orderService interface {
	CreateOrder(ctx context.Context, userID, requestID string, items []entity.Item) (string, error)
	GetOrder(ctx context.Context, orderID string) (entity.Order, error)
	ListOrders(ctx context.Context, userID, afterOrderID string, limit uint32) (entity.OrderPage, error)
	PayOrder(ctx context.Context, orderID string) error
	CancelOrder(ctx context.Context, orderID string) error
}
