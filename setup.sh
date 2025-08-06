#!/bin/bash
# Setup script for SmogPing - Install system-wide like an RPM package

set -e

echo "=== SmogPing System Installation ==="
echo "This script will install SmogPing system-wide with proper permissions."
echo "You may be prompted for sudo password multiple times."
echo ""

# Check if running as root (not recommended)
if [[ $EUID -eq 0 ]]; then
    echo "Warning: Running as root. This script should be run as a regular user with sudo access."
    read -p "Continue anyway? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# Build the application
echo "Building smogping..."
go build -v -a -ldflags="-s -w" -o smogping main.go

# Create system user and group
echo "Creating smogping user and group..."
if ! getent group smogping >/dev/null; then
    sudo groupadd -r smogping
fi
if ! getent passwd smogping >/dev/null; then
    sudo useradd -r -g smogping -d /var/lib/smogping -s /sbin/nologin \
        -c "SmogPing monitoring service" smogping
fi

# Create directories
echo "Creating system directories..."
sudo mkdir -p /usr/bin
sudo mkdir -p /usr/share/smogping
sudo mkdir -p /etc/smogping
sudo mkdir -p /etc/sysconfig
sudo mkdir -p /var/lib/smogping

# Install binary
echo "Installing binary to /usr/bin/smogping..."
sudo install -m 0755 smogping /usr/bin/smogping

# Set capabilities for ICMP ping (requires libcap-utils package)
echo "Setting capabilities for ICMP ping..."
if command -v setcap >/dev/null 2>&1; then
    sudo setcap cap_net_raw=+ep /usr/bin/smogping
    echo "✓ Capabilities set successfully"
else
    echo "⚠ Warning: setcap command not found. Install libcap-utils package:"
    echo "  Debian/Ubuntu: sudo apt-get install libcap2-bin"
    echo "  RHEL/CentOS/Fedora: sudo yum install libcap"
    echo "  OpenSUSE: sudo zypper install libcap-progs"
    echo ""
    echo "Alternative: Run SmogPing as root (not recommended) or use sudo:"
    echo "  sudo /usr/bin/smogping"
    echo ""
    echo "After installing libcap utilities, run:"
    echo "  sudo setcap cap_net_raw=+ep /usr/bin/smogping"
fi

# Install webapp files
echo "Installing webapp files..."
sudo cp -r webapp /usr/share/smogping/

# Install configuration files
echo "Installing configuration files..."
sudo install -m 0644 config.default.toml /etc/smogping/config.default.toml
sudo install -m 0644 targets.toml /etc/smogping/targets.toml.example

# Create default config files if they don't exist
if [[ ! -f /etc/smogping/config.toml ]]; then
    sudo cp config.default.toml /etc/smogping/config.toml
    echo "Created default /etc/smogping/config.toml"
fi
if [[ ! -f /etc/smogping/targets.toml ]]; then
    sudo cp targets.toml /etc/smogping/targets.toml
    echo "Created default /etc/smogping/targets.toml"
fi

# Install webapp config
if [[ -f webapp/config.php ]]; then
    sudo install -m 0644 webapp/config.php /etc/smogping/webapp.config.php
    echo "Installed webapp configuration"
fi

# Install sysconfig file
echo "Installing service configuration..."
sudo install -m 0644 smogping.sysconfig /etc/sysconfig/smogping

# Install systemd service
echo "Installing systemd service..."
sudo install -m 0644 smogping.service /etc/systemd/system/smogping.service
sudo systemctl daemon-reload

# Set ownership
echo "Setting file ownership..."
sudo chown smogping:smogping /etc/smogping/config.toml
sudo chown smogping:smogping /etc/smogping/targets.toml
if [[ -f /etc/smogping/webapp.config.php ]]; then
    sudo chown smogping:smogping /etc/smogping/webapp.config.php
fi
sudo chown -R smogping:smogping /var/lib/smogping

# Install documentation
echo "Installing documentation..."
sudo mkdir -p /usr/share/doc/smogping
sudo install -m 0644 *.md /usr/share/doc/smogping/ 2>/dev/null || true

# Install setup script for future uninstall
echo "Installing setup script..."
sudo install -m 0755 setup.sh /usr/share/smogping/setup.sh

