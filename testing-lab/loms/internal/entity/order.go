package entity

import (
	"errors"
	"time"
)

var (
	// Базовые ошибки (ожидаются контроллерами, конвертерами и тестами)
	ErrNotFound        = errors.New("not found")
	ErrAlreadyExists   = errors.New("already exists")
	ErrInvalidArgument = errors.New("invalid argument")
	ErrInternal        = errors.New("internal error")
	ErrConflict        = errors.New("idempotency conflict")

	// Специфичные ошибки бизнес-логики
	ErrDuplicateSKU = errors.New("SKU already exists")
	ErrState        = errors.New("operation not allowed")
	ErrStock        = errors.New("insufficient stock")
	ErrDeclined     = errors.New("payment declined")
	ErrUnavailable  = errors.New("dependency unavailable")
	ErrDependency   = errors.New("dependency failed")

	// Алиасы для совместимости
	ErrMissing = ErrNotFound
	ErrInvalid = ErrInvalidArgument
)

// OrderStatus повторяет имена значений enum из loms.proto: их же ожидает API
// в поле status, поэтому контроллер просто подставляет строку в proto-enum.
type OrderStatus string

const (
	StatusAwaitingPayment OrderStatus = "ORDER_STATUS_AWAITING_PAYMENT"
	StatusPaymentPending  OrderStatus = "ORDER_STATUS_PAYMENT_PENDING"
	StatusPaid            OrderStatus = "ORDER_STATUS_PAID"
	StatusCancelled       OrderStatus = "ORDER_STATUS_CANCELLED"
	StatusFailed          OrderStatus = "ORDER_STATUS_FAILED"
)

const (
	CurrencyRUB = "RUB"
	// MaxOrderAmount — потолок суммы заказа в копейках. Тесты ждут ошибку при
	// overflow: 5 SKU × 1000 шт × 1_000_000 = 5·10^9 > MaxUint32.
	MaxOrderAmount uint64 = 1<<32 - 1
)

// Item — позиция заказа. Name/Price фиксируются в момент оформления, чтобы
// изменения каталога не влияли на уже созданный заказ.
type Item struct {
	SKU   string
	Count uint32
	Name  string
	Price uint32
}

type Transition struct {
	Status     OrderStatus
	OccurredAt time.Time
}

type Order struct {
	OrderID          string
	UserID           string
	Status           OrderStatus
	Items            []Item
	TotalAmountMinor uint32
	Currency         string
	Payments         []PaymentSummary
	History          []Transition
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// OrderRequest хранит ключ идемпотентности и канонический состав:
// повтор с тем же ключом и тем же составом возвращает тот же заказ,
// повтор с другим составом — ошибку.
type OrderRequest struct {
	RequestID string
	UserID    string
	Items     []Item
	OrderID   string
}

// Matches сравнивает только «входные» поля (SKU+Count); Name/Price — снапшот,
// они могут отличаться от исходного запроса и не влияют на идемпотентность.
func (r OrderRequest) Matches(userID string, items []Item) bool {
	if r.UserID != userID || len(r.Items) != len(items) {
		return false
	}
	for i := range items {
		if r.Items[i].SKU != items[i].SKU || r.Items[i].Count != items[i].Count {
			return false
		}
	}
	return true
}

type OrderPage struct {
	Orders           []Order
	NextAfterOrderID string
}

type PaymentStatus string

const (
	// PaymentStatusPending — внутренний статус LOMS: результат оплаты неизвестен.
	// Биллинг такого статуса не возвращает.
	PaymentStatusPending   PaymentStatus = "PAYMENT_STATUS_PENDING"
	PaymentStatusSucceeded PaymentStatus = "PAYMENT_STATUS_SUCCEEDED"
	PaymentStatusDeclined  PaymentStatus = "PAYMENT_STATUS_DECLINED"
)

// MaxDeclinedAttempts — сколько подтверждённых отказов закрывают заказ.
const MaxDeclinedAttempts = 3

type Antifraud struct {
	RiskScore     float64
	ModelVersion  string
	DeclineReason string
}

// PaymentSummary — одна попытка оплаты. Status=PaymentStatusPending до ответа
// биллинга; PaymentID и Antifraud появляются только после ответа.
type PaymentSummary struct {
	PaymentID   string
	ExternalID  string
	AmountMinor uint64
	Currency    string
	Status      PaymentStatus
	Antifraud   *Antifraud
}
