module github.com/swoopsh/swoops/agent

go 1.25.0

require (
	github.com/swoopsh/swoops/pkg v0.0.0
	google.golang.org/grpc v1.77.0
)

require github.com/modelcontextprotocol/go-sdk v1.4.0 // indirect

replace github.com/swoopsh/swoops/pkg => ../pkg
