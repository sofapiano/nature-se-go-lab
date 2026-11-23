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
	inbox    *mocks.MockinboxRepository
}

func setup(t *testing.T) deps {
	t.Helper()
	ctrl := gomock.NewController(t)
	d := deps{
		sender:   mocks.NewMocksender(ctrl),
		bindings: mocks.NewMockbindings(ctrl),
		inbox:    mocks.NewMockinboxRepository(ctrl),
	}
	d.svc = New(d.sender, d.bindings, d.inbox)
	return d
}

// ---------- BindTelegram ----------

func TestBindTelegram_StoresChatUnderCanonicalUserID(t *testing.T) {
	d := setup(t)
	d.bindings.EXPECT().Set(gomock.Any(), "01900000-0000-7000-8000-aaaaaaaaaaaa", "1001")

	err := d.svc.BindTelegram(context.Background(), "01900000-0000-7000-8000-AAAAAAAAAAAA", "1001")
	require.NoError(t, err)
}

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
			d := setup(t)
			err := d.svc.BindTelegram(context.Background(), userID, tc.chatID)
			require.ErrorIs(t, err, entity.ErrInvalid)
		})
	}
}

func TestBindTelegram_AcceptsNegativeGroupChatID(t *testing.T) {
	d := setup(t)
	d.bindings.EXPECT().Set(gomock.Any(), userID, "-1002003")

	err := d.svc.BindTelegram(context.Background(), userID, "-1002003")
	require.NoError(t, err)
}

// ---------- AcceptOrderStatusChanged (inbox intake) ----------

func TestAccept_PassesEventIDAndCanonicalUser(t *testing.T) {
	d := setup(t)
	d.inbox.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, e entity.InboxEvent) error {
			require.Equal(t, "01900000-0000-7000-8000-0000000000aa", e.EventID)
			require.Equal(t, userID, e.UserID)
			require.Equal(t, orderID, e.OrderID)
			require.Equal(t, "paid", e.Status)
			return nil
		})

	err := d.svc.AcceptOrderStatusChanged(context.Background(),
		"01900000-0000-7000-8000-0000000000AA", orderID, userID, "paid")
	require.NoError(t, err)
}

func TestAccept_GeneratesIDWhenEmpty(t *testing.T) {
	d := setup(t)
	d.inbox.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, e entity.InboxEvent) error {
			require.NotEmpty(t, e.EventID)
			return nil
		})

	require.NoError(t, d.svc.AcceptOrderStatusChanged(context.Background(), "", orderID, userID, "paid"))
}

func TestAccept_PropagatesStorageError(t *testing.T) {
	d := setup(t)
	sentinel := errors.New("db down")
	d.inbox.EXPECT().Save(gomock.Any(), gomock.Any()).Return(sentinel)

	err := d.svc.AcceptOrderStatusChanged(context.Background(), "id", orderID, userID, "paid")
	require.ErrorIs(t, err, sentinel)
}

// ---------- SendOrderStatusChanged (delivery, используется воркером) ----------

func TestSend_NoBindingSkipsSilently(t *testing.T) {
	d := setup(t)
	d.bindings.EXPECT().Get(gomock.Any(), userID).Return("", false)

	err := d.svc.SendOrderStatusChanged(context.Background(), orderID, userID, "paid")
	require.NoError(t, err)
}

func TestSend_UsesChatAndFormatsText(t *testing.T) {
	d := setup(t)
	d.bindings.EXPECT().Get(gomock.Any(), userID).Return("1001", true)
	d.sender.EXPECT().Send(gomock.Any(), "1001",
		"order_id="+orderID+" user_id="+userID+" status=paid").Return(nil)

	require.NoError(t, d.svc.SendOrderStatusChanged(context.Background(), orderID, userID, "paid"))
}

func TestSend_NormalizesUserIDForLookup(t *testing.T) {
	d := setup(t)
	d.bindings.EXPECT().Get(gomock.Any(), "01900000-0000-7000-8000-aaaaaaaaaaaa").Return("1001", true)
	d.sender.EXPECT().Send(gomock.Any(), "1001", gomock.Any()).Return(nil)

	err := d.svc.SendOrderStatusChanged(context.Background(),
		orderID, "01900000-0000-7000-8000-AAAAAAAAAAAA", "paid")
	require.NoError(t, err)
}

func TestSend_PropagatesSenderError(t *testing.T) {
	d := setup(t)
	sentinel := errors.New("network down")
	d.bindings.EXPECT().Get(gomock.Any(), userID).Return("1001", true)
	d.sender.EXPECT().Send(gomock.Any(), gomock.Any(), gomock.Any()).Return(sentinel)

	err := d.svc.SendOrderStatusChanged(context.Background(), orderID, userID, "paid")
	require.ErrorIs(t, err, sentinel)
}

func TestSend_RebindUsesNewChat(t *testing.T) {
	d := setup(t)

	d.bindings.EXPECT().Set(gomock.Any(), userID, "2001")
	require.NoError(t, d.svc.BindTelegram(context.Background(), userID, "2001"))

	d.bindings.EXPECT().Get(gomock.Any(), userID).Return("2001", true)
	d.sender.EXPECT().Send(gomock.Any(), "2001", gomock.Any()).Return(nil)
	require.NoError(t, d.svc.SendOrderStatusChanged(context.Background(), orderID, userID, "paid"))

	d.bindings.EXPECT().Set(gomock.Any(), userID, "-1002003")
	require.NoError(t, d.svc.BindTelegram(context.Background(), userID, "-1002003"))

	d.bindings.EXPECT().Get(gomock.Any(), userID).Return("-1002003", true)
	d.sender.EXPECT().Send(gomock.Any(), "-1002003", gomock.Any()).Return(nil)
	require.NoError(t, d.svc.SendOrderStatusChanged(context.Background(), orderID, userID, "paid"))
}