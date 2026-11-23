package order

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/controller/converter"
	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
	lomspb "github.com/igoroutine-courses/microservices.ecommerce.pkg/generated/loms/api/loms/v1"
)

type Controller struct {
	service orderService
}

var _ lomspb.LomsServer = (*Controller)(nil)

func New(service orderService) *Controller {
	return &Controller{service: service}
}

func (c *Controller) CreateOrder(ctx context.Context, req *lomspb.CreateOrderRequest) (*lomspb.CreateOrderResponse, error) {
	items := make([]entity.Item, 0, len(req.GetItems()))
	for _, it := range req.GetItems() {
		items = append(items, entity.Item{SKU: it.GetSku(), Count: it.GetCount()})
	}

	id, err := c.service.CreateOrder(ctx, req.GetUserId(), req.GetRequestId(), items)
	if err != nil {
		return nil, converter.Error(err)
	}
	return &lomspb.CreateOrderResponse{OrderId: id}, nil
}

func (c *Controller) GetOrder(ctx context.Context, req *lomspb.GetOrderRequest) (*lomspb.GetOrderResponse, error) {
	o, err := c.service.GetOrder(ctx, req.GetOrderId())
	if err != nil {
		return nil, converter.Error(err)
	}
	return &lomspb.GetOrderResponse{Order: toProto(o)}, nil
}

func (c *Controller) ListOrders(ctx context.Context, req *lomspb.ListOrdersRequest) (*lomspb.ListOrdersResponse, error) {
	page, err := c.service.ListOrders(ctx, req.GetUserId(), req.GetAfterOrderId(), req.GetLimit())
	if err != nil {
		return nil, converter.Error(err)
	}

	out := make([]*lomspb.Order, 0, len(page.Orders))
	for _, o := range page.Orders {
		out = append(out, toProto(o))
	}
	return &lomspb.ListOrdersResponse{Orders: out, NextAfterOrderId: page.NextAfterOrderID}, nil
}

func (c *Controller) PayOrder(ctx context.Context, req *lomspb.PayOrderRequest) (*emptypb.Empty, error) {
	if err := c.service.PayOrder(ctx, req.GetOrderId()); err != nil {
		return nil, converter.Error(err)
	}
	return &emptypb.Empty{}, nil
}

func (c *Controller) CancelOrder(ctx context.Context, req *lomspb.CancelOrderRequest) (*emptypb.Empty, error) {
	if err := c.service.CancelOrder(ctx, req.GetOrderId()); err != nil {
		return nil, converter.Error(err)
	}
	return &emptypb.Empty{}, nil
}

func toProto(o entity.Order) *lomspb.Order {
	items := make([]*lomspb.Item, 0, len(o.Items))
	for _, it := range o.Items {
		items = append(items, &lomspb.Item{Sku: it.SKU, Count: it.Count, Name: it.Name, Price: it.Price})
	}

	history := make([]*lomspb.Transition, 0, len(o.History))
	for _, h := range o.History {
		history = append(history, &lomspb.Transition{
			Status:     statusToProto(h.Status),
			OccurredAt: timestamppb.New(h.OccurredAt),
		})
	}

	payments := make([]*lomspb.PaymentSummary, 0, len(o.Payments))
	for _, p := range o.Payments {
		payments = append(payments, paymentToProto(p))
	}
	var payment *lomspb.PaymentSummary
	if len(payments) > 0 {
		payment = payments[len(payments)-1]
	}

	return &lomspb.Order{
		OrderId:          o.OrderID,
		UserId:           o.UserID,
		Status:           statusToProto(o.Status),
		Items:            items,
		TotalAmountMinor: uint64(o.TotalAmountMinor),
		Currency:         o.Currency,
		Payment:          payment,
		Payments:         payments,
		History:          history,
		CreatedAt:        timestamppb.New(o.CreatedAt),
		UpdatedAt:        timestamppb.New(o.UpdatedAt),
	}
}

func paymentToProto(p entity.PaymentSummary) *lomspb.PaymentSummary {
	out := &lomspb.PaymentSummary{
		PaymentId:   p.PaymentID,
		ExternalId:  p.ExternalID,
		AmountMinor: p.AmountMinor,
		Currency:    p.Currency,
		Status:      string(p.Status),
	}
	if p.Antifraud != nil {
		out.Antifraud = &lomspb.Antifraud{
			RiskScore:     p.Antifraud.RiskScore,
			ModelVersion:  p.Antifraud.ModelVersion,
			DeclineReason: p.Antifraud.DeclineReason,
		}
	}
	return out
}

func statusToProto(s entity.OrderStatus) lomspb.OrderStatus {
	if v, ok := lomspb.OrderStatus_value[string(s)]; ok {
		return lomspb.OrderStatus(v)
	}
	return lomspb.OrderStatus_ORDER_STATUS_UNSPECIFIED
}
