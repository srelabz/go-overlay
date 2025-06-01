#!/bin/bash

set -e

echo "|=== Testing TM Orchestrator Graceful Shutdown ===|"

if ! docker image inspect tm-orchestrator-test > /dev/null 2>&1; then
  echo "Building Docker image..."
  docker build -t tm-orchestrator-test .
fi

echo "Starting container in background..."
CONTAINER_ID=$(docker run -d -p 8080:80 --name tm-test-graceful tm-orchestrator-test)
echo "Container started with ID: $CONTAINER_ID"

echo "Waiting for services to start..."
sleep 10

echo "Testing nginx..."
if curl -s http://localhost:8080 > /dev/null; then
  echo "✓ Nginx is responding"
else
  echo "✗ Nginx is not responding"
fi

echo ""
echo "|=== Container Logs ===|"
docker logs $CONTAINER_ID --tail 20

echo ""
echo "|=== Testing Graceful Shutdown with SIGTERM ===|"
echo "Sending SIGTERM to container..."
docker kill --signal=SIGTERM $CONTAINER_ID

sleep 2
if docker ps --filter "id=$CONTAINER_ID" --format "{{.ID}}" | grep -q $CONTAINER_ID; then
  echo "Container is still running, waiting for graceful shutdown..."
  sleep 10
fi

echo ""
echo "|=== Final Container Logs ===|"
docker logs $CONTAINER_ID --tail 30

echo ""
echo "Cleaning up..."
docker rm -f $CONTAINER_ID 2>/dev/null || true

echo "Test completed!"
