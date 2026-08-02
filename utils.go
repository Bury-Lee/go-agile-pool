package agilepool

import (
	"crypto/rand"
	"encoding/hex"
)

// newTraceID generates a random 16-byte hex-encoded trace identifier.
func newTraceID() (string, error) {
	var b [16]byte
	_, err := rand.Read(b[:])
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
