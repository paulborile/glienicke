package integration

import (
	"testing"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip04"
	"github.com/nbd-wtf/go-nostr/nip44"
	"github.com/paul/glienicke/internal/testutil"
	"github.com/stretchr/testify/assert"
)

func TestNIP44_EncryptedEvent(t *testing.T) {
	// Setup: Relay and clients
	url, _, cleanup, _ := setupRelay(t)
	defer cleanup()

	// Generate sender and receiver key pairs
	senderSecretKey := nostr.GeneratePrivateKey()
	senderPublicKey, _ := nostr.GetPublicKey(senderSecretKey)

	receiverSecretKey := nostr.GeneratePrivateKey()
	receiverPublicKey, _ := nostr.GetPublicKey(receiverSecretKey)

	// Create a NIP-44 encrypted message
	plaintext := "Hello, NIP-44!"
	conversationKey, err := nip04.ComputeSharedSecret(receiverPublicKey, senderSecretKey)
	assert.NoError(t, err, "Failed to get shared secret")
	var key32Byte [32]byte
	copy(key32Byte[:], conversationKey)
	ciphertext, err := nip44.Encrypt(plaintext, key32Byte)
	assert.NoError(t, err, "Failed to encrypt message with NIP-44")

	// Create and sign the event
	nostrEvent := nostr.Event{
		PubKey:    senderPublicKey,
		CreatedAt: nostr.Now(),
		Kind:      nostr.KindEncryptedDirectMessage,
		Tags:      nostr.Tags{{"p", receiverPublicKey}},
		Content:   ciphertext,
	}
	err = nostrEvent.Sign(senderSecretKey)
	assert.NoError(t, err, "Failed to sign event")

	localEvent := convertNostrEventToLocalEvent(&nostrEvent)

	// Sender client connects and sends the event
	senderClient, err := testutil.NewWSClient(url)
	assert.NoError(t, err, "Failed to create sender WebSocket client")
	defer senderClient.Close()

	err = senderClient.SendEvent(localEvent)
	assert.NoError(t, err, "Failed to send event from sender")

	// Expect OK response
	accepted, msg, err := senderClient.ExpectOK(localEvent.ID, 5*time.Second)
	assert.NoError(t, err, "Failed to receive OK from sender")
	assert.True(t, accepted, "Event was not accepted by relay: %s", msg)

	// Receiver client connects and subscribes
	receiverClient, err := testutil.NewWSClient(url)
	assert.NoError(t, err, "Failed to create receiver WebSocket client")
	defer receiverClient.Close()

	nostrFilter := nostr.Filter{
		Kinds: []int{nostr.KindEncryptedDirectMessage},
		Tags:  nostr.TagMap{"p": []string{receiverPublicKey}},
	}
	localFilter := convertNostrFilterToLocalFilter(&nostrFilter)

	err = receiverClient.SendReq("nip44-sub", localFilter)
	assert.NoError(t, err, "Failed to send REQ from receiver")

	// Wait for and receive the event
	receivedEvent, err := receiverClient.ExpectEvent("nip44-sub", 5*time.Second)
	assert.NoError(t, err, "Receiver failed to receive event")
	assert.NotNil(t, receivedEvent, "Received event is nil")

	// Decrypt the message
	receivedConversationKey, err := nip04.ComputeSharedSecret(receivedEvent.PubKey, receiverSecretKey)
	assert.NoError(t, err, "Failed to get shared secret for decryption")
	var receivedKey32Byte [32]byte
	copy(receivedKey32Byte[:], receivedConversationKey)
	decryptedText, err := nip44.Decrypt(receivedEvent.Content, receivedKey32Byte)
	assert.NoError(t, err, "Failed to decrypt message")

	// Verify the content
	assert.Equal(t, plaintext, decryptedText, "Decrypted content does not match original plaintext")
}
