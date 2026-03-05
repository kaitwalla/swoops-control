#!/bin/bash

set -e

echo "=== Testing Swoops in Docker ==="
echo ""

# Clean up any existing container
docker rm -f swoops-test 2>/dev/null || true

# Create a fresh database FIRST
rm -f test-swoops.db
echo "Creating test user in database..."
echo -e "testpass123\ntestpass123" | DB_PATH=test-swoops.db ./bin/swoopsd create-user admin admin@test.com

# Verify user was created
echo ""
echo "Verifying user in database:"
sqlite3 test-swoops.db "SELECT username, length(password_hash) FROM users;"
echo ""

# Start container with the debug binary and database
echo "Starting Docker container..."
docker run -d --name swoops-test \
  -p 18080:8080 \
  -v "$(pwd)/bin/swoopsd-linux-debug:/usr/local/bin/swoopsd:ro" \
  -v "$(pwd)/test-swoops.db:/app/swoops.db:rw" \
  -w /app \
  ubuntu:22.04 \
  /usr/local/bin/swoopsd

echo "Waiting for server to start..."
sleep 3

echo ""
echo "Testing login with correct password..."
curl -v -X POST http://localhost:18080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"testpass123"}' \
  2>&1 | grep -E "< HTTP|username|password|token|401|200"

echo ""
echo ""
echo "Server logs:"
docker logs swoops-test 2>&1 | tail -30

echo ""
echo "Cleaning up..."
docker rm -f swoops-test
