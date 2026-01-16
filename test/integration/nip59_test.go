package integration

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip04"
	"github.com/nbd-wtf/go-nostr/nip44"
	"github.com/paul/glienicke/internal/testutil"
	"github.com/stretchr/testify/assert"
)

func TestNIP59_GiftWrapEvent(t *testing.T) {
	// Setup: Relay and clients
	url, _, cleanup, _ := setupRelay(t)
	defer cleanup()

	// Generate sender and receiver key pairs
	senderSecretKey := nostr.GeneratePrivateKey()
	senderPublicKey, _ := nostr.GetPublicKey(senderSecretKey)

	receiverSecretKey := nostr.GeneratePrivateKey()
	receiverPublicKey, _ := nostr.GetPublicKey(receiverSecretKey)

	// 1. Create a rumor (unsigned event)
	rumor := nostr.Event{
		PubKey:    senderPublicKey,
		CreatedAt: nostr.Now(),
		Kind:      nostr.KindEncryptedDirectMessage,
		Tags:      nostr.Tags{{"p", receiverPublicKey}},
		Content:   "Hello, NIP-59!",
	}
	// Manually compute rumor ID for checking later, as it's not signed
	rumor.ID = rumor.GetID()

	// 2. Create a seal (kind:13)
	seal := nostr.Event{
		CreatedAt: nostr.Now(),
		Kind:      13,
		Tags:      nostr.Tags{},
	}

	// Encrypt the rumor and set it as the seal's content
	conversationKey, err := nip04.ComputeSharedSecret(receiverPublicKey, senderSecretKey)
	assert.NoError(t, err, "Failed to compute shared secret for seal")
	var key32Byte [32]byte
	copy(key32Byte[:], conversationKey)

	rumorJSON, err := json.Marshal(rumor)
	assert.NoError(t, err, "Failed to marshal rumor")

	seal.Content, err = nip44.Encrypt(string(rumorJSON), key32Byte)
	assert.NoError(t, err, "Failed to encrypt rumor for seal")
	seal.Sign(senderSecretKey)

	// 3. Create a gift wrap (kind:1059)
	giftWrap := nostr.Event{
		CreatedAt: nostr.Now(),
		Kind:      1059,
		Tags:      nostr.Tags{{"p", receiverPublicKey}},
	}

	// Encrypt the seal and set it as the gift wrap's content
	conversationKey, err = nip04.ComputeSharedSecret(receiverPublicKey, senderSecretKey)
	assert.NoError(t, err, "Failed to compute shared secret for gift wrap")
	copy(key32Byte[:], conversationKey)

	sealJSON, err := json.Marshal(seal)
	assert.NoError(t, err, "Failed to marshal seal")

	giftWrap.Content, err = nip44.Encrypt(string(sealJSON), key32Byte)
	assert.NoError(t, err, "Failed to encrypt seal for gift wrap")
	giftWrap.Sign(senderSecretKey)

	localEvent := convertNostrEventToLocalEvent(&giftWrap)

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

	// Verify that the rumor was NOT stored
	checkerClient, err := testutil.NewWSClient(url)
	assert.NoError(t, err, "Failed to create checker WebSocket client")
	defer checkerClient.Close()

	rumorFilter := nostr.Filter{IDs: []string{rumor.ID}}
	localRumorFilter := convertNostrFilterToLocalFilter(&rumorFilter)

	err = checkerClient.SendReq("rumor-check", localRumorFilter)
	assert.NoError(t, err, "Failed to send REQ from checker")

	// Expect to NOT receive the rumor event
	_, err = checkerClient.ExpectEvent("rumor-check", 1*time.Second)
	assert.Error(t, err, "Expected to NOT receive the rumor event, but it was received")

	// Receiver client connects and subscribes
	receiverClient, err := testutil.NewWSClient(url)
	assert.NoError(t, err, "Failed to create receiver WebSocket client")
	defer receiverClient.Close()

	nostrFilter := nostr.Filter{
		Kinds: []int{1059},
		Tags:  nostr.TagMap{"p": []string{receiverPublicKey}},
	}
	localFilter := convertNostrFilterToLocalFilter(&nostrFilter)

	err = receiverClient.SendReq("nip59-sub", localFilter)
	assert.NoError(t, err, "Failed to send REQ from receiver")

	// Wait for and receive the event
	receivedEvent, err := receiverClient.ExpectEvent("nip59-sub", 5*time.Second)
	assert.NoError(t, err, "Receiver failed to receive event")
	assert.NotNil(t, receivedEvent, "Received event is nil")

	// Decrypt the gift wrap
	receivedConversationKey, err := nip04.ComputeSharedSecret(receivedEvent.PubKey, receiverSecretKey)
	assert.NoError(t, err, "Failed to get shared secret for gift wrap decryption")
	copy(key32Byte[:], receivedConversationKey)

	decryptedSealJSON, err := nip44.Decrypt(receivedEvent.Content, key32Byte)
	assert.NoError(t, err, "Failed to decrypt gift wrap")

	var receivedSeal nostr.Event
	err = json.Unmarshal([]byte(decryptedSealJSON), &receivedSeal)
	assert.NoError(t, err, "Failed to unmarshal received seal")

	// Decrypt the seal
	receivedConversationKey, err = nip04.ComputeSharedSecret(receivedSeal.PubKey, receiverSecretKey)
	assert.NoError(t, err, "Failed to get shared secret for seal decryption")
	copy(key32Byte[:], receivedConversationKey)

	decryptedRumorJSON, err := nip44.Decrypt(receivedSeal.Content, key32Byte)
	assert.NoError(t, err, "Failed to decrypt seal")

	var receivedRumor nostr.Event
	err = json.Unmarshal([]byte(decryptedRumorJSON), &receivedRumor)
	assert.NoError(t, err, "Failed to unmarshal received rumor")

	// Verify the content
	assert.Equal(t, rumor.Content, receivedRumor.Content, "Decrypted content does not match original plaintext")
}
