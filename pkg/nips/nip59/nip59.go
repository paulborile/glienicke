package nip59

import (
	"encoding/json"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip04"
	"github.com/nbd-wtf/go-nostr/nip44"
)

const (
	SealKind     = 13
	GiftWrapKind = 1059
)

// UnwrapGift unwraps a gift wrap event to reveal the seal.
func UnwrapGift(giftWrap *nostr.Event, privateKey string) (*nostr.Event, error) {
	conversationKey, err := nip04.ComputeSharedSecret(giftWrap.PubKey, privateKey)
	if err != nil {
		return nil, err
	}

	var key32Byte [32]byte
	copy(key32Byte[:], conversationKey)

	sealJSON, err := nip44.Decrypt(giftWrap.Content, key32Byte)
	if err != nil {
		return nil, err
	}

	var seal nostr.Event
	if err := json.Unmarshal([]byte(sealJSON), &seal); err != nil {
		return nil, err
	}

	return &seal, nil
}

// UnwrapSeal unwraps a seal event to reveal the rumor.
func UnwrapSeal(seal *nostr.Event, privateKey string) (*nostr.Event, error) {
	conversationKey, err := nip04.ComputeSharedSecret(seal.PubKey, privateKey)
	if err != nil {
		return nil, err
	}

	var key32Byte [32]byte
	copy(key32Byte[:], conversationKey)

	rumorJSON, err := nip44.Decrypt(seal.Content, key32Byte)
	if err != nil {
		return nil, err
	}

	var rumor nostr.Event
	if err := json.Unmarshal([]byte(rumorJSON), &rumor); err != nil {
		return nil, err
	}

	return &rumor, nil
}

// IsGiftWrap checks if an event is a gift wrap.
func IsGiftWrap(event *nostr.Event) bool {
	return event.Kind == GiftWrapKind
}

// IsSeal checks if an event is a seal.
func IsSeal(event *nostr.Event) bool {
	return event.Kind == SealKind
}
