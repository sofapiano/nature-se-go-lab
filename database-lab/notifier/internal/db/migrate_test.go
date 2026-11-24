package db

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateStableLockID(t *testing.T) {
	a := generateStableLockID("notifier")
	require.Equal(t, a, generateStableLockID("notifier"))
	require.GreaterOrEqual(t, a, int64(0))
	require.NotEqual(t, a, generateStableLockID("cart"))
}
