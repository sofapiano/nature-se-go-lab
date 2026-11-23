package interceptor_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/igoroutine-courses/microservices.ecommerce.notifier/internal/controller/interceptor"
	notifierpb "github.com/igoroutine-courses/microservices.ecommerce.pkg/generated/notifier/api/v1"
)

func setup(t *testing.T) grpc.UnaryServerInterceptor {
	t.Helper()
	ic, err := interceptor.Validation()
	require.NoError(t, err)
	require.NotNil(t, ic)
	return ic
}

func call(ic grpc.UnaryServerInterceptor, req any, handler grpc.UnaryHandler) (any, error) {
	return ic(context.Background(), req, &grpc.UnaryServerInfo{}, handler)
}

func TestValidation_ValidRequestPassesToHandler(t *testing.T) {
	ic := setup(t)

	sentinel := &emptypb.Empty{}
	got, err := call(ic, &notifierpb.BindTelegramRequest{
		UserId: "01900000-0000-7000-8000-000000000001",
		ChatId: "1001",
	}, func(context.Context, any) (any, error) { return sentinel, nil })
	require.NoError(t, err)
	require.Equal(t, sentinel, got)
}

func TestValidation_InvalidRequest_RejectedBeforeHandler(t *testing.T) {
	ic := setup(t)

	// chat_id="0" проходит regex, но uuid невалиден — правило отвергнет запрос.
	_, err := call(ic, &notifierpb.BindTelegramRequest{
		UserId: "not-a-uuid",
		ChatId: "1001",
	}, func(context.Context, any) (any, error) {
		t.Fatal("handler must not be called on invalid request")
		return nil, nil
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestValidation_NonProtoRequest_ReturnsInternal(t *testing.T) {
	ic := setup(t)

	_, err := call(ic, struct{ X int }{X: 1},
		func(context.Context, any) (any, error) { return nil, nil })
	require.Equal(t, codes.Internal, status.Code(err))
}