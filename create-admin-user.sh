#!/bin/bash

# Script to create an initial admin user for Swoops
# Usage: ./create-admin-user.sh <username> <email>
# Password will be read securely from stdin

set -e

# Create unique temp file and ensure cleanup
TEMP_GO_FILE=$(mktemp /tmp/create_swoops_user.XXXXXX.go)
trap "rm -f '$TEMP_GO_FILE'" EXIT INT TERM

if [ "$#" -ne 2 ]; then
    echo "Usage: $0 <username> <email>"
    echo "Password will be read securely from stdin"
    echo "Example: $0 admin admin@example.com"
    exit 1
fi

USERNAME="$1"
EMAIL="$2"

# Create a temporary Go program to create the user
cat > "$TEMP_GO_FILE" <<'EOF'
package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/kaitwalla/swoops-control/pkg/models"
	"github.com/kaitwalla/swoops-control/server/internal/store"
)

func main() {
	if len(os.Args) != 4 {
		log.Fatal("Usage: create_user <username> <email> <password>")
	}

	username := os.Args[1]
	email := os.Args[2]
	password := os.Args[3]

	// Open database
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./swoops.db"
	}

	s, err := store.New(dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	// Create admin user
	req := &models.CreateUserRequest{
		Username: username,
		Email:    email,
		Password: password,
		FullName: "Administrator",
		IsAdmin:  true,
	}

	user, err := s.CreateUser(req)
	if err != nil {
		log.Fatalf("Failed to create user: %v", err)
	}

	fmt.Printf("✓ Admin user created successfully!\n")
	fmt.Printf("  ID:       %s\n", user.ID)
	fmt.Printf("  Username: %s\n", user.Username)
	fmt.Printf("  Email:    %s\n", user.Email)
	fmt.Printf("  Is Admin: %v\n", user.IsAdmin)
	fmt.Printf("  Created:  %s\n", user.CreatedAt.Format(time.RFC3339))
	fmt.Printf("\nYou can now login at the web interface with these credentials.\n")
}
EOF

# Build and run the temporary program
echo "Creating admin user..."
cd server
DB_PATH="../swoops.db" go run "$TEMP_GO_FILE" "$USERNAME" "$EMAIL"

echo ""
echo "Admin user creation complete!"
