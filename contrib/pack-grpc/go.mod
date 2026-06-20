module go.klarlabs.de/agent/contrib/pack-grpc

go 1.26.2

require (
	go.klarlabs.de/agent v0.0.0
	google.golang.org/grpc v1.81.1
)

require (
	go.opentelemetry.io/otel/sdk/metric v1.44.0 // indirect
	golang.org/x/net v0.54.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260504160031-60b97b32f348 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace go.klarlabs.de/agent => ../..
