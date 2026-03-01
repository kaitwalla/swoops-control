#!/bin/bash
set -e

# Certificate Generation Script for Swoops
# Generates self-signed certificates for testing/development
#
# For production, use:
# - Let's Encrypt (via Caddy or certbot) for HTTP
# - Proper CA-signed certificates for gRPC mTLS

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

info() {
    echo -e "${BLUE}ℹ ${NC}$1"
}

success() {
    echo -e "${GREEN}✓${NC} $1"
}

warn() {
    echo -e "${YELLOW}⚠${NC} $1"
}

# Configuration
CERT_DIR="${1:-./certs}"
DOMAIN="${2:-swoops.example.com}"
DAYS_VALID=365

info "Generating self-signed certificates for Swoops"
warn "These certificates are for TESTING/DEVELOPMENT only!"
warn "For production, use Let's Encrypt or a proper CA."
echo

# Create certificate directory
mkdir -p "$CERT_DIR"
info "Certificate directory: $CERT_DIR"
echo

# Check for openssl
if ! command -v openssl &> /dev/null; then
    echo "Error: openssl is required but not installed"
    exit 1
fi

# Step 1: Generate Certificate Authority (CA)
info "Step 1: Generating Certificate Authority (CA)..."

openssl req -x509 -newkey rsa:4096 -days $DAYS_VALID -nodes \
    -keyout "$CERT_DIR/ca-key.pem" \
    -out "$CERT_DIR/ca-cert.pem" \
    -subj "/CN=Swoops Test CA/O=Swoops/C=US" 2>/dev/null

success "CA certificate created"
echo "  Certificate: $CERT_DIR/ca-cert.pem"
echo "  Private key: $CERT_DIR/ca-key.pem"
echo

# Step 2: Generate gRPC server certificate
info "Step 2: Generating gRPC server certificate..."

# Create certificate request
openssl req -newkey rsa:4096 -nodes \
    -keyout "$CERT_DIR/grpc-server-key.pem" \
    -out "$CERT_DIR/grpc-server-req.pem" \
    -subj "/CN=$DOMAIN/O=Swoops/C=US" 2>/dev/null

# Create extension file for SAN (Subject Alternative Names)
cat > "$CERT_DIR/grpc-server-ext.cnf" <<EOF
subjectAltName = DNS:$DOMAIN,DNS:localhost,IP:127.0.0.1
extendedKeyUsage = serverAuth
EOF

# Sign the certificate with CA
openssl x509 -req -in "$CERT_DIR/grpc-server-req.pem" -days $DAYS_VALID \
    -CA "$CERT_DIR/ca-cert.pem" \
    -CAkey "$CERT_DIR/ca-key.pem" \
    -CAcreateserial \
    -out "$CERT_DIR/grpc-server-cert.pem" \
    -extfile "$CERT_DIR/grpc-server-ext.cnf" 2>/dev/null

success "gRPC server certificate created"
echo "  Certificate: $CERT_DIR/grpc-server-cert.pem"
echo "  Private key: $CERT_DIR/grpc-server-key.pem"
echo

# Step 3: Generate HTTP server certificate (for direct deployment)
info "Step 3: Generating HTTP server certificate..."

openssl req -newkey rsa:4096 -nodes \
    -keyout "$CERT_DIR/http-server-key.pem" \
    -out "$CERT_DIR/http-server-req.pem" \
    -subj "/CN=$DOMAIN/O=Swoops/C=US" 2>/dev/null

cat > "$CERT_DIR/http-server-ext.cnf" <<EOF
subjectAltName = DNS:$DOMAIN,DNS:localhost,IP:127.0.0.1
extendedKeyUsage = serverAuth
EOF

openssl x509 -req -in "$CERT_DIR/http-server-req.pem" -days $DAYS_VALID \
    -CA "$CERT_DIR/ca-cert.pem" \
    -CAkey "$CERT_DIR/ca-key.pem" \
    -CAcreateserial \
    -out "$CERT_DIR/http-server-cert.pem" \
    -extfile "$CERT_DIR/http-server-ext.cnf" 2>/dev/null

