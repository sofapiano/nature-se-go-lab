package notification

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/igoroutine-courses/microservices.ecommerce.notifier/internal/entity"
)

type (
	bindings interface {
		Get(ctx context.Context, userID string) (string, bool)
		Set(ctx context.Context, userID, chatID string)
	}

	sender interface {
		Send(ctx context.Context, chatID, text string) error
	}
)

type service struct {
	sender   sender
	bindings bindings
}

func New(sender sender, bindings bindings) *service {
	return &service{sender: sender, bindings: bindings}
}

// BindTelegram валидирует chat_id и сохраняет привязку. Валидация формата
// (regex) уже сделана на границе gRPC; здесь — бизнес-инварианты: значение
// ненулевое и влезает в int64 (Bot API принимает именно int64).
func (s *service) BindTelegram(ctx context.Context, userID, chatID string) error {
	value, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil || value == 0 {
		return fmt.Errorf("%w: chat_id must be a non-zero int64", entity.ErrInvalid)
	}
	s.bindings.Set(ctx, strings.ToLower(userID), chatID)
	return nil
}

// SendOrderStatusChanged — fire-and-forget. Без привязки сообщение не
// отправляется, но и ошибки нет: это нормальный сценарий, а не сбой.
// Ошибку Telegram возвращаем наверх — LOMS её залогирует и не откатит заказ.
func (s *service) SendOrderStatusChanged(ctx context.Context, orderID, userID, status string) error {
	chatID, ok := s.bindings.Get(ctx, strings.ToLower(userID))
	if !ok {
		return nil
	}
	text := fmt.Sprintf("order_id=%s user_id=%s status=%s", orderID, userID, status)
	return s.sender.Send(ctx, chatID, text)
}

// CanonicalUserID — единственное место, где userID приводится к нижнему
// регистру: bind и notify приходят из разных мест, но должны сходиться.
func CanonicalUserID(userID string) string {
	return strings.ToLower(userID)
}
