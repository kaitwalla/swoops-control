# Stage 1: Build agent
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make

# Copy go workspace files
COPY go.work go.work.sum ./

# Copy ALL workspace modules (required by go.work)
COPY pkg/ ./pkg/
COPY agent/go.mod agent/go.sum ./agent/
COPY server/go.mod server/go.sum ./server/

# Download dependencies (workspace-aware)
WORKDIR /app/agent
RUN go mod download

# Copy agent source
COPY agent/ ./

# Build agent binary
WORKDIR /app
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/swoops-agent ./agent/cmd/swoops-agent

# Stage 2: Runtime image
FROM alpine:latest

LABEL org.opencontainers.image.title="Swoops Agent"
LABEL org.opencontainers.image.description="Swoops agent for managing AI sessions on remote hosts"
LABEL org.opencontainers.image.source="https://github.com/swoopsh/swoops"

# Install runtime dependencies
RUN apk add --no-cache ca-certificates git tmux openssh-client

# Create non-root user
RUN addgroup -g 1000 swoops && \
    adduser -D -u 1000 -G swoops swoops

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/bin/swoops-agent /usr/local/bin/swoops-agent

# Switch to non-root user
USER swoops

# Set default environment variables
ENV SWOOPS_AGENT_TOKEN=""

# Run agent
ENTRYPOINT ["/usr/local/bin/swoops-agent"]
CMD ["run", "--server", "swoopsd:9090", "--host-id", ""]
