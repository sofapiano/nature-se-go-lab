package entity

import "time"

// OutboxEvent представляет событие для асинхронной отправки.
type OutboxEvent struct {
	EventID    string
	EventType  string
	OrderID    string
	UserID     string
	Status     string
	Payload    string // JSON-строка
	StatusSent string // "pending", "sent", "failed"
	LeaseUntil time.Time
	Attempts   int
	LastError  string
}