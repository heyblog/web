package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

const SignatureHeader = "X-Zhblogs-Signature-256"

func Sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func Verify(secret, header string, body []byte) bool {
	if secret == "" || header == "" {
		return false
	}

	expected := Sign(secret, body)
	return hmac.Equal([]byte(expected), []byte(strings.TrimSpace(header)))
}
