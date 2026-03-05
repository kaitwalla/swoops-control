#!/bin/bash
set -e

# Swoops Interactive Setup Script
# This script guides you through configuring Swoops for production deployment
SETUP_SCRIPT_VERSION="1.2.2"

# Parse command line arguments for non-interactive agent setup
NON_INTERACTIVE=false
AGENT_SERVER=""
AGENT_DOWNLOAD_CA=false
AGENT_HTTP_URL=""
AGENT_HOST_ID=""
AGENT_AUTH_TOKEN_ARG=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --server)
            AGENT_SERVER="$2"
            NON_INTERACTIVE=true
            shift 2
            ;;
        --download-ca)
            AGENT_DOWNLOAD_CA=true
            shift
            ;;
        --http-url)
            AGENT_HTTP_URL="$2"
            shift 2
            ;;
        --host-id)
            AGENT_HOST_ID="$2"
            shift 2
            ;;
        --auth-token)
            AGENT_AUTH_TOKEN_ARG="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 [--server HOST:PORT] [--download-ca] [--http-url URL] [--host-id ID] [--auth-token TOKEN]"
            exit 1
            ;;
    esac
done

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
info() {
    echo -e "${BLUE}ℹ ${NC}$1"
}

success() {
    echo -e "${GREEN}✓${NC} $1"
}

warn() {
    echo -e "${YELLOW}⚠${NC} $1"
}

error() {
    echo -e "${RED}✗${NC} $1"
}

prompt() {
    local varname=$1
    local prompt_text=$2
    local default=$3

    if [ -n "$default" ]; then
        echo -ne "${BLUE}?${NC} $prompt_text [$default]: " >&2
        read -r value </dev/tty
        if [ -z "$value" ]; then
            value="$default"
        fi
        printf -v "$varname" '%s' "$value"
    else
        echo -ne "${BLUE}?${NC} $prompt_text: " >&2
        read -r value </dev/tty
        printf -v "$varname" '%s' "$value"
    fi
}

prompt_password() {
    local varname=$1
    local prompt_text=$2

    echo -ne "${BLUE}?${NC} $prompt_text: " >&2
    read -rs value </dev/tty
    echo >&2
    printf -v "$varname" '%s' "$value"
}

confirm() {
    local prompt_text=$1
    local default=${2:-n}
    local response

    if [ "$default" = "y" ]; then
        echo -ne "${BLUE}?${NC} $prompt_text [Y/n]: " >&2
        read -r response </dev/tty
        response=${response:-y}
    else
        echo -ne "${BLUE}?${NC} $prompt_text [y/N]: " >&2
        read -r response </dev/tty
        response=${response:-n}
    fi

    [[ "$response" =~ ^[Yy]$ ]]
}

generate_random_key() {
    openssl rand -hex 32 2>/dev/null || head -c 32 /dev/urandom | xxd -p -c 32
}

# Banner
echo -e "${GREEN}"
cat <<EOF
   ____
  / _____|_      _____   ___  _ __  ___
  \___ \ \ \ /\ / / _ \ / _ \| '_ \/ __|
   ___) | \ V  V / (_) | (_) | |_) \__ \
  |____/   \_/\_/ \___/ \___/| .__/|___/
                              |_|
  Interactive Setup Script
  Version $SETUP_SCRIPT_VERSION
EOF
echo -e "${NC}"

# Non-interactive agent setup mode
if [ "$NON_INTERACTIVE" = true ]; then
    info "Running non-interactive agent setup..."
    info "Server: $AGENT_SERVER"

    # Skip to agent installation
    INSTALL_SERVER=false
    INSTALL_AGENT=true

    # Parse server address
    CONTROL_PLANE_HOST="${AGENT_SERVER%:*}"
    CONTROL_PLANE_PORT="${AGENT_SERVER#*:}"

    # Set host ID to provided value or hostname
    if [ -n "$AGENT_HOST_ID" ]; then
        HOST_ID="$AGENT_HOST_ID"
    else
        HOST_ID=$(hostname)
    fi

    # Determine TLS settings based on flags
    if [ "$AGENT_DOWNLOAD_CA" = true ]; then
        AGENT_TLS_ENABLED=true
        AGENT_MTLS_ENABLED=false
        AGENT_CA_PATH="/etc/swoops/certs/server-ca.pem"
    else
        AGENT_TLS_ENABLED=false
        AGENT_MTLS_ENABLED=false
    fi

    # Use provided auth token or leave empty
    if [ -n "$AGENT_AUTH_TOKEN_ARG" ]; then
        AGENT_AUTH_TOKEN="$AGENT_AUTH_TOKEN_ARG"
    else
        AGENT_AUTH_TOKEN=""
    fi

    # Jump to binary installation (skip all the interactive setup)
    SKIP_INTERACTIVE=true
else
    SKIP_INTERACTIVE=false
    info "This script will help you configure Swoops for production deployment."
    echo
fi

# Detect OS early (needed for update path)
OS="unknown"
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    OS="linux"
    if command -v systemctl &> /dev/null; then
        INIT_SYSTEM="systemd"
    else
        INIT_SYSTEM="other"
    fi
elif [[ "$OSTYPE" == "darwin"* ]]; then
    OS="macos"
    INIT_SYSTEM="launchd"
fi

# Check if swoopsd is already installed (skip in non-interactive mode)
if [ "$SKIP_INTERACTIVE" = false ] && command -v swoopsd &> /dev/null; then
    SWOOPSD_PATH=$(which swoopsd)
    SWOOPSD_VERSION=$(swoopsd --version 2>/dev/null | head -1 || echo "unknown")

    echo -e "${YELLOW}⚠${NC} Swoops is already installed at: $SWOOPSD_PATH"
    echo "  Current version: $SWOOPSD_VERSION"
    echo

    if confirm "Would you like to update to the latest version instead of running full setup?" "y"; then
        info "Updating swoopsd..."

        # Download latest release from GitHub
        GITHUB_REPO="kaitwalla/swoops-control"

        # Detect architecture
        ARCH=$(uname -m)
        if [ "$ARCH" = "x86_64" ]; then
            BINARY_ARCH="amd64"
        elif [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then
            BINARY_ARCH="arm64"
        else
            error "Unsupported architecture: $ARCH"
            exit 1
        fi

        # Detect OS
        if [ "$OS" = "linux" ]; then
            BINARY_OS="linux"
        elif [ "$OS" = "macos" ]; then
            BINARY_OS="darwin"
        else
            error "Unsupported OS: $OS"
            exit 1
        fi

        # Get latest release info
        info "Fetching latest release information..."
        LATEST_RELEASE=$(curl -s https://api.github.com/repos/$GITHUB_REPO/releases/latest)
        LATEST_VERSION=$(echo "$LATEST_RELEASE" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

        if [ -z "$LATEST_VERSION" ]; then
            error "Could not fetch latest release information"
            exit 1
        fi

        info "Latest version: $LATEST_VERSION"

        # Download binary
        BINARY_NAME="swoopsd-${BINARY_OS}-${BINARY_ARCH}"
        DOWNLOAD_URL="https://github.com/$GITHUB_REPO/releases/download/$LATEST_VERSION/$BINARY_NAME"

        info "Downloading from $DOWNLOAD_URL..."
        TEMP_BINARY=$(mktemp)

        if command -v curl &> /dev/null; then
            if ! curl -fsSL "$DOWNLOAD_URL" -o "$TEMP_BINARY"; then
                error "Failed to download binary from $DOWNLOAD_URL"
                rm -f "$TEMP_BINARY"
                exit 1
            fi
        elif command -v wget &> /dev/null; then
            if ! wget -q "$DOWNLOAD_URL" -O "$TEMP_BINARY"; then
                error "Failed to download binary from $DOWNLOAD_URL"
                rm -f "$TEMP_BINARY"
                exit 1
            fi
        else
            error "Neither curl nor wget found. Please install one of them first."
            exit 1
        fi

        # Make binary executable
        chmod +x "$TEMP_BINARY"

        # Replace existing binary
        info "Installing new binary to $SWOOPSD_PATH..."
        sudo mv "$TEMP_BINARY" "$SWOOPSD_PATH"

        # On Linux, grant CAP_NET_BIND_SERVICE for autocert (port 443/80)
        if [ "$OS" = "linux" ]; then
            if command -v setcap &> /dev/null; then
                info "Granting CAP_NET_BIND_SERVICE capability for port 443/80 binding..."
                if sudo setcap 'cap_net_bind_service=+ep' "$SWOOPSD_PATH"; then
                    success "✓ CAP_NET_BIND_SERVICE capability granted"
                else
                    warn "Failed to grant CAP_NET_BIND_SERVICE capability"
                    warn "If using autocert, run: sudo setcap 'cap_net_bind_service=+ep' $SWOOPSD_PATH"
                fi
            else
                warn "setcap command not found - cannot grant CAP_NET_BIND_SERVICE capability"
                warn "Install libcap2-bin (Debian/Ubuntu) or libcap (RHEL/Fedora), then run:"
                warn "  sudo setcap 'cap_net_bind_service=+ep' $SWOOPSD_PATH"
            fi
        fi

        success "Update complete! Updated to $LATEST_VERSION"
        echo
        info "To apply the update, restart swoopsd:"
        echo "  sudo systemctl restart swoopsd"
        exit 0
    fi

    echo
    info "Continuing with full setup..."
    echo

    # When continuing with full setup, mark that binaries need to be downloaded
    FORCE_DOWNLOAD_BINARIES=true
fi

if [ "$SKIP_INTERACTIVE" = false ]; then
    info "Detected OS: $OS ($INIT_SYSTEM)"
    echo

    # Step 1: Choose what to install
    echo -e "${GREEN}Step 1: Choose Components${NC}"
    echo "  1) Control Plane (server)"
    echo "  2) Agent"
    echo "  3) Both (server + agent on same machine)"
    prompt INSTALL_TYPE "What would you like to install? [1-3]" "1"

    INSTALL_SERVER=false
    INSTALL_AGENT=false

    case $INSTALL_TYPE in
        1) INSTALL_SERVER=true ;;
        2) INSTALL_AGENT=true ;;
        3) INSTALL_SERVER=true; INSTALL_AGENT=true ;;
        *) error "Invalid choice"; exit 1 ;;
    esac
    echo
