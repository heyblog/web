package site

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

const (
	ShortIDLength           = 9
	ShortIDCollisionRetries = 5
	base62Alphabet          = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	base62Limit             = byte(248)
)

var ErrInvalidShortID = errors.New("invalid short ID")

// NewShortID returns a cryptographically random, fixed-width Base62 site ID.
func NewShortID() (string, error) {
	return generateShortID(rand.Reader)
}

// ValidateShortID checks the case-sensitive fixed-width Base62 route identifier.
func ValidateShortID(value string) error {
	if len(value) != ShortIDLength {
		return ErrInvalidShortID
	}
	for index := range value {
		if !isASCIIAlphanumeric(value[index]) {
			return ErrInvalidShortID
		}
	}
	return nil
}

func generateShortID(source io.Reader) (string, error) {
	result := make([]byte, ShortIDLength)
	buffer := make([]byte, 32)
	written := 0

	for written < len(result) {
		if _, err := io.ReadFull(source, buffer); err != nil {
			return "", fmt.Errorf("read short ID entropy: %w", err)
		}
		for _, value := range buffer {
			if value >= base62Limit {
				continue
			}
			result[written] = base62Alphabet[int(value)%len(base62Alphabet)]
			written++
			if written == len(result) {
				break
			}
		}
	}

	return string(result), nil
}
