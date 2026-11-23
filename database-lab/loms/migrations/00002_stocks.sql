-- +goose Up
-- +goose StatementBegin

-- Исправляем тип price в существующей таблице products
ALTER TABLE loms.products ALTER COLUMN price TYPE BIGINT;

-- Добавляем таблицу остатков
CREATE TABLE IF NOT EXISTS loms.stocks (
    sku VARCHAR(255) PRIMARY KEY,
    count BIGINT NOT NULL DEFAULT 0 CHECK (count >= 0 AND count <= 1000000000),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Добавляем таблицу поступлений (для идемпотентности)
CREATE TABLE IF NOT EXISTS loms.stock_receipts (
    operation_id VARCHAR(255) PRIMARY KEY,
    sku VARCHAR(255) NOT NULL,
    count BIGINT NOT NULL CHECK (count > 0),
    received_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_stock_receipts_sku ON loms.stock_receipts(sku);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS loms.idx_stock_receipts_sku;
DROP TABLE IF EXISTS loms.stock_receipts;
DROP TABLE IF EXISTS loms.stocks;
-- +goose StatementEnd