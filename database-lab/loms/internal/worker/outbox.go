package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
)

// notifierClient определяет метод, который нам нужен от адаптера Notifier.
type notifierClient interface {
	SendOrderStatusChanged(ctx context.Context, userID, orderID, status string) error
}

// outboxRepository определяет методы, нужные воркеру.
type outboxRepository interface {
	FetchAndLock(ctx context.Context, limit int) ([]entity.OutboxEvent, error)
	MarkSent(ctx context.Context, eventID string) error
	MarkFailed(ctx context.Context, eventID, errMsg string) error
}

type OutboxWorker struct {
	repo      outboxRepository
	notifier  notifierClient
	interval  time.Duration
	batchSize int
}

func NewOutboxWorker(repo outboxRepository, notifier notifierClient, interval time.Duration, batchSize int) *OutboxWorker {
	return &OutboxWorker{
		repo:      repo,
		notifier:  notifier,
		interval:  interval,
		batchSize: batchSize,
	}
}

// Start запускает бесконечный цикл обработки с заданным интервалом.
func (w *OutboxWorker) Start(ctx context.Context) {
	slog.Info("outbox worker started")
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("outbox worker stopped gracefully")
			return
		case <-ticker.C:
			w.processBatch(ctx)
		}
	}
}

func (w *OutboxWorker) processBatch(ctx context.Context) {
	events, err := w.repo.FetchAndLock(ctx, w.batchSize)
	if err != nil {
		slog.Error("failed to fetch outbox events", "error", err)
		return
	}

	for _, ev := range events {
		// Сетевой вызов вне транзакции БД
		err := w.notifier.SendOrderStatusChanged(ctx, ev.UserID, ev.OrderID, ev.Status)
		if err != nil {
			slog.Warn("failed to send notification, will retry",
				"event_id", ev.EventID, "order_id", ev.OrderID, "error", err)
			_ = w.repo.MarkFailed(ctx, ev.EventID, err.Error())
		} else {
			slog.Info("notification sent successfully", "event_id", ev.EventID, "order_id", ev.OrderID)
			_ = w.repo.MarkSent(ctx, ev.EventID)
		}
	}
}