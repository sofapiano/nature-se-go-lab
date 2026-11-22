package stocks

import (
	"context"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
)

//go:generate mockgen -source=ports.go -destination=mocks/ports_mock.go -package=mocks

type (
	stocksRepository interface {
		Get(ctx context.Context, sku string) (uint64, error)
		Set(ctx context.Context, sku string, count uint64) error
		// Receive применяет поступление один раз по operation_id.
		Receive(ctx context.Context, receipt entity.StockReceipt) error
	}

	// productsRepository нужен только для проверки существования товара.
	productsRepository interface {
		Get(ctx context.Context, sku string) (entity.Product, error)
	}
)
