#!/bin/bash
# Setup script for SmogPing

set -e

echo "Setting up SmogPing..."

# Build the application
echo "Building smogping..."
go build -o smogping main.go

# Set capabilities for ICMP ping (alternative to running as root)
echo "Setting capabilities for ICMP ping..."
sudo setcap cap_net_raw=+ep ./smogping

echo "Setup complete!"
echo ""
echo "You can now run the application with:"
echo "  ./smogping"
echo ""
echo "Or install as a systemd service:"
echo "  sudo cp smogping.service /etc/systemd/system/"
echo "  sudo systemctl daemon-reload"
echo "  sudo systemctl enable smogping"
echo "  sudo systemctl start smogping"
