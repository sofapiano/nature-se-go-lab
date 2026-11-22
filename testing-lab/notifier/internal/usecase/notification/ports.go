package notification

import "context"

//go:generate mockgen -source=ports.go -destination=mocks/ports_mock.go -package=mocks

type (
	bindings interface {
		Get(ctx context.Context, userID string) (string, bool)
		Set(ctx context.Context, userID, chatID string)
	}

	sender interface {
		Send(ctx context.Context, chatID, text string) error
	}
)
