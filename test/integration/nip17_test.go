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

func TestNIP17_PrivateDirectMessage(t *testing.T) {
	// Setup: Relay and clients
	url, _, cleanup := setupRelay(t)
	defer cleanup()

	// Generate sender and receiver key pairs
	senderSecretKey := nostr.GeneratePrivateKey()
	senderPublicKey, _ := nostr.GetPublicKey(senderSecretKey)

	receiverSecretKey := nostr.GeneratePrivateKey()
	receiverPublicKey, _ := nostr.GetPublicKey(receiverSecretKey)

	// 1. Create a NIP-17 private message (rumor: kind 14)
	privateMessage := "This is a private NIP-17 message."
	rumor := nostr.Event{
		PubKey:    senderPublicKey,
		CreatedAt: nostr.Now(),
		Kind:      14, // NIP-17 kind for private chat message
		Tags:      nostr.Tags{{"p", receiverPublicKey}},
		Content:   privateMessage,
	}
	// Manually compute rumor ID as it's not signed
	rumor.ID = rumor.GetID()

	// 2. Create a seal (kind:13)
	seal := nostr.Event{
		CreatedAt: nostr.Now(),
		Kind:      13,
		Tags:      nostr.Tags{},
	}

	// Encrypt the rumor and set it as the seal's content using NIP-44
	conversationKey, err := nip04.ComputeSharedSecret(receiverPublicKey, senderSecretKey)
	assert.NoError(t, err, "Failed to compute shared secret for seal")
	var key32Byte [32]byte
	copy(key32Byte[:], conversationKey)

	rumorJSON, err := json.Marshal(rumor)
	assert.NoError(t, err, "Failed to marshal rumor")

	seal.Content, err = nip44.Encrypt(string(rumorJSON), key32Byte)
	assert.NoError(t, err, "Failed to encrypt rumor for seal")
	seal.Sign(senderSecretKey)

	// 3. Create a gift wrap (kind:1059) as per NIP-59
	giftWrap := nostr.Event{
		CreatedAt: nostr.Now(),
		Kind:      1059,
		Tags:      nostr.Tags{{"p", receiverPublicKey}},
	}

	// Encrypt the seal and set it as the gift wrap's content using NIP-44
	conversationKey, err = nip04.ComputeSharedSecret(receiverPublicKey, senderSecretKey)
	assert.NoError(t, err, "Failed to compute shared secret for gift wrap")
	copy(key32Byte[:], conversationKey)

	sealJSON, err := json.Marshal(seal)
	assert.NoError(t, err, "Failed to marshal seal")

	giftWrap.Content, err = nip44.Encrypt(string(sealJSON), key32Byte)
	assert.NoError(t, err, "Failed to encrypt seal for gift wrap")
	giftWrap.Sign(senderSecretKey)

	localGiftWrap := convertNostrEventToLocalEvent(&giftWrap)

	// Sender client connects and sends the gift wrap event
	senderClient, err := testutil.NewWSClient(url)
	assert.NoError(t, err, "Failed to create sender WebSocket client")
	defer senderClient.Close()

	err = senderClient.SendEvent(localGiftWrap)
	assert.NoError(t, err, "Failed to send gift wrap event from sender")

	// Expect OK response from relay for the gift wrap
	accepted, msg, err := senderClient.ExpectOK(localGiftWrap.ID, 5*time.Second)
	assert.NoError(t, err, "Failed to receive OK from sender")
	assert.True(t, accepted, "Gift wrap event was not accepted by relay: %s", msg)

	// Verify that the rumor (kind 14 event) was NOT stored by the relay
	checkerClient, err := testutil.NewWSClient(url)
	assert.NoError(t, err, "Failed to create checker WebSocket client")
	defer checkerClient.Close()

	rumorFilter := nostr.Filter{IDs: []string{rumor.ID}, Kinds: []int{14}}
	localRumorFilter := convertNostrFilterToLocalFilter(&rumorFilter)

	err = checkerClient.SendReq("rumor-check", localRumorFilter)
	assert.NoError(t, err, "Failed to send REQ from checker for rumor")

	// Expect to NOT receive the rumor event
	_, err = checkerClient.ExpectEvent("rumor-check", 1*time.Second)
	assert.Error(t, err, "Expected to NOT receive the NIP-17 rumor event, but it was received")

	// Receiver client connects and subscribes to kind 1059 events tagged with their public key
	receiverClient, err := testutil.NewWSClient(url)
	assert.NoError(t, err, "Failed to create receiver WebSocket client")
	defer receiverClient.Close()

	nostrFilter := nostr.Filter{
		Kinds: []int{1059},
		Tags:  nostr.TagMap{"p": []string{receiverPublicKey}},
	}
	localFilter := convertNostrFilterToLocalFilter(&nostrFilter)

	err = receiverClient.SendReq("nip17-sub", localFilter)
	assert.NoError(t, err, "Failed to send REQ from receiver for nip17-sub")

	// Wait for and receive the gift wrap event
	receivedGiftWrap, err := receiverClient.ExpectEvent("nip17-sub", 5*time.Second)
	assert.NoError(t, err, "Receiver failed to receive gift wrap event")
	assert.NotNil(t, receivedGiftWrap, "Received gift wrap event is nil")

	// Decrypt the gift wrap to reveal the seal
	receivedConversationKey, err := nip04.ComputeSharedSecret(receivedGiftWrap.PubKey, receiverSecretKey)
	assert.NoError(t, err, "Failed to get shared secret for gift wrap decryption")
	copy(key32Byte[:], receivedConversationKey)

	decryptedSealJSON, err := nip44.Decrypt(receivedGiftWrap.Content, key32Byte)
	assert.NoError(t, err, "Failed to decrypt gift wrap")

	var receivedSeal nostr.Event
	err = json.Unmarshal([]byte(decryptedSealJSON), &receivedSeal)
	assert.NoError(t, err, "Failed to unmarshal received seal")

	// Decrypt the seal to reveal the rumor (NIP-17 private message)
	receivedConversationKey, err = nip04.ComputeSharedSecret(receivedSeal.PubKey, receiverSecretKey)
	assert.NoError(t, err, "Failed to get shared secret for seal decryption")
	copy(key32Byte[:], receivedConversationKey)

	decryptedRumorJSON, err := nip44.Decrypt(receivedSeal.Content, key32Byte)
	assert.NoError(t, err, "Failed to decrypt seal")

	var receivedRumor nostr.Event
	err = json.Unmarshal([]byte(decryptedRumorJSON), &receivedRumor)
	assert.NoError(t, err, "Failed to unmarshal received rumor")

	// Verify the content of the NIP-17 private message
	assert.Equal(t, privateMessage, receivedRumor.Content, "Decrypted NIP-17 message content does not match original")
	assert.Equal(t, senderPublicKey, receivedRumor.PubKey, "Decrypted NIP-17 message pubkey does not match original sender")
	assert.Equal(t, receiverPublicKey, receivedRumor.Tags.GetFirst([]string{"p"}).Value(), "Decrypted NIP-17 message recipient tag does not match original")
	assert.Equal(t, 14, receivedRumor.Kind, "Decrypted NIP-17 message kind is not 14")
}
