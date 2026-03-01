#!/bin/bash
# Swoops installation script
# Usage: curl -fsSL https://raw.githubusercontent.com/YOUR_USERNAME/swoops/main/install.sh | bash
# Or: ./install.sh [--server|--agent|--all] [--version VERSION] [--install-dir DIR]

set -e

# Configuration
REPO="swoopsh/swoops"
VERSION="${SWOOPS_VERSION:-latest}"
INSTALL_DIR="${SWOOPS_INSTALL_DIR:-}"
COMPONENT="all"  # server, agent, or all

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Print functions
info() {
    echo -e "${GREEN}==>${NC} $1"
}

warn() {
    echo -e "${YELLOW}Warning:${NC} $1"
}

error() {
    echo -e "${RED}Error:${NC} $1"
    exit 1
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --server)
            COMPONENT="server"
            shift
            ;;
        --agent)
            COMPONENT="agent"
            shift
            ;;
        --all)
            COMPONENT="all"
            shift
            ;;
        --version)
            VERSION="$2"
            shift 2
            ;;
        --install-dir)
            INSTALL_DIR="$2"
            shift 2
            ;;
        --help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --server         Install only the server (swoopsd)"
            echo "  --agent          Install only the agent (swoops-agent)"
            echo "  --all            Install both server and agent (default)"
            echo "  --version VER    Install specific version (default: latest)"
            echo "  --install-dir    Install directory (default: /usr/local/bin or ./)"
            echo "  --help           Show this help message"
            echo ""
            echo "Environment variables:"
            echo "  SWOOPS_VERSION        Version to install"
            echo "  SWOOPS_INSTALL_DIR    Installation directory"
            exit 0
            ;;
        *)
            error "Unknown option: $1. Use --help for usage information."
            ;;
    esac
done

# Detect OS and architecture
detect_platform() {
    local os="$(uname -s)"
    local arch="$(uname -m)"

    case "$os" in
        Linux*)
            OS="linux"
            ;;
        Darwin*)
            OS="darwin"
            ;;
        *)
            error "Unsupported operating system: $os"
            ;;
    esac

    case "$arch" in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        *)
            error "Unsupported architecture: $arch"
            ;;
    esac

    PLATFORM="${OS}-${ARCH}"
    info "Detected platform: $PLATFORM"
}

# Determine installation directory
setup_install_dir() {
    if [ -n "$INSTALL_DIR" ]; then
        # User specified a directory
        if [ ! -d "$INSTALL_DIR" ]; then
            error "Installation directory does not exist: $INSTALL_DIR"
        fi
        if [ ! -w "$INSTALL_DIR" ]; then
            error "Installation directory is not writable: $INSTALL_DIR"
        fi
    else
        # Try common directories
        if [ -w "/usr/local/bin" ]; then
            INSTALL_DIR="/usr/local/bin"
        elif [ -w "$HOME/.local/bin" ]; then
            INSTALL_DIR="$HOME/.local/bin"
            warn "$HOME/.local/bin may not be in your PATH"
        else
            INSTALL_DIR="."
            warn "No writable system directory found. Installing to current directory."
            warn "You may need to move the binaries to a directory in your PATH manually."
        fi
    fi

    info "Installing to: $INSTALL_DIR"
}

# Get the latest release version
get_latest_version() {
    if [ "$VERSION" = "latest" ]; then
        info "Fetching latest release version..."
        VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
        if [ -z "$VERSION" ]; then
            error "Failed to fetch latest version. Please specify a version with --version"
        fi
        info "Latest version: $VERSION"
    fi
}

# Download and install a binary
install_binary() {
    local binary_name="$1"
    local download_name="${binary_name}-${PLATFORM}"
    local url="https://github.com/$REPO/releases/download/${VERSION}/${download_name}"

    info "Downloading $binary_name..."

    if ! curl -fsSL -o "/tmp/${download_name}" "$url"; then
        error "Failed to download $binary_name from $url"
    fi

    chmod +x "/tmp/${download_name}"

    if [ "$INSTALL_DIR" = "." ]; then
        mv "/tmp/${download_name}" "./${binary_name}"
        info "Installed $binary_name to $(pwd)/${binary_name}"
    else
        mv "/tmp/${download_name}" "${INSTALL_DIR}/${binary_name}"
        info "Installed $binary_name to ${INSTALL_DIR}/${binary_name}"
    fi
}

# Main installation
main() {
    info "Swoops installation script"

    detect_platform
    setup_install_dir
    get_latest_version

    case "$COMPONENT" in
        server)
            install_binary "swoopsd"
            ;;
        agent)
            install_binary "swoops-agent"
            ;;
        all)
            install_binary "swoopsd"
            install_binary "swoops-agent"
            ;;
    esac

    echo ""
    info "Installation complete!"
    echo ""
    echo "To get started:"
    echo "  # Start the server (generates API key on first run)"
    echo "  swoopsd"
    echo ""
    echo "  # Start an agent (after registering a host)"
    echo "  swoops-agent run --server 127.0.0.1:9090 --host-id <host-id>"
    echo ""
    echo "For more information, visit: https://github.com/$REPO"
}

main
