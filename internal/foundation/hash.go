package foundation

import (
	"crypto/sha256"
	"encoding/hex"
)

func HashString(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}
