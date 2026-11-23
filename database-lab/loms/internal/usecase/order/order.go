package order

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/db"
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

type Service struct {
	orders     orderRepository
	products   productRepository
	stocks     stocksRepository
	billing    billingClient
	notifier   notificationClient
	transactor db.Transactor
	outbox     outboxRepository
}

func New(
	orders orderRepository,
	products productRepository,
	stocks stocksRepository,
	billing billingClient,
	notifier notificationClient,
	transactor db.Transactor,
	outbox outboxRepository,
) *Service {
	return &Service{
		orders:     orders,
		products:   products,
		stocks:     stocks,
		billing:    billing,
		notifier:   notifier,
		transactor: transactor,
		outbox:     outbox,
	}
}

func (s *Service) CreateOrder(ctx context.Context, userID, requestID string, in []entity.Item) (string, error) {
	userID = entity.CanonicalID(userID)
	requestID = entity.CanonicalID(requestID)

	items, err := canonicalize(in)
	if err != nil {
		return "", err
	}

	var orderID string
	err = s.transactor.WithTx(ctx, func(txCtx context.Context) error {
		// 1. Идемпотентность
		if stored, ok := s.orders.GetRequest(txCtx, requestID); ok {
			if !stored.Matches(userID, items) {
				return fmt.Errorf("%w: request_id reused with different payload", entity.ErrConflict)
			}
			orderID = stored.OrderID
			return nil
		}

		// 2. Проверка товаров и расчёт суммы
		var total uint64
		total, err = s.resolveProducts(txCtx, items)
		if err != nil {
			return err
		}
		if total > entity.MaxOrderAmount {
			return fmt.Errorf("%w: order total exceeds limit", entity.ErrState)
		}

		// 3. Резерв остатков (В ТОЙ ЖЕ ТРАНЗАКЦИИ!)
		if err = s.stocks.Reserve(txCtx, items); err != nil {
			return err
		}

		// 4. Сохранение заказа и истории
		newOrderID := entity.NewUUIDv7()
		now := timeNow()
		order := entity.Order{
			OrderID:          newOrderID,
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
		s.orders.Save(txCtx, order, requestID, items)

		outboxEvent := entity.OutboxEvent{
			EventID:   entity.NewUUIDv7(),
			EventType: "order_status_changed", //nolint:goconst // event type literal is clear in this context
			OrderID:   newOrderID,
			UserID:    userID,
			Status:    "awaiting_payment",
			Payload:   fmt.Sprintf(`{"order_id": "%s", "status": "awaiting_payment"}`, newOrderID),
		}
		if err = s.outbox.Save(txCtx, outboxEvent); err != nil {
			return fmt.Errorf("save outbox event: %w", err)
		}

		orderID = newOrderID
		return nil
	})

	if err != nil {
		return "", err
	}

	return orderID, nil
}

func (s *Service) GetOrder(ctx context.Context, orderID string) (entity.Order, error) {
	return s.orders.Get(ctx, entity.CanonicalID(orderID))
}

// ListOrders отдаёт страницу заказов пользователя по возрастанию order_id.
// limit==0 трактуется как 20, max — 100.
func (s *Service) ListOrders(ctx context.Context, userID, afterOrderID string, limit uint32) (entity.OrderPage, error) {
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

// PayOrder переводит заказ через состояния оплаты
func (s *Service) PayOrder(ctx context.Context, orderID string) error {
	orderID = entity.CanonicalID(orderID)

	var externalID string
	var isNewPayment bool
	var o entity.Order

	// Фаза 1: Короткая транзакция для фиксации намерения или получения состояния
	err := s.transactor.WithTx(ctx, func(txCtx context.Context) error {
		var err error
		o, err = s.orders.Get(txCtx, orderID)
		if err != nil {
			return err
		}

		switch o.Status {
		case entity.StatusPaid:
			return nil // Уже оплачен, идемпотентность
		case entity.StatusPaymentPending:
			if len(o.Payments) == 0 {
				return status.Error(codes.Internal, "payment_pending without a payment record")
			}
			externalID = o.Payments[len(o.Payments)-1].ExternalID
			isNewPayment = false
			return nil
		case entity.StatusAwaitingPayment:
			externalID = entity.NewUUIDv7()
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
			s.orders.Update(txCtx, o)
			isNewPayment = true
			return nil
		default:
			return fmt.Errorf("%w: cannot pay order in status %s", entity.ErrState, o.Status)
		}
	})
	if err != nil {
		return err
	}

	if o.Status == entity.StatusPaid {
		return nil
	}

	// Фаза 2: Сетевой вызов ВНЕ транзакции
	var resp entity.PaymentSummary
	if isNewPayment {
		resp, err = s.billing.CreatePayment(ctx, externalID, o.UserID, uint64(o.TotalAmountMinor), o.Currency)
	} else {
		resp, err = s.billing.GetPayment(ctx, externalID)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				resp, err = s.billing.CreatePayment(ctx, externalID, o.UserID, uint64(o.TotalAmountMinor), o.Currency)
			}
		}
	}
	if err != nil {
		return err // Заказ остаётся в payment_pending, следующий вызов попробует снова
	}

	// Фаза 3: Новая транзакция для сохранения результата
	return s.transactor.WithTx(ctx, func(txCtx context.Context) error {
		currentOrder, err := s.orders.Get(txCtx, orderID)
		if err != nil {
			return err
		}
		return s.applyPaymentResult(txCtx, currentOrder, resp)
	})
}

