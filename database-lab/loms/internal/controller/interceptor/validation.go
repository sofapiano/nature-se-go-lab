package interceptor

import (
	"context"
	"errors"

	"buf.build/go/protovalidate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// Validation возвращает unary-интерцептор, который до вызова обработчика проверяет
// запрос по правилам protovalidate из .proto. Нарушение правил — InvalidArgument,
// и до бизнес-логики такой запрос не доходит.
func Validation() (grpc.UnaryServerInterceptor, error) {
	validator, err := protovalidate.New()
	if err != nil {
		return nil, err
	}

	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		msg, ok := req.(proto.Message)
		if !ok {
			return nil, status.Error(codes.Internal, "internal error")
		}

		if err := validator.Validate(msg); err != nil {
			var violation *protovalidate.ValidationError
			if errors.As(err, &violation) {
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}

			// Не нарушение правил, а сбой самой валидации (например, кривое правило).
			return nil, status.Error(codes.Internal, "internal error")
		}

		return handler(ctx, req)
	}, nil
}
