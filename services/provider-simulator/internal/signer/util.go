package signer

import (
	"crypto/rand"
	"encoding/hex"
)

// generateID creates a random ID string for use in signatures.
func generateID() string {
	bytes := make([]byte, 12)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
