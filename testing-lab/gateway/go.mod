module github.com/igoroutine-courses/microservices.ecommerce.gateway

go 1.27.1

replace github.com/igoroutine-courses/microservices.ecommerce.pkg => ../pkg

require (
	github.com/caarlos0/env/v10 v10.0.0
	github.com/igoroutine-courses/microservices.ecommerce.pkg v0.0.0-00010101000000-000000000000
	google.golang.org/grpc v1.85.0
	google.golang.org/protobuf v1.36.12
)

require (
	buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go v1.36.12-20260825204119-511051f7f437.2 // indirect
	golang.org/x/net v0.60.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.42.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260819154853-08b0e4226688 // indirect
)
