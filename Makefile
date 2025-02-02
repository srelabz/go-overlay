BINARY_NAME=entrypoint
VERSION=$(shell git describe --tags --always --dirty)
BUILD_DIR=build

PLATFORMS=linux darwin

.PHONY: all clean build $(PLATFORMS) linux

all: clean build

clean:
	rm -rf $(BUILD_DIR)

build: $(PLATFORMS)

linux:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
		-ldflags "-X main.version=$(VERSION)" \
		-o $(BUILD_DIR)/$(BINARY_NAME) \
		main.go

$(PLATFORMS):
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=$@ GOARCH=amd64 go build \
		-ldflags "-X main.version=$(VERSION)" \
		-o $(BUILD_DIR)/$(BINARY_NAME)-$@-amd64 \
		main.go 