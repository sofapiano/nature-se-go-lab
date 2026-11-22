package config

// import "github.com/caarlos0/env/v10"

type (
	Config struct {
		Clients struct {
			NotifierCallbackAddr string `env:"NOTIFIER_GRPC_ADDR" envDefault:""`
		}

		GRPC struct {
			Port string `env:"GRPC_PORT" envDefault:"50052"`
		}
	}
)
