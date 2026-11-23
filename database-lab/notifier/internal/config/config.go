package config

import "github.com/caarlos0/env/v10"

type Config struct {
	GRPC struct {
		Port string `env:"GRPC_PORT" envDefault:"50053"`
	}
	Telegram struct {
		BaseURL string `env:"TELEGRAM_API_BASE_URL" envDefault:"https://api.telegram.org"`
		Token   string `env:"TELEGRAM_BOT_TOKEN" envDefault:"test-token"`
	}
	PG struct {
		Host     string `env:"POSTGRES_HOST" envDefault:"localhost"`
		Port     string `env:"POSTGRES_PORT" envDefault:"5432"`
		DB       string `env:"POSTGRES_DB" envDefault:"ecommerce"`
		User     string `env:"POSTGRES_USER" envDefault:"ecommerce_user"`
		Password string `env:"POSTGRES_PASSWORD" envDefault:"12345"`
	}
}

func New() (*Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
