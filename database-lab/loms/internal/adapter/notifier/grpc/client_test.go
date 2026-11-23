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
	"google.golang.org/protobuf/types/known/emptypb"

	notifiergrpc "github.com/igoroutine-courses/microservices.ecommerce.loms/internal/adapter/notifier/grpc"
	notifierpb "github.com/igoroutine-courses/microservices.ecommerce.pkg/generated/notifier/api/v1"
)

const bufSize = 1 << 20

type fakeNotifier struct {
	notifierpb.UnimplementedNotifierServer
	sendFn func(context.Context, *notifierpb.OrderStatusChangedNotificationRequest) (*emptypb.Empty, error)
}

func (f *fakeNotifier) SendOrderStatusChangedNotification(
	ctx context.Context, r *notifierpb.OrderStatusChangedNotificationRequest,
) (*emptypb.Empty, error) {
	return f.sendFn(ctx, r)
}

func startServer(t *testing.T, fake *fakeNotifier) *googlegrpc.ClientConn {
	t.Helper()
	lis := bufconn.Listen(bufSize)
	srv := googlegrpc.NewServer()
	notifierpb.RegisterNotifierServer(srv, fake)
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

// TestSendOrderStatusChanged_PassesFields — проверить, что user_id, order_id
// и status уходят в notifier как есть (без случайных преобразований).
func TestSendOrderStatusChanged_PassesFields(t *testing.T) {
	var captured *notifierpb.OrderStatusChangedNotificationRequest
	fake := &fakeNotifier{
		sendFn: func(_ context.Context, r *notifierpb.OrderStatusChangedNotificationRequest) (*emptypb.Empty, error) {
			captured = r
			return &emptypb.Empty{}, nil
		},
	}
	c := notifiergrpc.New(startServer(t, fake))

	require.NoError(t, c.SendOrderStatusChanged(context.Background(), "u-1", "o-1", "paid"))
	require.Equal(t, "u-1", captured.GetUserId())
	require.Equal(t, "o-1", captured.GetOrderId())
	require.Equal(t, "paid", captured.GetStatus())
}

func TestSendOrderStatusChanged_PropagatesError(t *testing.T) {
	fake := &fakeNotifier{
		sendFn: func(context.Context, *notifierpb.OrderStatusChangedNotificationRequest) (*emptypb.Empty, error) {
			return nil, status.Error(codes.Internal, "boom")
		},
	}
	c := notifiergrpc.New(startServer(t, fake))

	err := c.SendOrderStatusChanged(context.Background(), "u", "o", "paid")
	require.Error(t, err)
	require.Equal(t, codes.Internal, status.Code(err))
}
