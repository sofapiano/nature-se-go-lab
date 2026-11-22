package product

import (
	"context"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
)

//go:generate mockgen -source=ports.go -destination=mocks/ports_mock.go -package=mocks

// repository — интерфейс потребителя: usecase знает, что ему нужно от
// хранилища, и не зависит от конкретной реализации.
type repository interface {
	Create(ctx context.Context, p entity.Product) error
	Get(ctx context.Context, sku string) (entity.Product, error)
	Update(ctx context.Context, p entity.Product) error
	// List возвращает до limit товаров с SKU строго больше afterSKU.
	List(ctx context.Context, afterSKU string, limit int) ([]entity.Product, error)
}
