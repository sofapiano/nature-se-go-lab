package cart

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/igoroutine-courses/microservices.ecommerce.cart/internal/entity"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type (
	cartRepository interface {
		Get(ctx context.Context, userID string) (entity.Cart, error)
		Add(ctx context.Context, userID, sku string, count uint32, available uint64) error
		Set(ctx context.Context, userID, sku string, count uint32, available uint64) error
		Delete(ctx context.Context, userID, sku string) error
		Clear(ctx context.Context, userID string) error
	}

	checkoutRepository interface {
		Get(ctx context.Context, userID, requestID string) (entity.Checkout, bool)
		GetPending(ctx context.Context, userID string) (entity.Checkout, bool)
		Save(ctx context.Context, checkout entity.Checkout)
		Delete(ctx context.Context, userID, requestID string)
	}

	catalogClient interface {
		GetProduct(ctx context.Context, sku string) (entity.Product, error)
	}

	lomsClient interface {
		GetStock(ctx context.Context, sku string) (uint64, error)
		CreateOrder(ctx context.Context, userID, requestID string, items []entity.Item) (string, error)
	}
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

func (s *Service) ListCart(ctx context.Context, userID string) ([]entity.Item, uint32, error) {
	cart, err := s.carts.Get(ctx, strings.ToLower(userID))
	if err != nil {
		return nil, 0, err
	}
	items := make([]entity.Item, 0, len(cart.Items))
	// Сумму копим в uint64: 5 позиций × 1000 шт × 1_000_000 = 5·10^9 > MaxUint32.
	// Каждое произведение тоже считаем в uint64 — защитно, как resolveProducts в LOMS.
	var total uint64
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
		total += uint64(product.Price) * uint64(item.Count)
	}
	// Контракт total_price — uint32. Не влезает — отказываем, а не заворачиваем по модулю.
	if total > math.MaxUint32 {
		return nil, 0, fmt.Errorf("%w: cart total exceeds uint32 at current prices", entity.ErrTotalOverflow)
	}
	return items, uint32(total), nil
}

// CheckoutCart оформляет корзину. Идемпотентность — по паре (user_id, request_id).
// Намерение (pending + состав) фиксируется ДО вызова LOMS: потерянный ответ не
// должен приводить ко второму заказу. Пока результат прежней операции не уточнён,
// новая операция с другим ключом не начинается. Распределённая транзакция не
// нужна: уточнение делает идемпотентный CreateOrder у LOMS.
func (s *Service) CheckoutCart(ctx context.Context, userID, requestID string) (string, error) {
	userID = strings.ToLower(userID)
	requestID = strings.ToLower(requestID)

	s.checkoutMx.Lock()
	defer s.checkoutMx.Unlock()

	if existing, ok := s.checkout.Get(ctx, userID, requestID); ok {
		if existing.OrderID != "" {
			return existing.OrderID, nil // завершённая операция: идемпотентный повтор
		}
		// Незавершённая: повторяем ТО ЖЕ САМОЕ (сохранённый состав, тот же ключ).
		return s.finishCheckout(ctx, existing)
	}

	if _, ok := s.checkout.GetPending(ctx, userID); ok {
		return "", entity.ErrCheckoutPending
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

	attempt := entity.Checkout{UserID: userID, RequestID: requestID, Items: items}
	s.checkout.Save(ctx, attempt) // идентичность операции — ДО RPC

	return s.finishCheckout(ctx, attempt)
}

// finishCheckout выполняет (или повторяет) RPC и финализирует запись.
// Вызывается под checkoutMx.
func (s *Service) finishCheckout(ctx context.Context, attempt entity.Checkout) (string, error) {
	orderID, err := s.loms.CreateOrder(ctx, attempt.UserID, attempt.RequestID, attempt.Items)
	if err != nil {
		if definitelyNotCreated(err) {
			// LOMS доказал, что заказа нет — попытку можно забыть.
			s.checkout.Delete(ctx, attempt.UserID, attempt.RequestID)
		}
		// Неоднозначная ошибка: pending остаётся, уточнит повтор с тем же ключом.
		return "", err
	}

	attempt.OrderID = orderID
	s.checkout.Save(ctx, attempt)

	if err := s.carts.Clear(ctx, attempt.UserID); err != nil {
		return "", err
	}
	return orderID, nil
}

// definitelyNotCreated: эти коды LOMS возвращает только ДО сохранения заказа,
// значит заказ гарантированно не создан. Остальные (Unavailable, DeadlineExceeded,
// Unknown, Internal, ...) могли замаскировать успех. Классификация по gRPC-коду
// в usecase — уже принятый в курсе паттерн: см. resolvePayment в LOMS.
func definitelyNotCreated(err error) bool {
	switch status.Code(err) {
	case codes.InvalidArgument, codes.NotFound, codes.FailedPrecondition, codes.AlreadyExists:
		return true
	default:
		return false
	}
}

// ClearCart запрещён, пока результат checkout неизвестен: под тем же замком,
// что и CheckoutCart, иначе проверка рассинхронизируется с самим оформлением.
func (s *Service) ClearCart(ctx context.Context, userID string) error {
	userID = strings.ToLower(userID)

	s.checkoutMx.Lock()
	defer s.checkoutMx.Unlock()

	if _, ok := s.checkout.GetPending(ctx, userID); ok {
		return entity.ErrCheckoutPending
	}
	return s.carts.Clear(ctx, userID)
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
