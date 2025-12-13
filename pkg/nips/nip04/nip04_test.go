package nip04

import (
	"testing"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip04"
	"github.com/stretchr/testify/assert"
)

func TestParseContent(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		expectError bool
		expectedCT  string
		expectedIV  string
	}{
		{
			name:        "valid content",
			content:     "ciphertext?iv=initialization_vector",
			expectError: false,
			expectedCT:  "ciphertext",
			expectedIV:  "initialization_vector",
		},
		{
			name:        "missing iv separator",
			content:     "ciphertextiv",
			expectError: true,
		},
		{
			name:        "empty iv",
			content:     "ciphertext?iv=",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ct, iv, err := ParseContent(tt.content)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedCT, ct)
				assert.Equal(t, tt.expectedIV, iv)
			}
		})
	}
}

func TestGetRecipientPubKey(t *testing.T) {
	tests := []struct {
		name          string
		tags          [][]string
		expectedKey   string
		expectedFound bool
	}{
		{
			name:          "single p tag",
			tags:          [][]string{{"p", "pubkey1"}},
			expectedKey:   "pubkey1",
			expectedFound: true,
		},
		{
			name:          "multiple tags with p tag",
			tags:          [][]string{{"e", "event1"}, {"p", "pubkey1"}, {"p", "pubkey2"}},
			expectedKey:   "pubkey1", // returns first found
			expectedFound: true,
		},
		{
			name:          "no p tag",
			tags:          [][]string{{"e", "event1"}, {"t", "tag1"}},
			expectedKey:   "",
			expectedFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, found := GetRecipientPubKey(tt.tags)
			assert.Equal(t, tt.expectedKey, key)
			assert.Equal(t, tt.expectedFound, found)
		})
	}
}

func TestIsEncryptedDirectMessage(t *testing.T) {
	assert.True(t, IsEncryptedDirectMessage(EncryptedDirectMessageKind))
	assert.False(t, IsEncryptedDirectMessage(1))
	assert.False(t, IsEncryptedDirectMessage(14))
}

func TestEncryptDecrypt(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	plaintext := "Hello, NIP-04 encryption!"

	// Test encryption
	encrypted, err := Encrypt(plaintext, key)
	assert.NoError(t, err)
	assert.NotEmpty(t, encrypted)

	// Validate format
	assert.NoError(t, ValidateContent(encrypted))

	// Test decryption
	decrypted, err := Decrypt(encrypted, key)
	assert.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestEncryptWithInvalidKey(t *testing.T) {
	key := make([]byte, 16) // Wrong key size
	plaintext := "test"

	_, err := Encrypt(plaintext, key)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key must be 32 bytes")
}

func TestDecryptWithInvalidKey(t *testing.T) {
	key := make([]byte, 16) // Wrong key size
	content := "ciphertext?iv=dGVzdA=="

	_, err := Decrypt(content, key)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key must be 32 bytes")
}

func TestEncryptDecryptWithNostrLibrary(t *testing.T) {
	// Generate test keys
	senderPrivKey := nostr.GeneratePrivateKey()
	senderPubKey, _ := nostr.GetPublicKey(senderPrivKey)

	receiverPrivKey := nostr.GeneratePrivateKey()
	receiverPubKey, _ := nostr.GetPublicKey(receiverPrivKey)

	// Test NIP-04 encryption/decryption workflow
	plaintext := "Test message from sender to receiver"

	// Compute shared secret (sender's perspective)
	conversationKey, err := nip04.ComputeSharedSecret(receiverPubKey, senderPrivKey)
	assert.NoError(t, err)
	assert.Len(t, conversationKey, 32)

	// Encrypt message
	encrypted, err := Encrypt(plaintext, conversationKey)
	assert.NoError(t, err)

	// Create event
	event := &nostr.Event{
		PubKey:    senderPubKey,
		CreatedAt: nostr.Now(),
		Kind:      EncryptedDirectMessageKind,
		Tags:      nostr.Tags{{"p", receiverPubKey}},
		Content:   encrypted,
	}

	err = event.Sign(senderPrivKey)
	assert.NoError(t, err)

	// Verify event structure
	assert.True(t, IsEncryptedDirectMessage(event.Kind))
	tags := make([][]string, len(event.Tags))
	for i, tag := range event.Tags {
		tags[i] = tag
	}
	recipient, found := GetRecipientPubKey(tags)
	assert.True(t, found)
	assert.Equal(t, receiverPubKey, recipient)

	// Receiver decrypts
	receiverConversationKey, err := nip04.ComputeSharedSecret(event.PubKey, receiverPrivKey)
	assert.NoError(t, err)

	decrypted, err := Decrypt(event.Content, receiverConversationKey)
	assert.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}
