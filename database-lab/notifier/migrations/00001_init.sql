-- +goose Up
CREATE SCHEMA IF NOT EXISTS notifier;

CREATE TABLE IF NOT EXISTS notifier.user_bindings (
    user_id VARCHAR(255) PRIMARY KEY,
    chat_id BIGINT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
DROP TABLE IF EXISTS notifier.user_bindings;
DROP SCHEMA IF EXISTS notifier;