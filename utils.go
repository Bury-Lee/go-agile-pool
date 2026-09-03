package agilepool

import (
	"crypto/rand"
	"encoding/hex"
)

func newTraceID() (string, error) {
	var b [16]byte
	_, err := rand.Read(b[:])
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
