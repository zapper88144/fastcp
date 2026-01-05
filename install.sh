#!/bin/bash
#
# FastCP Installation Script
# Usage: curl -fsSL https://fastcp.org/install.sh | bash
#

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
GITHUB_REPO="fastcp/fastcp"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/fastcp"
DATA_DIR="/var/lib/fastcp"
LOG_DIR="/var/log/fastcp"

# Detect platform
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    if [ "$OS" != "linux" ]; then
        echo -e "${RED}FastCP only supports Linux. Detected: $OS${NC}"
        echo -e "${YELLOW}For local development, use: FASTCP_DEV=1 go run ./cmd/fastcp${NC}"
        exit 1
    fi

    case "$ARCH" in
        x86_64|amd64)
            PLATFORM="linux-x86_64"
            ;;
        aarch64|arm64)
            PLATFORM="linux-aarch64"
            ;;
        *)
            echo -e "${RED}Unsupported architecture: $ARCH${NC}"
            echo -e "${YELLOW}Supported: x86_64, aarch64 (arm64)${NC}"
            exit 1
            ;;
    esac

    echo -e "${BLUE}Detected platform: ${PLATFORM}${NC}"
}

# Get latest version
get_latest_version() {
    VERSION=$(curl -s "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
    if [ -z "$VERSION" ]; then
        echo -e "${RED}Failed to get latest version${NC}"
        exit 1
    fi
    echo -e "${BLUE}Latest version: ${VERSION}${NC}"
}

# Download and install FastCP
install_fastcp() {
    echo -e "${YELLOW}Downloading FastCP...${NC}"
    
    DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/fastcp-${PLATFORM}"
    
    # Download to temp file
    TMP_FILE=$(mktemp)
    curl -fsSL "$DOWNLOAD_URL" -o "$TMP_FILE"
    
    # Make executable and move to install dir
    chmod +x "$TMP_FILE"
    sudo mv "$TMP_FILE" "${INSTALL_DIR}/fastcp"
    
    echo -e "${GREEN}FastCP installed to ${INSTALL_DIR}/fastcp${NC}"
}

# Create directories
create_directories() {
    echo -e "${YELLOW}Creating directories...${NC}"
    
    sudo mkdir -p "$CONFIG_DIR"
    sudo mkdir -p "$DATA_DIR"
    sudo mkdir -p "$LOG_DIR"
    
    # Set permissions
    sudo chmod 755 "$CONFIG_DIR"
    sudo chmod 755 "$DATA_DIR"
    sudo chmod 755 "$LOG_DIR"
}

# Create systemd service
create_systemd_service() {
    echo -e "${YELLOW}Creating systemd service...${NC}"
    
    sudo tee /etc/systemd/system/fastcp.service > /dev/null << EOF
[Unit]
Description=FastCP - Modern PHP Hosting Control Panel
After=network.target

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/fastcp
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

    sudo systemctl daemon-reload
    echo -e "${GREEN}Systemd service created${NC}"
}

# Print success message
print_success() {
    echo ""
    echo -e "${GREEN}╔═══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║                                                               ║${NC}"
    echo -e "${GREEN}║   FastCP installed successfully!                              ║${NC}"
    echo -e "${GREEN}║                                                               ║${NC}"
    echo -e "${GREEN}╚═══════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo -e "${BLUE}Quick Start:${NC}"
    echo ""
    echo "  # Start FastCP service"
    echo "  sudo systemctl start fastcp"
    echo ""
    echo "  # Enable on boot"
    echo "  sudo systemctl enable fastcp"
    echo ""
    echo "  # Check status"
    echo "  sudo systemctl status fastcp"
    echo ""
    echo -e "${BLUE}Access:${NC}"
    echo ""
    echo "  Admin Panel: http://YOUR_SERVER_IP:8080"
    echo "  Username:    admin"
    echo "  Password:    fastcp2024!"
    echo ""
    echo -e "${YELLOW}Note: FrankenPHP will be auto-downloaded on first run.${NC}"
    echo ""
}

# Main
main() {
    echo ""
    echo -e "${GREEN}╔═══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║                                                               ║${NC}"
    echo -e "${GREEN}║   FastCP Installer                                            ║${NC}"
    echo -e "${GREEN}║   Modern PHP Hosting Control Panel                            ║${NC}"
    echo -e "${GREEN}║                                                               ║${NC}"
    echo -e "${GREEN}╚═══════════════════════════════════════════════════════════════╝${NC}"
    echo ""

    detect_platform
    get_latest_version
    install_fastcp
    create_directories
    create_systemd_service
    print_success
}

main "$@"

