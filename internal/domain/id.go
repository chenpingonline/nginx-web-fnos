package domain

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
)

var idPattern = regexp.MustCompile(`^[a-f0-9]{12,64}$`)

func RandomID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Sprintf("generate random id: %v", err))
	}
	return hex.EncodeToString(buf)
}

func ValidID(value string) bool {
	return idPattern.MatchString(value)
}
