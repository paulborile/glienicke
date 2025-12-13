package nip59

import (
	"crypto/rand"
	"encoding/json"
	"fmt"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip04"
	"github.com/nbd-wtf/go-nostr/nip44"
	"github.com/paul/glienicke/pkg/event"
)

const (
	SealKind     = 13
	GiftWrapKind = 1059
)

var (
	ErrInvalidSeal     = fmt.Errorf("invalid seal event")
	ErrInvalidGiftWrap = fmt.Errorf("invalid gift wrap event")
	ErrDecryption      = fmt.Errorf("decryption failed")
)

// GenerateConversationKey generates a shared secret using ECDH
func GenerateConversationKey(privateKey, publicKey string) ([]byte, error) {
	return nip04.ComputeSharedSecret(publicKey, privateKey)
}

// CreateSeal creates a NIP-59 seal event from a rumor
func CreateSeal(rumor *event.Event, senderPrivateKey string) (*nostr.Event, error) {
	// Convert local event to nostr event
	nostrRumor := &nostr.Event{
		ID:        rumor.ID,
		PubKey:    rumor.PubKey,
		CreatedAt: nostr.Timestamp(rumor.CreatedAt),
		Kind:      rumor.Kind,
		Tags:      convertTagsToNostr(rumor.Tags),
		Content:   rumor.Content,
		// Note: rumors are unsigned, so Sig is empty
	}

	// Marshal rumor to JSON
	rumorJSON, err := json.Marshal(nostrRumor)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal rumor: %w", err)
	}

	// Get sender's public key
	senderPublicKey, err := nostr.GetPublicKey(senderPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get sender public key: %w", err)
	}

	// Generate conversation key with self (seal is encrypted to sender)
	conversationKey, err := GenerateConversationKey(senderPrivateKey, senderPublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to generate conversation key: %w", err)
	}

	var key32Byte [32]byte
	copy(key32Byte[:], conversationKey)

	// Encrypt rumor with NIP-44
	encryptedContent, err := nip44.Encrypt(string(rumorJSON), key32Byte)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt rumor: %w", err)
	}

	// Create seal event
	seal := &nostr.Event{
		CreatedAt: nostr.Now(),
		Kind:      SealKind,
		Content:   encryptedContent,
		Tags:      nostr.Tags{},
	}

	// Sign the seal
	if err := seal.Sign(senderPrivateKey); err != nil {
		return nil, fmt.Errorf("failed to sign seal: %w", err)
	}

	return seal, nil
}

// CreateGiftWrap creates a NIP-59 gift wrap event from a seal
func CreateGiftWrap(seal *nostr.Event, recipientPublicKey, senderPrivateKey string) (*nostr.Event, error) {
	// Marshal seal to JSON
	sealJSON, err := json.Marshal(seal)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal seal: %w", err)
	}

	// Generate conversation key with recipient
	conversationKey, err := GenerateConversationKey(senderPrivateKey, recipientPublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to generate conversation key: %w", err)
	}

	var key32Byte [32]byte
	copy(key32Byte[:], conversationKey)

	// Encrypt seal with NIP-44
	encryptedContent, err := nip44.Encrypt(string(sealJSON), key32Byte)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt seal: %w", err)
	}

	// Generate random created_at within reasonable range to avoid fingerprinting
	createdAt := nostr.Now()
	// Add some randomness (± 1 hour)
	randomBytes := make([]byte, 4)
	rand.Read(randomBytes)
	randomOffset := int64(randomBytes[0]) - 128     // -128 to +127
	createdAt += nostr.Timestamp(randomOffset * 60) // Convert to minutes

	// Create gift wrap event
	giftWrap := &nostr.Event{
		CreatedAt: createdAt,
		Kind:      GiftWrapKind,
		Content:   encryptedContent,
		Tags:      nostr.Tags{{"p", recipientPublicKey}},
	}

	// Sign the gift wrap
	if err := giftWrap.Sign(senderPrivateKey); err != nil {
		return nil, fmt.Errorf("failed to sign gift wrap: %w", err)
	}

	return giftWrap, nil
}

// UnwrapGiftFull fully unwraps a gift wrap to reveal the rumor
func UnwrapGiftFull(giftWrap *nostr.Event, privateKey string) (*event.Event, error) {
	// First unwrap to seal
	seal, err := UnwrapGiftToSeal(giftWrap, privateKey)
	if err != nil {
		return nil, err
	}

	// Then unwrap seal to rumor
	rumor, err := UnwrapSealFull(seal, privateKey)
	if err != nil {
		return nil, err
	}

	return rumor, nil
}

// UnwrapGiftToSeal unwraps a gift wrap event to reveal the seal
func UnwrapGiftToSeal(giftWrap *nostr.Event, privateKey string) (*nostr.Event, error) {
	if giftWrap.Kind != GiftWrapKind {
		return nil, ErrInvalidGiftWrap
	}

	conversationKey, err := GenerateConversationKey(privateKey, giftWrap.PubKey)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryption, err)
	}

	var key32Byte [32]byte
	copy(key32Byte[:], conversationKey)

	sealJSON, err := nip44.Decrypt(giftWrap.Content, key32Byte)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryption, err)
	}

	var seal nostr.Event
	if err := json.Unmarshal([]byte(sealJSON), &seal); err != nil {
		return nil, fmt.Errorf("failed to unmarshal seal: %w", err)
	}

	if seal.Kind != SealKind {
		return nil, ErrInvalidSeal
	}

	return &seal, nil
}

