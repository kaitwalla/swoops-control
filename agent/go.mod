module github.com/kaitwalla/swoops-control/agent

go 1.25.0

require (
	github.com/gorilla/websocket v1.5.3
	github.com/kaitwalla/swoops-control/pkg v0.0.0
)

require google.golang.org/grpc v1.77.0 // indirect

require (
	github.com/google/jsonschema-go v0.4.2 // indirect
	github.com/modelcontextprotocol/go-sdk v1.4.0
	github.com/segmentio/asm v1.1.3 // indirect
	github.com/segmentio/encoding v0.5.3 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	golang.org/x/net v0.49.0 // indirect
	golang.org/x/oauth2 v0.34.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251022142026-3a174f9686a8 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
)

replace github.com/kaitwalla/swoops-control/pkg => ../pkg
