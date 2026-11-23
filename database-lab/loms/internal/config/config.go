package config

import (
	"fmt"

	"github.com/caarlos0/env/v10"
)

type (
	Config struct {
		Clients struct {
			NotifierGrpcAddr string `env:"NOTIFIER_GRPC_ADDR" envDefault:"localhost:50053"`
			BillingGrpcAddr  string `env:"BILLING_GRPC_ADDR"  envDefault:"localhost:50054"`
		}

		GRPC struct {
			Port string `env:"GRPC_PORT" envDefault:"50052"`
		}

		PG struct {
			Host     string `env:"POSTGRES_HOST" envDefault:"localhost"`
			Port     string `env:"POSTGRES_PORT" envDefault:"5432"`
			DB       string `env:"POSTGRES_DB" envDefault:"ecommerce"`
			User     string `env:"POSTGRES_USER" envDefault:"ecommerce_user"`
			Password string `env:"POSTGRES_PASSWORD" envDefault:"12345"`
		}
	}
)

// Load читает конфигурацию из переменных окружения.
func Load() (Config, error) {
	var cfg Config

	if err := env.Parse(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	return cfg, nil
}
