#!/bin/bash

# Test the complete authentication flow locally

set -e

echo "=== Testing Swoops Authentication Flow ==="
echo ""

# Clean up old test database
rm -f test-swoops.db

# Build the binary
echo "1. Building swoopsd..."
make build-server > /dev/null 2>&1
echo "   ✓ Build complete"

# Create a test user with a known password
TEST_PASSWORD="test123456"
echo ""
echo "2. Creating test user with password: $TEST_PASSWORD"
echo -e "$TEST_PASSWORD\n$TEST_PASSWORD" | DB_PATH=test-swoops.db ./bin/swoopsd create-user testuser test@example.com
echo ""

# Check what's in the database
echo "3. Checking database..."
sqlite3 test-swoops.db "SELECT username, length(password_hash) as hash_len, substr(password_hash, 1, 10) as hash_start FROM users;"
echo ""

# Start the server in background
echo "4. Starting server..."
DB_PATH=test-swoops.db ./bin/swoopsd --config /dev/null > swoops-test.log 2>&1 &
SERVER_PID=$!
echo "   Server PID: $SERVER_PID"

# Wait for server to start
sleep 3

# Test login
echo ""
echo "5. Testing login via API..."
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"testuser\",\"password\":\"$TEST_PASSWORD\"}")

HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | head -n-1)

echo "   HTTP Status: $HTTP_CODE"
echo "   Response: $BODY"

# Kill server
kill $SERVER_PID 2>/dev/null || true

echo ""
if [ "$HTTP_CODE" = "200" ]; then
    echo "✓ SUCCESS: Login worked!"
    exit 0
else
    echo "✗ FAILED: Login returned $HTTP_CODE"
    echo ""
    echo "Server logs:"
    tail -20 swoops-test.log
    exit 1
fi
