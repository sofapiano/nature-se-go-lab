package entity

import (
	"strings"
)

// MaxProductPrice — максимальная цена товара в минимальных единицах валюты.
const MaxProductPrice = 1_000_000

// Product — товар каталога. SKU всегда хранится в каноническом (верхнем) регистре.
type Product struct {
	SKU    string
	Name   string
	Price  uint32
	Active bool
}

// ProductPage — страница каталога.
type ProductPage struct {
	Products     []Product
	NextAfterSKU string
}

// CanonicalSKU приводит артикул к каноническому виду.
func CanonicalSKU(sku string) string {
	return strings.ToUpper(strings.TrimSpace(sku))
}