// applyPaymentResult фиксирует финальный статус платежа и переводит заказ.
func (s *Service) applyPaymentResult(ctx context.Context, o entity.Order, resp entity.PaymentSummary) error {
	var found bool
	for i := range o.Payments {
		if o.Payments[i].ExternalID == resp.ExternalID {
			o.Payments[i] = resp
			found = true
			break
		}
	}
	if !found {
		o.Payments = append(o.Payments, resp)
	}

	now := timeNow()

	switch resp.Status {
	case entity.PaymentStatusSucceeded:
		o.Status = entity.StatusPaid
		o.History = append(o.History, entity.Transition{Status: entity.StatusPaid, OccurredAt: now})
		o.UpdatedAt = now
		s.orders.Update(ctx, o)

		outboxEvent := entity.OutboxEvent{
			EventID:   entity.NewUUIDv7(),
			EventType: "order_status_changed",
			OrderID:   o.OrderID,
			UserID:    o.UserID,
			Status:    string(o.Status),
			Payload:   fmt.Sprintf(`{"order_id": "%s", "status": "%s", "payment_status": "%s"}`, o.OrderID, o.Status, resp.Status),
		}
		if err := s.outbox.Save(ctx, outboxEvent); err != nil {
			return fmt.Errorf("save outbox event: %w", err)
		}

		return nil

	case entity.PaymentStatusDeclined:
		if countDeclined(o.Payments) >= entity.MaxDeclinedAttempts {
			o.Status = entity.StatusFailed
			o.History = append(o.History, entity.Transition{Status: entity.StatusFailed, OccurredAt: now})
			o.UpdatedAt = now
			s.orders.Update(ctx, o)
			if err := s.stocks.Release(ctx, o.Items); err != nil {
				slog.ErrorContext(ctx, "release stock on failure", slog.String("order_id", o.OrderID), slog.Any("error", err))
			}
			outboxEvent := entity.OutboxEvent{
				EventID:   entity.NewUUIDv7(),
				EventType: "order_status_changed",
				OrderID:   o.OrderID,
				UserID:    o.UserID,
				Status:    string(o.Status),
				Payload:   fmt.Sprintf(`{"order_id": "%s", "status": "%s", "payment_status": "%s"}`, o.OrderID, o.Status, resp.Status),
			}
			if err := s.outbox.Save(ctx, outboxEvent); err != nil {
				return fmt.Errorf("save outbox event: %w", err)
			}
		} else {
			o.Status = entity.StatusAwaitingPayment
			o.History = append(o.History, entity.Transition{Status: entity.StatusAwaitingPayment, OccurredAt: now})
			o.UpdatedAt = now
			s.orders.Update(ctx, o)

			outboxEvent := entity.OutboxEvent{
				EventID:   entity.NewUUIDv7(),
				EventType: "order_status_changed",
				OrderID:   o.OrderID,
				UserID:    o.UserID,
				Status:    string(o.Status),
				Payload:   fmt.Sprintf(`{"order_id": "%s", "status": "%s", "payment_status": "%s"}`, o.OrderID, o.Status, resp.Status),
			}
			if err := s.outbox.Save(ctx, outboxEvent); err != nil {
				return fmt.Errorf("save outbox event: %w", err)
			}
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

// CancelOrder, то же самое.
func (s *Service) CancelOrder(ctx context.Context, orderID string) error {
	orderID = entity.CanonicalID(orderID)

	return s.transactor.WithTx(ctx, func(txCtx context.Context) error {
		o, err := s.orders.Get(txCtx, orderID)
		if err != nil {
			return err
		}
		if o.Status != entity.StatusAwaitingPayment {
			return fmt.Errorf("%w: cannot cancel order in status %s", entity.ErrState, o.Status)
		}

		if err := s.stocks.Release(txCtx, o.Items); err != nil {
			return err
		}

		now := timeNow()
		o.Status = entity.StatusCancelled
		o.History = append(o.History, entity.Transition{
			Status:     entity.StatusCancelled,
			OccurredAt: now,
		})
		o.UpdatedAt = now
		s.orders.Update(txCtx, o)

		outboxEvent := entity.OutboxEvent{
			EventID:   entity.NewUUIDv7(),
			EventType: "order_status_changed",
			OrderID:   o.OrderID,
			UserID:    o.UserID,
			Status:    "cancelled",
			Payload:   fmt.Sprintf(`{"order_id": "%s", "status": "cancelled"}`, o.OrderID),
		}
		if err := s.outbox.Save(txCtx, outboxEvent); err != nil {
			return fmt.Errorf("save outbox event: %w", err)
		}

		return nil
	})
}

// resolveProducts подставляет name/price из каталога и возвращает сумму в копейках.
func (s *Service) resolveProducts(ctx context.Context, items []entity.Item) (uint64, error) {
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

// canonicalize приводит входные позиции к виду «sku upper + count», запрещает
// дубли SKU (после нормализации) и нулевые count.
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
	return out, nil
}

// timeNow вынесено, чтобы в будущем подменить в тестах, если понадобится.
var timeNow = func() time.Time { return time.Now().UTC() }
