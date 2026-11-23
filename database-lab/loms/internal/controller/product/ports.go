package product

import (
	"context"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
)

//go:generate mockgen -source=ports.go -destination=mocks/ports_mock.go -package=mocks

type productService interface {
	Create(ctx context.Context, p entity.Product) (string, error)
	Get(ctx context.Context, sku string) (entity.Product, error)
	Update(ctx context.Context, p entity.Product) error
	List(ctx context.Context, afterSKU string, limit int) (entity.ProductPage, error)
}
