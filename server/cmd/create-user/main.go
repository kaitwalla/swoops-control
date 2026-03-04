package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/kaitwalla/swoops-control/pkg/models"
	"github.com/kaitwalla/swoops-control/server/internal/store"
	"golang.org/x/term"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: create-user <username> <email>")
		fmt.Println("Password will be read securely from stdin")
		fmt.Println("Example: create-user admin admin@example.com")
		os.Exit(1)
	}

	username := os.Args[1]
	email := os.Args[2]

	// Read password securely from stdin
	var password string
	if term.IsTerminal(int(syscall.Stdin)) {
		// Interactive mode: prompt for password without echo
		fmt.Print("Enter password: ")
		passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println() // New line after password input
		if err != nil {
			log.Fatalf("Failed to read password: %v", err)
		}
		password = string(passwordBytes)

		// Confirm password
		fmt.Print("Confirm password: ")
		confirmBytes, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println() // New line after confirmation
		if err != nil {
			log.Fatalf("Failed to read password confirmation: %v", err)
		}

		if password != string(confirmBytes) {
			log.Fatal("Passwords do not match")
		}
	} else {
		// Non-interactive mode: read from pipe/stdin
		reader := bufio.NewReader(os.Stdin)
		line, err := reader.ReadString('\n')
		if err != nil {
			log.Fatalf("Failed to read password from stdin: %v", err)
		}
		password = strings.TrimSpace(line)
	}

	if password == "" {
		log.Fatal("Password cannot be empty")
	}

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
