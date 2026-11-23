package notification_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	. "github.com/igoroutine-courses/microservices.ecommerce.notifier/internal/controller/notification"
	mocks "github.com/igoroutine-courses/microservices.ecommerce.notifier/internal/controller/notification/mocks"
	"github.com/igoroutine-courses/microservices.ecommerce.notifier/internal/entity"
	notifierpb "github.com/igoroutine-courses/microservices.ecommerce.pkg/generated/notifier/api/v1"
)

const (
	userID  = "01900000-0000-7000-8000-000000000001"
	orderID = "01900000-0000-7000-8000-000000000002"
)

func setup(t *testing.T) (*Controller, *mocks.MocknotificationService) {
	t.Helper()
	ctrl := gomock.NewController(t)
	svc := mocks.NewMocknotificationService(ctrl)
	return New(svc), svc
}

func TestBindTelegram_Success(t *testing.T) {
	c, svc := setup(t)
	svc.EXPECT().BindTelegram(gomock.Any(), userID, "1001").Return(nil)

	_, err := c.BindTelegram(context.Background(), &notifierpb.BindTelegramRequest{
		UserId: userID, ChatId: "1001",
	})
	require.NoError(t, err)
}

func TestBindTelegram_ErrorMapping(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want codes.Code
	}{
		{"invalid", entity.ErrInvalid, codes.InvalidArgument},
		{"unavailable", entity.ErrUnavailable, codes.Unavailable},
		{"dependency", entity.ErrDependency, codes.Internal},
		{"unknown", errors.New("boom"), codes.Internal},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, svc := setup(t)
			svc.EXPECT().BindTelegram(gomock.Any(), gomock.Any(), gomock.Any()).Return(tc.err)

			_, err := c.BindTelegram(context.Background(), &notifierpb.BindTelegramRequest{
				UserId: userID, ChatId: "1001",
			})
			require.Equal(t, tc.want, status.Code(err))
		})
	}
}

func TestSendNotification_Success(t *testing.T) {
	c, svc := setup(t)
	svc.EXPECT().AcceptOrderStatusChanged(gomock.Any(), gomock.Any(), orderID, userID, "paid").Return(nil)

	_, err := c.SendOrderStatusChangedNotification(context.Background(),
		&notifierpb.OrderStatusChangedNotificationRequest{
			OrderId: orderID, UserId: userID, Status: "paid",
		})
	require.NoError(t, err)
}

func TestSendNotification_UnavailableMapsToUnavailable(t *testing.T) {
	c, svc := setup(t)
	svc.EXPECT().AcceptOrderStatusChanged(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entity.ErrUnavailable)

	_, err := c.SendOrderStatusChangedNotification(context.Background(),
		&notifierpb.OrderStatusChangedNotificationRequest{
			OrderId: orderID, UserId: userID, Status: "paid",
		})
	require.Equal(t, codes.Unavailable, status.Code(err))
}