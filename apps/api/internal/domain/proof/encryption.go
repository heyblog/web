package proof

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	"errors"
)

// Encrypt uses the key identifier as authenticated data, preventing ciphertext swaps.
func Encrypt(master []byte, keyID string, key *ecdsa.PrivateKey) ([]byte, error) {
	if key == nil || keyID == "" {
		return nil, ErrInvalid
	}
	aead, err := keyCipher(master)
	if err != nil {
		return nil, err
	}
	encoded, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	return aead.Seal(nonce, nonce, encoded, []byte(keyID)), nil
}
func Decrypt(master []byte, keyID string, ciphertext []byte) (*ecdsa.PrivateKey, error) {
	aead, err := keyCipher(master)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < aead.NonceSize()+aead.Overhead() {
		return nil, ErrInvalid
	}
	encoded, err := aead.Open(nil, ciphertext[:aead.NonceSize()], ciphertext[aead.NonceSize():], []byte(keyID))
	if err != nil {
		return nil, ErrInvalid
	}
	return x509.ParseECPrivateKey(encoded)
}
func keyCipher(master []byte) (cipher.AEAD, error) {
	if len(master) != 32 {
		return nil, errors.New("request signing encryption key must contain 32 bytes")
	}
	block, err := aes.NewCipher(master)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
