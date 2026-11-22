package notification

import "context"

//go:generate mockgen -source=ports.go -destination=mocks/ports_mock.go -package=mocks

type notificationService interface {
	BindTelegram(ctx context.Context, userID, chatID string) error
	SendOrderStatusChanged(ctx context.Context, orderID, userID, status string) error
}
