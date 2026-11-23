package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/igoroutine-courses/microservices.ecommerce.notifier/internal/entity"
)

type inboxRepository interface {
	Claim(ctx context.Context, limit int) ([]entity.InboxEvent, error)
	MarkProcessed(ctx context.Context, eventID string, lease time.Time) error
	MarkRetry(ctx context.Context, eventID string, lease time.Time, backoffSec int, errMsg string) error
}

// delivery — логика доставки (usecase notification.Service): привязка + Telegram.
type delivery interface {
	SendOrderStatusChanged(ctx context.Context, orderID, userID, status string) error
}

type InboxWorker struct {
	repo      inboxRepository
	delivery  delivery
	interval  time.Duration
	batchSize int
}

func NewInboxWorker(repo inboxRepository, delivery delivery, interval time.Duration, batchSize int) *InboxWorker {
	return &InboxWorker{repo: repo, delivery: delivery, interval: interval, batchSize: batchSize}
}

func (w *InboxWorker) Start(ctx context.Context) {
	slog.Info("inbox worker started")
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("inbox worker stopped gracefully")
			return
		case <-ticker.C:
			w.processBatch(ctx)
		}
	}
}

func (w *InboxWorker) processBatch(ctx context.Context) {
	events, err := w.repo.Claim(ctx, w.batchSize)
	if err != nil {
		slog.Error("claim inbox events", "error", err)
		return
	}

	for _, ev := range events {
		// ⚠️ Золотое правило: вызов Telegram ВНЕ транзакции БД.
		err := w.delivery.SendOrderStatusChanged(ctx, ev.OrderID, ev.UserID, ev.Status)
		if err != nil {
			backoff := min(60, 5*ev.Attempts)
			slog.Warn("telegram delivery failed, schedule retry",
				"event_id", ev.EventID, "attempts", ev.Attempts, "error", err)
			_ = w.repo.MarkRetry(ctx, ev.EventID, ev.LeaseUntil, backoff, err.Error())
			continue
		}
		// processed ставим ТОЛЬКО после успешного ответа Telegram.
		_ = w.repo.MarkProcessed(ctx, ev.EventID, ev.LeaseUntil)
	}
}