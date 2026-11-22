package cart

import (
	"context"
	"sync"

	"github.com/igoroutine-courses/microservices.ecommerce.cart/internal/entity"
)

type Repository struct {
	mu    sync.RWMutex
	carts map[string]map[string]uint32
}

func New() *Repository {
	return &Repository{carts: make(map[string]map[string]uint32)}
}

func (r *Repository) Get(_ context.Context, userID string) (entity.Cart, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	stored := r.carts[userID]
	items := make([]entity.Item, 0, len(stored))
	for sku, count := range stored {
		items = append(items, entity.Item{SKU: sku, Count: count})
	}
	return entity.Cart{UserID: userID, Items: items}, nil
}

func (r *Repository) Add(_ context.Context, userID, sku string, count uint32, available uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := r.carts[userID]
	if items == nil {
		items = make(map[string]uint32)
		r.carts[userID] = items
	}
	next := items[sku] + count
	if next > entity.MaxItemCount {
		return entity.ErrConflict
	}
	if uint64(next) > available {
		return entity.ErrInsufficientStock
	}
	items[sku] = next
	return nil
}

func (r *Repository) Set(_ context.Context, userID, sku string, count uint32, available uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := r.carts[userID]
	if items == nil {
		items = make(map[string]uint32)
		r.carts[userID] = items
	}
	if count > entity.MaxItemCount {
		return entity.ErrConflict
	}
	if uint64(count) > available {
		return entity.ErrInsufficientStock
	}
	items[sku] = count
	return nil
}

func (r *Repository) Delete(_ context.Context, userID, sku string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := r.carts[userID]
	if items == nil {
		return nil
	}
	delete(items, sku)
	if len(items) == 0 {
		delete(r.carts, userID)
	}
	return nil
}

func (r *Repository) Clear(_ context.Context, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.carts, userID)
	return nil
}
