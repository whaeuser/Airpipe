#!/bin/sh
set -e

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

RELAY="__RELAY_URL__"
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

if [ "$RELAY" != "https://drop.example.com" ] && [ "$RELAY" != "__RELAY_URL__" ]; then
    case "$SHELL" in
        */zsh)  RC="$HOME/.zshrc" ;;
        */bash) RC="$HOME/.bashrc" ;;
        *)      RC="" ;;
    esac

    echo
    if [ -n "$RC" ] && [ -e /dev/tty ]; then
        printf "Add 'export DROP_RELAY=%s' to %s? [y/N] " "$RELAY" "$RC"
        read REPLY < /dev/tty
        case "$REPLY" in
            [yY]*)
                echo "export DROP_RELAY=$RELAY" >> "$RC"
                echo "Added. Restart your shell or run: export DROP_RELAY=$RELAY"
                exit 0
                ;;
        esac
    fi
    echo "To use this relay by default:"
    echo "  export DROP_RELAY=$RELAY"
fi
