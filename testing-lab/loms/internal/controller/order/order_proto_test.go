package order

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
	lomspb "github.com/igoroutine-courses/microservices.ecommerce.pkg/generated/loms/api/loms/v1"
)

func TestStatusToProto(t *testing.T) {
	cases := map[entity.OrderStatus]lomspb.OrderStatus{
		entity.StatusAwaitingPayment: lomspb.OrderStatus_ORDER_STATUS_AWAITING_PAYMENT,
		entity.StatusPaymentPending:  lomspb.OrderStatus_ORDER_STATUS_PAYMENT_PENDING,
		entity.StatusPaid:            lomspb.OrderStatus_ORDER_STATUS_PAID,
		entity.StatusCancelled:       lomspb.OrderStatus_ORDER_STATUS_CANCELLED,
		entity.StatusFailed:          lomspb.OrderStatus_ORDER_STATUS_FAILED,
		"UNKNOWN":                    lomspb.OrderStatus_ORDER_STATUS_UNSPECIFIED,
	}
	for in, want := range cases {
		require.Equal(t, want, statusToProto(in), "statusToProto(%q)", in)
	}
}

func TestPaymentToProto_WithoutAntifraud(t *testing.T) {
	got := paymentToProto(entity.PaymentSummary{
		PaymentID:   "pm-1",
		ExternalID:  "ext-1",
		AmountMinor: 500,
		Currency:    "RUB",
		Status:      entity.PaymentStatusPending,
	})
	require.Equal(t, "pm-1", got.GetPaymentId())
	require.Equal(t, "PAYMENT_STATUS_PENDING", got.GetStatus())
	require.Nil(t, got.GetAntifraud())
}

func TestPaymentToProto_WithAntifraud(t *testing.T) {
	got := paymentToProto(entity.PaymentSummary{
		PaymentID: "pm-1",
		Status:    entity.PaymentStatusDeclined,
		Antifraud: &entity.Antifraud{
			RiskScore:     0.95,
			ModelVersion:  "v3",
			DeclineReason: "antifraud_high_risk",
		},
	})
	require.NotNil(t, got.GetAntifraud())
	require.Equal(t, 0.95, got.GetAntifraud().GetRiskScore())
	require.Equal(t, "antifraud_high_risk", got.GetAntifraud().GetDeclineReason())
}

// TestToProto_FullOrder — проверяет ключевые свойства:
// - payment = последний платёж
// - payments = все платежи в порядке создания
// - history и timestamps сериализуются
// - items без потерь
func TestToProto_FullOrder(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	o := entity.Order{
		OrderID:          "order-1",
		UserID:           "u-1",
		Status:           entity.StatusPaid,
		TotalAmountMinor: 300,
		Currency:         "RUB",
		Items: []entity.Item{
			{SKU: "SKU-1", Count: 2, Name: "Widget", Price: 100},
			{SKU: "SKU-2", Count: 1, Name: "Gadget", Price: 100},
		},
		Payments: []entity.PaymentSummary{
			{PaymentID: "pm-1", ExternalID: "ext-1", Status: entity.PaymentStatusDeclined},
			{PaymentID: "pm-2", ExternalID: "ext-2", Status: entity.PaymentStatusSucceeded},
		},
		History: []entity.Transition{
			{Status: entity.StatusAwaitingPayment, OccurredAt: now},
			{Status: entity.StatusPaid, OccurredAt: now.Add(time.Minute)},
		},
		CreatedAt: now,
		UpdatedAt: now.Add(time.Minute),
	}

	got := toProto(o)

	require.Equal(t, "order-1", got.GetOrderId())
	require.Equal(t, "u-1", got.GetUserId())
	require.Equal(t, lomspb.OrderStatus_ORDER_STATUS_PAID, got.GetStatus())
	require.Equal(t, uint64(300), got.GetTotalAmountMinor())
	require.Equal(t, "RUB", got.GetCurrency())
	require.Len(t, got.GetItems(), 2)
	require.Equal(t, "Widget", got.GetItems()[0].GetName())
	require.Len(t, got.GetPayments(), 2)
	require.Equal(t, "pm-1", got.GetPayments()[0].GetPaymentId())
	require.Equal(t, "pm-2", got.GetPayments()[1].GetPaymentId())
	require.Equal(t, "pm-2", got.GetPayment().GetPaymentId()) // последний
	require.Len(t, got.GetHistory(), 2)
	require.False(t, got.GetCreatedAt().AsTime().IsZero())
	require.False(t, got.GetUpdatedAt().AsTime().IsZero())
}

func TestToProto_NoPayments(t *testing.T) {
	got := toProto(entity.Order{OrderID: "order-1", Status: entity.StatusAwaitingPayment})

	require.Nil(t, got.GetPayment())
	require.Empty(t, got.GetPayments())
	require.Empty(t, got.GetHistory())
}
