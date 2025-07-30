# SmogPing Makefile

.PHONY: build run clean test deps

# Build the application
build:
	go build -o smogping main.go

# Run the application
run:
	go run main.go

# Clean build artifacts
clean:
	rm -f smogping

# Run tests (when we add them)
test:
	go test ./...

# Install/update dependencies
deps:
	go mod tidy
	go mod download

# Build for different platforms
build-linux:
	GOOS=linux GOARCH=amd64 go build -o smogping-linux-amd64 main.go

build-windows:
	GOOS=windows GOARCH=amd64 go build -o smogping-windows-amd64.exe main.go

build-mac:
	GOOS=darwin GOARCH=amd64 go build -o smogping-darwin-amd64 main.go

# Build all platforms
build-all: build-linux build-windows build-mac

# Check for required permissions (ping often requires special permissions)
check-permissions:
	@echo "Checking if we can create ICMP sockets..."
	@go run -c 'package main; import "github.com/go-ping/ping"; func main() { p, _ := ping.NewPinger("127.0.0.1"); p.SetPrivileged(false); println("OK") }' || echo "May need to run with sudo or set capabilities"
