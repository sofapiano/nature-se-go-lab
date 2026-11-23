package entity

import "time"

const (
	InboxPending    = "pending"
	InboxInProgress = "in_progress"
	InboxProcessed  = "processed"
)

type InboxEvent struct {
	EventID    string
	OrderID    string
	UserID     string
	Status     string
	State      string
	LeaseUntil time.Time
	Attempts   int
	LastError  string
}