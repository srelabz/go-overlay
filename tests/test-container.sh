#!/bin/bash

set -e

echo "|=== TM Orchestrator Test Container ===|"

echo "Building Docker image..."
docker build -t tm-orchestrator-test .

echo "Docker image built successfully!"

echo "Starting container..."
echo "Press Ctrl+C to test graceful shutdown"
echo "You can also test with: docker kill --signal=SIGTERM <container_id>"
echo ""

docker run --rm -p 8080:80 --name tm-test tm-orchestrator-test

echo "Container stopped."
