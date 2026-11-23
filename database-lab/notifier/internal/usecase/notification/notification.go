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
	inbox    inboxRepository
}

func New(sender sender, bindings bindings, inbox inboxRepository) *Service {
	return &Service{sender: sender, bindings: bindings, inbox: inbox}
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

// AcceptOrderStatusChanged — входная точка gRPC: сначала надёжно регистрируем
// событие в inbox, и только потом отвечаем «принято». Повтор с тем же event_id
// в любом состоянии (pending/in_progress/processed) не создаёт вторую задачу.
func (s *Service) AcceptOrderStatusChanged(ctx context.Context, eventID, orderID, userID, status string) error {
	if eventID == "" {
		// Legacy-вызов без ключа дедупликации считаем новым событием.
		eventID = entity.NewUUIDv7()
	}

	event := entity.InboxEvent{
		EventID: strings.ToLower(eventID),
		OrderID: orderID,
		UserID:  strings.ToLower(userID),
		Status:  status,
	}
	if err := s.inbox.Save(ctx, event); err != nil {
		return fmt.Errorf("accept inbox event: %w", err)
	}
	return nil
}

// SendOrderStatusChanged — логика ДОСТАВКИ, которую вызывает inbox-воркер.
// Без привязки сообщение не отправляется, но и ошибки нет: это нормальный
// сценарий, а не сбой. Ошибку Telegram возвращаем наверх — воркер запланирует retry.
func (s *Service) SendOrderStatusChanged(ctx context.Context, orderID, userID, status string) error {
	chatID, ok := s.bindings.Get(ctx, strings.ToLower(userID))
	if !ok {
		return nil
	}
	text := fmt.Sprintf("order_id=%s user_id=%s status=%s", orderID, userID, status)
	return s.sender.Send(ctx, chatID, text)
}
