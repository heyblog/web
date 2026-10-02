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

var (
	ErrInvalidCustomID = errors.New("invalid custom ID")
	ErrInvalidShortID  = errors.New("invalid short ID")
)

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

// ValidateCustomID checks the case-sensitive custom route grammar before persistence.
func ValidateCustomID(value string) error {
	if len(value) < 3 || len(value) > 32 || !isASCIIAlphanumeric(value[0]) || !isASCIIAlphanumeric(value[len(value)-1]) {
		return ErrInvalidCustomID
	}
	previousSeparator := false
	for index := range value {
		character := value[index]
		if isASCIIAlphanumeric(character) {
			previousSeparator = false
			continue
		}
		separator := character == '-' || character == '_'
		if !separator || previousSeparator {
			return ErrInvalidCustomID
		}
		previousSeparator = true
	}
	return nil
}

func isASCIIAlphanumeric(character byte) bool {
	return character >= '0' && character <= '9' ||
		character >= 'A' && character <= 'Z' ||
		character >= 'a' && character <= 'z'
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