# Install Apache configuration if Apache is present
if command -v apache2 >/dev/null 2>&1 || command -v httpd >/dev/null 2>&1; then
    echo "Installing Apache configuration..."
    
    # Detect Apache configuration directory
    if [[ -d /etc/apache2/conf-available ]]; then
        # Debian/Ubuntu style
        sudo install -m 0644 apache.conf /etc/apache2/conf-available/smogping.conf
        echo "Apache configuration installed to /etc/apache2/conf-available/smogping.conf"
        echo "To enable, run: sudo a2enconf smogping && sudo systemctl reload apache2"
    elif [[ -d /etc/httpd/conf.d ]]; then
        # RHEL/CentOS/Fedora style
        sudo install -m 0644 apache.conf /etc/httpd/conf.d/smogping.conf
        echo "Apache configuration installed to /etc/httpd/conf.d/smogping.conf"
        echo "Restart Apache to activate: sudo systemctl restart httpd"
    elif [[ -d /etc/apache2/conf.d ]]; then
        # OpenSUSE style
        sudo install -m 0644 apache.conf /etc/apache2/conf.d/smogping.conf
        echo "Apache configuration installed to /etc/apache2/conf.d/smogping.conf"
        echo "Restart Apache to activate: sudo systemctl restart apache2"
    else
        # Install to documentation for manual setup
        sudo install -m 0644 apache.conf /usr/share/doc/smogping/smogping-apache.conf
        echo "Apache configuration saved to /usr/share/doc/smogping/smogping-apache.conf"
        echo "Please copy to your Apache configuration directory manually."
    fi
else
    echo "Apache not detected. Configuration saved to /usr/share/doc/smogping/smogping-apache.conf"
    sudo install -m 0644 smogping-apache.conf /usr/share/doc/smogping/smogping-apache.conf
fi

echo ""
echo "=== Installation Complete! ==="
echo ""
echo "SmogPing has been installed system-wide:"
echo "  Binary: /usr/bin/smogping"
echo "  Webapp: /usr/share/smogping/webapp/"
echo "  Config: /etc/smogping/"
echo "  Service: /etc/systemd/system/smogping.service"
echo "  Sysconfig: /etc/sysconfig/smogping"
echo "  Data: /var/lib/smogping/"
echo "  Apache Config: Check installation messages above"
echo ""
echo "Next steps:"
echo "1. Edit /etc/smogping/config.toml with your InfluxDB settings"
echo "2. Edit /etc/smogping/targets.toml with your monitoring targets"
echo "3. Configure webapp at /etc/smogping/webapp.config.php"
echo "4. (Optional) Edit /etc/sysconfig/smogping for service options"
echo "5. Enable Apache configuration (see messages above)"
echo "6. Enable and start the SmogPing service:"
echo "   sudo systemctl enable smogping"
echo "   sudo systemctl start smogping"
echo "7. Check service status:"
echo "   sudo systemctl status smogping"
echo "8. Access the web dashboard at: http://your-server/smogping"
echo ""
echo "To uninstall later, run:"
echo "   sudo /usr/share/smogping/setup.sh uninstall"
echo ""
echo "Or create an alias for convenience:"
echo "   echo 'alias smogping-uninstall=\"sudo /usr/share/smogping/setup.sh uninstall\"' >> ~/.bashrc"

# Uninstall function
uninstall_smogping() {
    echo "=== SmogPing Uninstallation ==="
    echo "This will remove SmogPing from your system."
    read -p "Are you sure? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Uninstall cancelled."
        exit 0
    fi
    
    echo "Stopping and disabling service..."
    sudo systemctl stop smogping 2>/dev/null || true
    sudo systemctl disable smogping 2>/dev/null || true
    
    echo "Removing files..."
    sudo rm -f /usr/bin/smogping
    sudo rm -rf /usr/share/smogping
    sudo rm -f /etc/systemd/system/smogping.service
    sudo rm -f /etc/sysconfig/smogping
    sudo rm -rf /usr/share/doc/smogping
    
    # Remove Apache configuration
    sudo rm -f /etc/apache2/conf-available/smogping.conf 2>/dev/null || true
    sudo rm -f /etc/httpd/conf.d/smogping.conf 2>/dev/null || true
    sudo rm -f /etc/apache2/conf.d/smogping.conf 2>/dev/null || true
    
    echo "Note: If you enabled the Apache configuration, disable it manually:"
    echo "  Debian/Ubuntu: sudo a2disconf smogping && sudo systemctl reload apache2"
    echo "  RHEL/CentOS/Fedora: sudo systemctl restart httpd"
    echo "  OpenSUSE: sudo systemctl restart apache2"
    
    echo "Reloading systemd..."
    sudo systemctl daemon-reload
    
    echo "Removing user and group..."
    sudo userdel smogping 2>/dev/null || true
    sudo groupdel smogping 2>/dev/null || true
    
    echo ""
    echo "SmogPing has been uninstalled."
    echo "Configuration files in /etc/smogping/ and data in /var/lib/smogping/ were preserved."
    echo "Remove them manually if desired:"
    echo "  sudo rm -rf /etc/smogping"
    echo "  sudo rm -rf /var/lib/smogping"
}

# Handle command line arguments
if [[ "$1" == "uninstall" ]]; then
    uninstall_smogping
    exit 0
fi
