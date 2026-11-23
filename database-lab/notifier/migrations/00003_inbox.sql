-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS notifier.inbox (
    event_id    VARCHAR(255) PRIMARY KEY,
    order_id    VARCHAR(255) NOT NULL,
    user_id     VARCHAR(255) NOT NULL,
    status      VARCHAR(50)  NOT NULL,
    state       VARCHAR(20)  NOT NULL DEFAULT 'pending',
    lease_until TIMESTAMP WITH TIME ZONE,
    attempts    INTEGER      NOT NULL DEFAULT 0,
    last_error  TEXT,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_inbox_state_lease ON notifier.inbox(state, lease_until);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS notifier.inbox;
-- +goose StatementEnd