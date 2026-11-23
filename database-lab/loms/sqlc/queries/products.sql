-- name: CreateProduct :one
INSERT INTO loms.products (sku, name, price, active)
VALUES ($1, $2, $3, $4)
RETURNING sku, name, price, active;

-- name: GetProduct :one
SELECT sku, name, price, active
FROM loms.products
WHERE sku = $1;

-- name: UpdateProduct :one
UPDATE loms.products
SET name = $2, price = $3, active = $4
WHERE sku = $1
RETURNING sku, name, price, active;

-- name: ListProducts :many
SELECT sku, name, price, active
FROM loms.products
WHERE sku > $1 AND active = true
ORDER BY sku ASC
LIMIT $2;