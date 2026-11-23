-- +goose Up
-- +goose StatementBegin

-- Таблица позиций корзины
CREATE TABLE IF NOT EXISTS cart.cart_items (
    user_id VARCHAR(255) NOT NULL,
    sku VARCHAR(255) NOT NULL,
    count INTEGER NOT NULL CHECK (count > 0),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, sku),
    FOREIGN KEY (user_id) REFERENCES cart.carts(user_id) ON DELETE CASCADE
);

CREATE INDEX idx_cart_items_user_id ON cart.cart_items(user_id);

-- Таблица идемпотентных checkout-записей
CREATE TABLE IF NOT EXISTS cart.checkouts (
    request_id VARCHAR(255) PRIMARY KEY,
    order_id VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_checkouts_user_id ON cart.checkouts(user_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS cart.checkouts;
DROP TABLE IF EXISTS cart.cart_items;
-- +goose StatementEnd