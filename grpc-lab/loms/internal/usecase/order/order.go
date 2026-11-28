package order

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Границы страницы ListOrders дублируют правила валидации в loms.proto.
// Проверяются здесь защитно, чтобы usecase не зависел от того, что его
// вызвали только через gRPC-интерцептор валидации.
const (
	defaultOrderPageSize uint32 = 20
	maxOrderPageSize     uint32 = 100
)

type (
	orderRepository interface {
		Get(ctx context.Context, orderID string) (entity.Order, error)
		GetRequest(ctx context.Context, userID, requestID string) (entity.OrderRequest, bool)
		Save(ctx context.Context, order entity.Order, requestID string, items []entity.Item)
		Update(ctx context.Context, order entity.Order)
		ListByUser(ctx context.Context, userID, afterOrderID string, limit int) ([]entity.Order, error)
	}

	productRepository interface {
		Get(ctx context.Context, sku string) (entity.Product, error)
	}

	stocksRepository interface {
		Reserve(ctx context.Context, items []entity.Item) error
		Release(ctx context.Context, items []entity.Item) error
	}

	billingClient interface {
		CreatePayment(ctx context.Context, externalID, userID string, amountMinor uint64, currency string) (entity.PaymentSummary, error)
		GetPayment(ctx context.Context, externalID string) (entity.PaymentSummary, error)
	}

	notificationClient interface {
		SendOrderStatusChanged(ctx context.Context, userID, orderID, status string) error
	}
)

type service struct {
	orders   orderRepository
	products productRepository
	stocks   stocksRepository
	billing  billingClient
	notifier notificationClient

	// stateMx сериализует создание заказов и переходы состояний (cancel/pay):
	// идемпотентность (GetRequest+Save), резерв остатков и проверка статуса
	// перед Cancel/Pay должны быть под одним замком. В MVP с in-memory это ок.
	stateMx sync.Mutex
}

func New(
	orders orderRepository,
	products productRepository,
	stocks stocksRepository,
	billing billingClient,
	notifier notificationClient,
) *service {
	return &service{
		orders:   orders,
		products: products,
		stocks:   stocks,
		billing:  billing,
		notifier: notifier,
	}
}

