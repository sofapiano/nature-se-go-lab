package stocks

import (
	"context"
	"fmt"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
)

type (
	stocksRepository interface {
		Get(ctx context.Context, sku string) (uint64, error)
		Set(ctx context.Context, sku string, count uint64) error
		// Receive применяет поступление один раз по operation_id (см. реализацию).
		Receive(ctx context.Context, receipt entity.StockReceipt) error
	}

	productsRepository interface {
		Get(ctx context.Context, sku string) (entity.Product, error)
	}
)

type service struct {
	stocks   stocksRepository
	products productsRepository
}

func New(stocks stocksRepository, products productsRepository) *service {
	return &service{
		stocks:   stocks,
		products: products,
	}
}

// Get возвращает остаток. Для неизвестного или ещё не пополнявшегося товара он
// равен нулю, поэтому существование товара здесь не проверяем.
func (s *service) Get(ctx context.Context, sku string) (uint64, error) {
	return s.stocks.Get(ctx, entity.CanonicalSKU(sku))
}

// Set заменяет остаток существующего товара.
func (s *service) Set(ctx context.Context, sku string, count uint64) error {
	sku = entity.CanonicalSKU(sku)

	if count > entity.MaxStock {
		return fmt.Errorf("%w: count must not exceed %d", entity.ErrInvalid, entity.MaxStock)
	}

	if _, err := s.products.Get(ctx, sku); err != nil {
		return err
	}

	return s.stocks.Set(ctx, sku, count)
}

// Receive принимает поступление существующего товара. Повторный вызов с тем же
// operation_id и теми же данными безопасен.
func (s *service) Receive(ctx context.Context, receipt entity.StockReceipt) error {
	receipt.SKU = entity.CanonicalSKU(receipt.SKU)
	receipt.OperationID = entity.CanonicalID(receipt.OperationID)

	switch {
	case receipt.OperationID == "":
		return fmt.Errorf("%w: operation_id is required", entity.ErrInvalid)
	case receipt.Count < 1 || receipt.Count > entity.MaxStock:
		return fmt.Errorf("%w: count must be in [1, %d]", entity.ErrInvalid, entity.MaxStock)
	}

	if _, err := s.products.Get(ctx, receipt.SKU); err != nil {
		return err
	}

	return s.stocks.Receive(ctx, receipt)
}