success "HTTP server certificate created"
echo "  Certificate: $CERT_DIR/http-server-cert.pem"
echo "  Private key: $CERT_DIR/http-server-key.pem"
echo

# Step 4: Generate agent client certificates (for mTLS)
info "Step 4: Generating agent client certificate (for mTLS)..."

openssl req -newkey rsa:4096 -nodes \
    -keyout "$CERT_DIR/agent-key.pem" \
    -out "$CERT_DIR/agent-req.pem" \
    -subj "/CN=swoops-agent/O=Swoops/C=US" 2>/dev/null

cat > "$CERT_DIR/agent-ext.cnf" <<EOF
extendedKeyUsage = clientAuth
EOF

openssl x509 -req -in "$CERT_DIR/agent-req.pem" -days $DAYS_VALID \
    -CA "$CERT_DIR/ca-cert.pem" \
    -CAkey "$CERT_DIR/ca-key.pem" \
    -CAcreateserial \
    -out "$CERT_DIR/agent-cert.pem" \
    -extfile "$CERT_DIR/agent-ext.cnf" 2>/dev/null

success "Agent client certificate created"
echo "  Certificate: $CERT_DIR/agent-cert.pem"
echo "  Private key: $CERT_DIR/agent-key.pem"
echo

# Step 5: Create CA copies for server/client verification
info "Step 5: Creating CA copies for verification..."

cp "$CERT_DIR/ca-cert.pem" "$CERT_DIR/client-ca.pem"
cp "$CERT_DIR/ca-cert.pem" "$CERT_DIR/server-ca.pem"

success "CA copies created"
echo "  Client CA: $CERT_DIR/client-ca.pem (for server to verify clients)"
echo "  Server CA: $CERT_DIR/server-ca.pem (for clients to verify server)"
echo

# Cleanup temporary files
rm -f "$CERT_DIR"/*.pem.srl \
      "$CERT_DIR"/*-req.pem \
      "$CERT_DIR"/*.cnf

# Set permissions
chmod 600 "$CERT_DIR"/*-key.pem
chmod 644 "$CERT_DIR"/*-cert.pem "$CERT_DIR"/*-ca.pem

success "Cleaned up temporary files and set permissions"
echo

# Summary
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}Certificate Generation Complete!${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo

echo -e "${BLUE}Generated certificates:${NC}"
echo
echo "📁 $CERT_DIR/"
echo "├── ca-cert.pem              (Certificate Authority)"
echo "├── ca-key.pem               (CA private key - keep secure!)"
echo "├── grpc-server-cert.pem     (gRPC server certificate)"
echo "├── grpc-server-key.pem      (gRPC server private key)"
echo "├── http-server-cert.pem     (HTTP server certificate)"
echo "├── http-server-key.pem      (HTTP server private key)"
echo "├── agent-cert.pem           (Agent client certificate)"
echo "├── agent-key.pem            (Agent client private key)"
echo "├── client-ca.pem            (CA for client verification)"
echo "└── server-ca.pem            (CA for server verification)"
echo

echo -e "${BLUE}Usage in Swoops configuration:${NC}"
echo
echo "Server (swoopsd.yaml):"
echo "  server:"
echo "    tls_enabled: true"
echo "    tls_cert: $CERT_DIR/http-server-cert.pem"
echo "    tls_key: $CERT_DIR/http-server-key.pem"
echo "  grpc:"
echo "    tls_cert: $CERT_DIR/grpc-server-cert.pem"
echo "    tls_key: $CERT_DIR/grpc-server-key.pem"
echo "    client_ca: $CERT_DIR/client-ca.pem"
echo "    insecure: false"
echo "    require_mtls: true"
echo

echo "Agent (swoops-agent run):"
echo "  --tls-cert $CERT_DIR/agent-cert.pem"
echo "  --tls-key $CERT_DIR/agent-key.pem"
echo "  --server-ca $CERT_DIR/server-ca.pem"
echo "  --insecure=false"
echo

warn "Remember: These are self-signed certificates for testing only!"
warn "For production, use Let's Encrypt or certificates from a trusted CA."
echo

success "All done! 🎉"
