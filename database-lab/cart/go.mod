module github.com/igoroutine-courses/microservices.ecommerce.cart

go 1.27.3

require github.com/caarlos0/env/v10 v10.0.0

replace github.com/igoroutine-courses/microservices.ecommerce.pkg => ../pkg

require (
	buf.build/go/protovalidate v1.4.0
	github.com/caarlos0/env/v10 v10.0.0
	github.com/igoroutine-courses/microservices.ecommerce.pkg v0.0.0-00010101000000-000000000000
	github.com/jackc/pgx/v5 v5.13.0
	github.com/pressly/goose/v3 v3.30.1
	github.com/stretchr/testify v1.12.1
	go.uber.org/mock v0.6.0
	google.golang.org/grpc v1.85.0
	google.golang.org/protobuf v1.36.12
)

require (
	buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go v1.36.12-20260825204119-511051f7f437.2 // indirect
	cel.dev/cel-go v0.32.0 // indirect
	cel.dev/expr v0.25.3 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.3 // indirect
	github.com/mfridman/interpolate v0.2.0 // indirect
	github.com/sethvargo/go-retry v0.5.0 // indirect
	github.com/sqlc-dev/sqlc v1.31.1 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.6 // indirect
	golang.org/x/exp v0.0.0-20261109185158-a52e3aef5920 // indirect
	golang.org/x/net v0.62.0 // indirect
	golang.org/x/sync v0.25.0 // indirect
	golang.org/x/sys v0.50.0 // indirect
	golang.org/x/text v0.44.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20261110000943-09e21b237f2c // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20261110000943-09e21b237f2c // indirect
)
