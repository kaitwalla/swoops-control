.PHONY: all build build-web build-server build-agent dev dev-server dev-web clean

all: build

# Build the frontend, copy dist into server embed dir, then build the server
build: build-web build-server build-agent

build-web:
	cd web && npm run build
	rm -rf server/internal/frontend/dist
	cp -r web/dist server/internal/frontend/dist

build-server: build-web
	cd . && go build -o bin/swoopsd ./server/cmd/swoopsd

build-agent:
	cd . && go build -o bin/swoops-agent ./agent/cmd/swoops-agent

# Development: run server and frontend dev server concurrently
dev:
	@echo "Starting server and frontend dev server..."
	@make -j2 dev-server dev-web

dev-server:
	go run ./server/cmd/swoopsd

dev-web:
	cd web && npm run dev

clean:
	rm -rf bin/ web/dist server/internal/frontend/dist

# Cross-compile agent for all platforms
build-agent-all:
	GOOS=linux GOARCH=amd64 go build -o bin/swoops-agent-linux-amd64 ./agent/cmd/swoops-agent
	GOOS=linux GOARCH=arm64 go build -o bin/swoops-agent-linux-arm64 ./agent/cmd/swoops-agent
	GOOS=darwin GOARCH=amd64 go build -o bin/swoops-agent-darwin-amd64 ./agent/cmd/swoops-agent
	GOOS=darwin GOARCH=arm64 go build -o bin/swoops-agent-darwin-arm64 ./agent/cmd/swoops-agent
