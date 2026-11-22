package config_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/igoroutine-courses/microservices.ecommerce.cart/internal/config"
)

// unsetEnv — os.Unsetenv с восстановлением исходного значения на Cleanup.
// Нельзя t.Setenv("KEY", ""): caarlos0/env может трактовать пустую строку
// как «установлено» и не применить envDefault. t.Setenv не умеет unset.
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	orig, existed := os.LookupEnv(key)
	require.NoError(t, os.Unsetenv(key))
	t.Cleanup(func() {
		if existed {
			_ = os.Setenv(key, orig)
			return
		}
		_ = os.Unsetenv(key)
	})
}

func TestNew_Defaults(t *testing.T) {
	unsetEnv(t, "GRPC_PORT")
	unsetEnv(t, "LOMS_GRPC_ADDR")

	cfg, err := config.New()
	require.NoError(t, err)
	require.Equal(t, "50051", cfg.GRPC.Port)
	require.Equal(t, "localhost:50052", cfg.Clients.LOMSGrpcAddr)
}

func TestNew_FromEnv(t *testing.T) {
	t.Setenv("GRPC_PORT", "9100")
	t.Setenv("LOMS_GRPC_ADDR", "loms:9999")

	cfg, err := config.New()
	require.NoError(t, err)
	require.Equal(t, "9100", cfg.GRPC.Port)
	require.Equal(t, "loms:9999", cfg.Clients.LOMSGrpcAddr)
}
