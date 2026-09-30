// Package correlation mints the handle a caller uses to trace one message.
package correlation

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// NewID returns 12 random bytes, hex-encoded.
func NewID() (string, error) {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generating correlation id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
