package entity

import "errors"

const MaxItemCount uint32 = 1000

type Item struct {
	SKU   string
	Count uint32
	Name  string
	Price uint32
}

type Cart struct {
	UserID string
	Items  []Item
}

type Product struct {
	SKU    string
	Name   string
	Price  uint32
	Active bool
}

// Checkout — операция оформления корзины. Ключ — пара (UserID, RequestID):
// request_id принадлежит пользователю, разные пользователи могут использовать
// одинаковые ключи. OrderID == "" означает pending: намерение записано до RPC,
// результат ещё не уточнён. Items — зафиксированный состав операции: повтор
// отправляет именно его, а не текущую корзину.
type Checkout struct {
	UserID    string
	RequestID string
	Items     []Item
	OrderID   string
}

var (
	ErrNotFound          = errors.New("not found")
	ErrConflict          = errors.New("conflict")
	ErrInsufficientStock = errors.New("insufficient stock")
	ErrProductInactive   = errors.New("product is inactive")
	ErrCartEmpty         = errors.New("cart is empty")
	ErrTotalOverflow     = errors.New("cart total overflow")
	ErrCheckoutPending   = errors.New("checkout result is unknown")
)
