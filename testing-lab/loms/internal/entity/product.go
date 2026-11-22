package entity

import (
	"strings"
)

// MaxProductPrice — максимальная цена товара в минимальных единицах валюты.
// То же число зашито в правилах валидации product.proto: если меняем здесь,
// меняем и там.
const MaxProductPrice = 1_000_000

// Product — товар каталога. SKU всегда хранится в каноническом (верхнем) регистре,
// потому что артикулы нечувствительны к регистру: "test-1" и "TEST-1" — один товар.
//
// Price — цена в минимальных единицах валюты (копейках). Дробных чисел в деньгах
// не используем, чтобы не терять точность.
type Product struct {
	SKU    string
	Name   string
	Price  uint32
	Active bool
}

// ProductPage — страница каталога. NextAfterSKU пустой, если страница последняя;
// иначе его нужно передать как after_sku, чтобы получить следующую страницу.
type ProductPage struct {
	Products     []Product
	NextAfterSKU string
}

// CanonicalSKU приводит артикул к каноническому виду.
//
// Регистр не важен, важны сами символы: поэтому мы обрезаем пробелы по краям и
// поднимаем регистр. Так "  test-1 " и "TEST-1" дадут одно и то же значение.
func CanonicalSKU(sku string) string {
	return strings.ToUpper(strings.TrimSpace(sku))
}
