.PHONY: all build build-frontend build-backend run dev clean install-deps

# Variables
BINARY_NAME=fastcp
GO_BUILD_FLAGS=-ldflags="-s -w"

all: build

# Install dependencies
install-deps:
	@echo "Installing Go dependencies..."
	cd . && go mod download
	@echo "Installing Node dependencies..."
	cd web && npm install

# Build frontend
build-frontend:
	@echo "Building frontend..."
	cd web && npm run build

# Build backend
build-backend:
	@echo "Building backend..."
	CGO_ENABLED=0 go build $(GO_BUILD_FLAGS) -o bin/$(BINARY_NAME) ./cmd/fastcp

# Build both
build: build-frontend build-backend
	@echo "Build complete! Binary at bin/$(BINARY_NAME)"

# Run in development mode
dev:
	@echo "Starting FastCP in development mode..."
	@echo "Data directory: ./.fastcp/"
	@echo "Admin panel: http://localhost:8080"
	@echo "Proxy: http://localhost:8000"
	@echo ""
	FASTCP_DEV=1 go run ./cmd/fastcp

# Run frontend dev server (with hot reload)
dev-frontend:
	cd web && npm run dev

# Run production binary
run: build
	./bin/$(BINARY_NAME)

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf web/node_modules/
	rm -rf static/

# Format code
fmt:
	go fmt ./...
	cd web && npm run lint --fix 2>/dev/null || true

# Run tests
test:
	go test -v ./...

# Create release
release: clean build
	@echo "Creating release archive..."
	tar -czf fastcp-$(shell uname -s | tr '[:upper:]' '[:lower:]')-$(shell uname -m).tar.gz -C bin $(BINARY_NAME)

# Help
help:
	@echo "FastCP Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make install-deps   Install all dependencies"
	@echo "  make build          Build frontend and backend"
	@echo "  make dev            Run in development mode (uses ./.fastcp/)"
	@echo "  make dev-frontend   Run frontend dev server with hot reload"
	@echo "  make run            Build and run production binary"
	@echo "  make clean          Clean build artifacts"
	@echo "  make test           Run tests"
	@echo "  make release        Create release archive"
	@echo ""
	@echo "Environment Variables:"
	@echo "  FASTCP_DEV=1        Enable development mode (local directories)"
	@echo "  FASTCP_DATA_DIR     Override data directory"
	@echo "  FASTCP_SITES_DIR    Override sites directory"
	@echo "  FASTCP_LOG_DIR      Override log directory"
	@echo "  FASTCP_CONFIG_DIR   Override config directory"
	@echo "  FASTCP_BINARY       Override FrankenPHP binary path"
	@echo "  FASTCP_PORT         Override proxy HTTP port (default: 80/8000)"
	@echo "  FASTCP_SSL_PORT     Override proxy HTTPS port (default: 443/8443)"
	@echo "  FASTCP_LISTEN       Override admin panel listen address"

