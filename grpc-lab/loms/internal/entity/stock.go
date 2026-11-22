package entity

// MaxStock — максимальный остаток одного товара. То же число зашито в правилах
// валидации stocks.proto: если меняешь здесь, поменяй и там.
const MaxStock = 1_000_000_000

// StockReceipt — поступление товара на склад. OperationID — ключ идемпотентности:
// одно и то же поступление можно присылать повторно, и оно применится один раз.
type StockReceipt struct {
	OperationID string
	SKU         string
	Count       uint64
}
