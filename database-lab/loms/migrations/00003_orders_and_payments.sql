-- +goose Up
-- +goose StatementBegin

-- Таблица заказов
CREATE TABLE IF NOT EXISTS loms.orders (
    order_id VARCHAR(255) PRIMARY KEY,
    request_id VARCHAR(255) UNIQUE, -- Ключ идемпотентности
    user_id VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'ORDER_STATUS_AWAITING_PAYMENT',
    total_amount_minor BIGINT NOT NULL,
    currency VARCHAR(10) NOT NULL DEFAULT 'RUB',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_orders_user_id ON loms.orders(user_id);
CREATE INDEX idx_orders_status ON loms.orders(status);

-- Таблица позиций заказа (snapshot на момент создания)
CREATE TABLE IF NOT EXISTS loms.order_items (
    order_id VARCHAR(255) NOT NULL,
    sku VARCHAR(255) NOT NULL,
    count INTEGER NOT NULL CHECK (count > 0),
    name VARCHAR(512) NOT NULL,
    price_minor BIGINT NOT NULL CHECK (price_minor >= 0),
    PRIMARY KEY (order_id, sku),
    FOREIGN KEY (order_id) REFERENCES loms.orders(order_id) ON DELETE CASCADE
);

CREATE INDEX idx_order_items_order_id ON loms.order_items(order_id);

-- Таблица истории статусов заказа
CREATE TABLE IF NOT EXISTS loms.order_status_history (
    id BIGSERIAL PRIMARY KEY,
    order_id VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL,
    occurred_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (order_id) REFERENCES loms.orders(order_id) ON DELETE CASCADE
);

CREATE INDEX idx_order_status_history_order_id ON loms.order_status_history(order_id);

-- Таблица платежей
CREATE TABLE IF NOT EXISTS loms.payments (
    payment_id VARCHAR(255) PRIMARY KEY,
    order_id VARCHAR(255) NOT NULL,
    external_id VARCHAR(255) NOT NULL UNIQUE,
    amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
    currency VARCHAR(10) NOT NULL DEFAULT 'RUB',
    status VARCHAR(50) NOT NULL DEFAULT 'PAYMENT_STATUS_PENDING',
    antifraud_risk_score DOUBLE PRECISION,
    antifraud_model_version VARCHAR(100),
    antifraud_decline_reason TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (order_id) REFERENCES loms.orders(order_id) ON DELETE CASCADE
);

CREATE INDEX idx_payments_order_id ON loms.payments(order_id);
CREATE INDEX idx_payments_external_id ON loms.payments(external_id);

-- Таблица outbox
CREATE TABLE IF NOT EXISTS loms.outbox (
    event_id VARCHAR(255) PRIMARY KEY,
    event_type VARCHAR(100) NOT NULL,
    order_id VARCHAR(255) NOT NULL,
    user_id VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL,
    payload JSONB NOT NULL,
    status_sent VARCHAR(20) NOT NULL DEFAULT 'pending',
    lease_until TIMESTAMP WITH TIME ZONE,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_outbox_status_sent ON loms.outbox(status_sent);
CREATE INDEX idx_outbox_lease_until ON loms.outbox(lease_until);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS loms.outbox;
DROP TABLE IF EXISTS loms.payments;
DROP TABLE IF EXISTS loms.order_status_history;
DROP TABLE IF EXISTS loms.order_items;
DROP TABLE IF EXISTS loms.orders;
-- +goose StatementEnd