// UnwrapSealFull unwraps a seal event to reveal the rumor
func UnwrapSealFull(seal *nostr.Event, privateKey string) (*event.Event, error) {
	if seal.Kind != SealKind {
		return nil, ErrInvalidSeal
	}

	conversationKey, err := GenerateConversationKey(privateKey, seal.PubKey)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryption, err)
	}

	var key32Byte [32]byte
	copy(key32Byte[:], conversationKey)

	rumorJSON, err := nip44.Decrypt(seal.Content, key32Byte)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryption, err)
	}

	var nostrRumor nostr.Event
	if err := json.Unmarshal([]byte(rumorJSON), &nostrRumor); err != nil {
		return nil, fmt.Errorf("failed to unmarshal rumor: %w", err)
	}

	// Convert nostr event back to local event format
	rumor := &event.Event{
		ID:        nostrRumor.ID,
		PubKey:    nostrRumor.PubKey,
		CreatedAt: int64(nostrRumor.CreatedAt),
		Kind:      nostrRumor.Kind,
		Tags:      convertTagsFromNostr(nostrRumor.Tags),
		Content:   nostrRumor.Content,
		Sig:       nostrRumor.Sig,
	}

	return rumor, nil
}

// ValidateGiftWrap validates a gift wrap event structure
func ValidateGiftWrap(evt *nostr.Event) error {
	if evt.Kind != GiftWrapKind {
		return fmt.Errorf("invalid kind: expected %d, got %d", GiftWrapKind, evt.Kind)
	}

	if evt.Content == "" {
		return fmt.Errorf("gift wrap content cannot be empty")
	}

	if evt.Sig == "" {
		return fmt.Errorf("gift wrap must be signed")
	}

	return nil
}

// ValidateSeal validates a seal event structure
func ValidateSeal(evt *nostr.Event) error {
	if evt.Kind != SealKind {
		return fmt.Errorf("invalid kind: expected %d, got %d", SealKind, evt.Kind)
	}

	if evt.Content == "" {
		return fmt.Errorf("seal content cannot be empty")
	}

	if evt.Sig == "" {
		return fmt.Errorf("seal must be signed")
	}

	return nil
}

// Helper functions to convert between local and nostr tag formats
func convertTagsToNostr(tags [][]string) nostr.Tags {
	nostrTags := make(nostr.Tags, len(tags))
	for i, tag := range tags {
		nostrTags[i] = tag
	}
	return nostrTags
}

func convertTagsFromNostr(tags nostr.Tags) [][]string {
	localTags := make([][]string, len(tags))
	for i, tag := range tags {
		localTags[i] = tag
	}
	return localTags
}

// IsGiftWrap checks if an event is a gift wrap.
func IsGiftWrap(event *nostr.Event) bool {
	return event.Kind == GiftWrapKind
}

// IsSeal checks if an event is a seal.
func IsSeal(event *nostr.Event) bool {
	return event.Kind == SealKind
}

// UnwrapGift unwraps a gift wrap event to reveal the seal.
func UnwrapGift(giftWrap *nostr.Event, privateKey string) (*nostr.Event, error) {
	if giftWrap.Kind != GiftWrapKind {
		return nil, ErrInvalidGiftWrap
	}

	conversationKey, err := GenerateConversationKey(privateKey, giftWrap.PubKey)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryption, err)
	}

	var key32Byte [32]byte
	copy(key32Byte[:], conversationKey)

	sealJSON, err := nip44.Decrypt(giftWrap.Content, key32Byte)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryption, err)
	}

	var seal nostr.Event
	if err := json.Unmarshal([]byte(sealJSON), &seal); err != nil {
		return nil, fmt.Errorf("failed to unmarshal seal: %w", err)
	}

	if seal.Kind != SealKind {
		return nil, ErrInvalidSeal
	}

	return &seal, nil
}

// UnwrapSeal unwraps a seal event to reveal the rumor.
func UnwrapSeal(seal *nostr.Event, privateKey string) (*nostr.Event, error) {
	if seal.Kind != SealKind {
		return nil, ErrInvalidSeal
	}

	conversationKey, err := GenerateConversationKey(privateKey, seal.PubKey)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryption, err)
	}

	var key32Byte [32]byte
	copy(key32Byte[:], conversationKey)

	rumorJSON, err := nip44.Decrypt(seal.Content, key32Byte)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryption, err)
	}

	var rumor nostr.Event
	if err := json.Unmarshal([]byte(rumorJSON), &rumor); err != nil {
		return nil, fmt.Errorf("failed to unmarshal rumor: %w", err)
	}

	return &rumor, nil
}
