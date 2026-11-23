package cart

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/igoroutine-courses/microservices.ecommerce.cart/internal/entity"
)

type Service struct {
	carts    cartRepository
	checkout checkoutRepository
	catalog  catalogClient
	loms     lomsClient

	// checkoutMx сериализует CheckoutCart: идемпотентность по request_id и
	// очистка корзины должны быть под одним замком, чтобы параллельные повторы
	// с тем же ключом не создали второй заказ и не гонялись за пустой корзиной.
	checkoutMx sync.Mutex
}

func New(carts cartRepository, checkout checkoutRepository, catalog catalogClient, loms lomsClient) *Service {
	return &Service{
		carts:    carts,
		checkout: checkout,
		catalog:  catalog,
		loms:     loms,
	}
}

func (s *Service) AddItem(ctx context.Context, userID, sku string, count uint32) error {
	sku, available, err := s.check(ctx, sku)
	if err != nil {
		return err
	}
	return s.carts.Add(ctx, strings.ToLower(userID), sku, count, available)
}

func (s *Service) SetItemQuantity(ctx context.Context, userID, sku string, count uint32) error {
	sku, available, err := s.check(ctx, sku)
	if err != nil {
		return err
	}
	return s.carts.Set(ctx, strings.ToLower(userID), sku, count, available)
}

func (s *Service) DeleteItem(ctx context.Context, userID, sku string) error {
	return s.carts.Delete(ctx, strings.ToLower(userID), strings.ToUpper(sku))
}

func (s *Service) ClearCart(ctx context.Context, userID string) error {
	return s.carts.Clear(ctx, strings.ToLower(userID))
}

func (s *Service) ListCart(ctx context.Context, userID string) ([]entity.Item, uint32, error) {
	cart, err := s.carts.Get(ctx, strings.ToLower(userID))
	if err != nil {
		return nil, 0, err
	}
	items := make([]entity.Item, 0, len(cart.Items))
	var total uint32
	for _, item := range cart.Items {
		product, err := s.catalog.GetProduct(ctx, item.SKU)
		if err != nil {
			if errors.Is(err, entity.ErrNotFound) {
				continue
			}
			return nil, 0, err
		}
		item.Name = product.Name
		item.Price = product.Price
		items = append(items, item)
		total += product.Price * item.Count
	}
	return items, total, nil
}

// CheckoutCart создаёт заказ из корзины. Идемпотентность: повтор с тем же
// request_id возвращает тот же order_id, не создавая второй заказ. Корзина
// очищается только после успешного создания; при любой ошибке LOMS она
// остаётся нетронутой.
func (s *Service) CheckoutCart(ctx context.Context, userID, requestID string) (string, error) {
	userID = strings.ToLower(userID)
	requestID = strings.ToLower(requestID)

	s.checkoutMx.Lock()
	defer s.checkoutMx.Unlock()

	if existing, ok := s.checkout.Get(ctx, requestID); ok {
		return existing, nil
	}

	cart, err := s.carts.Get(ctx, userID)
	if err != nil {
		return "", err
	}
	if len(cart.Items) == 0 {
		return "", entity.ErrCartEmpty
	}

	items := make([]entity.Item, 0, len(cart.Items))
	for _, item := range cart.Items {
		items = append(items, entity.Item{SKU: item.SKU, Count: item.Count})
	}

	orderID, err := s.loms.CreateOrder(ctx, userID, requestID, items)
	if err != nil {
		return "", err
	}

	s.checkout.Save(ctx, requestID, orderID)

	if err := s.carts.Clear(ctx, userID); err != nil {
		return "", err
	}
	return orderID, nil
}

func (s *Service) check(ctx context.Context, sku string) (string, uint64, error) {
	sku = strings.ToUpper(sku)
	product, err := s.catalog.GetProduct(ctx, sku)
	if err != nil {
		return "", 0, err
	}
	if !product.Active {
		return "", 0, entity.ErrProductInactive
	}
	available, err := s.loms.GetStock(ctx, sku)
	if err != nil {
		return "", 0, err
	}
	return sku, available, nil
}
