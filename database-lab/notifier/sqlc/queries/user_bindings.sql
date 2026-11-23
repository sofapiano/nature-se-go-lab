-- name: GetBinding :one
SELECT chat_id
FROM notifier.user_bindings
WHERE user_id = $1;

-- name: SetBinding :exec
INSERT INTO notifier.user_bindings (user_id, chat_id)
VALUES ($1, $2)
ON CONFLICT (user_id) DO UPDATE SET chat_id = EXCLUDED.chat_id;

-- name: UpsertBinding :one
INSERT INTO notifier.user_bindings (user_id, chat_id)
VALUES ($1, $2)
ON CONFLICT (user_id)
DO UPDATE SET chat_id = EXCLUDED.chat_id
RETURNING user_id, chat_id, created_at;
