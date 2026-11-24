package entity

import "errors"

var (
	ErrInvalid     = errors.New("invalid notification")
	ErrUnavailable = errors.New("telegram unavailable")
	ErrDependency  = errors.New("telegram request failed")
	ErrConflict    = errors.New("inbox event payload conflict")
)

// Binding — связка «пользователь → чат». chat_id храним строкой: группы
// получают отрицательные id, а int64-диапазон в тестах — часть контракта.
type Binding struct {
	UserID string
	ChatID string
}
