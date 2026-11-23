package cart

import (
	"context"

	"github.com/igoroutine-courses/microservices.ecommerce.cart/internal/entity"
)

//go:generate mockgen -source=ports.go -destination=mocks/ports_mock.go -package=mocks

// cartService — интерфейс потребителя (контроллера). Меняется только при
// расширении набора gRPC-методов.
type cartService interface {
	AddItem(ctx context.Context, userID, sku string, count uint32) error
	SetItemQuantity(ctx context.Context, userID, sku string, count uint32) error
	DeleteItem(ctx context.Context, userID, sku string) error
	ListCart(ctx context.Context, userID string) ([]entity.Item, uint32, error)
	ClearCart(ctx context.Context, userID string) error
	CheckoutCart(ctx context.Context, userID, requestID string) (string, error)
}
