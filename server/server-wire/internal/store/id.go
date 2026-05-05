package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

func randomID(prefix string) string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("generate random id: %v", err))
	}
	return prefix + "-" + hex.EncodeToString(b[:])
}
