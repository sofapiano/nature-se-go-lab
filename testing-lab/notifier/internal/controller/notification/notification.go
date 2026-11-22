package notification

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/igoroutine-courses/microservices.ecommerce.notifier/internal/entity"
	notifierpb "github.com/igoroutine-courses/microservices.ecommerce.pkg/generated/notifier/api/v1"
)

type Controller struct {
	service notificationService
}

var _ notifierpb.NotifierServer = (*Controller)(nil)

func New(service notificationService) *Controller {
	return &Controller{service: service}
}

func (c *Controller) BindTelegram(ctx context.Context, req *notifierpb.BindTelegramRequest) (*emptypb.Empty, error) {
	if err := c.service.BindTelegram(ctx, req.GetUserId(), req.GetChatId()); err != nil {
		return nil, toGRPC(err)
	}
	return &emptypb.Empty{}, nil
}

func (c *Controller) SendOrderStatusChangedNotification(
	ctx context.Context,
	req *notifierpb.OrderStatusChangedNotificationRequest,
) (*emptypb.Empty, error) {
	if err := c.service.SendOrderStatusChanged(ctx, req.GetOrderId(), req.GetUserId(), req.GetStatus()); err != nil {
		return nil, toGRPC(err)
	}
	return &emptypb.Empty{}, nil
}

func toGRPC(err error) error {
	switch {
	case errors.Is(err, entity.ErrInvalid):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, entity.ErrUnavailable):
		return status.Error(codes.Unavailable, err.Error())
	case errors.Is(err, entity.ErrDependency):
		return status.Error(codes.Internal, "telegram dependency failed")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
