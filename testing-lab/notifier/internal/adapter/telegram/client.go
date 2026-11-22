package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/igoroutine-courses/microservices.ecommerce.notifier/internal/entity"
)

type httpClient interface {
	Do(*http.Request) (*http.Response, error)
}

type Client struct {
	baseURL string
	token   string
	http    httpClient
}

func New(baseURL, token string, client httpClient) *Client {
	return &Client{baseURL: baseURL, token: token, http: client}
}

type sendRequest struct {
	ChatID string `json:"chat_id"`
	Text   string `json:"text"`
}

// sendResponse — минимум полей, которые нужны для проверки «ok: false».
// Ответ 200 с ok=false — ошибка доставки (Telegram так сообщает о проблемах
// бота/чата), см. TASK.md.
type sendResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description,omitempty"`
}

func (c *Client) Send(ctx context.Context, chatID, text string) error {
	body, err := json.Marshal(sendRequest{ChatID: chatID, Text: text})
	if err != nil {
		return fmt.Errorf("%w: marshal body: %v", entity.ErrDependency, err)
	}

	url := fmt.Sprintf("%s/bot%s/sendMessage", c.baseURL, c.token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("%w: build request: %v", entity.ErrDependency, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", entity.ErrDependency, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: telegram returned %d", entity.ErrUnavailable, resp.StatusCode)
	}

	var result sendResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("%w: decode response: %v", entity.ErrDependency, err)
	}
	if !result.OK {
		return fmt.Errorf("%w: telegram declined: %s", entity.ErrDependency, result.Description)
	}
	return nil
}
