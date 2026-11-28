package checkout

import (
	"context"
	"sync"

	"github.com/igoroutine-courses/microservices.ecommerce.cart/internal/entity"
)

// key — составной ключ идемпотентности. Структура лучше склейки строк:
// нет риска коллизии разделителей, сравнение делает сам Go.
type key struct {
	userID    string
	requestID string
}

// Repository хранит операции checkout: завершённые (OrderID != "") и
// незавершённые (OrderID == ""). Pending-запись создаётся ДО вызова LOMS
// и переживает потерю ответа.
type Repository struct {
	mx       sync.RWMutex
	requests map[key]entity.Checkout
}

func New() *Repository {
	return &Repository{requests: map[key]entity.Checkout{}}
}

func (r *Repository) Get(_ context.Context, userID, requestID string) (entity.Checkout, bool) {
	r.mx.RLock()
	defer r.mx.RUnlock()

	ch, ok := r.requests[key{userID, requestID}]
	return ch, ok
}

// GetPending ищет незавершённую операцию пользователя. Она может быть только
// одна: новая не начинается, пока прежняя не уточнена. Линейный поиск по map
// допустим для in-memory MVP; инвариант «не больше одной» поддерживает usecase.
func (r *Repository) GetPending(_ context.Context, userID string) (entity.Checkout, bool) {
	r.mx.RLock()
	defer r.mx.RUnlock()

	for k, ch := range r.requests {
		if k.userID == userID && ch.OrderID == "" {
			return ch, true
		}
	}
	return entity.Checkout{}, false
}

func (r *Repository) Save(_ context.Context, ch entity.Checkout) {
	r.mx.Lock()
	defer r.mx.Unlock()

	r.requests[key{ch.UserID, ch.RequestID}] = ch
}

// Delete забывает попытку, о которой достоверно известно, что заказ не создан.
func (r *Repository) Delete(_ context.Context, userID, requestID string) {
	r.mx.Lock()
	defer r.mx.Unlock()

	delete(r.requests, key{userID, requestID})
}
