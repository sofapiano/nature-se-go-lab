package interceptor_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/controller/interceptor"
	lomspb "github.com/igoroutine-courses/microservices.ecommerce.pkg/generated/loms/api/loms/v1"
)

func setup(t *testing.T) grpc.UnaryServerInterceptor {
	t.Helper()
	ic, err := interceptor.Validation()
	require.NoError(t, err)
	require.NotNil(t, ic)
	return ic
}

// call — прогоняет один запрос через интерцептор с handler, который либо
// возвращает sentinel, либо t.Fatal (если вызываться не должен).
func call(ic grpc.UnaryServerInterceptor, req any, handler grpc.UnaryHandler) (any, error) {
	return ic(context.Background(), req, &grpc.UnaryServerInfo{}, handler)
}

func TestValidation_ValidRequestPassesToHandler(t *testing.T) {
	ic := setup(t)

	sentinel := &emptypb.Empty{}
	got, err := call(ic, &lomspb.GetOrderRequest{OrderId: "01900000-0000-7000-8000-000000000001"},
		func(context.Context, any) (any, error) { return sentinel, nil })
	require.NoError(t, err)
	require.Equal(t, sentinel, got)
}

// TestValidation_InvalidRequest_RejectedBeforeHandler — правило protovalidate
// из proto (uuid + pattern) отвергает пустой order_id до бизнес-логики.
func TestValidation_InvalidRequest_RejectedBeforeHandler(t *testing.T) {
	ic := setup(t)

	_, err := call(ic, &lomspb.GetOrderRequest{OrderId: ""},
		func(context.Context, any) (any, error) {
			t.Fatal("handler must not be called on invalid request")
			return nil, nil
		})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

// TestValidation_NonProtoRequest_ReturnsInternal — если по какой-то причине
// пришёл не proto-месседж (например, добавили новый метод и забыли тип),
// не паникуем.
func TestValidation_NonProtoRequest_ReturnsInternal(t *testing.T) {
	ic := setup(t)

	_, err := call(ic, struct{ X int }{X: 1},
		func(context.Context, any) (any, error) { return nil, nil })
	require.Equal(t, codes.Internal, status.Code(err))
}
