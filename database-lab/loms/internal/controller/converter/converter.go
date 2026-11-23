package converter

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
)

const (
	// DefaultPageSize — размер страницы, если клиент не передал limit.
	DefaultPageSize = 50
	// MaxPageSize — верхняя граница; те же 100 зашиты в правилах валидации proto.
	MaxPageSize = 100
)

// Error переводит доменную ошибку в gRPC-статус. Это единственное место, где
// доменные ошибки узнают про коды транспорта.
//
// Ошибки, которые мы не ожидали, наружу не светим: клиент получает общее
// "internal error" без подробностей.
func Error(err error) error {
	if err == nil {
		return nil
	}

	// Уже gRPC-статус (например, ответ другого сервиса) — отдаём как есть.
	if _, ok := status.FromError(err); ok {
		return err
	}

	var code codes.Code

	switch {
	case errors.Is(err, entity.ErrNotFound):
		code = codes.NotFound
	case errors.Is(err, entity.ErrConflict):
		code = codes.AlreadyExists
	case errors.Is(err, entity.ErrInvalid):
		code = codes.InvalidArgument
	case errors.Is(err, entity.ErrState), errors.Is(err, entity.ErrStock), errors.Is(err, entity.ErrDeclined):
		code = codes.FailedPrecondition
	case errors.Is(err, entity.ErrUnavailable):
		code = codes.Unavailable
	case errors.Is(err, context.DeadlineExceeded):
		code = codes.DeadlineExceeded
	case errors.Is(err, context.Canceled):
		code = codes.Canceled
	default:
		// ErrDeclined и ErrDependency сюда попадут, пока их маппинг не добавлен
		// в срезе с оплатой.
		return status.Error(codes.Internal, "internal error")
	}

	return status.Error(code, err.Error())
}

// PageSize превращает limit из запроса в размер страницы: 0 означает
// «не указано» и заменяется значением по умолчанию.
func PageSize(limit uint32) int {
	if limit == 0 {
		return DefaultPageSize
	}

	return int(min(limit, MaxPageSize))
}

// OrderStatus — заготовка под срез «заказы».
func OrderStatus() {}
