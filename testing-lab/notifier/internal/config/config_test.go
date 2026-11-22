package config_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/igoroutine-courses/microservices.ecommerce.notifier/internal/config"
)

// unsetEnv — os.Unsetenv с восстановлением. Нельзя t.Setenv("KEY", ""): env
// трактует пустую строку как «установлено» и не применит envDefault.
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
	unsetEnv(t, "TELEGRAM_API_BASE_URL")
	unsetEnv(t, "TELEGRAM_BOT_TOKEN")

	cfg, err := config.New()
	require.NoError(t, err)
	require.Equal(t, "50053", cfg.GRPC.Port)
	require.Equal(t, "https://api.telegram.org", cfg.Telegram.BaseURL)
	require.Equal(t, "test-token", cfg.Telegram.Token)
}

func TestNew_FromEnv(t *testing.T) {
	t.Setenv("GRPC_PORT", "9100")
	t.Setenv("TELEGRAM_API_BASE_URL", "http://telegram:8080")
	t.Setenv("TELEGRAM_BOT_TOKEN", "prod-token")

	cfg, err := config.New()
	require.NoError(t, err)
	require.Equal(t, "9100", cfg.GRPC.Port)
	require.Equal(t, "http://telegram:8080", cfg.Telegram.BaseURL)
	require.Equal(t, "prod-token", cfg.Telegram.Token)
}
