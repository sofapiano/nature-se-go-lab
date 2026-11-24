package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewPool_InvalidDSN(t *testing.T) {
	_, err := NewPool(context.Background(), Config{DSN: "://bad"})
	require.ErrorContains(t, err, "parse dsn")
}

func TestNewPool_Unreachable(t *testing.T) {
	_, err := NewPool(context.Background(), Config{
		DSN:      "postgres://u:p@127.0.0.1:1/db?sslmode=disable",
		MaxConns: 1,
	})
	require.ErrorContains(t, err, "ping database")
}
