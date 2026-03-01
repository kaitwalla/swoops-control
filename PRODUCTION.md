# Production Deployment Guide

This guide covers deploying Swoops in a production environment with all security features enabled.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Deployment Architecture](#deployment-architecture)
- [Reverse Proxy with Automatic HTTPS (Recommended)](#reverse-proxy-with-automatic-https-recommended)
- [TLS Certificates (Direct Deployment)](#tls-certificates-direct-deployment)
- [Configuration](#configuration)
- [Docker Deployment](#docker-deployment)
- [Kubernetes Deployment](#kubernetes-deployment)
- [Monitoring](#monitoring)
- [Security Checklist](#security-checklist)
- [Troubleshooting](#troubleshooting)

## Prerequisites

### Required
- Go 1.23+ (for building from source)
- Docker 24+ and Docker Compose (for containerized deployment)
- Valid TLS certificates for HTTPS and mTLS
- PostgreSQL or SQLite database
- Reverse proxy (nginx, Caddy, or similar) recommended

### Recommended
- Prometheus for metrics collection
- Grafana for visualization
- Systemd or equivalent for service management
- Firewall configured to restrict access

## Quick Setup (Interactive)

For the fastest setup experience, use the interactive setup script:

```bash
./setup.sh
```

The setup script will:
- Guide you through all configuration decisions
- Generate configuration files (swoopsd.yaml, agent.env)
- Optionally generate self-signed certificates for testing
- Configure reverse proxy (Caddy or nginx)
- Install systemd/launchd services
- Provide complete next steps

**Continue reading for manual setup details and advanced configurations.**

## Deployment Architecture

For production deployments, we recommend using a reverse proxy (Caddy or nginx) in front of Swoops:

```
Internet
    │
    ├─ HTTPS:443 ──────> Reverse Proxy (Caddy/nginx)
    │                    • Automatic Let's Encrypt
    │                    • Certificate renewal
    │                    ├─> HTTP:8080 ──> Swoops (HTTP API + WebSocket)
    │                    └─> 9090 ──────> Swoops (gRPC with mTLS passthrough)
    │
    └─ gRPC:9090 ─────> Swoops Agent Connections (TLS/mTLS)
```

**Benefits:**
- ✅ Automatic Let's Encrypt certificate management
- ✅ Automatic certificate renewal (no manual intervention)
- ✅ Separation of concerns (certificate handling vs application logic)
- ✅ Easy to add rate limiting, caching, or WAF
- ✅ Standard production pattern

## Reverse Proxy with Automatic HTTPS (Recommended)

### Option 1: Caddy (Automatic HTTPS)

Caddy automatically obtains and renews Let's Encrypt certificates with zero configuration.

**Install Caddy:**
```bash
# Debian/Ubuntu
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt update
sudo apt install caddy

# macOS
brew install caddy
```

**Caddyfile** (`/etc/caddy/Caddyfile`):
```caddy
# HTTPS for Web UI and API
swoops.example.com {
    # Reverse proxy to Swoops HTTP API
    reverse_proxy localhost:8080

    # Enable WebSocket support
    @websockets {
        header Connection *Upgrade*
        header Upgrade websocket
    }
    reverse_proxy @websockets localhost:8080

    # Security headers
    header {
        Strict-Transport-Security "max-age=31536000; includeSubDomains; preload"
        X-Content-Type-Options "nosniff"
        X-Frame-Options "DENY"
        Referrer-Policy "strict-origin-when-cross-origin"
    }

    # Access logging
    log {
        output file /var/log/caddy/swoops-access.log
    }
}

# gRPC passthrough for agent connections (with mTLS)
# Note: gRPC agents connect directly to port 9090, bypassing Caddy
```

**Swoops Configuration** (`swoopsd.yaml`):
```yaml
server:
  host: 127.0.0.1  # Only listen on localhost
  port: 8080
  external_url: https://swoops.example.com
  tls_enabled: false  # Caddy handles TLS
  allowed_origins:
    - https://swoops.example.com

grpc:
  host: 0.0.0.0  # Must be public for agent connections
  port: 9090
  tls_cert: /etc/swoops/certs/grpc-server-cert.pem
  tls_key: /etc/swoops/certs/grpc-server-key.pem
  client_ca: /etc/swoops/certs/client-ca.pem
  insecure: false
  require_mtls: true

database:
  path: /var/lib/swoops/swoops.db

auth:
  api_key: your-secure-api-key
```

**Start Services:**
```bash
# Start Swoops
sudo systemctl start swoopsd

# Start Caddy
sudo systemctl restart caddy

# Verify
curl https://swoops.example.com/api/v1/health
```

**That's it!** Caddy automatically:
- Obtains Let's Encrypt certificate on first request
- Renews certificates before expiration
- Redirects HTTP to HTTPS
- Handles WebSocket upgrades

### Option 2: nginx with certbot

**Install nginx and certbot:**
```bash
sudo apt-get update
sudo apt-get install nginx certbot python3-certbot-nginx
```

**nginx Configuration** (`/etc/nginx/sites-available/swoops`):
```nginx
# HTTP to HTTPS redirect
server {
    listen 80;
    server_name swoops.example.com;

    # Allow certbot challenges
    location /.well-known/acme-challenge/ {
        root /var/www/certbot;
    }

    location / {
        return 301 https://$server_name$request_uri;
    }
}

# HTTPS server
server {
    listen 443 ssl http2;
    server_name swoops.example.com;

    # SSL certificates (managed by certbot)
    ssl_certificate /etc/letsencrypt/live/swoops.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/swoops.example.com/privkey.pem;

    # SSL configuration
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers 'ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384';
    ssl_prefer_server_ciphers off;
    ssl_session_cache shared:SSL:10m;
    ssl_session_timeout 10m;

    # Security headers
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains; preload" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-Frame-Options "DENY" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;

    # Proxy settings
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket support
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        # Timeouts for WebSocket
        proxy_connect_timeout 7d;
        proxy_send_timeout 7d;
        proxy_read_timeout 7d;
    }

    # Access logging
    access_log /var/log/nginx/swoops-access.log;
    error_log /var/log/nginx/swoops-error.log;
}
```

**Enable site and obtain certificate:**
```bash
# Enable the site
sudo ln -s /etc/nginx/sites-available/swoops /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx

# Obtain Let's Encrypt certificate
sudo certbot --nginx -d swoops.example.com --agree-tos --email admin@example.com

# Test auto-renewal
sudo certbot renew --dry-run
```

**Swoops Configuration** (same as Caddy example above):
```yaml
server:
  host: 127.0.0.1
  port: 8080
  tls_enabled: false  # nginx handles TLS
```

**Auto-renewal:** certbot automatically sets up a systemd timer for renewal.

### Option 3: Docker Compose with Caddy

**docker-compose.yml:**
```yaml
version: '3.8'

services:
  caddy:
    image: caddy:2-alpine
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
      - "443:443/udp"  # HTTP/3
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro
      - caddy-data:/data
      - caddy-config:/config
    networks:
      - swoops-net
    depends_on:
      - swoopsd

  swoopsd:
    image: swoops/control-plane:latest
    restart: unless-stopped
    expose:
      - "8080"  # Internal only, accessed via Caddy
    ports:
      - "9090:9090"  # gRPC must be publicly accessible
    environment:
      - SWOOPS_API_KEY=${SWOOPS_API_KEY}
      - SWOOPS_TLS_ENABLED=false  # Caddy handles TLS
      - SWOOPS_EXTERNAL_URL=https://swoops.example.com
      - SWOOPS_GRPC_INSECURE=false
      - SWOOPS_GRPC_TLS_CERT=/certs/grpc-server-cert.pem
      - SWOOPS_GRPC_TLS_KEY=/certs/grpc-server-key.pem
      - SWOOPS_GRPC_CLIENT_CA=/certs/client-ca.pem
      - SWOOPS_GRPC_REQUIRE_MTLS=true
    volumes:
      - swoops-data:/var/lib/swoops
      - ./certs:/certs:ro
    networks:
      - swoops-net
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/api/v1/health"]
      interval: 30s
      timeout: 3s
      retries: 3

  prometheus:
    image: prom/prometheus:latest
    restart: unless-stopped
    ports:
      - "9091:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - prometheus-data:/prometheus
    networks:
      - swoops-net

volumes:
  caddy-data:
  caddy-config:
  swoops-data:
  prometheus-data:

networks:
  swoops-net:
    driver: bridge
```

**Caddyfile:**
```caddy
swoops.example.com {
    reverse_proxy swoopsd:8080
}
```

**Start:**
```bash
export SWOOPS_API_KEY=$(openssl rand -hex 32)
docker-compose -f docker-compose.caddy.yml up -d
```

Caddy automatically obtains and renews Let's Encrypt certificates!

**See also:**
- Complete example: [`docker-compose.caddy.yml`](docker-compose.caddy.yml)
- Production Caddyfile: [`Caddyfile.example`](Caddyfile.example)
- nginx config: [`nginx.conf.example`](nginx.conf.example)

## TLS Certificates (Direct Deployment)

### Generating Self-Signed Certificates (Development/Testing)

```bash
# Generate CA certificate
openssl req -x509 -newkey rsa:4096 -days 365 -nodes \
  -keyout ca-key.pem -out ca-cert.pem \
  -subj "/CN=Swoops CA"

# Generate server certificate
openssl req -newkey rsa:4096 -nodes \
  -keyout server-key.pem -out server-req.pem \
  -subj "/CN=swoops.example.com"

openssl x509 -req -in server-req.pem -days 365 \
  -CA ca-cert.pem -CAkey ca-key.pem -CAcreateserial \
  -out server-cert.pem

# Generate client certificate (for mTLS)
openssl req -newkey rsa:4096 -nodes \
  -keyout client-key.pem -out client-req.pem \
  -subj "/CN=swoops-agent"

openssl x509 -req -in client-req.pem -days 365 \
  -CA ca-cert.pem -CAkey ca-key.pem -CAcreateserial \
  -out client-cert.pem
```

### Using Let's Encrypt (Manual - Direct Deployment Only)

**Note:** This is only needed if deploying Swoops directly without a reverse proxy. For most production deployments, use the reverse proxy approach above.

```bash
# Install certbot
sudo apt-get install certbot

# Stop Swoops temporarily (certbot needs port 80)
sudo systemctl stop swoopsd

# Generate certificates
sudo certbot certonly --standalone \
  -d swoops.example.com \
  --agree-tos \
  --email admin@example.com

# Certificates will be in:
# /etc/letsencrypt/live/swoops.example.com/fullchain.pem
# /etc/letsencrypt/live/swoops.example.com/privkey.pem

# Configure Swoops to use the certificates
cat > swoopsd.yaml <<EOF
server:
  tls_enabled: true
  tls_cert: /etc/letsencrypt/live/swoops.example.com/fullchain.pem
  tls_key: /etc/letsencrypt/live/swoops.example.com/privkey.pem
EOF

# Setup auto-renewal (requires Swoops restart after renewal)
sudo systemctl enable certbot.timer
```

**Limitation:** Manual deployment requires restarting Swoops after certificate renewal. Consider using a reverse proxy for automatic renewal without downtime.

## Configuration

### Production Configuration File

Create `swoopsd.yaml`:

```yaml
server:
  host: 0.0.0.0
  port: 8080
  external_url: https://swoops.example.com
  allowed_origins:
    - https://swoops.example.com
  tls_enabled: true
  tls_cert: /etc/swoops/certs/server-cert.pem
  tls_key: /etc/swoops/certs/server-key.pem

database:
  path: /var/lib/swoops/swoops.db

grpc:
  host: 0.0.0.0
  port: 9090
  tls_cert: /etc/swoops/certs/grpc-server-cert.pem
  tls_key: /etc/swoops/certs/grpc-server-key.pem
  client_ca: /etc/swoops/certs/client-ca.pem
  insecure: false
  require_mtls: true

auth:
  api_key: your-strong-random-api-key-here
```

### Environment Variables

For containerized deployments, use environment variables:

```bash
# HTTP Server
SWOOPS_TLS_ENABLED=true
SWOOPS_TLS_CERT=/etc/swoops/certs/server-cert.pem
SWOOPS_TLS_KEY=/etc/swoops/certs/server-key.pem
SWOOPS_EXTERNAL_URL=https://swoops.example.com

# gRPC Server
SWOOPS_GRPC_INSECURE=false
SWOOPS_GRPC_TLS_CERT=/etc/swoops/certs/grpc-server-cert.pem
SWOOPS_GRPC_TLS_KEY=/etc/swoops/certs/grpc-server-key.pem
SWOOPS_GRPC_CLIENT_CA=/etc/swoops/certs/client-ca.pem
SWOOPS_GRPC_REQUIRE_MTLS=true

# Authentication
SWOOPS_API_KEY=your-strong-random-api-key-here

# Database
SWOOPS_DB_PATH=/var/lib/swoops/swoops.db
```

## Docker Deployment

### Build Images

```bash
# Build control plane
docker build -f server/Dockerfile -t swoops/control-plane:latest .

# Build agent
docker build -f agent/Dockerfile -t swoops/agent:latest .
```

### Docker Compose (Production)

Create `docker-compose.prod.yml`:

```yaml
version: '3.8'

services:
  swoopsd:
    image: swoops/control-plane:latest
    restart: unless-stopped
    ports:
      - "443:8080"  # HTTPS
      - "9090:9090" # gRPC with mTLS
    environment:
      - SWOOPS_API_KEY=${SWOOPS_API_KEY}
      - SWOOPS_TLS_ENABLED=true
      - SWOOPS_TLS_CERT=/certs/server-cert.pem
      - SWOOPS_TLS_KEY=/certs/server-key.pem
      - SWOOPS_GRPC_INSECURE=false
      - SWOOPS_GRPC_TLS_CERT=/certs/grpc-server-cert.pem
      - SWOOPS_GRPC_TLS_KEY=/certs/grpc-server-key.pem
      - SWOOPS_GRPC_CLIENT_CA=/certs/client-ca.pem
      - SWOOPS_GRPC_REQUIRE_MTLS=true
      - SWOOPS_DB_PATH=/var/lib/swoops/swoops.db
    volumes:
      - swoops-data:/var/lib/swoops
      - ./certs:/certs:ro
    networks:
      - swoops-net
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "--no-check-certificate", "https://localhost:8080/api/v1/health"]
      interval: 30s
      timeout: 3s
      retries: 3

  prometheus:
    image: prom/prometheus:latest
    restart: unless-stopped
    ports:
      - "9091:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - prometheus-data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
      - '--storage.tsdb.retention.time=30d'
    networks:
      - swoops-net

  grafana:
    image: grafana/grafana:latest
    restart: unless-stopped
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=${GRAFANA_ADMIN_PASSWORD}
      - GF_USERS_ALLOW_SIGN_UP=false
      - GF_SERVER_ROOT_URL=https://grafana.example.com
    volumes:
      - grafana-data:/var/lib/grafana
      - ./grafana-datasources.yml:/etc/grafana/provisioning/datasources/datasources.yml:ro
    networks:
      - swoops-net

volumes:
  swoops-data:
  prometheus-data:
  grafana-data:

networks:
  swoops-net:
    driver: bridge
```

### Start Services

```bash
# Set environment variables
export SWOOPS_API_KEY=$(openssl rand -hex 32)
export GRAFANA_ADMIN_PASSWORD=$(openssl rand -hex 16)

# Start services
docker-compose -f docker-compose.prod.yml up -d

# View logs
docker-compose -f docker-compose.prod.yml logs -f swoopsd
```

## Kubernetes Deployment

### Namespace and Secrets

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: swoops

---
apiVersion: v1
kind: Secret
metadata:
  name: swoops-api-key
  namespace: swoops
type: Opaque
stringData:
  api-key: "your-strong-random-api-key-here"

---
apiVersion: v1
kind: Secret
metadata:
  name: swoops-tls
  namespace: swoops
type: kubernetes.io/tls
data:
  tls.crt: <base64-encoded-cert>
  tls.key: <base64-encoded-key>

---
apiVersion: v1
kind: Secret
metadata:
  name: swoops-grpc-tls
  namespace: swoops
type: Opaque
data:
  server.crt: <base64-encoded-cert>
  server.key: <base64-encoded-key>
  ca.crt: <base64-encoded-ca-cert>
```

### Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: swoopsd
  namespace: swoops
spec:
  replicas: 3
  selector:
    matchLabels:
      app: swoopsd
  template:
    metadata:
      labels:
        app: swoopsd
    spec:
      containers:
      - name: swoopsd
        image: swoops/control-plane:latest
        ports:
        - containerPort: 8080
          name: https
        - containerPort: 9090
          name: grpc
        env:
        - name: SWOOPS_API_KEY
          valueFrom:
            secretKeyRef:
              name: swoops-api-key
              key: api-key
        - name: SWOOPS_TLS_ENABLED
          value: "true"
        - name: SWOOPS_TLS_CERT
          value: "/certs/http/tls.crt"
        - name: SWOOPS_TLS_KEY
          value: "/certs/http/tls.key"
        - name: SWOOPS_GRPC_INSECURE
          value: "false"
        - name: SWOOPS_GRPC_TLS_CERT
          value: "/certs/grpc/server.crt"
        - name: SWOOPS_GRPC_TLS_KEY
          value: "/certs/grpc/server.key"
        - name: SWOOPS_GRPC_CLIENT_CA
          value: "/certs/grpc/ca.crt"
        - name: SWOOPS_GRPC_REQUIRE_MTLS
          value: "true"
        volumeMounts:
        - name: http-tls
          mountPath: /certs/http
          readOnly: true
        - name: grpc-tls
          mountPath: /certs/grpc
          readOnly: true
        - name: data
          mountPath: /var/lib/swoops
        livenessProbe:
          httpGet:
            path: /api/v1/health
            port: 8080
            scheme: HTTPS
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /api/v1/health
            port: 8080
            scheme: HTTPS
          initialDelaySeconds: 5
          periodSeconds: 10
      volumes:
      - name: http-tls
        secret:
          secretName: swoops-tls
      - name: grpc-tls
        secret:
          secretName: swoops-grpc-tls
      - name: data
        persistentVolumeClaim:
          claimName: swoops-data

---
apiVersion: v1
kind: Service
metadata:
  name: swoopsd
  namespace: swoops
spec:
  type: LoadBalancer
  ports:
  - port: 443
    targetPort: 8080
    name: https
  - port: 9090
    targetPort: 9090
    name: grpc
  selector:
    app: swoopsd

---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: swoops-data
  namespace: swoops
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
```

## Agent Deployment

### SystemD Service (Linux)

Create `/etc/systemd/system/swoops-agent.service`:

```ini
[Unit]
Description=Swoops Agent
After=network.target

[Service]
Type=simple
User=swoops
Group=swoops
WorkingDirectory=/opt/swoops
Environment="SWOOPS_AGENT_TOKEN=your-agent-token-here"
ExecStart=/usr/local/bin/swoops-agent run \
  --server swoops.example.com:9090 \
  --host-id your-host-id \
  --tls-cert /etc/swoops/certs/client-cert.pem \
  --tls-key /etc/swoops/certs/client-key.pem \
  --server-ca /etc/swoops/certs/server-ca.pem \
  --insecure=false
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable swoops-agent
sudo systemctl start swoops-agent
sudo systemctl status swoops-agent
```

### Launchd (macOS)

Create `~/Library/LaunchAgents/com.swoops.agent.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.swoops.agent</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/swoops-agent</string>
        <string>run</string>
        <string>--server</string>
        <string>swoops.example.com:9090</string>
        <string>--host-id</string>
        <string>your-host-id</string>
        <string>--tls-cert</string>
        <string>/etc/swoops/certs/client-cert.pem</string>
        <string>--tls-key</string>
        <string>/etc/swoops/certs/client-key.pem</string>
        <string>--server-ca</string>
        <string>/etc/swoops/certs/server-ca.pem</string>
        <string>--insecure=false</string>
    </array>
    <key>EnvironmentVariables</key>
    <dict>
        <key>SWOOPS_AGENT_TOKEN</key>
        <string>your-agent-token-here</string>
    </dict>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/var/log/swoops-agent.log</string>
    <key>StandardErrorPath</key>
    <string>/var/log/swoops-agent.error.log</string>
</dict>
</plist>
```

Load and start:

```bash
launchctl load ~/Library/LaunchAgents/com.swoops.agent.plist
launchctl start com.swoops.agent
```

## Monitoring

### Prometheus Configuration

`prometheus.yml`:

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s
  external_labels:
    cluster: 'swoops-prod'

scrape_configs:
  - job_name: 'swoops'
    scheme: https
    tls_config:
      insecure_skip_verify: false  # Set to true for self-signed certs
    static_configs:
      - targets: ['swoops.example.com:443']
    metrics_path: '/metrics'
    scrape_interval: 10s
```

### Grafana Data Source

`grafana-datasources.yml`:

```yaml
apiVersion: 1

datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://prometheus:9090
    isDefault: true
    editable: false
```

### Key Metrics to Monitor

- `swoops_http_requests_total` - Total HTTP requests (paths normalized to prevent unbounded cardinality)
- `swoops_http_request_duration_seconds` - HTTP request latency
- `swoops_agent_connections_active` - Active agent connections
- `swoops_agent_connections_total` - Total agent connection attempts
- `swoops_agent_connection_errors_total` - Agent connection errors
- `swoops_agent_heartbeats_received_total` - Heartbeats received
- `swoops_sessions_active` - Active sessions
- `swoops_sessions_total` - Total sessions created
- `swoops_hosts_total` - Total hosts by status
- `swoops_websocket_connections_active` - Active WebSocket connections

**Note:** Swoops automatically normalizes HTTP paths in metrics to prevent unbounded cardinality. Resource IDs are replaced with `:id` placeholders (e.g., `/api/v1/sessions/abc123` → `/api/v1/sessions/:id`). This ensures metrics cardinality remains bounded even with millions of resources.

### Alerting Rules

`prometheus-alerts.yml`:

```yaml
groups:
  - name: swoops
    interval: 30s
    rules:
      - alert: SwoopsDown
        expr: up{job="swoops"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Swoops control plane is down"

      - alert: HighErrorRate
        expr: rate(swoops_agent_connection_errors_total[5m]) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High agent connection error rate"

      - alert: NoAgentConnections
        expr: swoops_agent_connections_active == 0
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "No active agent connections"
```

## Security Checklist

### Before Going to Production

- [ ] **TLS Enabled**: HTTPS enabled for HTTP server
- [ ] **mTLS Configured**: Client certificates required for agent connections
- [ ] **Strong API Keys**: Use cryptographically secure random API keys (32+ bytes)
- [ ] **Certificate Validation**: Valid certificates from trusted CA
- [ ] **Firewall Rules**: Restrict access to ports 8080 (HTTPS) and 9090 (gRPC)
- [ ] **Database Encryption**: Enable database encryption at rest
- [ ] **Secrets Management**: Use secret management system (Vault, Kubernetes Secrets)
- [ ] **Monitoring**: Prometheus + Grafana configured
- [ ] **Logging**: Centralized logging configured (ELK, Loki, etc.)
- [ ] **Backups**: Automated database backups configured
- [ ] **Rate Limiting**: Configure rate limiting on reverse proxy
- [ ] **CORS**: Restrict allowed origins to known domains
- [ ] **Updates**: Establish process for security updates

### Hardening

1. **Run as non-root user**: Container and systemd service use non-privileged user
2. **Read-only filesystem**: Mount certificates and configs as read-only
3. **Network policies**: Restrict network access using firewall/Kubernetes NetworkPolicies
4. **Resource limits**: Set CPU and memory limits
5. **Security scanning**: Scan Docker images for vulnerabilities

## Troubleshooting

### Connection Issues

**Agent can't connect to control plane:**

```bash
# Test gRPC connectivity
grpcurl -insecure swoops.example.com:9090 list

# Check agent logs
journalctl -u swoops-agent -f

# Verify certificates
openssl s_client -connect swoops.example.com:9090 -cert client-cert.pem -key client-key.pem
```

**HTTP requests failing:**

```bash
# Test HTTPS endpoint
curl -v https://swoops.example.com/api/v1/health

# Check TLS certificate
openssl s_client -connect swoops.example.com:443 -servername swoops.example.com
```

### Performance Issues

```bash
# Check metrics for high latency
curl https://swoops.example.com/metrics | grep duration

# Check active connections
curl https://swoops.example.com/metrics | grep active

# Database size
du -h /var/lib/swoops/swoops.db
```

### Common Errors

**"invalid authentication token":**
- Verify `SWOOPS_AGENT_TOKEN` matches host's `AgentAuthToken`
- Check agent is using correct `--host-id`

**"x509: certificate signed by unknown authority":**
- Ensure `--server-ca` points to correct CA certificate
- Verify certificate chain is complete

**"connection refused":**
- Check firewall allows port 9090
- Verify gRPC server is listening: `netstat -tulpn | grep 9090`
- Check Docker port mappings

**Docker build fails with "module not found":**
- Ensure all workspace modules are copied in Dockerfile
- Both server and agent Dockerfiles must copy `pkg/`, `server/go.mod`, and `agent/go.mod`
- Go workspaces require all modules listed in `go.work` to be present

**WebSocket connections failing:**
- Verify metrics middleware preserves `http.Hijacker` interface (fixed in v0.3.0+)
- Check WebSocket endpoint uses correct auth token: `?token=YOUR_API_KEY`
- Test with: `wscat -c "ws://localhost:8080/api/v1/ws/sessions/{id}/output?token=YOUR_KEY"`

**High Prometheus memory usage:**
- Metrics cardinality is automatically bounded via path normalization
- If seeing unbounded growth, verify version includes path normalization fix (v0.3.0+)
- Check for custom metrics or labels with unbounded values

## Support

- Documentation: https://github.com/swoopsh/swoops
- Issues: https://github.com/swoopsh/swoops/issues
- Security: security@swoops.sh (for security vulnerabilities)
