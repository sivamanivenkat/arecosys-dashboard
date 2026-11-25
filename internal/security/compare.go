package security

import (
	"crypto/sha256"
	"crypto/subtle"
)

func Same(a, b string) bool {
	x, y := sha256.Sum256([]byte(a)), sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(x[:], y[:]) == 1
}
