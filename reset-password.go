package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"syscall"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: go run reset-password.go <username>")
		fmt.Println("Example: go run reset-password.go admin")
		os.Exit(1)
	}

	username := os.Args[1]

	// Prompt for new password
	fmt.Print("Enter new password: ")
	passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		fmt.Printf("\nError reading password: %v\n", err)
		os.Exit(1)
	}
	password := string(passwordBytes)
	fmt.Println()

	if len(password) < 8 {
		fmt.Println("Password must be at least 8 characters long")
		os.Exit(1)
	}

	// Confirm password
	fmt.Print("Confirm password: ")
	confirmBytes, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		fmt.Printf("\nError reading password: %v\n", err)
		os.Exit(1)
	}
	confirm := string(confirmBytes)
	fmt.Println()

	if password != confirm {
		fmt.Println("Passwords do not match")
		os.Exit(1)
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Printf("Error hashing password: %v\n", err)
		os.Exit(1)
	}

	// Determine database path
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "swoops.db"
	}

	// Open database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Check if user exists
	var exists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE username = ?)", username).Scan(&exists)
	if err != nil {
		fmt.Printf("Error checking user: %v\n", err)
		os.Exit(1)
	}

	if !exists {
		fmt.Printf("User '%s' not found\n", username)
		os.Exit(1)
	}

	// Prompt for confirmation
	fmt.Printf("This will reset the password for user '%s'. Continue? (yes/no): ", username)
	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))

	if response != "yes" && response != "y" {
		fmt.Println("Aborted")
		os.Exit(0)
	}

	// Update password
	_, err = db.Exec("UPDATE users SET password_hash = ? WHERE username = ?", string(hashedPassword), username)
	if err != nil {
		fmt.Printf("Error updating password: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Password reset successfully for user '%s'\n", username)
}
