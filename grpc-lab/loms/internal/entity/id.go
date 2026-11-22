package entity

import (
	"strings"
)

// CanonicalID приводит UUID к каноническому виду (нижний регистр): один и тот же
// идентификатор в разном регистре — это один и тот же идентификатор.
func CanonicalID(id string) string {
	return strings.ToLower(id)
}
