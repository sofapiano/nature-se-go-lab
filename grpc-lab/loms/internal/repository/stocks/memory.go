package stocks

import (
	"context"
	"fmt"
	"sync"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
)

// repository хранит остатки и принятые поступления. SKU и идентификаторы операций
// здесь уже канонические — приведение к ним делает usecase.
type repository struct {
	mx       sync.RWMutex
	balances map[string]uint64              // доступный остаток по SKU
	reserved map[string]uint64              // обязательства по возврату: сколько вернёт Release
	receipts map[string]entity.StockReceipt // принятые поступления по operation_id
}

func New() *repository {
	return &repository{
		balances: map[string]uint64{},
		reserved: map[string]uint64{},
		receipts: map[string]entity.StockReceipt{},
	}
}

// Get возвращает остаток; у товара, которому остаток ещё не задавали, он нулевой.
func (r *repository) Get(_ context.Context, sku string) (uint64, error) {
	r.mx.RLock()
	defer r.mx.RUnlock()

	return r.balances[sku], nil
}

// Set заменяет доступный остаток. Инвариант склада: available + reserved ≤
// MaxStock. Set увеличивает сумму, поэтому проверяем её вместе с обязательствами:
// иначе последующий Release выведет остаток за предел. Отказ — до изменения состояния.
func (r *repository) Set(_ context.Context, sku string, count uint64) error {
	r.mx.Lock()
	defer r.mx.Unlock()

	if count+r.reserved[sku] > entity.MaxStock {
		return fmt.Errorf("stock of %s with reservations would exceed %d: %w", sku, entity.MaxStock, entity.ErrState)
	}
	r.balances[sku] = count
	return nil
}

// Receive применяет поступление ровно один раз. Проверка ключа, проверка
// переполнения и запись идут под одной блокировкой, поэтому параллельные
// повторы одной операции не могут увеличить остаток дважды.
//
//   - Ключ уже был с теми же данными — ничего не меняем, считаем успехом.
//   - Ключ уже был с другими данными — entity.ErrConflict.
//   - Остаток превысил бы entity.MaxStock — entity.ErrState, ничего не меняем и
//     ключ не запоминаем (после исправления остатка операцию можно повторить).
func (r *repository) Receive(_ context.Context, receipt entity.StockReceipt) error {
	r.mx.Lock()
	defer r.mx.Unlock()

	if previous, done := r.receipts[receipt.OperationID]; done {
		if previous != receipt {
			return fmt.Errorf("operation %s was already used with different data: %w", receipt.OperationID, entity.ErrConflict)
		}

		return nil
	}

	total := r.balances[receipt.SKU] + receipt.Count
	if total+r.reserved[receipt.SKU] > entity.MaxStock {
		return fmt.Errorf("stock of %s would exceed %d: %w", receipt.SKU, entity.MaxStock, entity.ErrState)
	}

	r.balances[receipt.SKU] = total
	r.receipts[receipt.OperationID] = receipt

	return nil
}

// Reserve списывает count по каждому SKU. Сначала проверяются все позиции,
// и только если все проходят — применяются изменения: если хотя бы одной не
// хватает, не списывается ничего.
func (r *repository) Reserve(_ context.Context, items []entity.Item) error {
	r.mx.Lock()
	defer r.mx.Unlock()

	for _, item := range items {
		if r.balances[item.SKU] < uint64(item.Count) {
			return fmt.Errorf("insufficient stock for %s: %w", item.SKU, entity.ErrStock)
		}
	}
	for _, item := range items {
		r.balances[item.SKU] -= uint64(item.Count)
		r.reserved[item.SKU] += uint64(item.Count)
	}
	return nil
}

// Release возвращает ранее зарезервированное: перекладывает из reserved в
// available. Сумма available + reserved не меняется, поэтому предел не
// нарушается по построению — его охраняют Set/Receive при увеличении суммы.
// Идемпотентности здесь нет: usecase вызывает Release только после проверки
// статуса заказа, каждое списание компенсируется не более одного раза.
func (r *repository) Release(_ context.Context, items []entity.Item) error {
	r.mx.Lock()
	defer r.mx.Unlock()

	for _, item := range items {
		r.balances[item.SKU] += uint64(item.Count)
		r.reserved[item.SKU] -= uint64(item.Count)
	}
	return nil
}
