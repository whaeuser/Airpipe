#!/bin/sh
set -e

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

URL="https://github.com/whaeuser/Airpipe/releases/latest/download/drop-${OS}-${ARCH}"

echo "Downloading drop for ${OS}-${ARCH}..."
curl -sL "$URL" -o /tmp/drop
chmod +x /tmp/drop

# Install to /usr/local/bin, use sudo if needed
if [ -w /usr/local/bin ]; then
    mv /tmp/drop /usr/local/bin/drop
    echo "Installed to /usr/local/bin/drop"
else
    echo "Need sudo to install to /usr/local/bin"
    sudo mv /tmp/drop /usr/local/bin/drop
    echo "Installed to /usr/local/bin/drop"
fi

echo "Done! Run: drop send <file>"
