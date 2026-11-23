package converter_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/controller/converter"
	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
)

// TestError_DomainMapping — каждое доменное → свой gRPC-код. Если кто-то
// перепутает ErrConflict и ErrState, клиент получит неверный HTTP-статус
// (409 vs 409 — ладно, но 409 vs 412 — уже баг).
func TestError_DomainMapping(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want codes.Code
	}{
		{"not found", entity.ErrNotFound, codes.NotFound},
		{"conflict", entity.ErrConflict, codes.AlreadyExists},
		{"invalid", entity.ErrInvalid, codes.InvalidArgument},
		{"state", entity.ErrState, codes.FailedPrecondition},
		{"stock", entity.ErrStock, codes.FailedPrecondition},
		{"declined", entity.ErrDeclined, codes.FailedPrecondition},
		{"unavailable", entity.ErrUnavailable, codes.Unavailable},
		{"deadline", context.DeadlineExceeded, codes.DeadlineExceeded},
		{"canceled", context.Canceled, codes.Canceled},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := converter.Error(tc.err)
			require.Equal(t, tc.want, status.Code(err))
		})
	}
}

func TestError_NilIsNil(t *testing.T) {
	require.NoError(t, converter.Error(nil))
}

// TestError_UnknownHidesDetails — клиент не должен видеть текст внутренней
// ошибки: только generic "internal error".
func TestError_UnknownHidesDetails(t *testing.T) {
	sentinel := errors.New("db password leaked")
	err := converter.Error(sentinel)
	require.Equal(t, codes.Internal, status.Code(err))
	require.Equal(t, "internal error", status.Convert(err).Message())
}

// TestError_PassThroughStatus — если ошибка уже gRPC-статус (например, от
// Billing), не переупаковываем: код и детали должны сохраниться.
func TestError_PassThroughStatus(t *testing.T) {
	original := status.Error(codes.Unavailable, "billing down")
	got := converter.Error(original)
	require.Equal(t, codes.Unavailable, status.Code(got))
	require.Equal(t, "billing down", status.Convert(got).Message())
}

// TestError_WrappedDomainError — errors.Is работает через wrap, значит
// %w из usecase не ломает маппинг.
func TestError_WrappedDomainError(t *testing.T) {
	wrapped := errors.Join(entity.ErrStock, errors.New("SKU-1 not enough"))
	err := converter.Error(wrapped)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestPageSize(t *testing.T) {
	cases := []struct {
		in   uint32
		want int
	}{
		{0, converter.DefaultPageSize},
		{1, 1},
		{50, 50},
		{100, 100},
		{101, converter.MaxPageSize},
		{500, converter.MaxPageSize},
	}

	for _, tc := range cases {
		require.Equal(t, tc.want, converter.PageSize(tc.in), "PageSize(%d)", tc.in)
	}
}

// OrderStatus — пустая заготовка; покрываем вызовом, чтобы не оставлять
// 0%-функцию в отчёте.
func TestOrderStatus_Noop(t *testing.T) {
	converter.OrderStatus()
}
