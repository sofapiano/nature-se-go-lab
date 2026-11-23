-- +goose Up
-- +goose StatementBegin

-- Исправляем тип chat_id
ALTER TABLE notifier.user_bindings ALTER COLUMN chat_id TYPE VARCHAR(255);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Не можем откатить изменение типа без потери данных, поэтому просто оставляем как есть
-- +goose StatementEnd