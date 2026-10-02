package site

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestGenerateShortIDUsesBase62AndRejectsBiasedBytes(t *testing.T) {
	t.Parallel()

	input := append(bytes.Repeat([]byte{248}, 32), bytes.Repeat([]byte{61}, 32)...)
	got, err := generateShortID(bytes.NewReader(input))
	if err != nil {
		t.Fatalf("generateShortID() error = %v", err)
	}
	if got != strings.Repeat("z", ShortIDLength) {
		t.Fatalf("generateShortID() = %q, want nine z characters", got)
	}
}

func TestGenerateShortIDReportsEntropyFailure(t *testing.T) {
	t.Parallel()

	_, err := generateShortID(bytes.NewReader(nil))
	if err == nil {
		t.Fatal("generateShortID() error = nil, want entropy error")
	}
}

func TestValidateCustomIDAcceptsCaseSensitiveRouteAlphabet(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"My_Blog-2026", "abc", "A1_b-2"} {
		if err := ValidateCustomID(value); err != nil {
			t.Errorf("ValidateCustomID(%q) error = %v", value, err)
		}
	}
	for _, value := range []string{"ab", "_blog", "blog-", "my--blog", "my_-blog", "blog space"} {
		if err := ValidateCustomID(value); !errors.Is(err, ErrInvalidCustomID) {
			t.Errorf("ValidateCustomID(%q) error = %v, want ErrInvalidCustomID", value, err)
		}
	}
}

func TestValidateShortIDAcceptsOnlyFixedWidthBase62(t *testing.T) {
	t.Parallel()

	if err := ValidateShortID("A1b2C3d4E"); err != nil {
		t.Fatalf("ValidateShortID() error = %v", err)
	}
	for _, value := range []string{"short", "A1b2C3d4-", "A1b2C3d4_", "A1b2C3d4Ef"} {
		if err := ValidateShortID(value); !errors.Is(err, ErrInvalidShortID) {
			t.Errorf("ValidateShortID(%q) error = %v, want ErrInvalidShortID", value, err)
		}
	}
}
