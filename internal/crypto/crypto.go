package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
)

// DeriveKey converts an arbitrary-length password into a 32-byte AES-256 key
// using SHA-256. Used for deriving the backup encryption key from the user's
// master password.
func DeriveKey(password string) []byte {
	hash := sha256.Sum256([]byte(password))
	return hash[:]
}

// Encrypt encrypts plaintext using AES-256-GCM with the provided 32-byte key.
// A random 12-byte nonce is generated for each call and prepended to the
// returned ciphertext: [ nonce (12 bytes) | ciphertext + tag ].
func Encrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("błąd tworzenia szyfru AES: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("błąd tworzenia GCM: %w", err)
	}

	// Generate a fresh random nonce for every encryption operation.
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("błąd generowania nonce: %w", err)
	}

	// Seal appends the encrypted+authenticated ciphertext after the nonce.
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts data produced by Encrypt. It expects the nonce prepended
// to the ciphertext and verifies the GCM authentication tag automatically —
// any tampering returns an error.
func Decrypt(key, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("błąd tworzenia szyfru AES: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("błąd tworzenia GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New("dane są zbyt krótkie — brakuje nonce")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		// This fires on wrong key or tampered data.
		return nil, errors.New("odszyfrowanie nie powiodło się — nieprawidłowy klucz lub dane zostały zmienione")
	}

	return plaintext, nil
}
