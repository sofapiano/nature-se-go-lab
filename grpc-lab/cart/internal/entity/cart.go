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

var (
	ErrNotFound          = errors.New("not found")
	ErrConflict          = errors.New("conflict")
	ErrInsufficientStock = errors.New("insufficient stock")
	ErrProductInactive   = errors.New("product is inactive")
	ErrCartEmpty         = errors.New("cart is empty")
)
