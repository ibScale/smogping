#!/bin/bash
# test-rpm-build.sh - Test SmogPing RPM build

set -e

echo "=== SmogPing RPM Build Test ==="

# Check if we're in the right directory
if [[ ! -f "main.go" ]] || [[ ! -f "smogping.spec" ]]; then
    echo "Error: Run this script from the SmogPing source directory"
    exit 1
fi

# Create build environment
echo "Setting up RPM build environment..."
mkdir -p ~/rpmbuild/{BUILD,RPMS,SOURCES,SPECS,SRPMS}
echo '%_topdir %(echo $HOME)/rpmbuild' > ~/.rpmmacros

# Create source tarball
echo "Creating source tarball..."
tar --exclude='.git*' --exclude='*.rpm' --exclude='rpmbuild' \
    --exclude='test-rpm-build.sh' \
    -czf ~/rpmbuild/SOURCES/smogping-1.0.0.tar.gz \
    --transform 's,^\.,smogping-1.0.0,' .

# Copy patch file to SOURCES
if [[ -f "webapp-config-path.patch" ]]; then
    cp webapp-config-path.patch ~/rpmbuild/SOURCES/
    echo "✓ Copied patch file"
else
    echo "Warning: Patch file webapp-config-path.patch not found"
fi

# Copy spec file
cp smogping.spec ~/rpmbuild/SPECS/

# Check for required files
echo "Checking required files..."
required_files=(
    "main.go"
    "config.default.toml" 
    "targets.toml"
    "smogping.service"
    "smogping.sysconfig"
    "smogping-apache.conf"
    "README.md"
    "LICENSE.txt"
    "webapp-config-path.patch"
)

for file in "${required_files[@]}"; do
    if [[ ! -f "$file" ]]; then
        echo "Warning: Required file $file not found"
    else
        echo "✓ Found $file"
    fi
done

# Build package
echo "Building RPM package..."
if rpmbuild -ba ~/rpmbuild/SPECS/smogping.spec; then
    echo ""
    echo "=== Build completed successfully! ==="
else
    echo ""
    echo "=== Build failed! ==="
    echo "Check the errors above and:"
    echo "1. Ensure all required files exist"
    echo "2. Check Go modules with: go mod tidy"
    echo "3. Verify systemd service file exists"
    exit 1
fi
echo ""
echo "Built packages:"
ls -la ~/rpmbuild/RPMS/*/smogping* 2>/dev/null || echo "No binary RPMs found"
ls -la ~/rpmbuild/SRPMS/smogping* 2>/dev/null || echo "No source RPMs found"

echo ""
echo "To install the package:"
echo "sudo rpm -ivh ~/rpmbuild/RPMS/x86_64/smogping-*.rpm"

echo ""
echo "To test the package:"
echo "rpmlint ~/rpmbuild/RPMS/x86_64/smogping-*.rpm"
