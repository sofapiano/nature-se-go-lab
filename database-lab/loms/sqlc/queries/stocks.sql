-- name: GetStock :one
SELECT count
FROM loms.stocks
WHERE sku = $1;

-- name: SetStock :exec
INSERT INTO loms.stocks (sku, count)
VALUES ($1, $2)
ON CONFLICT (sku) DO UPDATE SET count = EXCLUDED.count;

-- name: CreateReceipt :execrows
INSERT INTO loms.stock_receipts (operation_id, sku, count)
VALUES ($1, $2, $3)
ON CONFLICT (operation_id) DO NOTHING;

-- name: GetReceipt :one
SELECT sku, count
FROM loms.stock_receipts
WHERE operation_id = $1;

-- name: MaterializeStock :exec
INSERT INTO loms.stocks (sku, count)
VALUES ($1, 0)
ON CONFLICT (sku) DO NOTHING;

-- name: LockStock :one
SELECT count
FROM loms.stocks
WHERE sku = $1
FOR UPDATE;

-- name: UpdateStock :exec
UPDATE loms.stocks
SET count = $2
WHERE sku = $1;

-- name: ReserveStock :exec
UPDATE loms.stocks
SET count = count - $2
WHERE sku = $1 AND count >= $2;

-- name: ReleaseStock :exec
UPDATE loms.stocks
SET count = count + $2
WHERE sku = $1;
