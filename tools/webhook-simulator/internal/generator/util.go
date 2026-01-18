package generator

import (
	"crypto/rand"
	"encoding/hex"
)

// generateID creates a random ID string for use in webhook payloads.
func generateID() string {
	bytes := make([]byte, 12)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
