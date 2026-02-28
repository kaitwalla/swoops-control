package models

import (
	"crypto/rand"
	"fmt"
	"time"
)

// NewID generates a time-sortable unique ID (simplified ULID-like).
// Format: 10 chars timestamp (ms hex) + 12 chars random hex = 22 chars.
func NewID() string {
	ms := time.Now().UnixMilli()
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%010x%012x", ms, b)
}
