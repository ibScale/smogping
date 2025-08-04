Name:           smogping
Version:        1.0.0
Release:        1%{?dist}
Summary:        Network monitoring tool with InfluxDB integration

License:        GPL-3.0
URL:            https://github.com/your-org/smogping
Source0:        %{name}-%{version}.tar.gz
Patch0:         webapp-config-path.patch

BuildRequires:  golang >= 1.19
BuildRequires:  systemd-rpm-macros
Requires:       systemd
Requires(pre):  shadow-utils

%description
SmogPing is a Go application that monitors network quality by pinging targets 
and storing metrics in an InfluxDB v2 bucket. It measures RTT (Round Trip Time), 
Jitter, and Packet Loss with a color coded graphing dashboard.

It's a spiritual successor to the venerable SmokePing, designed for modern 
containerized environments with better scaling for 1000+ targets.

%prep
%autosetup -p1

%build
# Build the Go binary
export GO111MODULE=on
export GOPROXY=direct
go mod download
go build -v -a -ldflags="-s -w" -o smogping main.go

%install
# Create directories
install -d %{buildroot}%{_bindir}
install -d %{buildroot}%{_datadir}/%{name}
install -d %{buildroot}%{_sysconfdir}/%{name}
install -d %{buildroot}%{_unitdir}
install -d %{buildroot}%{_localstatedir}/lib/%{name}

# Install binary
install -m 0755 smogping %{buildroot}%{_bindir}/smogping

# Install webapp files
cp -r webapp %{buildroot}%{_datadir}/%{name}/

# Install webapp configuration files
install -m 0644 webapp/config.example.php %{buildroot}%{_datadir}/%{name}/webapp/config.example.php
install -m 0644 webapp/config.php %{buildroot}%{_sysconfdir}/%{name}/webapp.config.php

# Install configuration files
install -m 0644 config.default.toml %{buildroot}%{_sysconfdir}/%{name}/config.default.toml
install -m 0644 targets.toml %{buildroot}%{_sysconfdir}/%{name}/targets.toml.example

# Create default config files during build (will be managed by %post)
cp config.default.toml %{buildroot}%{_sysconfdir}/%{name}/config.toml
cp targets.toml %{buildroot}%{_sysconfdir}/%{name}/targets.toml

# Install systemd service file
install -m 0644 smogping.service %{buildroot}%{_unitdir}/smogping.service

# Install sysconfig file
install -d %{buildroot}%{_sysconfdir}/sysconfig
install -m 0644 smogping.sysconfig %{buildroot}%{_sysconfdir}/sysconfig/smogping

# Install documentation
install -d %{buildroot}%{_docdir}/%{name}
install -m 0644 README.md %{buildroot}%{_docdir}/%{name}/
install -m 0644 *.md %{buildroot}%{_docdir}/%{name}/

# Install Apache configuration as documentation
install -m 0644 smogping-apache.conf %{buildroot}%{_docdir}/%{name}/smogping-apache.conf

%pre
# Create smogping user and group
getent group smogping >/dev/null || groupadd -r smogping
getent passwd smogping >/dev/null || \
    useradd -r -g smogping -d %{_localstatedir}/lib/%{name} -s /sbin/nologin \
    -c "SmogPing monitoring service" smogping
exit 0

%post
%systemd_post smogping.service

# Set ownership of config files
chown smogping:smogping %{_sysconfdir}/%{name}/config.toml
chown smogping:smogping %{_sysconfdir}/%{name}/targets.toml
chown smogping:smogping %{_sysconfdir}/%{name}/webapp.config.php

# Set ownership of data directory
chown -R smogping:smogping %{_localstatedir}/lib/%{name}

echo "SmogPing installed successfully!"
echo ""
echo "Next steps:"
echo "1. Edit /etc/smogping/config.toml with your InfluxDB settings"
echo "2. Edit /etc/smogping/targets.toml with your monitoring targets"
echo "3. Edit /etc/smogping/webapp.config.php with your webapp settings"
echo "4. (Optional) Edit /etc/sysconfig/smogping to customize service startup options"
echo "5. Enable and start the service:"
echo "   sudo systemctl enable smogping"
echo "   sudo systemctl start smogping"
echo ""
echo "Documentation available in: %{_docdir}/%{name}/"

%preun
%systemd_preun smogping.service

%postun
%systemd_postun_with_restart smogping.service

%files
%license LICENSE.txt
%doc %{_docdir}/%{name}/
%{_bindir}/smogping
%{_datadir}/%{name}/
%{_unitdir}/smogping.service
%dir %{_sysconfdir}/%{name}
%config(noreplace) %{_sysconfdir}/%{name}/config.default.toml
%config(noreplace) %{_sysconfdir}/%{name}/targets.toml.example
%config(noreplace) %{_sysconfdir}/%{name}/config.toml
%config(noreplace) %{_sysconfdir}/%{name}/targets.toml
%config(noreplace) %{_sysconfdir}/%{name}/webapp.config.php
%config(noreplace) %{_sysconfdir}/sysconfig/%{name}
%attr(0755, smogping, smogping) %dir %{_localstatedir}/lib/%{name}

%changelog
* Tue Jul 29 2025 Package Maintainer <maintainer@example.com> - 1.0.0-1
- Initial RPM package
- Network monitoring with InfluxDB integration
- Individual ping schedules with object pooling
- Source IP configuration support
- Comprehensive alarm system
- Dynamic configuration reloading
- Syslog integration
