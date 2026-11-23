package notification_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/igoroutine-courses/microservices.ecommerce.notifier/internal/entity"
	. "github.com/igoroutine-courses/microservices.ecommerce.notifier/internal/usecase/notification"
	mocks "github.com/igoroutine-courses/microservices.ecommerce.notifier/internal/usecase/notification/mocks"
)

const (
	userID  = "01900000-0000-7000-8000-000000000001"
	orderID = "01900000-0000-7000-8000-000000000002"
)

type deps struct {
	svc      *Service
	sender   *mocks.Mocksender
	bindings *mocks.Mockbindings
}

func setup(t *testing.T) deps {
	t.Helper()
	ctrl := gomock.NewController(t)
	d := deps{
		sender:   mocks.NewMocksender(ctrl),
		bindings: mocks.NewMockbindings(ctrl),
	}
	d.svc = New(d.sender, d.bindings, mocks.NewMockinboxRepository(ctrl))
	return d
}

// ---------- BindTelegram ----------

func TestBindTelegram_StoresChatUnderCanonicalUserID(t *testing.T) {
	d := setup(t)

	// usecase нормализует user_id (bind и notify приходят в разном регистре).
	d.bindings.EXPECT().Set(gomock.Any(), "01900000-0000-7000-8000-aaaaaaaaaaaa", "1001")

	err := d.svc.BindTelegram(context.Background(), "01900000-0000-7000-8000-AAAAAAAAAAAA", "1001")
	require.NoError(t, err)
}

// TestBindTelegram_RejectsInvalidChatID — все варианты, при которых Set не
// должен вызываться (важно: некорректная привязка не затирает корректную).
func TestBindTelegram_RejectsInvalidChatID(t *testing.T) {
	cases := []struct {
		name   string
		chatID string
	}{
		{"empty", ""},
		{"zero", "0"},
		{"not a number", "abc"},
		{"overflow int64", "9223372036854775808"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := setup(t) // Set не ожидается — упадёт при вызове

			err := d.svc.BindTelegram(context.Background(), userID, tc.chatID)
			require.ErrorIs(t, err, entity.ErrInvalid)
		})
	}
}

// TestBindTelegram_AcceptsNegativeGroupChatID — группы имеют отрицательный
// chat_id, usecase должен их принимать.
func TestBindTelegram_AcceptsNegativeGroupChatID(t *testing.T) {
	d := setup(t)
	d.bindings.EXPECT().Set(gomock.Any(), userID, "-1002003")

	err := d.svc.BindTelegram(context.Background(), userID, "-1002003")
	require.NoError(t, err)
}

// ---------- SendOrderStatusChanged ----------

// TestSend_NoBindingSkipsSilently — без привязки сообщение не отправляется,
// но ошибки нет: это нормальная ситуация (см. TASK.md).
func TestSend_NoBindingSkipsSilently(t *testing.T) {
	d := setup(t)

	d.bindings.EXPECT().Get(gomock.Any(), userID).Return("", false)
	// sender.Send НЕ ожидается

	err := d.svc.SendOrderStatusChanged(context.Background(), orderID, userID, "paid")
	require.NoError(t, err)
}

// TestSend_UsesChatAndFormatsText — chat_id уходит правильный, а текст содержит
// order_id, user_id и status в ожидаемом формате (по нему интеграционные
// тесты ищут уведомления через `strings.Contains`).
func TestSend_UsesChatAndFormatsText(t *testing.T) {
	d := setup(t)

	d.bindings.EXPECT().Get(gomock.Any(), userID).Return("1001", true)
	d.sender.EXPECT().Send(gomock.Any(), "1001",
		"order_id="+orderID+" user_id="+userID+" status=paid").Return(nil)

	require.NoError(t, d.svc.SendOrderStatusChanged(context.Background(), orderID, userID, "paid"))
}

// TestSend_NormalizesUserIDForLookup — bind был под upper-case UUID, notify
// приходит под lower-case (или наоборот): ключ должен совпасть.
func TestSend_NormalizesUserIDForLookup(t *testing.T) {
	d := setup(t)

	d.bindings.EXPECT().Get(gomock.Any(), "01900000-0000-7000-8000-aaaaaaaaaaaa").Return("1001", true)
	d.sender.EXPECT().Send(gomock.Any(), "1001", gomock.Any()).Return(nil)

	err := d.svc.SendOrderStatusChanged(context.Background(),
		orderID, "01900000-0000-7000-8000-AAAAAAAAAAAA", "paid")
	require.NoError(t, err)
}

// TestSend_PropagatesSenderError — ошибка Telegram возвращается наверх,
// LOMS её залогирует (у нас fire-and-forget на уровне usecase order, но
// именно здесь ошибка должна дойти до LOMS).
func TestSend_PropagatesSenderError(t *testing.T) {
	d := setup(t)

	sentinel := errors.New("network down")
	d.bindings.EXPECT().Get(gomock.Any(), userID).Return("1001", true)
	d.sender.EXPECT().Send(gomock.Any(), gomock.Any(), gomock.Any()).Return(sentinel)

	err := d.svc.SendOrderStatusChanged(context.Background(), orderID, userID, "paid")
	require.ErrorIs(t, err, sentinel)
}

// TestSend_RebindUsesNewChat — вторая привязка заменяет первую: следующий
// notify уходит уже в новый чат.
func TestSend_RebindUsesNewChat(t *testing.T) {
	d := setup(t)

	// Первая привязка: чат 2001, отправка.
	d.bindings.EXPECT().Set(gomock.Any(), userID, "2001")
	require.NoError(t, d.svc.BindTelegram(context.Background(), userID, "2001"))

	d.bindings.EXPECT().Get(gomock.Any(), userID).Return("2001", true)
	d.sender.EXPECT().Send(gomock.Any(), "2001", gomock.Any()).Return(nil)
	require.NoError(t, d.svc.SendOrderStatusChanged(context.Background(), orderID, userID, "paid"))

	// Rebind: чат -1002003, отправка в новый.
	d.bindings.EXPECT().Set(gomock.Any(), userID, "-1002003")
	require.NoError(t, d.svc.BindTelegram(context.Background(), userID, "-1002003"))

	d.bindings.EXPECT().Get(gomock.Any(), userID).Return("-1002003", true)
	d.sender.EXPECT().Send(gomock.Any(), "-1002003", gomock.Any()).Return(nil)
	require.NoError(t, d.svc.SendOrderStatusChanged(context.Background(), orderID, userID, "paid"))
}
