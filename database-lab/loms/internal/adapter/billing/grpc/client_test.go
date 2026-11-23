package grpc_test

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	billinggrpc "github.com/igoroutine-courses/microservices.ecommerce.loms/internal/adapter/billing/grpc"
	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
	billingpb "github.com/igoroutine-courses/microservices.ecommerce.pkg/generated/billing/api/billing/v1"
)

const bufSize = 1 << 20

type fakeBilling struct {
	billingpb.UnimplementedBillingServer

	createFn func(context.Context, *billingpb.CreatePaymentRequest) (*billingpb.Payment, error)
	getFn    func(context.Context, *billingpb.GetPaymentRequest) (*billingpb.Payment, error)
}

func (f *fakeBilling) CreatePayment(ctx context.Context, r *billingpb.CreatePaymentRequest) (*billingpb.Payment, error) {
	return f.createFn(ctx, r)
}
func (f *fakeBilling) GetPayment(ctx context.Context, r *billingpb.GetPaymentRequest) (*billingpb.Payment, error) {
	return f.getFn(ctx, r)
}

func startServer(t *testing.T, fake *fakeBilling) *googlegrpc.ClientConn {
	t.Helper()
	lis := bufconn.Listen(bufSize)
	srv := googlegrpc.NewServer()
	billingpb.RegisterBillingServer(srv, fake)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := googlegrpc.NewClient(
		"passthrough://bufnet",
		googlegrpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		googlegrpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// TestCreatePayment_MapsRequestAndResponse — проверяем что клиент корректно
// собирает запрос и правильно маппит поля и enum PaymentStatus.
func TestCreatePayment_MapsRequestAndResponse(t *testing.T) {
	var gotReq *billingpb.CreatePaymentRequest
	fake := &fakeBilling{
		createFn: func(_ context.Context, r *billingpb.CreatePaymentRequest) (*billingpb.Payment, error) {
			gotReq = r
			return &billingpb.Payment{
				PaymentId:     "pm-1",
				ExternalId:    r.GetExternalId(),
				AmountMinor:   r.GetAmountMinor(),
				Currency:      r.GetCurrency(),
				Status:        billingpb.PaymentStatus_PAYMENT_STATUS_SUCCEEDED,
				RiskScore:     0.1,
				ModelVersion:  "v1",
				DeclineReason: "",
			}, nil
		},
	}
	c := billinggrpc.New(startServer(t, fake))

	got, err := c.CreatePayment(context.Background(), "ext-1", "u-1", 500, "RUB")
	require.NoError(t, err)
	require.Equal(t, "ext-1", gotReq.GetExternalId())
	require.Equal(t, "u-1", gotReq.GetUserId())
	require.Equal(t, int64(500), gotReq.GetAmountMinor())
	require.Equal(t, "RUB", gotReq.GetCurrency())

	require.Equal(t, entity.PaymentStatusSucceeded, got.Status)
	require.Equal(t, "pm-1", got.PaymentID)
	require.Equal(t, "ext-1", got.ExternalID)
	require.NotNil(t, got.Antifraud)
	require.Equal(t, "v1", got.Antifraud.ModelVersion)
}

func TestCreatePayment_Declined(t *testing.T) {
	fake := &fakeBilling{
		createFn: func(_ context.Context, r *billingpb.CreatePaymentRequest) (*billingpb.Payment, error) {
			return &billingpb.Payment{
				PaymentId: "pm-2", ExternalId: r.GetExternalId(),
				Status:        billingpb.PaymentStatus_PAYMENT_STATUS_DECLINED,
				DeclineReason: "simulated_decline",
			}, nil
		},
	}
	c := billinggrpc.New(startServer(t, fake))

	got, err := c.CreatePayment(context.Background(), "ext", "u", 100, "RUB")
	require.NoError(t, err)
	require.Equal(t, entity.PaymentStatusDeclined, got.Status)
	require.Equal(t, "simulated_decline", got.Antifraud.DeclineReason)
}

func TestCreatePayment_PropagatesError(t *testing.T) {
	fake := &fakeBilling{
		createFn: func(context.Context, *billingpb.CreatePaymentRequest) (*billingpb.Payment, error) {
			return nil, status.Error(codes.Unavailable, "billing down")
		},
	}
	c := billinggrpc.New(startServer(t, fake))

	_, err := c.CreatePayment(context.Background(), "ext", "u", 100, "RUB")
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func TestGetPayment_Maps(t *testing.T) {
	fake := &fakeBilling{
		getFn: func(_ context.Context, r *billingpb.GetPaymentRequest) (*billingpb.Payment, error) {
			return &billingpb.Payment{
				PaymentId: "pm-3", ExternalId: r.GetExternalId(),
				Status: billingpb.PaymentStatus_PAYMENT_STATUS_SUCCEEDED,
			}, nil
		},
	}
	c := billinggrpc.New(startServer(t, fake))

	got, err := c.GetPayment(context.Background(), "ext-3")
	require.NoError(t, err)
	require.Equal(t, "pm-3", got.PaymentID)
	require.Equal(t, entity.PaymentStatusSucceeded, got.Status)
}

func TestGetPayment_NotFound(t *testing.T) {
	fake := &fakeBilling{
		getFn: func(context.Context, *billingpb.GetPaymentRequest) (*billingpb.Payment, error) {
			return nil, status.Error(codes.NotFound, "no such payment")
		},
	}
	c := billinggrpc.New(startServer(t, fake))

	_, err := c.GetPayment(context.Background(), "ext-x")
	require.Equal(t, codes.NotFound, status.Code(err))
}
