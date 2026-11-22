package order

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
)

// repository хранит заказы и индексы идемпотентности. Сериализация создания
// идёт через usecase (createMx); здесь только разделяемый доступ.
type repository struct {
	mx       sync.RWMutex
	orders   map[string]entity.Order
	requests map[string]entity.OrderRequest
}

func New() *repository {
	return &repository{
		orders:   map[string]entity.Order{},
		requests: map[string]entity.OrderRequest{},
	}
}

func (r *repository) Get(_ context.Context, orderID string) (entity.Order, error) {
	r.mx.RLock()
	defer r.mx.RUnlock()

	o, ok := r.orders[orderID]
	if !ok {
		return entity.Order{}, fmt.Errorf("order %q: %w", orderID, entity.ErrNotFound)
	}
	return o, nil
}

// GetRequest возвращает сохранённый ключ идемпотентности, если он был.
func (r *repository) GetRequest(_ context.Context, requestID string) (entity.OrderRequest, bool) {
	r.mx.RLock()
	defer r.mx.RUnlock()

	req, ok := r.requests[requestID]
	return req, ok
}

// Save атомарно сохраняет заказ и ключ. Вызывается из usecase под createMx,
// поэтому конкуренции нет; RWMutex — только для защиты от параллельных чтений.
func (r *repository) Save(_ context.Context, order entity.Order, requestID string, items []entity.Item) {
	r.mx.Lock()
	defer r.mx.Unlock()

	r.orders[order.OrderID] = order
	r.requests[requestID] = entity.OrderRequest{
		RequestID: requestID,
		UserID:    order.UserID,
		Items:     items,
		OrderID:   order.OrderID,
	}
}

// Update сохраняет изменённое состояние заказа. Индекс идемпотентности не
// трогаем: он привязан к моменту создания и от переходов не зависит.
func (r *repository) Update(_ context.Context, order entity.Order) {
	r.mx.Lock()
	defer r.mx.Unlock()

	r.orders[order.OrderID] = order
}

// ListByUser возвращает заказы пользователя, отсортированные по order_id,
// строго после afterOrderID (пустой — с самого начала). UUIDv7 монотонны,
// поэтому строковое сравнение = временной порядок.
func (r *repository) ListByUser(_ context.Context, userID, afterOrderID string, limit int) ([]entity.Order, error) {
	r.mx.RLock()
	defer r.mx.RUnlock()

	result := make([]entity.Order, 0, len(r.orders))
	for _, o := range r.orders {
		if o.UserID != userID {
			continue
		}
		if afterOrderID != "" && o.OrderID <= afterOrderID {
			continue
		}
		result = append(result, o)
	}

	slices.SortFunc(result, func(a, b entity.Order) int {
		return strings.Compare(a.OrderID, b.OrderID)
	})

	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}