fi

# Step 2: Deployment type (if installing server)
if [ "$SKIP_INTERACTIVE" = false ] && [ "$INSTALL_SERVER" = true ]; then
    echo -e "${GREEN}Step 2: Deployment Type${NC}"
    echo "  1) Automatic HTTPS (built-in Let's Encrypt) - Easiest!"
    echo "  2) Production with reverse proxy (Caddy/nginx)"
    echo "  3) Direct deployment with manual TLS"
    echo "  4) Development (HTTP only, no TLS)"
    prompt DEPLOY_TYPE "Deployment type [1-4]" "1"
    echo

    case $DEPLOY_TYPE in
        1)
            USE_AUTOCERT=true
            USE_REVERSE_PROXY=false
            USE_TLS=false
            ;;
        2)
            USE_AUTOCERT=false
            USE_REVERSE_PROXY=true
            USE_TLS=false  # Reverse proxy handles TLS
            ;;
        3)
            USE_AUTOCERT=false
            USE_REVERSE_PROXY=false
            USE_TLS=true
            ;;
        4)
            USE_AUTOCERT=false
            USE_REVERSE_PROXY=false
            USE_TLS=false
            warn "Development mode - not suitable for production!"
            ;;
        *) error "Invalid choice"; exit 1 ;;
    esac
fi

# Step 3: Server configuration
if [ "$SKIP_INTERACTIVE" = false ] && [ "$INSTALL_SERVER" = true ]; then
    echo -e "${GREEN}Step 3: Server Configuration${NC}"

    prompt DOMAIN "Domain name" "swoops.example.com"

    if [ "$USE_AUTOCERT" = true ]; then
        prompt HTTP_HOST "HTTP server bind address" "0.0.0.0"
        prompt HTTP_PORT "HTTPS server port" "443"
        prompt AUTOCERT_EMAIL "Email for Let's Encrypt notifications (optional)" ""
        EXTERNAL_URL="https://$DOMAIN"
        info "Autocert will also listen on port 80 for ACME challenges"
    elif [ "$USE_REVERSE_PROXY" = true ]; then
        prompt HTTP_HOST "HTTP server bind address (internal)" "127.0.0.1"
        prompt HTTP_PORT "HTTP server port (internal)" "8080"
        EXTERNAL_URL="https://$DOMAIN"
    else
        prompt HTTP_HOST "HTTP server bind address" "0.0.0.0"
        if [ "$USE_TLS" = true ]; then
            prompt HTTP_PORT "HTTPS server port" "443"
            EXTERNAL_URL="https://$DOMAIN:$HTTP_PORT"
        else
            prompt HTTP_PORT "HTTP server port" "8080"
            EXTERNAL_URL="http://$DOMAIN:$HTTP_PORT"
        fi
    fi

    prompt GRPC_HOST "gRPC server bind address" "0.0.0.0"
    prompt GRPC_PORT "gRPC server port" "9090"

    prompt DB_PATH "Database path" "/var/lib/swoops/swoops.db"

    # API Key
    if confirm "Generate random API key?" "y"; then
        API_KEY=$(generate_random_key)
        success "Generated API key: $API_KEY"
    else
        prompt_password API_KEY "Enter API key (32+ characters recommended)"
    fi

    # gRPC TLS
    echo
    if confirm "Enable TLS for gRPC connections?" "y"; then
        GRPC_TLS_ENABLED=true

        if confirm "Enable mTLS (client certificate authentication)?" "y"; then
            GRPC_MTLS_ENABLED=true
        else
            GRPC_MTLS_ENABLED=false
        fi

        # Set default certificate paths (will be updated later if certificates are generated)
        GRPC_CERT_PATH="/etc/swoops/certs/grpc-server-cert.pem"
        GRPC_KEY_PATH="/etc/swoops/certs/grpc-server-key.pem"
        GRPC_CLIENT_CA_PATH="/etc/swoops/certs/client-ca.pem"
    else
        GRPC_TLS_ENABLED=false
        GRPC_MTLS_ENABLED=false
        warn "gRPC will run without TLS - only use for development!"
    fi

    # HTTP TLS (direct deployment only)
    if [ "$USE_TLS" = true ]; then
        # Set default HTTP cert paths (for reverse proxy setups these won't be used)
        HTTP_CERT_PATH="/etc/letsencrypt/live/$DOMAIN/fullchain.pem"
        HTTP_KEY_PATH="/etc/letsencrypt/live/$DOMAIN/privkey.pem"
    fi

    echo
fi

# Step 4: Agent configuration
if [ "$SKIP_INTERACTIVE" = false ] && [ "$INSTALL_AGENT" = true ]; then
    echo -e "${GREEN}Step 4: Agent Configuration${NC}"

    if [ "$INSTALL_SERVER" = true ]; then
        # Installing on same machine as server
        CONTROL_PLANE_HOST="127.0.0.1"
        CONTROL_PLANE_PORT="$GRPC_PORT"
    else
        prompt CONTROL_PLANE_HOST "Control plane hostname/IP"
        prompt CONTROL_PLANE_PORT "Control plane gRPC port" "9090"
    fi

    prompt HOST_ID "Host ID (unique identifier for this machine)" "$(hostname)"

    if [ "$INSTALL_SERVER" != true ]; then
        # Need to get auth token from user
        warn "You'll need to register this host with the control plane first."
        echo "After registration, you'll receive an auth token."
        prompt_password AGENT_AUTH_TOKEN "Agent authentication token"
    else
        # Will be generated by server
        AGENT_AUTH_TOKEN="<will-be-generated>"
    fi

    # Agent TLS configuration
    echo
    if confirm "Use TLS for agent connection?" "y"; then
        AGENT_TLS_ENABLED=true

        if confirm "Use client certificate (mTLS)?" "y"; then
            AGENT_MTLS_ENABLED=true
        else
            AGENT_MTLS_ENABLED=false
        fi

        # Set default certificate paths (will be updated if certificates are generated)
        AGENT_CERT_PATH="/etc/swoops/certs/agent-cert.pem"
        AGENT_KEY_PATH="/etc/swoops/certs/agent-key.pem"
        AGENT_CA_PATH="/etc/swoops/certs/server-ca.pem"
    else
        AGENT_TLS_ENABLED=false
        AGENT_MTLS_ENABLED=false
        warn "Agent will connect without TLS - only use for development!"
    fi

    echo
