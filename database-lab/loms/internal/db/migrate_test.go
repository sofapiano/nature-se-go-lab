package db

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateStableLockID(t *testing.T) {
	a := generateStableLockID("loms")
	require.Equal(t, a, generateStableLockID("loms")) // детерминированность
	require.GreaterOrEqual(t, a, int64(0))            // неотрицательный
	require.NotEqual(t, a, generateStableLockID("cart"))
}
