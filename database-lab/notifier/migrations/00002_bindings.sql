-- +goose Up
-- +goose StatementBegin
ALTER TABLE notifier.user_bindings ALTER COLUMN chat_id TYPE VARCHAR(255);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE notifier.user_bindings ALTER COLUMN chat_id TYPE BIGINT USING chat_id::BIGINT;
-- +goose StatementEnd