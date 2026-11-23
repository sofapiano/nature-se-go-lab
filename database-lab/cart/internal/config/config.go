package config

import "github.com/caarlos0/env/v10"

type Config struct {
	Clients struct {
		LOMSGrpcAddr string `env:"LOMS_GRPC_ADDR" envDefault:"localhost:50052"`
	}

	GRPC struct {
		Port string `env:"GRPC_PORT" envDefault:"50051"`
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
