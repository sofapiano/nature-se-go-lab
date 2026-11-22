package product

import (
	"context"
	"fmt"
	"strings"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
)

type Service struct {
	repo repository
}

func New(repo repository) *Service {
	return &Service{repo: repo}
}

// Create создаёт активный товар и возвращает его SKU в каноническом виде.
func (s *Service) Create(ctx context.Context, p entity.Product) (string, error) {
	p.SKU = entity.CanonicalSKU(p.SKU)
	p.Active = true

	if err := validate(p); err != nil {
		return "", err
	}

	if err := s.repo.Create(ctx, p); err != nil {
		return "", err
	}

	return p.SKU, nil
}

func (s *Service) Get(ctx context.Context, sku string) (entity.Product, error) {
	return s.repo.Get(ctx, entity.CanonicalSKU(sku))
}

// Update целиком заменяет название, цену и признак активности товара.
func (s *Service) Update(ctx context.Context, p entity.Product) error {
	p.SKU = entity.CanonicalSKU(p.SKU)

	if err := validate(p); err != nil {
		return err
	}

	return s.repo.Update(ctx, p)
}

// List отдаёт страницу каталога. limit должен быть положительным: значение по
// умолчанию подставляет слой controller.
func (s *Service) List(ctx context.Context, afterSKU string, limit int) (entity.ProductPage, error) {
	if limit < 1 {
		return entity.ProductPage{}, fmt.Errorf("%w: limit must be positive", entity.ErrInvalid)
	}

	// Просим на один товар больше, чем нужно: если он нашёлся, значит, есть
	// следующая страница, и курсором для неё станет последний товар этой.
	items, err := s.repo.List(ctx, entity.CanonicalSKU(afterSKU), limit+1)
	if err != nil {
		return entity.ProductPage{}, err
	}

	page := entity.ProductPage{Products: items}
	if len(items) > limit {
		page.Products = items[:limit]
		page.NextAfterSKU = items[limit-1].SKU
	}

	return page, nil
}

// validate — бизнес-инварианты. Формат входных данных проверяет protovalidate на
// границе gRPC, но usecase не должен полагаться на то, что его вызвали только оттуда.
func validate(p entity.Product) error {
	switch {
	case p.SKU == "":
		return fmt.Errorf("%w: sku is required", entity.ErrInvalid)
	case strings.TrimSpace(p.Name) == "":
		return fmt.Errorf("%w: name is required", entity.ErrInvalid)
	case p.Price < 1 || p.Price > entity.MaxProductPrice:
		return fmt.Errorf("%w: price must be in [1, %d]", entity.ErrInvalid, entity.MaxProductPrice)
	}

	return nil
}
