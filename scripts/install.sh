#!/bin/bash

# TYP - Terminal YouTube Player Install Script

set -e

echo "Installing TYP (Terminal YouTube Player)..."

# Check dependencies
for cmd in go mpv yt-dlp; do
    if ! command -v $cmd &> /dev/null; then
        echo "Error: $cmd is not installed. Please install it first."
        exit 1
    fi
done

# Build the project
make build

# Install binary
sudo cp bin/typ /usr/local/bin/typ

# Setup config directory
mkdir -p "$HOME/.config/typ"
mkdir -p "$XDG_RUNTIME_DIR/typ"

echo "Installation complete!"
echo "You can now run 'typ --daemon' to start the background service"
echo "and 'typ' to start the TUI."
