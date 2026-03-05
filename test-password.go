package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: go run test-password.go <hash>")
		fmt.Println("This will prompt for a password and test if it matches the hash")
		os.Exit(1)
	}

	hash := os.Args[1]

	// Read password
	var password string
	if term.IsTerminal(int(syscall.Stdin)) {
		fmt.Print("Enter password to test: ")
		passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			fmt.Printf("Error reading password: %v\n", err)
			os.Exit(1)
		}
		password = string(passwordBytes)
	} else {
		reader := bufio.NewReader(os.Stdin)
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Error reading password: %v\n", err)
			os.Exit(1)
		}
		password = strings.TrimSpace(line)
	}

	fmt.Printf("Password (raw bytes): %v\n", []byte(password))
	fmt.Printf("Password (length): %d\n", len(password))
	fmt.Printf("Password (trimmed): '%s'\n", strings.TrimSpace(password))
	fmt.Printf("Hash: %s\n\n", hash)

	// Test original password
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err == nil {
		fmt.Println("✓ Password matches (exact)")
	} else {
		fmt.Printf("✗ Password doesn't match (exact): %v\n", err)
	}

	// Test trimmed password
	trimmed := strings.TrimSpace(password)
	err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(trimmed))
	if err == nil {
		fmt.Println("✓ Password matches (trimmed)")
	} else {
		fmt.Printf("✗ Password doesn't match (trimmed): %v\n", err)
	}
}
