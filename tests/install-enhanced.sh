#!/bin/bash

set -e

INSTALL_DIR="/usr/local/bin"
SERVICE_NAME="tm-orchestrator"
GITHUB_REPO="tarcisiomiranda/tm-overlay"
VERSION="latest"

echo "|=== TM Orchestrator Enhanced Installer ===|"

detect_arch() {
  local arch
  case $(uname -m) in
    x86_64) arch="amd64" ;;
    aarch64) arch="arm64" ;;
    armv7l) arch="arm" ;;
    *) echo "Unsupported architecture: $(uname -m)"; exit 1 ;;
  esac
  echo $arch
}

download_binary() {
  local arch=$(detect_arch)
  local url="https://github.com/${GITHUB_REPO}/releases/latest/download/service-manager-linux-${arch}"

  echo "Downloading TM Orchestrator for ${arch}..."
  if command -v curl >/dev/null 2>&1; then
    curl -L "$url" -o "${SERVICE_NAME}"
  elif command -v wget >/dev/null 2>&1; then
    wget "$url" -O "${SERVICE_NAME}"
  else
    echo "Error: curl or wget is required"
    exit 1
  fi
}

install_local() {
  if [ -f "service-manager" ]; then
    echo "Using local service-manager binary..."
    cp service-manager "${SERVICE_NAME}"
  else
    echo "Error: service-manager binary not found"
    echo "Please compile first or run this script from the project directory"
    exit 1
  fi
}

if [ "$EUID" -ne 0 ]; then
  echo "This script requires root privileges. Trying with sudo..."
  exec sudo "$0" "$@"
fi

if [ "$1" = "--local" ] || [ -f "service-manager" ]; then
  install_local
else
  echo "Local binary not found. Would you like to:"
  echo "1) Use local binary (compile first)"
  echo "2) Download from GitHub releases"
  echo "3) Exit"
  read -p "Choice [1-3]: " choice
  
  case $choice in
    1) install_local ;;
    2) download_binary ;;
    *) echo "Exiting..."; exit 0 ;;
  esac
fi

chmod +x "${SERVICE_NAME}"

echo "Installing to ${INSTALL_DIR}..."
mv "${SERVICE_NAME}" "${INSTALL_DIR}/"

ln -sf "${INSTALL_DIR}/${SERVICE_NAME}" "${INSTALL_DIR}/entrypoint"

if command -v ${SERVICE_NAME} >/dev/null 2>&1; then
  echo "✓ Installation successful!"
  echo ""
  echo "Available commands:"
  echo "  ${SERVICE_NAME}                    # Start daemon mode"
  echo "  ${SERVICE_NAME} list               # List services"
  echo "  ${SERVICE_NAME} status             # Show system status"
  echo "  ${SERVICE_NAME} restart <service>  # Restart a service"
  echo "  ${SERVICE_NAME} install            # Reinstall/update"
  echo ""
  echo "For Docker usage, you can now use:"
  echo "  ENTRYPOINT [\"${SERVICE_NAME}\"]"
else
  echo "✗ Installation failed"
  exit 1
fi

if [ ! -f "/etc/tm-orchestrator/services.toml" ]; then
  echo "Creating example configuration..."
  mkdir -p /etc/tm-orchestrator
  cat > /etc/tm-orchestrator/services.toml << 'EOF'
# TM Orchestrator Configuration Example
[timeouts]
post_script_timeout = 5
service_shutdown_timeout = 15
global_shutdown_timeout = 45
dependency_wait_timeout = 120

[[services]]
name = "nginx"
command = "/usr/sbin/nginx"
args = ["-g", "daemon off;"]
enabled = true
required = false

[[services]]
name = "app"
command = "/usr/local/bin/myapp"
depends_on = "nginx"
wait_after = 3
enabled = true
required = true
EOF
  echo "Example config created at: /etc/tm-orchestrator/services.toml"
fi

echo ""
echo "|=== Installation Complete ===|"
