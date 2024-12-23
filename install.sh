#!/bin/sh

REPO="tarcisiomiranda/tm-overlay"
BIN_NAME="service-manager"
DOWNLOAD_URL="https://github.com/$REPO/releases/latest/download/$BIN_NAME"
DESTINATION="/entrypoint"

echo "[INFO] Starting installation of $BIN_NAME..."

if ! command -v curl &> /dev/null; then
  echo "[ERROR] curl is not installed. Please install curl and try again."
  exit 1
fi

echo "[INFO] Downloading $BIN_NAME from $DOWNLOAD_URL..."
curl -L -o "$DESTINATION" "$DOWNLOAD_URL"
if [ $? -ne 0 ]; then
  echo "[ERROR] Failed to download $BIN_NAME. Please check the URL or your internet connection."
  exit 1
fi

echo "[INFO] Making $BIN_NAME executable..."
chmod +x "$DESTINATION"
if [ $? -ne 0 ]; then
  echo "[ERROR] Failed to make $BIN_NAME executable."
  exit 1
fi

echo "[INFO] $BIN_NAME has been installed successfully at $DESTINATION."
echo "[INFO] You can now use it as the entrypoint in your Dockerfile."

exit 0
