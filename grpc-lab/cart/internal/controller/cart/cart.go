package cart

import (
	"context"
	"errors"

	"buf.build/go/protovalidate"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/igoroutine-courses/microservices.ecommerce.cart/internal/entity"
	cartpb "github.com/igoroutine-courses/microservices.ecommerce.pkg/generated/cart/api/cart/v1"
)

type cartService interface {
	AddItem(ctx context.Context, userID, sku string, count uint32) error
	SetItemQuantity(ctx context.Context, userID, sku string, count uint32) error
	DeleteItem(ctx context.Context, userID, sku string) error
	ListCart(ctx context.Context, userID string) ([]entity.Item, uint32, error)
	ClearCart(ctx context.Context, userID string) error
	CheckoutCart(ctx context.Context, userID, requestID string) (string, error)
}

type Controller struct {
	service   cartService
	validator protovalidate.Validator
}

var _ cartpb.CartServer = (*Controller)(nil)

func New(service cartService, validator protovalidate.Validator) *Controller {
	return &Controller{service: service, validator: validator}
}

func (c *Controller) AddItem(ctx context.Context, req *cartpb.AddItemRequest) (*emptypb.Empty, error) {
	if err := c.check(req); err != nil {
		return nil, err
	}
	if err := c.service.AddItem(ctx, req.GetUserId(), req.GetSku(), req.GetCount()); err != nil {
		return nil, toGRPC(err)
	}
	return &emptypb.Empty{}, nil
}

func (c *Controller) SetItemQuantity(ctx context.Context, req *cartpb.SetItemQuantityRequest) (*emptypb.Empty, error) {
	if err := c.check(req); err != nil {
		return nil, err
	}
	if err := c.service.SetItemQuantity(ctx, req.GetUserId(), req.GetSku(), req.GetCount()); err != nil {
		return nil, toGRPC(err)
	}
	return &emptypb.Empty{}, nil
}

func (c *Controller) DeleteItem(ctx context.Context, req *cartpb.DeleteItemRequest) (*emptypb.Empty, error) {
	if err := c.check(req); err != nil {
		return nil, err
	}
	if err := c.service.DeleteItem(ctx, req.GetUserId(), req.GetSku()); err != nil {
		return nil, toGRPC(err)
	}
	return &emptypb.Empty{}, nil
}

func (c *Controller) ClearCart(ctx context.Context, req *cartpb.ClearCartRequest) (*emptypb.Empty, error) {
	if err := c.check(req); err != nil {
		return nil, err
	}
	if err := c.service.ClearCart(ctx, req.GetUserId()); err != nil {
		return nil, toGRPC(err)
	}
	return &emptypb.Empty{}, nil
}

func (c *Controller) ListCart(ctx context.Context, req *cartpb.ListCartRequest) (*cartpb.ListCartResponse, error) {
	if err := c.check(req); err != nil {
		return nil, err
	}
	items, total, err := c.service.ListCart(ctx, req.GetUserId())
	if err != nil {
		return nil, toGRPC(err)
	}
	out := make([]*cartpb.Item, 0, len(items))
	for _, item := range items {
		out = append(out, &cartpb.Item{
			Sku:   item.SKU,
			Count: item.Count,
			Name:  item.Name,
			Price: item.Price,
		})
	}
	return &cartpb.ListCartResponse{Items: out, TotalPrice: total}, nil
}

func (c *Controller) CheckoutCart(ctx context.Context, req *cartpb.CheckoutCartRequest) (*cartpb.CheckoutCartResponse, error) {
	if err := c.check(req); err != nil {
		return nil, err
	}
	orderID, err := c.service.CheckoutCart(ctx, req.GetUserId(), req.GetRequestId())
	if err != nil {
		return nil, toGRPC(err)
	}
	return &cartpb.CheckoutCartResponse{OrderId: orderID}, nil
}

func (c *Controller) check(req proto.Message) error {
	if err := c.validator.Validate(req); err != nil {
		return status.Error(codes.InvalidArgument, "invalid request")
	}
	return nil
}

// toGRPC пропускает уже сформированные status-ошибки (например, от LOMS)
// без повторного маппинга: коды уже правильные, искажение только навредит.
func toGRPC(err error) error {
	if _, ok := status.FromError(err); ok {
		return err
	}
	switch {
	case errors.Is(err, entity.ErrNotFound):
		return status.Error(codes.NotFound, "resource not found")
	case errors.Is(err, entity.ErrCartEmpty):
		return status.Error(codes.FailedPrecondition, "cart is empty")
	case errors.Is(err, entity.ErrProductInactive),
		errors.Is(err, entity.ErrInsufficientStock),
		errors.Is(err, entity.ErrConflict):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
