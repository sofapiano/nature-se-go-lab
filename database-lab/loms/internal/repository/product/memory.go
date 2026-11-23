package product

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
)

// repository — каталог в памяти.
//
// Кроме map по SKU держим отдельный отсортированный срез ключей: так страница
// каталога достаётся бинарным поиском курсора, без сортировки при каждом запросе.
// SKU здесь всегда канонические — приведение к ним делает usecase.
type repository struct {
	mx       sync.RWMutex
	products map[string]entity.Product
	skus     []string // отсортирован по возрастанию
}

func New() *repository {
	return &repository{
		products: map[string]entity.Product{},
	}
}

// Create добавляет товар. Проверка «уже есть» и вставка идут под одной блокировкой,
// поэтому из параллельных вызовов с одним SKU успешным будет ровно один.
func (r *repository) Create(_ context.Context, p entity.Product) error {
	r.mx.Lock()
	defer r.mx.Unlock()

	if _, exists := r.products[p.SKU]; exists {
		return fmt.Errorf("product %q already exists: %w", p.SKU, entity.ErrConflict)
	}

	r.products[p.SKU] = p
	i, _ := slices.BinarySearch(r.skus, p.SKU)
	r.skus = slices.Insert(r.skus, i, p.SKU)

	return nil
}

func (r *repository) Get(_ context.Context, sku string) (entity.Product, error) {
	r.mx.RLock()
	defer r.mx.RUnlock()

	p, ok := r.products[sku]
	if !ok {
		return entity.Product{}, notFound(sku)
	}

	return p, nil
}

// Update заменяет данные существующего товара.
func (r *repository) Update(_ context.Context, p entity.Product) error {
	r.mx.Lock()
	defer r.mx.Unlock()

	if _, ok := r.products[p.SKU]; !ok {
		return notFound(p.SKU)
	}

	r.products[p.SKU] = p

	return nil
}

// List возвращает до limit товаров с SKU строго больше afterSKU, по возрастанию.
// Пустой afterSKU означает «с самого начала».
func (r *repository) List(_ context.Context, afterSKU string, limit int) ([]entity.Product, error) {
	r.mx.RLock()
	defer r.mx.RUnlock()

	start := 0
	if afterSKU != "" {
		i, found := slices.BinarySearch(r.skus, afterSKU)
		start = i
		if found {
			start++ // сам курсор в страницу не входит
		}
	}

	end := min(start+max(limit, 0), len(r.skus))

	result := make([]entity.Product, 0, end-start)
	for _, sku := range r.skus[start:end] {
		result = append(result, r.products[sku])
	}

	return result, nil
}

func notFound(sku string) error {
	return fmt.Errorf("product %q: %w", sku, entity.ErrNotFound)
}