fi

# Step 5: Generate certificates
if [ "$SKIP_INTERACTIVE" = false ] && ([ "$GRPC_TLS_ENABLED" = true ] || [ "$USE_TLS" = true ] || [ "$AGENT_TLS_ENABLED" = true ]); then
    echo -e "${GREEN}Step 5: Certificate Generation${NC}"
    echo
    echo "Choose certificate generation method:"
    echo "  1) step-ca (Recommended for production - automated CA with renewal)"
    echo "  2) Self-signed with OpenSSL (Simple, for testing only)"
    echo "  3) Use existing certificates (I'll provide paths)"
    echo
    echo -ne "${BLUE}?${NC} Certificate method [1-3] [1]: " >&2
    read -r CERT_METHOD </dev/tty
    CERT_METHOD=${CERT_METHOD:-1}

    if [ "$CERT_METHOD" = "1" ]; then
        # step-ca method
        info "Setting up certificates with step-ca..."

        # Check if step CLI is installed
        if ! command -v step &> /dev/null; then
            warn "step CLI not found. Installing step..."

            # Detect architecture
            ARCH=$(uname -m)
            if [ "$ARCH" = "x86_64" ]; then
                STEP_ARCH="amd64"
            elif [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then
                STEP_ARCH="arm64"
            else
                error "Unsupported architecture: $ARCH"
                exit 1
            fi

            # Install step CLI based on OS
            STEP_VERSION="0.29.0"

            if [ "$OS" = "macos" ]; then
                # macOS installation
                if command -v brew &> /dev/null; then
                    info "Installing step via Homebrew..."
                    brew install step || {
                        error "Failed to install step via Homebrew"
                        exit 1
                    }
                else
                    error "Homebrew not found. Please install Homebrew first or install step manually from https://smallstep.com/docs/step-cli/installation"
                    exit 1
                fi
            elif [ "$OS" = "linux" ]; then
                # Linux installation
                if command -v apt-get &> /dev/null; then
                    # Debian/Ubuntu
                    info "Downloading step CLI for Debian/Ubuntu..."
                    STEP_URL="https://dl.smallstep.com/gh-release/cli/gh-release-header/v${STEP_VERSION}/step-cli_${STEP_VERSION}-1_${STEP_ARCH}.deb"

                    if command -v curl &> /dev/null; then
                        if ! curl -fsSL "$STEP_URL" -o /tmp/step-cli.deb; then
                            error "Failed to download step CLI from $STEP_URL"
                            exit 1
                        fi
                    elif command -v wget &> /dev/null; then
                        if ! wget --max-redirect=5 -q "$STEP_URL" -O /tmp/step-cli.deb; then
                            error "Failed to download step CLI from $STEP_URL"
                            exit 1
                        fi
                    else
                        error "Neither curl nor wget found. Please install one of them first."
                        exit 1
                    fi

                    info "Installing step CLI..."
                    if ! sudo dpkg -i /tmp/step-cli.deb; then
                        error "Failed to install step CLI"
                        rm -f /tmp/step-cli.deb
                        exit 1
                    fi
                    rm -f /tmp/step-cli.deb
                elif command -v yum &> /dev/null || command -v dnf &> /dev/null; then
                    # RHEL/CentOS/Fedora
                    PKG_MGR="yum"
                    command -v dnf &> /dev/null && PKG_MGR="dnf"

                    info "Downloading step CLI for RHEL/CentOS/Fedora..."
                    STEP_URL="https://dl.smallstep.com/gh-release/cli/gh-release-header/v${STEP_VERSION}/step-cli_${STEP_VERSION}-1_${STEP_ARCH}.rpm"

                    if command -v curl &> /dev/null; then
                        if ! curl -fsSL "$STEP_URL" -o /tmp/step-cli.rpm; then
                            error "Failed to download step CLI from $STEP_URL"
                            exit 1
                        fi
                    elif command -v wget &> /dev/null; then
                        if ! wget --max-redirect=5 -q "$STEP_URL" -O /tmp/step-cli.rpm; then
                            error "Failed to download step CLI from $STEP_URL"
                            exit 1
                        fi
                    else
                        error "Neither curl nor wget found. Please install one of them first."
                        exit 1
                    fi

                    info "Installing step CLI..."
                    if ! sudo $PKG_MGR install -y /tmp/step-cli.rpm; then
                        error "Failed to install step CLI"
                        rm -f /tmp/step-cli.rpm
                        exit 1
                    fi
                    rm -f /tmp/step-cli.rpm
                else
                    error "Unsupported Linux distribution. Please install step manually from https://smallstep.com/docs/step-cli/installation"
                    exit 1
                fi
            else
                error "Unsupported OS: $OS"
                exit 1
            fi

            # Verify installation
            if ! command -v step &> /dev/null; then
                error "step CLI installation failed - command not found after install"
                exit 1
            fi

            success "step CLI installed successfully (version $(step version | head -1))"
        fi

        CERT_DIR="/etc/swoops/certs"
        sudo mkdir -p "$CERT_DIR"

        # Clean up any existing step-ca processes and partial installations
        STEP_CA_DIR="/etc/swoops/step-ca"

        # Stop any running step-ca processes
        if pgrep -f "step-ca" > /dev/null; then
            warn "Found running step-ca process. Stopping it..."
            sudo pkill -f "step-ca" || true
            sleep 1
            success "Stopped existing step-ca processes"
        fi

        # Check for partial/incomplete installations
        if [ -d "$STEP_CA_DIR" ] && [ ! -f "$STEP_CA_DIR/config/ca.json" ]; then
            warn "Found incomplete step-ca installation at $STEP_CA_DIR"
            if confirm "Remove incomplete installation and start fresh?" "y"; then
                sudo rm -rf "$STEP_CA_DIR"
                success "Removed incomplete installation"
            else
                error "Cannot proceed with incomplete installation. Exiting."
                exit 1
            fi
        fi

        # Initialize a local step CA if not already done
        if [ ! -d "$STEP_CA_DIR" ]; then
            info "Initializing step-ca in $STEP_CA_DIR..."

            # Generate a random password for the CA
            CA_PASSWORD=$(generate_random_key)

            # Create temporary password file for initialization
            TEMP_PASS_FILE=$(mktemp)
            echo "$CA_PASSWORD" > "$TEMP_PASS_FILE"
            chmod 600 "$TEMP_PASS_FILE"

            # Initialize CA non-interactively
            sudo STEPPATH="$STEP_CA_DIR" step ca init \
                --name="Swoops Internal CA" \
                --dns="localhost" \
                --address=":9000" \
                --provisioner="admin" \
                --password-file="$TEMP_PASS_FILE" \
                --deployment-type=standalone \
                --acme

            # Store the password securely in the CA directory
            echo "$CA_PASSWORD" | sudo tee "$STEP_CA_DIR/.ca-password" > /dev/null
            sudo chmod 600 "$STEP_CA_DIR/.ca-password"

            # Clean up temporary password file
            rm -f "$TEMP_PASS_FILE"

            success "step-ca initialized"

            # Start step-ca as a background service
            info "Starting step-ca server..."
            sudo STEPPATH="$STEP_CA_DIR" step-ca "$STEP_CA_DIR/config/ca.json" \
                --password-file="$STEP_CA_DIR/.ca-password" > /dev/null 2>&1 &

            # Wait for CA to start
            sleep 2
            success "step-ca server started"
        else
            info "Using existing step-ca at $STEP_CA_DIR"
            # Make sure it's running
            if ! pgrep -f "step-ca" > /dev/null; then
                info "Starting step-ca server..."
                sudo STEPPATH="$STEP_CA_DIR" step-ca "$STEP_CA_DIR/config/ca.json" \
                    --password-file="$STEP_CA_DIR/.ca-password" > /dev/null 2>&1 &
                sleep 2
            fi
        fi

        # Set STEPPATH for certificate operations
        export STEPPATH="$STEP_CA_DIR"

        # Ensure cert directory exists and is writable
        sudo mkdir -p "$CERT_DIR"
        sudo chmod 755 "$CERT_DIR"

        # Generate gRPC server certificate
        info "Generating gRPC server certificate..."
        info "Domain: $DOMAIN"
        # Use temporary location first, then move with sudo
        TEMP_CERT_DIR=$(mktemp -d)

        # Debug: show the actual command and step version
        info "Step CLI version: $(step version 2>&1 | head -1)"

        # Wait for step-ca to be fully ready
        info "Waiting for step-ca to be ready..."
        for i in {1..10}; do
            if curl -k https://localhost:9000/health 2>/dev/null | grep -q "ok"; then
                success "step-ca is ready"
                break
            fi
            sleep 2
        done

        # Use step certificate create instead of step ca certificate
        # This creates certificates directly signed by our CA without needing the online CA
        info "Generating gRPC server certificate using step certificate..."

        sudo step certificate create \
            "$DOMAIN" \
            "$TEMP_CERT_DIR/grpc-server-cert.pem" \
            "$TEMP_CERT_DIR/grpc-server-key.pem" \
            --profile leaf \
            --not-after 8760h \
            --ca "$STEP_CA_DIR/certs/root_ca.crt" \
            --ca-key "$STEP_CA_DIR/secrets/root_ca_key" \
            --ca-password-file "$STEP_CA_DIR/.ca-password" \
            --no-password \
            --insecure \
            --san "$DOMAIN" \
            --san localhost \
            --san 127.0.0.1

        # Fix ownership of generated files
        sudo chown $(whoami):$(whoami) "$TEMP_CERT_DIR/grpc-server-cert.pem" "$TEMP_CERT_DIR/grpc-server-key.pem"

        # Move certificates to final location
        sudo mv "$TEMP_CERT_DIR/grpc-server-cert.pem" "$CERT_DIR/grpc-server-cert.pem"
        sudo mv "$TEMP_CERT_DIR/grpc-server-key.pem" "$CERT_DIR/grpc-server-key.pem"
        rmdir "$TEMP_CERT_DIR"

        # Generate agent client certificates if mTLS enabled
        if [ "$GRPC_MTLS_ENABLED" = true ] || [ "$AGENT_MTLS_ENABLED" = true ]; then
            info "Generating agent client certificate..."
            TEMP_CERT_DIR=$(mktemp -d)

            sudo step certificate create \
                "swoops-agent" \
                "$TEMP_CERT_DIR/agent-cert.pem" \
                "$TEMP_CERT_DIR/agent-key.pem" \
                --profile leaf \
                --not-after 8760h \
                --ca "$STEP_CA_DIR/certs/root_ca.crt" \
                --ca-key "$STEP_CA_DIR/secrets/root_ca_key" \
                --ca-password-file "$STEP_CA_DIR/.ca-password" \
                --no-password \
                --insecure

            # Fix ownership of generated files
            sudo chown $(whoami):$(whoami) "$TEMP_CERT_DIR/agent-cert.pem" "$TEMP_CERT_DIR/agent-key.pem"

            # Move certificates to final location
            sudo mv "$TEMP_CERT_DIR/agent-cert.pem" "$CERT_DIR/agent-cert.pem"
            sudo mv "$TEMP_CERT_DIR/agent-key.pem" "$CERT_DIR/agent-key.pem"
            rmdir "$TEMP_CERT_DIR"
        fi

        # Copy root CA certificate
        sudo cp "$STEP_CA_DIR/certs/root_ca.crt" "$CERT_DIR/ca-cert.pem"
        sudo cp "$STEP_CA_DIR/certs/root_ca.crt" "$CERT_DIR/client-ca.pem"
        sudo cp "$STEP_CA_DIR/certs/root_ca.crt" "$CERT_DIR/server-ca.pem"

        # Create swoops user if it doesn't exist (needed for certificate permissions)
        if ! id swoops &>/dev/null; then
            sudo useradd -r -s /bin/false swoops || true
        fi

        # Set proper permissions - readable by swoops user
        sudo chown -R swoops:swoops "$CERT_DIR"
        sudo chmod 755 "$CERT_DIR"
        sudo chmod 644 "$CERT_DIR"/*.pem
        sudo chmod 600 "$CERT_DIR"/*-key.pem

        # Also set permissions on step-ca directory for potential renewal operations
        sudo chown -R swoops:swoops "$STEP_CA_DIR"

        success "Certificates generated with step-ca"
        info "CA root certificate: $CERT_DIR/ca-cert.pem"
        info "Server certificate: $CERT_DIR/grpc-server-cert.pem"
        if [ "$GRPC_MTLS_ENABLED" = true ] || [ "$AGENT_MTLS_ENABLED" = true ]; then
            info "Agent certificate: $CERT_DIR/agent-cert.pem"
        fi

        echo
        warn "IMPORTANT: For remote agents, you have two options:"
        echo "  Option 1 (Recommended): Use --download-ca flag to fetch automatically:"
        echo "    swoops-agent run --server $GRPC_HOST:$GRPC_PORT --host-id <id> \\"
        echo "      --download-ca --insecure=false --http-url http://$HTTP_HOST:$HTTP_PORT"
        echo
        echo "  Option 2: Manually copy certificates to each agent machine:"
        echo "    scp $CERT_DIR/server-ca.pem user@agent:/etc/swoops/certs/"
        if [ "$GRPC_MTLS_ENABLED" = true ] || [ "$AGENT_MTLS_ENABLED" = true ]; then
            echo "    scp $CERT_DIR/agent-cert.pem user@agent:/etc/swoops/certs/"
            echo "    scp $CERT_DIR/agent-key.pem user@agent:/etc/swoops/certs/"
        fi
        echo

        # Update paths to use generated certificates
        GRPC_CERT_PATH="$CERT_DIR/grpc-server-cert.pem"
        GRPC_KEY_PATH="$CERT_DIR/grpc-server-key.pem"
        GRPC_CLIENT_CA_PATH="$CERT_DIR/client-ca.pem"
        AGENT_CERT_PATH="$CERT_DIR/agent-cert.pem"
        AGENT_KEY_PATH="$CERT_DIR/agent-key.pem"
        AGENT_CA_PATH="$CERT_DIR/server-ca.pem"

    elif [ "$CERT_METHOD" = "2" ]; then
        # Self-signed OpenSSL method (existing code)
        CERT_DIR="./certs"
        mkdir -p "$CERT_DIR"

        info "Generating self-signed certificates in $CERT_DIR..."

        # Generate CA
        openssl req -x509 -newkey rsa:4096 -days 365 -nodes \
            -keyout "$CERT_DIR/ca-key.pem" \
            -out "$CERT_DIR/ca-cert.pem" \
            -subj "/CN=Swoops CA" 2>/dev/null

        # Generate server certificate with SAN
        cat > "$CERT_DIR/server-ext.cnf" <<EOF
subjectAltName = DNS:$DOMAIN,DNS:localhost,IP:127.0.0.1
EOF

        openssl req -newkey rsa:4096 -nodes \
            -keyout "$CERT_DIR/grpc-server-key.pem" \
            -out "$CERT_DIR/grpc-server-req.pem" \
            -subj "/CN=$DOMAIN" 2>/dev/null

        openssl x509 -req -in "$CERT_DIR/grpc-server-req.pem" -days 365 \
            -CA "$CERT_DIR/ca-cert.pem" \
            -CAkey "$CERT_DIR/ca-key.pem" \
            -CAcreateserial \
            -out "$CERT_DIR/grpc-server-cert.pem" \
            -extfile "$CERT_DIR/server-ext.cnf" 2>/dev/null

        # Generate client certificate (for mTLS)
        if [ "$GRPC_MTLS_ENABLED" = true ] || [ "$AGENT_MTLS_ENABLED" = true ]; then
            openssl req -newkey rsa:4096 -nodes \
                -keyout "$CERT_DIR/agent-key.pem" \
                -out "$CERT_DIR/agent-req.pem" \
                -subj "/CN=swoops-agent" 2>/dev/null

            openssl x509 -req -in "$CERT_DIR/agent-req.pem" -days 365 \
                -CA "$CERT_DIR/ca-cert.pem" \
                -CAkey "$CERT_DIR/ca-key.pem" \
                -CAcreateserial \
                -out "$CERT_DIR/agent-cert.pem" 2>/dev/null
        fi

        # Copy CA as client-ca for server
        cp "$CERT_DIR/ca-cert.pem" "$CERT_DIR/client-ca.pem"

        # Copy CA as server-ca for agent
        cp "$CERT_DIR/ca-cert.pem" "$CERT_DIR/server-ca.pem"

        rm -f "$CERT_DIR"/*.pem.srl "$CERT_DIR"/*-req.pem "$CERT_DIR/server-ext.cnf"

        # Set proper permissions for self-signed certs
        chmod 644 "$CERT_DIR"/*.pem
        chmod 600 "$CERT_DIR"/*-key.pem

        success "Certificates generated in $CERT_DIR/"

        # Update paths to use generated certificates
        GRPC_CERT_PATH="$CERT_DIR/grpc-server-cert.pem"
        GRPC_KEY_PATH="$CERT_DIR/grpc-server-key.pem"
        GRPC_CLIENT_CA_PATH="$CERT_DIR/client-ca.pem"
        AGENT_CERT_PATH="$CERT_DIR/agent-cert.pem"
        AGENT_KEY_PATH="$CERT_DIR/agent-key.pem"
        AGENT_CA_PATH="$CERT_DIR/server-ca.pem"

        echo
    else
        # Option 3: Using existing certificates - ask for paths
        info "Please provide paths to your existing certificates."
        echo

        if [ "$GRPC_TLS_ENABLED" = true ]; then
            echo -e "${BLUE}gRPC Server Certificates:${NC}"
            prompt GRPC_CERT_PATH "gRPC server certificate path" "$GRPC_CERT_PATH"
            prompt GRPC_KEY_PATH "gRPC server key path" "$GRPC_KEY_PATH"
            if [ "$GRPC_MTLS_ENABLED" = true ]; then
                prompt GRPC_CLIENT_CA_PATH "Client CA certificate path" "$GRPC_CLIENT_CA_PATH"
            fi
            echo
        fi

        if [ "$USE_TLS" = true ]; then
            echo -e "${BLUE}HTTP Server Certificates:${NC}"
            prompt HTTP_CERT_PATH "HTTP server certificate path" "$HTTP_CERT_PATH"
            prompt HTTP_KEY_PATH "HTTP server key path" "$HTTP_KEY_PATH"
            echo
        fi

        if [ "$AGENT_TLS_ENABLED" = true ]; then
            echo -e "${BLUE}Agent Certificates:${NC}"
            prompt AGENT_CA_PATH "Server CA certificate path (for agent)" "$AGENT_CA_PATH"
            if [ "$AGENT_MTLS_ENABLED" = true ]; then
                prompt AGENT_CERT_PATH "Agent client certificate path" "$AGENT_CERT_PATH"
                prompt AGENT_KEY_PATH "Agent client key path" "$AGENT_KEY_PATH"
            fi
            echo
        fi

        if [ "$USE_REVERSE_PROXY" = true ]; then
            info "Note: Caddy/nginx will handle HTTP certificates automatically via Let's Encrypt."
        fi
    fi
fi

# Step 6: Write configuration files
if [ "$SKIP_INTERACTIVE" = false ]; then
    echo -e "${GREEN}Step 6: Writing Configuration${NC}"

    CONFIG_DIR="."
    if confirm "Create config in /etc/swoops?" "n"; then
        CONFIG_DIR="/etc/swoops"
        sudo mkdir -p "$CONFIG_DIR"
    else
        prompt CONFIG_DIR "Configuration directory" "."
        mkdir -p "$CONFIG_DIR"
    fi
else
    # Non-interactive mode: use /etc/swoops
    CONFIG_DIR="/etc/swoops"
    sudo mkdir -p "$CONFIG_DIR"
fi

# Server config
if [ "$INSTALL_SERVER" = true ]; then
    SERVER_CONFIG="$CONFIG_DIR/swoopsd.yaml"

    cat > "$SERVER_CONFIG" <<EOF
# Swoops Control Plane Configuration
# Generated by setup.sh on $(date)

server:
  host: $HTTP_HOST
  port: $HTTP_PORT
  external_url: $EXTERNAL_URL
  allowed_origins:
    - $EXTERNAL_URL
EOF

    if [ "$USE_AUTOCERT" = true ]; then
        cat >> "$SERVER_CONFIG" <<EOF
  tls_enabled: false
  autocert_enabled: true
  autocert_domain: $DOMAIN
EOF
        if [ -n "$AUTOCERT_EMAIL" ]; then
            cat >> "$SERVER_CONFIG" <<EOF
  autocert_email: $AUTOCERT_EMAIL
EOF
        fi
    elif [ "$USE_TLS" = true ]; then
        cat >> "$SERVER_CONFIG" <<EOF
  tls_enabled: true
  tls_cert: $HTTP_CERT_PATH
  tls_key: $HTTP_KEY_PATH
  autocert_enabled: false
EOF
    else
        cat >> "$SERVER_CONFIG" <<EOF
  tls_enabled: false
  autocert_enabled: false
EOF
    fi

    cat >> "$SERVER_CONFIG" <<EOF

database:
  path: $DB_PATH

grpc:
  host: $GRPC_HOST
  port: $GRPC_PORT
EOF

    if [ "$GRPC_TLS_ENABLED" = true ]; then
        cat >> "$SERVER_CONFIG" <<EOF
  insecure: false
  tls_cert: $GRPC_CERT_PATH
  tls_key: $GRPC_KEY_PATH
EOF
        if [ "$GRPC_MTLS_ENABLED" = true ]; then
            cat >> "$SERVER_CONFIG" <<EOF
  require_mtls: true
  client_ca: $GRPC_CLIENT_CA_PATH
EOF
        else
            cat >> "$SERVER_CONFIG" <<EOF
  require_mtls: false
EOF
        fi
    else
        cat >> "$SERVER_CONFIG" <<EOF
  insecure: true
  require_mtls: false
EOF
    fi

    cat >> "$SERVER_CONFIG" <<EOF

auth:
  api_key: $API_KEY
EOF

    success "Server configuration written to: $SERVER_CONFIG"

    # Create environment file for systemd
    if [ "$INIT_SYSTEM" = "systemd" ]; then
        ENV_FILE="$CONFIG_DIR/swoopsd.env"
        cat > "$ENV_FILE" <<EOF
# Swoops environment variables
SWOOPS_API_KEY=$API_KEY
SWOOPS_DB_PATH=$DB_PATH
EOF
        chmod 600 "$ENV_FILE"
        success "Environment file written to: $ENV_FILE"
    fi
fi

# Agent config (write to shell script for service installation)
if [ "$INSTALL_AGENT" = true ]; then
    AGENT_CONFIG="$CONFIG_DIR/agent.env"
    TEMP_AGENT_CONFIG=$(mktemp)

    cat > "$TEMP_AGENT_CONFIG" <<EOF
# Swoops Agent Configuration
# Generated by setup.sh on $(date)

# Connection settings
SWOOPS_SERVER=$CONTROL_PLANE_HOST:$CONTROL_PLANE_PORT
SWOOPS_HOST_ID=$HOST_ID
SWOOPS_AUTH_TOKEN=$AGENT_AUTH_TOKEN
EOF

    if [ "$AGENT_TLS_ENABLED" = true ]; then
        cat >> "$TEMP_AGENT_CONFIG" <<EOF

# TLS settings
SWOOPS_INSECURE=false
SWOOPS_SERVER_CA=$AGENT_CA_PATH
EOF
        if [ "$AGENT_MTLS_ENABLED" = true ]; then
            cat >> "$TEMP_AGENT_CONFIG" <<EOF
SWOOPS_CLIENT_CERT=$AGENT_CERT_PATH
SWOOPS_CLIENT_KEY=$AGENT_KEY_PATH
EOF
        fi
    else
        cat >> "$TEMP_AGENT_CONFIG" <<EOF

# TLS disabled (development only!)
SWOOPS_INSECURE=true
EOF
    fi

    # Move to final location with sudo and set proper permissions
    sudo mv "$TEMP_AGENT_CONFIG" "$AGENT_CONFIG"
    sudo chmod 644 "$AGENT_CONFIG"

    success "Agent configuration written to: $AGENT_CONFIG"
fi

echo

# Step 7: Reverse proxy configuration
if [ "$SKIP_INTERACTIVE" = false ] && [ "$INSTALL_SERVER" = true ] && [ "$USE_REVERSE_PROXY" = true ]; then
    echo -e "${GREEN}Step 7: Reverse Proxy Configuration${NC}"

    echo "Choose reverse proxy:"
    echo "  1) Caddy (automatic HTTPS)"
    echo "  2) nginx (with certbot)"
    echo "  3) Skip (I'll configure it manually)"
    prompt PROXY_TYPE "Reverse proxy type [1-3]" "1"

    case $PROXY_TYPE in
        1)
            CADDY_CONFIG="$CONFIG_DIR/Caddyfile"
            cat > "$CADDY_CONFIG" <<EOF
# Caddy configuration for Swoops
$DOMAIN {
    reverse_proxy $HTTP_HOST:$HTTP_PORT

    header {
        Strict-Transport-Security "max-age=31536000; includeSubDomains; preload"
        X-Content-Type-Options "nosniff"
        X-Frame-Options "DENY"
        Referrer-Policy "strict-origin-when-cross-origin"
    }
}
EOF
            success "Caddy configuration written to: $CADDY_CONFIG"
            info "To use: sudo caddy run --config $CADDY_CONFIG"
            ;;
        2)
            NGINX_CONFIG="$CONFIG_DIR/nginx-swoops.conf"
            cat > "$NGINX_CONFIG" <<EOF
# nginx configuration for Swoops
# Copy to /etc/nginx/sites-available/swoops
# Then: sudo ln -s /etc/nginx/sites-available/swoops /etc/nginx/sites-enabled/

server {
    listen 80;
    server_name $DOMAIN;

    location /.well-known/acme-challenge/ {
        root /var/www/certbot;
    }

    location / {
        return 301 https://\$server_name\$request_uri;
    }
}

server {
    listen 443 ssl http2;
    server_name $DOMAIN;

    ssl_certificate /etc/letsencrypt/live/$DOMAIN/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/$DOMAIN/privkey.pem;

    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_prefer_server_ciphers off;

    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains; preload" always;

    location / {
        proxy_pass http://$HTTP_HOST:$HTTP_PORT;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;

        # WebSocket support
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
EOF
            success "nginx configuration written to: $NGINX_CONFIG"
            info "Copy to /etc/nginx/sites-available/ and run certbot --nginx"
            ;;
        3)
            info "Skipping reverse proxy configuration."
            info "See Caddyfile.example or nginx.conf.example for reference."
            ;;
    esac
    echo
fi

# Step 8: Download and Install Binaries
echo -e "${GREEN}Step 8: Download and Install Binaries${NC}"

# Check if binaries are already installed
NEED_SERVER_BINARY=false
NEED_AGENT_BINARY=false

# If continuing full setup after declining quick update, force download
if [ "$FORCE_DOWNLOAD_BINARIES" = true ]; then
    info "Re-running setup detected - will download latest binaries"
    if [ "$INSTALL_SERVER" = true ]; then
        NEED_SERVER_BINARY=true
    fi
    if [ "$INSTALL_AGENT" = true ]; then
        NEED_AGENT_BINARY=true
    fi
else
    if [ "$INSTALL_SERVER" = true ]; then
        if ! command -v swoopsd &> /dev/null; then
            NEED_SERVER_BINARY=true
        else
            info "swoopsd already installed at $(which swoopsd)"
        fi
    fi

    if [ "$INSTALL_AGENT" = true ]; then
        if ! command -v swoops-agent &> /dev/null; then
            NEED_AGENT_BINARY=true
        else
            info "swoops-agent already installed at $(which swoops-agent)"
        fi
    fi
fi

if [ "$NEED_SERVER_BINARY" = true ] || [ "$NEED_AGENT_BINARY" = true ]; then
    GITHUB_REPO="kaitwalla/swoops-control"

    # Detect architecture
    ARCH=$(uname -m)
    if [ "$ARCH" = "x86_64" ]; then
        BINARY_ARCH="amd64"
    elif [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then
        BINARY_ARCH="arm64"
    else
        error "Unsupported architecture: $ARCH"
        exit 1
    fi

    # Detect OS
    if [ "$OS" = "linux" ]; then
        BINARY_OS="linux"
    elif [ "$OS" = "macos" ]; then
        BINARY_OS="darwin"
    else
        error "Unsupported OS: $OS"
        exit 1
    fi

    # Get latest release info
    info "Fetching latest release information from GitHub..."
    LATEST_RELEASE=$(curl -s https://api.github.com/repos/$GITHUB_REPO/releases/latest)
    LATEST_VERSION=$(echo "$LATEST_RELEASE" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

    if [ -z "$LATEST_VERSION" ]; then
        error "Could not fetch latest release information"
        exit 1
    fi

    info "Latest version: $LATEST_VERSION"

    # Download server binary
    if [ "$NEED_SERVER_BINARY" = true ]; then
        BINARY_NAME="swoopsd-${BINARY_OS}-${BINARY_ARCH}"
        DOWNLOAD_URL="https://github.com/$GITHUB_REPO/releases/download/$LATEST_VERSION/$BINARY_NAME"

        info "Downloading swoopsd from $DOWNLOAD_URL..."
        TEMP_BINARY=$(mktemp)

        if command -v curl &> /dev/null; then
            if ! curl -fsSL "$DOWNLOAD_URL" -o "$TEMP_BINARY"; then
                error "Failed to download swoopsd from $DOWNLOAD_URL"
                rm -f "$TEMP_BINARY"
                exit 1
            fi
        elif command -v wget &> /dev/null; then
            if ! wget -q "$DOWNLOAD_URL" -O "$TEMP_BINARY"; then
                error "Failed to download swoopsd from $DOWNLOAD_URL"
                rm -f "$TEMP_BINARY"
                exit 1
            fi
        else
            error "Neither curl nor wget found. Please install one of them first."
            exit 1
        fi

        chmod +x "$TEMP_BINARY"
        sudo mv "$TEMP_BINARY" /usr/local/bin/swoopsd
        success "Installed swoopsd to /usr/local/bin/swoopsd"

        # On Linux, grant CAP_NET_BIND_SERVICE for autocert (port 443/80)
        if [ "$OS" = "linux" ]; then
            if command -v setcap &> /dev/null; then
                info "Granting CAP_NET_BIND_SERVICE capability for port 443/80 binding..."
                if sudo setcap 'cap_net_bind_service=+ep' /usr/local/bin/swoopsd; then
                    success "✓ CAP_NET_BIND_SERVICE capability granted"
                else
                    warn "Failed to grant CAP_NET_BIND_SERVICE capability"
                    warn "If using autocert, run: sudo setcap 'cap_net_bind_service=+ep' /usr/local/bin/swoopsd"
                fi
            else
                warn "setcap command not found - cannot grant CAP_NET_BIND_SERVICE capability"
                warn "Install libcap2-bin (Debian/Ubuntu) or libcap (RHEL/Fedora), then run:"
                warn "  sudo setcap 'cap_net_bind_service=+ep' /usr/local/bin/swoopsd"
            fi
        fi
    fi

    # Download agent binary
    if [ "$NEED_AGENT_BINARY" = true ]; then
        BINARY_NAME="swoops-agent-${BINARY_OS}-${BINARY_ARCH}"
        DOWNLOAD_URL="https://github.com/$GITHUB_REPO/releases/download/$LATEST_VERSION/$BINARY_NAME"

        info "Downloading swoops-agent from $DOWNLOAD_URL..."
        TEMP_BINARY=$(mktemp)

        if command -v curl &> /dev/null; then
            if ! curl -fsSL "$DOWNLOAD_URL" -o "$TEMP_BINARY"; then
                error "Failed to download swoops-agent from $DOWNLOAD_URL"
                rm -f "$TEMP_BINARY"
                exit 1
            fi
        elif command -v wget &> /dev/null; then
            if ! wget -q "$DOWNLOAD_URL" -O "$TEMP_BINARY"; then
                error "Failed to download swoops-agent from $DOWNLOAD_URL"
                rm -f "$TEMP_BINARY"
                exit 1
            fi
        else
            error "Neither curl nor wget found. Please install one of them first."
            exit 1
        fi

        chmod +x "$TEMP_BINARY"
        sudo mv "$TEMP_BINARY" /usr/local/bin/swoops-agent
        success "Installed swoops-agent to /usr/local/bin/swoops-agent"
    fi

    echo
else
    info "Binaries already installed, skipping download"
    echo
fi

# Download CA certificate if requested in non-interactive mode
if [ "$NON_INTERACTIVE" = true ] && [ "$AGENT_DOWNLOAD_CA" = true ]; then
    info "Downloading CA certificate from $AGENT_HTTP_URL..."

    # Create cert directory
    sudo mkdir -p /etc/swoops/certs

    # Download the CA certificate
    CA_URL="$AGENT_HTTP_URL/api/ca-cert"
    if command -v curl &> /dev/null; then
        if ! sudo curl -fsSL "$CA_URL" -o /etc/swoops/certs/server-ca.pem; then
            error "Failed to download CA certificate from $CA_URL"
            warn "You may need to manually copy the CA certificate"
        else
            success "Downloaded CA certificate to /etc/swoops/certs/server-ca.pem"
        fi
    elif command -v wget &> /dev/null; then
        if ! sudo wget -q "$CA_URL" -O /etc/swoops/certs/server-ca.pem; then
            error "Failed to download CA certificate from $CA_URL"
            warn "You may need to manually copy the CA certificate"
        else
            success "Downloaded CA certificate to /etc/swoops/certs/server-ca.pem"
        fi
    fi

    # Set proper permissions
    sudo chmod 644 /etc/swoops/certs/server-ca.pem

    # Download client certificates for mTLS if host ID and auth token are provided
    if [ -n "$AGENT_HOST_ID" ] && [ -n "$AGENT_AUTH_TOKEN_ARG" ]; then
        info "Downloading client certificates for mTLS..."

        CLIENT_CERT_URL="$AGENT_HTTP_URL/api/v1/hosts/$AGENT_HOST_ID/client-cert?auth_token=$AGENT_AUTH_TOKEN_ARG"

        # Download client cert and key as JSON
        if command -v curl &> /dev/null; then
            CLIENT_CERT_JSON=$(curl -fsSL "$CLIENT_CERT_URL")
        elif command -v wget &> /dev/null; then
            CLIENT_CERT_JSON=$(wget -qO- "$CLIENT_CERT_URL")
        else
            error "Neither curl nor wget found - cannot download client certificates"
            warn "You may need to manually configure client certificates"
            CLIENT_CERT_JSON=""
        fi

        if [ -n "$CLIENT_CERT_JSON" ]; then
            # Extract cert and key from JSON (using grep and sed for portability)
            # Expected format: {"client_cert":"-----BEGIN CERTIFICATE-----\n...","client_key":"-----BEGIN EC PRIVATE KEY-----\n..."}

            # Extract client_cert field and decode escaped newlines
            CLIENT_CERT=$(echo "$CLIENT_CERT_JSON" | grep -o '"client_cert":"[^"]*"' | sed 's/"client_cert":"//;s/"$//' | sed 's/\\n/\n/g')
            CLIENT_KEY=$(echo "$CLIENT_CERT_JSON" | grep -o '"client_key":"[^"]*"' | sed 's/"client_key":"//;s/"$//' | sed 's/\\n/\n/g')

            if [ -n "$CLIENT_CERT" ] && [ -n "$CLIENT_KEY" ]; then
                # Write to temp files first, then move with sudo
                TEMP_CERT=$(mktemp)
                TEMP_KEY=$(mktemp)

                echo "$CLIENT_CERT" > "$TEMP_CERT"
                echo "$CLIENT_KEY" > "$TEMP_KEY"

                sudo mv "$TEMP_CERT" /etc/swoops/certs/client-cert.pem
                sudo mv "$TEMP_KEY" /etc/swoops/certs/client-key.pem

                sudo chmod 644 /etc/swoops/certs/client-cert.pem
                sudo chmod 600 /etc/swoops/certs/client-key.pem  # Private key should be restricted

                success "Downloaded client certificates for mTLS"
                AGENT_MTLS_ENABLED=true
            else
                warn "Failed to parse client certificates from server response"
                AGENT_MTLS_ENABLED=false
            fi
        else
            warn "Failed to download client certificates"
            AGENT_MTLS_ENABLED=false
        fi
    else
        AGENT_MTLS_ENABLED=false
    fi

    echo
fi

# Step 9: Service installation
if [ "$SKIP_INTERACTIVE" = false ]; then
    echo -e "${GREEN}Step 9: Service Installation${NC}"
fi

if [ "$SKIP_INTERACTIVE" = true ] || confirm "Install as system service?" "y"; then
    if [ "$INSTALL_SERVER" = true ]; then
        if [ "$INIT_SYSTEM" = "systemd" ]; then
            SERVICE_FILE="/etc/systemd/system/swoopsd.service"

            sudo tee "$SERVICE_FILE" > /dev/null <<EOF
[Unit]
Description=Swoops Control Plane
After=network.target

[Service]
Type=simple
User=swoops
Group=swoops
WorkingDirectory=/opt/swoops
EnvironmentFile=$CONFIG_DIR/swoopsd.env
ExecStart=$(which swoopsd || echo '/usr/local/bin/swoopsd') --config $SERVER_CONFIG
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

            success "Systemd service created: $SERVICE_FILE"

            # Create user
            if ! id swoops &>/dev/null; then
                sudo useradd -r -s /bin/false swoops || true
            fi

            # Create directories
            sudo mkdir -p /opt/swoops "$(dirname "$DB_PATH")"
            sudo chown -R swoops:swoops /opt/swoops "$(dirname "$DB_PATH")"

            # Create autocert cache directory if autocert is enabled
            if [ "$USE_AUTOCERT" = true ]; then
                sudo mkdir -p /var/cache/swoops/autocert
                sudo chown -R swoops:swoops /var/cache/swoops
                sudo chmod 700 /var/cache/swoops/autocert

                # Grant capability to bind to privileged ports (80, 443)
                info "Granting swoopsd permission to bind to ports 80 and 443..."
                SWOOPSD_PATH=$(which swoopsd || echo '/usr/local/bin/swoopsd')
                if command -v setcap &> /dev/null; then
                    sudo setcap 'cap_net_bind_service=+ep' "$SWOOPSD_PATH"
                    success "Granted CAP_NET_BIND_SERVICE capability to $SWOOPSD_PATH"
                else
                    warn "setcap not found. You may need to run as root or install libcap2-bin"
                    warn "Without this, swoopsd won't be able to bind to ports 80 and 443"
                fi
            fi

            sudo systemctl daemon-reload
            info "To start: sudo systemctl start swoopsd"
            info "To enable on boot: sudo systemctl enable swoopsd"

        elif [ "$INIT_SYSTEM" = "launchd" ]; then
            PLIST_FILE="$HOME/Library/LaunchAgents/com.swoops.server.plist"

            cat > "$PLIST_FILE" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.swoops.server</string>
    <key>ProgramArguments</key>
    <array>
        <string>$(which swoopsd || echo '/usr/local/bin/swoopsd')</string>
        <string>--config</string>
        <string>$SERVER_CONFIG</string>
    </array>
    <key>EnvironmentVariables</key>
    <dict>
        <key>SWOOPS_API_KEY</key>
        <string>$API_KEY</string>
    </dict>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/var/log/swoops-server.log</string>
    <key>StandardErrorPath</key>
    <string>/var/log/swoops-server.error.log</string>
</dict>
</plist>
EOF

            success "Launchd plist created: $PLIST_FILE"
            info "To start: launchctl load $PLIST_FILE"
        fi
    fi

    if [ "$INSTALL_AGENT" = true ]; then
        if [ "$INIT_SYSTEM" = "systemd" ]; then
            SERVICE_FILE="/etc/systemd/system/swoops-agent.service"

            # Build ExecStart command based on mTLS configuration
            if [ "$AGENT_MTLS_ENABLED" = true ]; then
                EXEC_START="$(which swoops-agent || echo '/usr/local/bin/swoops-agent') run --server \$SWOOPS_SERVER --host-id \$SWOOPS_HOST_ID --auth-token \$SWOOPS_AUTH_TOKEN --ca-cert /etc/swoops/certs/server-ca.pem --tls-cert /etc/swoops/certs/client-cert.pem --tls-key /etc/swoops/certs/client-key.pem"
            else
                EXEC_START="$(which swoops-agent || echo '/usr/local/bin/swoops-agent') run --server \$SWOOPS_SERVER --host-id \$SWOOPS_HOST_ID --auth-token \$SWOOPS_AUTH_TOKEN"
            fi

            sudo tee "$SERVICE_FILE" > /dev/null <<EOF
[Unit]
Description=Swoops Agent
After=network.target

[Service]
Type=simple
User=swoops
Group=swoops
WorkingDirectory=/opt/swoops
EnvironmentFile=$AGENT_CONFIG
ExecStart=$EXEC_START
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

            success "Systemd service created: $SERVICE_FILE"

            sudo systemctl daemon-reload
            info "To start: sudo systemctl start swoops-agent"
            info "To enable on boot: sudo systemctl enable swoops-agent"

        elif [ "$INIT_SYSTEM" = "launchd" ]; then
            PLIST_FILE="$HOME/Library/LaunchAgents/com.swoops.agent.plist"

            # Source environment variables for plist
            source "$AGENT_CONFIG"

            # Build program arguments array based on mTLS configuration
            if [ "$AGENT_MTLS_ENABLED" = true ]; then
                PLIST_ARGS="        <string>$(which swoops-agent || echo '/usr/local/bin/swoops-agent')</string>
        <string>run</string>
        <string>--server</string>
        <string>$SWOOPS_SERVER</string>
        <string>--host-id</string>
        <string>$SWOOPS_HOST_ID</string>
        <string>--auth-token</string>
        <string>$SWOOPS_AUTH_TOKEN</string>
        <string>--ca-cert</string>
        <string>/etc/swoops/certs/server-ca.pem</string>
        <string>--tls-cert</string>
        <string>/etc/swoops/certs/client-cert.pem</string>
        <string>--tls-key</string>
        <string>/etc/swoops/certs/client-key.pem</string>"
            else
                PLIST_ARGS="        <string>$(which swoops-agent || echo '/usr/local/bin/swoops-agent')</string>
        <string>run</string>
        <string>--server</string>
        <string>$SWOOPS_SERVER</string>
        <string>--host-id</string>
        <string>$SWOOPS_HOST_ID</string>
        <string>--auth-token</string>
        <string>$SWOOPS_AUTH_TOKEN</string>"
            fi

            cat > "$PLIST_FILE" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.swoops.agent</string>
    <key>ProgramArguments</key>
    <array>
$PLIST_ARGS
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>$HOME/Library/Logs/swoops-agent.log</string>
    <key>StandardErrorPath</key>
    <string>$HOME/Library/Logs/swoops-agent.error.log</string>
</dict>
</plist>
EOF

            success "Launchd plist created: $PLIST_FILE"
            info "To start: launchctl load $PLIST_FILE"
        fi
    fi
fi

echo

# Summary
if [ "$SKIP_INTERACTIVE" = true ]; then
    # Non-interactive summary
    success "Agent installation complete!"
    echo
    info "Agent installed at: /usr/local/bin/swoops-agent"
    info "Configuration: $AGENT_CONFIG"
    info "Server: $CONTROL_PLANE_HOST:$CONTROL_PLANE_PORT"
    info "Host ID: $HOST_ID"
    echo
    if [ "$INIT_SYSTEM" = "systemd" ]; then
        info "To start the agent: sudo systemctl start swoops-agent"
        info "To enable on boot: sudo systemctl enable swoops-agent"
        info "To check logs: sudo journalctl -u swoops-agent -f"
    elif [ "$INIT_SYSTEM" = "launchd" ]; then
        info "To start the agent: launchctl load $HOME/Library/LaunchAgents/com.swoops.agent.plist"
    fi
    echo
    if [ "$AGENT_DOWNLOAD_CA" = true ]; then
        info "Note: The agent will automatically download the CA certificate on first connection"
        info "HTTP URL: $AGENT_HTTP_URL"
    fi
    echo
    success "Happy orchestrating! 🚀"
    exit 0
fi

echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}Setup Complete!${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo

if [ "$INSTALL_SERVER" = true ]; then
    echo -e "${BLUE}Server Configuration:${NC}"
    echo "  Config file: $SERVER_CONFIG"
    echo "  External URL: $EXTERNAL_URL"
    echo "  API Key: $API_KEY"
    echo "  Database: $DB_PATH"
    if [ "$USE_AUTOCERT" = true ]; then
        echo "  HTTPS: Automatic (Let's Encrypt for $DOMAIN)"
        echo "  Ports: 443 (HTTPS), 80 (HTTP redirect + ACME challenges)"
    elif [ "$USE_REVERSE_PROXY" = true ]; then
        echo "  Reverse Proxy: Yes (configure $PROXY_TYPE)"
    elif [ "$USE_TLS" = true ]; then
        echo "  HTTPS: Manual TLS"
    fi
    if [ "$GRPC_TLS_ENABLED" = true ]; then
        echo "  gRPC TLS: Yes (mTLS: $GRPC_MTLS_ENABLED)"
    fi
    echo
fi

if [ "$INSTALL_AGENT" = true ]; then
    echo -e "${BLUE}Agent Configuration:${NC}"
    echo "  Config file: $AGENT_CONFIG"
    echo "  Control Plane: $CONTROL_PLANE_HOST:$CONTROL_PLANE_PORT"
    echo "  Host ID: $HOST_ID"
    if [ "$AGENT_TLS_ENABLED" = true ]; then
        echo "  TLS: Yes (mTLS: $AGENT_MTLS_ENABLED)"
    fi
    echo
fi

echo -e "${BLUE}Next Steps:${NC}"
if [ "$INSTALL_SERVER" = true ]; then
    echo "  1. Review configuration: cat $SERVER_CONFIG"
    if [ "$USE_AUTOCERT" = true ]; then
        echo "  2. Ensure ports 80 and 443 are open in your firewall"
        echo "  3. Ensure DNS for $DOMAIN points to this server's IP"
        echo "  4. Start server: sudo systemctl start swoopsd"
        echo "     (Let's Encrypt certificate will be obtained automatically on first request)"
    elif [ "$USE_REVERSE_PROXY" = true ]; then
        if [ "$PROXY_TYPE" = "1" ]; then
            echo "  2. Start Caddy: sudo caddy run --config $CONFIG_DIR/Caddyfile"
        elif [ "$PROXY_TYPE" = "2" ]; then
            echo "  2. Configure nginx and run certbot"
        fi
        echo "  3. Start server: sudo systemctl start swoopsd"
    else
        echo "  2. Start server: sudo systemctl start swoopsd"
    fi
    echo "  5. Check logs: sudo journalctl -u swoopsd -f"
    echo "  6. Access UI: $EXTERNAL_URL"
    echo "  7. Register hosts via API or Web UI"
fi

if [ "$INSTALL_AGENT" = true ]; then
    if [ "$INSTALL_SERVER" != true ]; then
        echo "  1. Register this host with control plane"
        if [ "$AGENT_TLS_ENABLED" = true ]; then
            echo "  2. Option A: Let agent download CA certificate automatically:"
            echo "     Add --download-ca --http-url http://server:8080 to agent command"
            echo "     OR"
            echo "     Option B: Copy certificates from server to this machine:"
            echo "     scp user@server:/etc/swoops/certs/server-ca.pem /etc/swoops/certs/"
            if [ "$AGENT_MTLS_ENABLED" = true ]; then
                echo "     scp user@server:/etc/swoops/certs/agent-cert.pem /etc/swoops/certs/"
                echo "     scp user@server:/etc/swoops/certs/agent-key.pem /etc/swoops/certs/"
            fi
        fi
        echo "  3. Update $AGENT_CONFIG with auth token"
    fi
    echo "  4. Start agent: sudo systemctl start swoops-agent"
    echo "  5. Check logs: sudo journalctl -u swoops-agent -f"
fi

echo
success "Setup complete! Happy orchestrating! 🚀"
