GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
BINARY_NAME=service-manager
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
	@echo "Installing tm-orchestrator..."
	@sudo cp $(BINARY_NAME) /usr/local/bin/tm-orchestrator
	@sudo chmod +x /usr/local/bin/tm-orchestrator
	@sudo ln -sf /usr/local/bin/tm-orchestrator /usr/local/bin/entrypoint
	@echo "✓ tm-orchestrator installed successfully!"
	@echo "Available commands:"
	@echo "  tm-orchestrator                    # Start daemon"
	@echo "  tm-orchestrator list               # List services"
	@echo "  tm-orchestrator status             # Show status"
	@echo "  tm-orchestrator restart <service>  # Restart service"

install-enhanced:
	@chmod +x tests/install-enhanced.sh
	@./tests/install-enhanced.sh --local

uninstall:
	@echo "Uninstalling tm-orchestrator..."
	@sudo rm -f /usr/local/bin/tm-orchestrator
	@sudo rm -f /usr/local/bin/entrypoint
	@sudo rm -rf /etc/tm-orchestrator
	@echo "✓ tm-orchestrator uninstalled"

docker: build
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

docker-run: docker
	@echo "Running Docker container..."
	docker run --rm -p 80:80 --name tm-test $(DOCKER_IMAGE):$(DOCKER_TAG)

docker-run-daemon: docker
	@echo "Running Docker container in background..."
	docker run -d -p 80:80 --name tm-test $(DOCKER_IMAGE):$(DOCKER_TAG)

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
	@echo "\nTesting tm-orchestrator commands:"
	@echo "1. Status:"
	@docker exec tm-test tm-orchestrator status || true
	@echo "\n2. List services:"
	@docker exec tm-test tm-orchestrator list || true
	@echo "\n3. Restart nginx:"
	@docker exec tm-test tm-orchestrator restart nginx || true
	@sleep 3
	@echo "\n4. List after restart:"
	@docker exec tm-test tm-orchestrator list || true
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
	@cp $(BINARY_UNIX) release/service-manager-linux-amd64
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
	@echo "  install                  Install tm-orchestrator in system PATH"
	@echo "  install-enhanced         Install using enhanced script"
	@echo "  uninstall                Remove tm-orchestrator from system"
	@echo "  docker                   Build Docker image"
	@echo "  docker-run               Run Docker container (foreground)"
	@echo "  docker-run-daemon        Run Docker container (background)"
	@echo "  docker-stop              Stop Docker container"
	@echo "  docker-logs              Show Docker container logs"
	@echo "  test-cli                 Test CLI commands with Docker"
	@echo "  test-graceful-shutdown   Test graceful shutdown functionality"
	@echo "  test-container           Test container functionality"
	@echo "  release                  Create release package"
	@echo "  help                     Show this help"
