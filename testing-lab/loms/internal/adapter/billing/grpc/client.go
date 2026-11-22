package grpc

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
	billingpb "github.com/igoroutine-courses/microservices.ecommerce.pkg/generated/billing/api/billing/v1"
)

// Client — тонкая обёртка над сгенерированным stub: маппинг proto → entity,
// ошибки gRPC не трогаем, их разбирает usecase по коду.
type Client struct {
	client billingpb.BillingClient
}

func Dial(address string) (*grpc.ClientConn, error) {
	return grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
}

func New(conn *grpc.ClientConn) *Client {
	return &Client{client: billingpb.NewBillingClient(conn)}
}

func (c *Client) CreatePayment(ctx context.Context, externalID, userID string, amountMinor uint64, currency string) (entity.PaymentSummary, error) {
	resp, err := c.client.CreatePayment(ctx, &billingpb.CreatePaymentRequest{
		ExternalId:  externalID,
		AmountMinor: int64(amountMinor),
		Currency:    currency,
		UserId:      userID,
	})
	if err != nil {
		return entity.PaymentSummary{}, err
	}
	return toEntity(resp), nil
}

func (c *Client) GetPayment(ctx context.Context, externalID string) (entity.PaymentSummary, error) {
	resp, err := c.client.GetPayment(ctx, &billingpb.GetPaymentRequest{ExternalId: externalID})
	if err != nil {
		return entity.PaymentSummary{}, err
	}
	return toEntity(resp), nil
}

func toEntity(p *billingpb.Payment) entity.PaymentSummary {
	var status entity.PaymentStatus
	switch p.GetStatus() {
	case billingpb.PaymentStatus_PAYMENT_STATUS_SUCCEEDED:
		status = entity.PaymentStatusSucceeded
	case billingpb.PaymentStatus_PAYMENT_STATUS_DECLINED:
		status = entity.PaymentStatusDeclined
	}
	return entity.PaymentSummary{
		PaymentID:   p.GetPaymentId(),
		ExternalID:  p.GetExternalId(),
		AmountMinor: uint64(p.GetAmountMinor()),
		Currency:    p.GetCurrency(),
		Status:      status,
		Antifraud: &entity.Antifraud{
			RiskScore:     p.GetRiskScore(),
			ModelVersion:  p.GetModelVersion(),
			DeclineReason: p.GetDeclineReason(),
		},
	}
}
