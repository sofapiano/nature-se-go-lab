package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
)

type mockOutboxRepo struct {
	fetchAndLockFunc func(ctx context.Context, limit int) ([]entity.OutboxEvent, error)
	markSentFunc     func(ctx context.Context, eventID string) error
	markFailedFunc   func(ctx context.Context, eventID, errMsg string) error
}

func (m *mockOutboxRepo) FetchAndLock(ctx context.Context, limit int) ([]entity.OutboxEvent, error) {
	if m.fetchAndLockFunc != nil {
		return m.fetchAndLockFunc(ctx, limit)
	}
	return nil, nil
}

func (m *mockOutboxRepo) MarkSent(ctx context.Context, eventID string) error {
	if m.markSentFunc != nil {
		return m.markSentFunc(ctx, eventID)
	}
	return nil
}

func (m *mockOutboxRepo) MarkFailed(ctx context.Context, eventID, errMsg string) error {
	if m.markFailedFunc != nil {
		return m.markFailedFunc(ctx, eventID, errMsg)
	}
	return nil
}

type mockNotifierClient struct {
	sendEventFunc func(ctx context.Context, eventID, userID, orderID, status string) error
}

func (m *mockNotifierClient) SendEvent(ctx context.Context, eventID, userID, orderID, status string) error {
	if m.sendEventFunc != nil {
		return m.sendEventFunc(ctx, eventID, userID, orderID, status)
	}
	return nil
}

func TestOutboxWorker_processBatch_Success(t *testing.T) {
	repo := &mockOutboxRepo{
		fetchAndLockFunc: func(ctx context.Context, limit int) ([]entity.OutboxEvent, error) {
			return []entity.OutboxEvent{
				{EventID: "e1", UserID: "u1", OrderID: "o1", Status: "new"},
			}, nil
		},
		markSentFunc: func(ctx context.Context, eventID string) error {
			if eventID != "e1" {
				t.Errorf("expected eventID e1, got %s", eventID)
			}
			return nil
		},
	}

	notifier := &mockNotifierClient{
		sendEventFunc: func(ctx context.Context, eventID, userID, orderID, status string) error {
			if eventID != "e1" {
				t.Errorf("expected eventID e1, got %s", eventID)
			}
			return nil
		},
	}

	w := NewOutboxWorker(repo, notifier, time.Second, 10)
	w.processBatch(context.Background())
}

func TestOutboxWorker_processBatch_Failure(t *testing.T) {
	repo := &mockOutboxRepo{
		fetchAndLockFunc: func(ctx context.Context, limit int) ([]entity.OutboxEvent, error) {
			return []entity.OutboxEvent{
				{EventID: "e1", UserID: "u1", OrderID: "o1", Status: "new"},
			}, nil
		},
		markFailedFunc: func(ctx context.Context, eventID, errMsg string) error {
			if eventID != "e1" {
				t.Errorf("expected eventID e1, got %s", eventID)
			}
			if errMsg != "network error" {
				t.Errorf("expected error 'network error', got %s", errMsg)
			}
			return nil
		},
	}

	notifier := &mockNotifierClient{
		sendEventFunc: func(ctx context.Context, eventID, userID, orderID, status string) error {
			return errors.New("network error")
		},
	}

	w := NewOutboxWorker(repo, notifier, time.Second, 10)
	w.processBatch(context.Background())
}

func TestOutboxWorker_processBatch_FetchError(t *testing.T) {
	repo := &mockOutboxRepo{
		fetchAndLockFunc: func(ctx context.Context, limit int) ([]entity.OutboxEvent, error) {
			return nil, errors.New("db error")
		},
	}

	notifier := &mockNotifierClient{}

	w := NewOutboxWorker(repo, notifier, time.Second, 10)
	w.processBatch(context.Background())
}