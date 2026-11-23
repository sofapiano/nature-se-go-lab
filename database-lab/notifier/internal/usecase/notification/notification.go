package notification

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/igoroutine-courses/microservices.ecommerce.notifier/internal/entity"
)

type Service struct {
	sender   sender
	bindings bindings
}

func New(sender sender, bindings bindings) *Service {
	return &Service{sender: sender, bindings: bindings}
}

// BindTelegram валидирует chat_id и сохраняет привязку. Валидация формата
// (regex) уже сделана на границе gRPC через protovalidate; здесь — бизнес-
// инварианты: значение ненулевое и влезает в int64 (Bot API принимает int64).
func (s *Service) BindTelegram(ctx context.Context, userID, chatID string) error {
	value, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil || value == 0 {
		return fmt.Errorf("%w: chat_id must be a non-zero int64", entity.ErrInvalid)
	}
	return s.bindings.Set(ctx, strings.ToLower(userID), chatID)
}

// SendOrderStatusChanged — fire-and-forget. Без привязки сообщение не
// отправляется, но и ошибки нет: это нормальный сценарий, а не сбой.
// Ошибку Telegram возвращаем наверх — LOMS её залогирует и не откатит заказ.
func (s *Service) SendOrderStatusChanged(ctx context.Context, orderID, userID, status string) error {
	chatID, ok := s.bindings.Get(ctx, strings.ToLower(userID))
	if !ok {
		return nil
	}
	text := fmt.Sprintf("order_id=%s user_id=%s status=%s", orderID, userID, status)
	return s.sender.Send(ctx, chatID, text)
}
