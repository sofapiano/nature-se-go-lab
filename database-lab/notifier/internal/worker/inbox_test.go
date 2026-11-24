package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/igoroutine-courses/microservices.ecommerce.notifier/internal/entity"
)

type mockInboxRepo struct {
	claimFunc         func(ctx context.Context, limit int) ([]entity.InboxEvent, error)
	markProcessedFunc func(ctx context.Context, eventID string, lease time.Time) error
	markRetryFunc     func(ctx context.Context, eventID string, lease time.Time, backoffSec int, errMsg string) error
}

func (m *mockInboxRepo) Claim(ctx context.Context, limit int) ([]entity.InboxEvent, error) {
	if m.claimFunc != nil {
		return m.claimFunc(ctx, limit)
	}
	return nil, nil
}

func (m *mockInboxRepo) MarkProcessed(ctx context.Context, eventID string, lease time.Time) error {
	if m.markProcessedFunc != nil {
		return m.markProcessedFunc(ctx, eventID, lease)
	}
	return nil
}

func (m *mockInboxRepo) MarkRetry(ctx context.Context, eventID string, lease time.Time, backoffSec int, errMsg string) error {
	if m.markRetryFunc != nil {
		return m.markRetryFunc(ctx, eventID, lease, backoffSec, errMsg)
	}
	return nil
}

type mockDelivery struct {
	sendOrderStatusChangedFunc func(ctx context.Context, orderID, userID, status string) error
}

func (m *mockDelivery) SendOrderStatusChanged(ctx context.Context, orderID, userID, status string) error {
	if m.sendOrderStatusChangedFunc != nil {
		return m.sendOrderStatusChangedFunc(ctx, orderID, userID, status)
	}
	return nil
}

func TestInboxWorker_processBatch_Success(t *testing.T) {
	lease := time.Now().Add(30 * time.Second)
	repo := &mockInboxRepo{
		claimFunc: func(ctx context.Context, limit int) ([]entity.InboxEvent, error) {
			return []entity.InboxEvent{
				{EventID: "e1", OrderID: "o1", UserID: "u1", Status: "new", LeaseUntil: lease, Attempts: 1},
			}, nil
		},
		markProcessedFunc: func(ctx context.Context, eventID string, l time.Time) error {
			if eventID != "e1" {
				t.Errorf("expected eventID e1, got %s", eventID)
			}
			return nil
		},
	}

	delivery := &mockDelivery{
		sendOrderStatusChangedFunc: func(ctx context.Context, orderID, userID, status string) error {
			return nil
		},
	}

	w := NewInboxWorker(repo, delivery, time.Second, 10)
	w.processBatch(context.Background())
}

func TestInboxWorker_processBatch_Failure(t *testing.T) {
	lease := time.Now().Add(30 * time.Second)
	repo := &mockInboxRepo{
		claimFunc: func(ctx context.Context, limit int) ([]entity.InboxEvent, error) {
			return []entity.InboxEvent{
				{EventID: "e1", OrderID: "o1", UserID: "u1", Status: "new", LeaseUntil: lease, Attempts: 2},
			}, nil
		},
		markRetryFunc: func(ctx context.Context, eventID string, l time.Time, backoffSec int, errMsg string) error {
			if eventID != "e1" {
				t.Errorf("expected eventID e1, got %s", eventID)
			}
			// backoffBaseSec = 1, attempts = 2 -> 1 * 2 = 2. min(60, 2) = 2
			if backoffSec != 2 {
				t.Errorf("expected backoffSec 2, got %d", backoffSec)
			}
			return nil
		},
	}

	delivery := &mockDelivery{
		sendOrderStatusChangedFunc: func(ctx context.Context, orderID, userID, status string) error {
			return errors.New("telegram error")
		},
	}

	w := NewInboxWorker(repo, delivery, time.Second, 10)
	w.processBatch(context.Background())
}

func TestInboxWorker_processBatch_ClaimError(t *testing.T) {
	repo := &mockInboxRepo{
		claimFunc: func(ctx context.Context, limit int) ([]entity.InboxEvent, error) {
			return nil, errors.New("db error")
		},
	}

	delivery := &mockDelivery{}

	w := NewInboxWorker(repo, delivery, time.Second, 10)
	w.processBatch(context.Background())
}
