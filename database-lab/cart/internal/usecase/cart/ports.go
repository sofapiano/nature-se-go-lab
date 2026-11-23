package cart

import (
	"context"

	"github.com/igoroutine-courses/microservices.ecommerce.cart/internal/entity"
)

//go:generate mockgen -source=ports.go -destination=mocks/ports_mock.go -package=mocks

// cartRepository — хранилище корзин. Под одним Lock делает «проверить
// available+existing» и запись — это даёт атомарность на уровне репозитория.
type cartRepository interface {
	Get(ctx context.Context, userID string) (entity.Cart, error)
	Add(ctx context.Context, userID, sku string, count uint32, available uint64) error
	Set(ctx context.Context, userID, sku string, count uint32, available uint64) error
	Delete(ctx context.Context, userID, sku string) error
	Clear(ctx context.Context, userID string) error
}

// checkoutRepository — идемпотентность checkout: request_id → order_id.
type checkoutRepository interface {
	Get(ctx context.Context, requestID string) (string, bool)
	Save(ctx context.Context, requestID, orderID string) error
}

// catalogClient — источник name/price/active для позиций корзины.
type catalogClient interface {
	GetProduct(ctx context.Context, sku string) (entity.Product, error)
}

// lomsClient — остатки и создание заказа в LOMS.
type lomsClient interface {
	GetStock(ctx context.Context, sku string) (uint64, error)
	CreateOrder(ctx context.Context, userID, requestID string, items []entity.Item) (string, error)
}