func (s *service) CreateOrder(ctx context.Context, userID, requestID string, in []entity.Item) (string, error) {
	userID = entity.CanonicalID(userID)
	requestID = entity.CanonicalID(requestID)

	items, err := canonicalize(in)
	if err != nil {
		return "", err
	}

	s.stateMx.Lock()
	defer s.stateMx.Unlock()

	if stored, ok := s.orders.GetRequest(ctx, userID, requestID); ok {
		if !stored.Matches(userID, items) {
			return "", fmt.Errorf("%w: request_id reused with different payload", entity.ErrConflict)
		}
		return stored.OrderID, nil
	}

	total, err := s.resolveProducts(ctx, items)
	if err != nil {
		return "", err
	}
	if total > entity.MaxOrderAmount {
		return "", fmt.Errorf("%w: order total exceeds limit", entity.ErrState)
	}

	if err := s.stocks.Reserve(ctx, items); err != nil {
		return "", err
	}

	orderID := entity.NewUUIDv7()
	now := timeNow()
	order := entity.Order{
		OrderID:          orderID,
		UserID:           userID,
		Status:           entity.StatusAwaitingPayment,
		Items:            items,
		TotalAmountMinor: uint32(total),
		Currency:         entity.CurrencyRUB,
		History: []entity.Transition{
			{Status: entity.StatusAwaitingPayment, OccurredAt: now},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.orders.Save(ctx, order, requestID, items)

	s.notifyOrderStatus(ctx, userID, orderID, "awaiting_payment")

	return orderID, nil
}

func (s *service) GetOrder(ctx context.Context, orderID string) (entity.Order, error) {
	return s.orders.Get(ctx, entity.CanonicalID(orderID))
}

// ListOrders отдаёт страницу заказов пользователя по возрастанию order_id.
// limit==0 трактуется как 20, max — 100.
func (s *service) ListOrders(ctx context.Context, userID, afterOrderID string, limit uint32) (entity.OrderPage, error) {
	userID = entity.CanonicalID(userID)
	afterOrderID = entity.CanonicalID(afterOrderID)
	if limit == 0 {
		limit = defaultOrderPageSize
	}
	if limit > maxOrderPageSize {
		limit = maxOrderPageSize
	}

	orders, err := s.orders.ListByUser(ctx, userID, afterOrderID, int(limit)+1)
	if err != nil {
		return entity.OrderPage{}, err
	}

	page := entity.OrderPage{Orders: orders}
	if len(orders) > int(limit) {
		page.Orders = orders[:limit]
		page.NextAfterOrderID = orders[limit-1].OrderID
	}
	return page, nil
}

// PayOrder переводит заказ через состояния оплаты. stateMx сериализует все
// переходы: конкурентные PayOrder/CancelOrder гарантированно видят свежее
// состояние и не создают дублирующих платежей.
func (s *service) PayOrder(ctx context.Context, orderID string) error {
	orderID = entity.CanonicalID(orderID)

	s.stateMx.Lock()
	defer s.stateMx.Unlock()

	o, err := s.orders.Get(ctx, orderID)
	if err != nil {
		return err
	}

	switch o.Status {
	case entity.StatusPaid:
		return nil // повторная оплата уже оплаченного — идемпотентный no-op
	case entity.StatusPaymentPending:
		return s.resolvePayment(ctx, o)
	case entity.StatusAwaitingPayment:
		return s.startPayment(ctx, o)
	default:
		return fmt.Errorf("%w: cannot pay order in status %s", entity.ErrState, o.Status)
	}
}

// startPayment создаёт новый платёж (новый external_id) и пробует провести его.
// Перед вызовом биллинга фиксируем payment_pending: если ответ потеряется,
// следующий вызов найдёт external_id и сможет проверить результат через GetPayment.
func (s *service) startPayment(ctx context.Context, o entity.Order) error {
	externalID := entity.NewUUIDv7()
	now := timeNow()

	o.Payments = append(o.Payments, entity.PaymentSummary{
		ExternalID:  externalID,
		AmountMinor: uint64(o.TotalAmountMinor),
		Currency:    o.Currency,
		Status:      entity.PaymentStatusPending,
	})
	o.Status = entity.StatusPaymentPending
	o.History = append(o.History, entity.Transition{
		Status:     entity.StatusPaymentPending,
		OccurredAt: now,
	})
	o.UpdatedAt = now
	s.orders.Update(ctx, o)

	resp, err := s.billing.CreatePayment(ctx, externalID, o.UserID, uint64(o.TotalAmountMinor), o.Currency)
	if err != nil {
		// Биллинг не дал однозначного результата (timeout, unavailable, cancel).
		// Заказ остаётся в payment_pending, резерв сохраняется, external_id запомнен.
		return err
	}
	return s.applyPaymentResult(ctx, o, resp)
}

// resolvePayment — вторая фаза: состояние payment_pending означает, что биллинг
// мог уже провести платёж. Сначала уточняем через GetPayment; NotFound значит,
// что вызов не дошёл до сохранения (например, UNAVAILABLE), и его можно повторить
// с тем же external_id — биллинг идемпотентен по external_id.
func (s *service) resolvePayment(ctx context.Context, o entity.Order) error {
	if len(o.Payments) == 0 {
		return status.Error(codes.Internal, "payment_pending without a payment record")
	}
	externalID := o.Payments[len(o.Payments)-1].ExternalID

	resp, err := s.billing.GetPayment(ctx, externalID)
	if err != nil {
		if status.Code(err) != codes.NotFound {
			return err
		}
		resp, err = s.billing.CreatePayment(ctx, externalID, o.UserID, uint64(o.TotalAmountMinor), o.Currency)
		if err != nil {
			return err
		}
	}
	return s.applyPaymentResult(ctx, o, resp)
}

// applyPaymentResult фиксирует финальный статус платежа и переводит заказ.
// stateMx уже удерживается вызывающим — здесь только мутация и сохранение.
func (s *service) applyPaymentResult(ctx context.Context, o entity.Order, resp entity.PaymentSummary) error {
	idx := len(o.Payments) - 1
	o.Payments[idx] = resp
	now := timeNow()

	switch resp.Status {
	case entity.PaymentStatusSucceeded:
		o.Status = entity.StatusPaid
		o.History = append(o.History, entity.Transition{Status: entity.StatusPaid, OccurredAt: now})
		o.UpdatedAt = now
		s.orders.Update(ctx, o)
		s.notifyOrderStatus(ctx, o.UserID, o.OrderID, "paid")
		return nil

	case entity.PaymentStatusDeclined:
		if countDeclined(o.Payments) >= entity.MaxDeclinedAttempts {
			o.Status = entity.StatusFailed
			o.History = append(o.History, entity.Transition{Status: entity.StatusFailed, OccurredAt: now})
			o.UpdatedAt = now
			s.orders.Update(ctx, o)
			if err := s.stocks.Release(ctx, o.Items); err != nil {
				slog.ErrorContext(ctx, "release stock on failure",
					slog.String("order_id", o.OrderID), slog.Any("error", err))
			}
			s.notifyOrderStatus(ctx, o.UserID, o.OrderID, "failed")
		} else {
			o.Status = entity.StatusAwaitingPayment
			o.History = append(o.History, entity.Transition{Status: entity.StatusAwaitingPayment, OccurredAt: now})
			o.UpdatedAt = now
			s.orders.Update(ctx, o)
		}
		return fmt.Errorf("%w: payment declined", entity.ErrDeclined)

	default:
		return status.Error(codes.Internal, "unexpected payment status")
	}
}

func countDeclined(payments []entity.PaymentSummary) int {
	n := 0
	for _, p := range payments {
		if p.Status == entity.PaymentStatusDeclined {
			n++
		}
	}
	return n
}

// CancelOrder — срез 8. То же самое.
func (s *service) CancelOrder(ctx context.Context, orderID string) error {
	orderID = entity.CanonicalID(orderID)

	s.stateMx.Lock()
	defer s.stateMx.Unlock()

	o, err := s.orders.Get(ctx, orderID)
	if err != nil {
		return err
	}
	if o.Status != entity.StatusAwaitingPayment {
		return fmt.Errorf("%w: cannot cancel order in status %s", entity.ErrState, o.Status)
	}

	if err := s.stocks.Release(ctx, o.Items); err != nil {
		return err
	}

	now := timeNow()
	o.Status = entity.StatusCancelled
	o.History = append(o.History, entity.Transition{
		Status:     entity.StatusCancelled,
		OccurredAt: now,
	})
	o.UpdatedAt = now
	s.orders.Update(ctx, o)

	s.notifyOrderStatus(ctx, o.UserID, orderID, "cancelled")
	return nil
}

// resolveProducts подставляет name/price из каталога и возвращает сумму в копейках.
func (s *service) resolveProducts(ctx context.Context, items []entity.Item) (uint64, error) {
	var total uint64
	for i := range items {
		p, err := s.products.Get(ctx, items[i].SKU)
		if err != nil {
			return 0, err
		}
		if !p.Active {
			return 0, fmt.Errorf("%w: product %s is inactive", entity.ErrState, items[i].SKU)
		}
		items[i].Name = p.Name
		items[i].Price = p.Price
		total += uint64(p.Price) * uint64(items[i].Count)
	}
	return total, nil
}

// canonicalize приводит входные позиции к каноническому виду: sku upper, count > 0, без дублей,
// отсортировано по SKU.
func canonicalize(in []entity.Item) ([]entity.Item, error) {
	out := make([]entity.Item, len(in))
	seen := make(map[string]struct{}, len(in))
	for i, it := range in {
		sku := entity.CanonicalSKU(it.SKU)
		if sku == "" {
			return nil, fmt.Errorf("%w: sku is required", entity.ErrInvalidArgument)
		}
		if it.Count == 0 {
			return nil, fmt.Errorf("%w: count must be positive", entity.ErrInvalidArgument)
		}
		if _, dup := seen[sku]; dup {
			return nil, fmt.Errorf("%w: duplicate sku %s", entity.ErrInvalidArgument, sku)
		}
		seen[sku] = struct{}{}
		out[i] = entity.Item{SKU: sku, Count: it.Count}
	}

	// Канонический порядок — по SKU: состав операции не должен зависеть от
	// порядка позиций во входе (и от порядка итерации map в cart). Дубликаты
	// запрещены выше, поэтому SKU задаёт полный порядок.
	slices.SortFunc(out, func(a, b entity.Item) int {
		return strings.Compare(a.SKU, b.SKU)
	})

	return out, nil
}

// notifyOrderStatus — хук среза 4. В срезе 5 notifier — no-op; в срезе 4
// заменим адаптер на реальный, а здесь уже готово. Ошибка логируется, заказ
// не откатывается — это требование «Что должно работать».
func (s *service) notifyOrderStatus(ctx context.Context, userID, orderID, status string) {
	if err := s.notifier.SendOrderStatusChanged(ctx, userID, orderID, status); err != nil {
		slog.WarnContext(ctx, "notify failed",
			slog.String("order_id", orderID), slog.Any("error", err))
	}
}

// timeNow вынесено, чтобы в будущем подменить в тестах, если понадобится.
var timeNow = func() time.Time { return time.Now().UTC() }
