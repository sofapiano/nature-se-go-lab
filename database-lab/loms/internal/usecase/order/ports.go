package order

import (
	"context"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
)

//go:generate mockgen -source=ports.go -destination=mocks/ports_mock.go -package=mocks

type (
	orderRepository interface {
		Get(ctx context.Context, orderID string) (entity.Order, error)
		GetRequest(ctx context.Context, requestID string) (entity.OrderRequest, bool)
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

	outboxRepository interface {
		Save(ctx context.Context, event entity.OutboxEvent) error
	}
)
