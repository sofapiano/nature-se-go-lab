package grpc

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	notifierpb "github.com/igoroutine-courses/microservices.ecommerce.pkg/generated/notifier/api/v1"
)

// Client — тонкая обёртка над stub. Ошибки не глотаем: usecase LOMS
// логирует их и не откатывает заказ (см. notifyOrderStatus).
type Client struct {
	client notifierpb.NotifierClient
}

func Dial(address string) (*grpc.ClientConn, error) {
	return grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
}

func New(conn *grpc.ClientConn) *Client {
	return &Client{client: notifierpb.NewNotifierClient(conn)}
}

func (c *Client) SendOrderStatusChanged(ctx context.Context, userID, orderID, status string) error {
	_, err := c.client.SendOrderStatusChangedNotification(ctx, &notifierpb.OrderStatusChangedNotificationRequest{
		OrderId: orderID,
		UserId:  userID,
		Status:  status,
	})
	if err != nil {
		return fmt.Errorf("send order status: %w", err)
	}
	return nil
}
