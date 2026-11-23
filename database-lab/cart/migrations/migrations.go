// Package migrations вшивает SQL-миграции сервиса в бинарник.
package migrations

import "embed"

// FS — виртуальная файловая система со всеми .sql миграциями этого сервиса.
//
//go:embed *.sql
var FS embed.FS
