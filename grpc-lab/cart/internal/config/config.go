package config

import "github.com/caarlos0/env/v10"

type Config struct {
	Clients struct {
		LOMSGrpcAddr string `env:"LOMS_GRPC_ADDR" envDefault:"localhost:50052"`
	}

	GRPC struct {
		Port string `env:"GRPC_PORT" envDefault:"50051"`
	}
}

func New() (*Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
