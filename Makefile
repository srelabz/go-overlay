GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
BINARY_NAME=go-supervisor
BINARY_UNIX=$(BINARY_NAME)_unix

DOCKER_IMAGE=tm-orchestrator
DOCKER_TAG=latest

.PHONY: all build clean test docker docker-run install uninstall help

all: test build

build:
	@echo "Building $(BINARY_NAME)..."
	$(GOBUILD) -o $(BINARY_NAME) -v .

build-linux:
	@echo "Building for Linux..."
	CGO_ENABLED=0 GOOS=linux $(GOBUILD) -o $(BINARY_UNIX) -v .

clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_UNIX)

test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

deps:
	@echo "Installing dependencies..."
	$(GOGET) -v ./...

install: build
	@echo "Installing go-supervisor..."
	@sudo cp $(BINARY_NAME) /go-supervisor
	@sudo chmod +x /go-supervisor
	@echo "✓ go-supervisor installed successfully!"
	@echo "Available commands:"
	@echo "  go-supervisor                    # Start daemon"
	@echo "  go-supervisor list               # List services"
	@echo "  go-supervisor status             # Show status"
	@echo "  go-supervisor restart <service>  # Restart service"

install-enhanced:
	@chmod +x tests/install-enhanced.sh
	@./tests/install-enhanced.sh --local

uninstall:
	@echo "Uninstalling go-supervisor..."
	@sudo rm -f /go-supervisor
	@sudo rm -f /usr/local/bin/entrypoint
	@sudo rm -rf /etc/tm-orchestrator
	@echo "✓ go-supervisor uninstalled"

docker: build
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

docker-release:
	@echo "Building Docker image from GitHub release (latest)..."
	docker build -f Dockerfile.release -t $(DOCKER_IMAGE):release-latest .

docker-release-version:
	@if [ -z "$(VERSION)" ]; then echo "Usage: make docker-release-version VERSION=v0.0.5"; exit 1; fi
	@echo "Building Docker image from GitHub release $(VERSION)..."
	docker build --build-arg VERSION=$(VERSION) -t $(DOCKER_IMAGE):$(VERSION) .

docker-run: docker
	@echo "Running Docker container..."
	docker run --rm -p 80:80 --name tm-test $(DOCKER_IMAGE):$(DOCKER_TAG)

docker-run-daemon: docker
	@echo "Running Docker container in background..."
	docker run -d -p 80:80 --name tm-test $(DOCKER_IMAGE):$(DOCKER_TAG)

docker-run-release:
	@echo "Running Docker container from release..."
	@$(MAKE) docker-release
	docker run --rm -p 80:80 --name tm-test $(DOCKER_IMAGE):release-latest

docker-stop:
	@echo "Stopping Docker container..."
	@docker stop tm-test || true
	@docker rm tm-test || true

docker-logs:
	@echo "Showing Docker container logs..."
	docker logs -f tm-test

test-cli: install
	@echo "Testing CLI commands..."
	@echo "Starting test container..."
	@$(MAKE) docker-run-daemon
	@sleep 5
	@echo "\nTesting go-supervisor commands:"
	@echo "1. Status:"
	@docker exec tm-test go-supervisor status || true
	@echo "\n2. List services:"
	@docker exec tm-test go-supervisor list || true
	@echo "\n3. Restart nginx:"
	@docker exec tm-test go-supervisor restart nginx || true
	@sleep 3
	@echo "\n4. List after restart:"
	@docker exec tm-test go-supervisor list || true
	@$(MAKE) docker-stop

test-graceful-shutdown:
	@chmod +x tests/test-graceful-shutdown.sh
	@./tests/test-graceful-shutdown.sh

test-container:
	@chmod +x tests/test-container.sh
	@./tests/test-container.sh

release: clean test build-linux
	@echo "Creating release..."
	@mkdir -p release
	@cp $(BINARY_UNIX) release/go-supervisor-linux-amd64
	@cp services.toml release/
	@cp tests/install-enhanced.sh release/
	@cp README.md release/
	@echo "✓ Release created in ./release/"

help:
	@echo "Available targets:"
	@echo "  build                    Build the binary"
	@echo "  build-linux              Build for Linux"
	@echo "  clean                    Clean build artifacts"
	@echo "  test                     Run tests"
	@echo "  install                  Install go-supervisor in system PATH"
	@echo "  install-enhanced         Install using enhanced script"
	@echo "  uninstall                Remove go-supervisor from system"
	@echo "  docker                   Build Docker image (local binary)"
	@echo "  docker-release           Build Docker image from latest GitHub release"
	@echo "  docker-release-version   Build Docker image from specific release VERSION=v0.0.5"
	@echo "  docker-run               Run Docker container (foreground)"
	@echo "  docker-run-daemon        Run Docker container (background)"
	@echo "  docker-run-release       Run Docker container from GitHub release"
	@echo "  docker-stop              Stop Docker container"
	@echo "  docker-logs              Show Docker container logs"
	@echo "  test-cli                 Test CLI commands with Docker"
	@echo "  test-graceful-shutdown   Test graceful shutdown functionality"
	@echo "  test-container           Test container functionality"
	@echo "  release                  Create release package"
	@echo "  help                     Show this help"
