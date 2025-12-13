package nip04

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

const (
	// EncryptedDirectMessageKind represents NIP-04 encrypted direct messages
	EncryptedDirectMessageKind = 4
)

var (
	ErrInvalidContentFormat = errors.New("invalid content format, expected '?iv=' separator")
	ErrInvalidIVLength      = errors.New("invalid IV length")
	ErrDecryptionFailed     = errors.New("decryption failed")
)

// IsEncryptedDirectMessage checks if an event is a NIP-04 encrypted direct message
func IsEncryptedDirectMessage(kind int) bool {
	return kind == EncryptedDirectMessageKind
}

// GetRecipientPubKey extracts the recipient's public key from NIP-04 event tags
func GetRecipientPubKey(tags [][]string) (string, bool) {
	for _, tag := range tags {
		if len(tag) >= 2 && tag[0] == "p" {
			return tag[1], true
		}
	}
	return "", false
}

// ParseContent parses NIP-04 content format "ciphertext?iv=iv"
func ParseContent(content string) (ciphertext, iv string, err error) {
	parts := strings.SplitN(content, "?iv=", 2)
	if len(parts) != 2 {
		return "", "", ErrInvalidContentFormat
	}

	ciphertext = parts[0]
	iv = parts[1]

	if len(iv) == 0 {
		return "", "", ErrInvalidIVLength
	}

	return ciphertext, iv, nil
}

// Encrypt encrypts plaintext using AES-256-CBC with the provided key
func Encrypt(plaintext string, key []byte) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("key must be 32 bytes for AES-256")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create ciphertext with padding
	plaintextBytes := []byte(plaintext)
	padding := aes.BlockSize - len(plaintextBytes)%aes.BlockSize
	ciphertext := make([]byte, len(plaintextBytes)+padding)
	copy(ciphertext, plaintextBytes)
	for i := len(plaintextBytes); i < len(ciphertext); i++ {
		ciphertext[i] = byte(padding)
	}

	// Generate random IV
	iv := make([]byte, aes.BlockSize)
	if _, err := rand.Read(iv); err != nil {
		return "", fmt.Errorf("failed to generate IV: %w", err)
	}

	// Encrypt
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, ciphertext)

	// Return in NIP-04 format: base64(ciphertext)?iv=base64(iv)
	return base64.StdEncoding.EncodeToString(ciphertext) + "?iv=" + base64.StdEncoding.EncodeToString(iv), nil
}

// Decrypt decrypts NIP-04 content using the provided key
func Decrypt(content string, key []byte) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("key must be 32 bytes for AES-256")
	}

	// Parse content
	ciphertextB64, ivB64, err := ParseContent(content)
	if err != nil {
		return "", fmt.Errorf("failed to parse content: %w", err)
	}

	// Decode base64
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	iv, err := base64.StdEncoding.DecodeString(ivB64)
	if err != nil {
		return "", fmt.Errorf("failed to decode IV: %w", err)
	}

	if len(iv) != aes.BlockSize {
		return "", ErrInvalidIVLength
	}

	if len(ciphertext)%aes.BlockSize != 0 {
		return "", fmt.Errorf("ciphertext is not a multiple of block size")
	}

	// Decrypt
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	// Remove padding
	if len(plaintext) == 0 {
		return "", ErrDecryptionFailed
	}

	padding := int(plaintext[len(plaintext)-1])
	if padding == 0 || padding > aes.BlockSize || padding > len(plaintext) {
		return "", ErrDecryptionFailed
	}

	// Verify padding
	for i := len(plaintext) - padding; i < len(plaintext); i++ {
		if plaintext[i] != byte(padding) {
			return "", ErrDecryptionFailed
		}
	}

	return string(plaintext[:len(plaintext)-padding]), nil
}

// ValidateContent checks if content follows NIP-04 format
func ValidateContent(content string) error {
	_, _, err := ParseContent(content)
	return err
}
