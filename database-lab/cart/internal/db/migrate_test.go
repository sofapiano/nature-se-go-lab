package db

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateStableLockID(t *testing.T) {
	a := generateStableLockID("cart")
	require.Equal(t, a, generateStableLockID("cart"))
	require.GreaterOrEqual(t, a, int64(0))
	require.NotEqual(t, a, generateStableLockID("loms"))
}
