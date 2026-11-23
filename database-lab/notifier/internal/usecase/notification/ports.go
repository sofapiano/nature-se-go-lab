package notification

import (
	"context"

	"github.com/igoroutine-courses/microservices.ecommerce.notifier/internal/entity"
)

//go:generate mockgen -source=ports.go -destination=mocks/ports_mock.go -package=mocks

type (
	bindings interface {
		Get(ctx context.Context, userID string) (string, bool)
		Set(ctx context.Context, userID, chatID string) error
	}

	sender interface {
		Send(ctx context.Context, chatID, text string) error
	}

	// inboxRepository — надёжный журнал входящих событий (дедупликация по event_id).
	inboxRepository interface {
		Save(ctx context.Context, event entity.InboxEvent) error
	}
)
