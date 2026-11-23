package telegram_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/igoroutine-courses/microservices.ecommerce.notifier/internal/adapter/telegram"
	"github.com/igoroutine-courses/microservices.ecommerce.notifier/internal/entity"
)

// requestBody — проекция тела запроса, которое уходит в Telegram.
// Своя структура, чтобы не зависеть от unexported sendRequest в адаптере.
type requestBody struct {
	ChatID string `json:"chat_id"`
	Text   string `json:"text"`
}

// TestSend_PostsCorrectRequest — проверяет URL, метод, Content-Type и тело.
// Это контракт с Bot API: если поменять путь или поле — сообщения перестанут
// доходить, но код продолжит компилироваться.
func TestSend_PostsCorrectRequest(t *testing.T) {
	var (
		gotPath, gotMethod, gotContentType string
		gotBody                            requestBody
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":1}}`))
	}))
	t.Cleanup(srv.Close)

	c := telegram.New(srv.URL, "test-token", srv.Client())
	require.NoError(t, c.Send(context.Background(), "1001", "hello"))

	require.Equal(t, "/bottest-token/sendMessage", gotPath)
	require.Equal(t, http.MethodPost, gotMethod)
	require.Equal(t, "application/json", gotContentType)
	require.Equal(t, requestBody{ChatID: "1001", Text: "hello"}, gotBody)
}

// TestSend_NonOKStatusIsUnavailable — 5xx и 4xx от Telegram = Unavailable
// (сервис живой, но временно не принимает запрос).
func TestSend_NonOKStatusIsUnavailable(t *testing.T) {
	for _, code := range []int{http.StatusInternalServerError, http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(code)
				_, _ = w.Write([]byte(`{"ok":false}`))
			}))
			t.Cleanup(srv.Close)

			c := telegram.New(srv.URL, "test-token", srv.Client())
			err := c.Send(context.Background(), "1", "x")
			require.ErrorIs(t, err, entity.ErrUnavailable)
		})
	}
}

// TestSend_OK200WithOKFalseIsError — ответ 200 с `{"ok": false}` от Telegram —
// ошибка доставки, даже если HTTP-статус успешный (TASK.md).
func TestSend_OK200WithOKFalseIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":false,"description":"chat not found"}`))
	}))
	t.Cleanup(srv.Close)

	c := telegram.New(srv.URL, "test-token", srv.Client())
	err := c.Send(context.Background(), "1", "x")
	require.ErrorIs(t, err, entity.ErrDependency)
	require.Contains(t, err.Error(), "chat not found")
}

// TestSend_MalformedResponseIsDependencyError — 200 с невалидным JSON
// не должен считаться успехом: лучше упасть, чем молча проглотить.
func TestSend_MalformedResponseIsDependencyError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	t.Cleanup(srv.Close)

	c := telegram.New(srv.URL, "test-token", srv.Client())
	err := c.Send(context.Background(), "1", "x")
	require.ErrorIs(t, err, entity.ErrDependency)
}

// TestSend_NetworkErrorIsDependencyError — недоступность Telegram
// (DNS, dial, timeout) = Dependency, чтобы LOMS её отличал от 5xx.
func TestSend_NetworkErrorIsDependencyError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	srv.Close() // закрыли — dial упадёт

	c := telegram.New(srv.URL, "test-token", srv.Client())
	err := c.Send(context.Background(), "1", "x")
	require.ErrorIs(t, err, entity.ErrDependency)
}
