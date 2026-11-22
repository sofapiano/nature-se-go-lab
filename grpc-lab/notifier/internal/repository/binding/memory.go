package binding

import (
	"context"
	"sync"
)

type Repository struct {
	mx    sync.RWMutex
	chats map[string]string
}

func New() *Repository {
	return &Repository{chats: map[string]string{}}
}

// Get возвращает chat_id и признак существования. Отсутствие привязки —
// не ошибка: notifier в этом случае просто ничего не отправляет.
func (r *Repository) Get(_ context.Context, userID string) (string, bool) {
	r.mx.RLock()
	defer r.mx.RUnlock()

	chat, ok := r.chats[userID]
	return chat, ok
}

// Set перезаписывает привязку: повторный BindTelegram заменяет прежний чат.
func (r *Repository) Set(_ context.Context, userID, chatID string) {
	r.mx.Lock()
	defer r.mx.Unlock()

	r.chats[userID] = chatID
}
