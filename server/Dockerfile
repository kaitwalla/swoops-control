# Stage 1: Build frontend
FROM node:20-alpine AS frontend-builder

WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: Build Go server
FROM golang:1.23-alpine AS server-builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make gcc musl-dev

# Copy go workspace files
COPY go.work go.work.sum ./

# Copy ALL workspace modules (required by go.work)
COPY pkg/ ./pkg/
COPY server/go.mod server/go.sum ./server/
COPY agent/go.mod agent/go.sum ./agent/

# Download dependencies (workspace-aware)
WORKDIR /app/server
RUN go mod download

# Copy server source
COPY server/ ./

# Copy built frontend from previous stage
COPY --from=frontend-builder /app/web/dist ./internal/frontend/dist

# Build server binary
WORKDIR /app
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o /app/bin/swoopsd ./server/cmd/swoopsd

# Stage 3: Runtime image
FROM alpine:latest

LABEL org.opencontainers.image.title="Swoops Control Plane"
LABEL org.opencontainers.image.description="Distributed AI Agent Orchestrator Control Plane"
LABEL org.opencontainers.image.source="https://github.com/swoopsh/swoops"

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata sqlite

# Create non-root user
RUN addgroup -g 1000 swoops && \
    adduser -D -u 1000 -G swoops swoops

# Create data directory
RUN mkdir -p /var/lib/swoops && \
    chown -R swoops:swoops /var/lib/swoops

WORKDIR /app

# Copy binary from builder
COPY --from=server-builder /app/bin/swoopsd /usr/local/bin/swoopsd

# Switch to non-root user
USER swoops

# Expose ports
EXPOSE 8080 9090

# Set default environment variables
ENV SWOOPS_DB_PATH=/var/lib/swoops/swoops.db

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/v1/health || exit 1

# Run server
ENTRYPOINT ["/usr/local/bin/swoopsd"]
