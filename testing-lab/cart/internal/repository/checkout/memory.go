package checkout

import (
	"context"
	"sync"
)

// Repository хранит связку request_id → order_id: повторный CheckoutCart
// с тем же ключом должен вернуть тот же заказ, а не создать новый.
type Repository struct {
	mx       sync.RWMutex
	requests map[string]string
}

func New() *Repository {
	return &Repository{requests: map[string]string{}}
}

func (r *Repository) Get(_ context.Context, requestID string) (string, bool) {
	r.mx.RLock()
	defer r.mx.RUnlock()

	orderID, ok := r.requests[requestID]
	return orderID, ok
}

func (r *Repository) Save(_ context.Context, requestID, orderID string) {
	r.mx.Lock()
	defer r.mx.Unlock()

	r.requests[requestID] = orderID
}
