package integration

import (
	"encoding/hex"
	"fmt"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/nbd-wtf/go-nostr"
	local_event "github.com/paul/glienicke/pkg/event"
)

// convertNostrEventToLocalEvent converts a nostr.Event to a local_event.Event
func convertNostrEventToLocalEvent(ne *nostr.Event) *local_event.Event {
	if ne == nil {
		return nil
	}

	localEvt := &local_event.Event{
		ID:        ne.ID,
		PubKey:    ne.PubKey,
		CreatedAt: ne.CreatedAt.Time().Unix(),
		Kind:      ne.Kind,
		Tags:      make([][]string, len(ne.Tags)),
		Content:   ne.Content,
		Sig:       ne.Sig,
	}

	for i, tag := range ne.Tags {
		localEvt.Tags[i] = []string(tag)
	}

	return localEvt
}

// convertNostrFilterToLocalFilter converts a nostr.Filter to a local_event.Filter
func convertNostrFilterToLocalFilter(nf *nostr.Filter) *local_event.Filter {
	if nf == nil {
		return nil
	}

	localFilter := &local_event.Filter{
		IDs:     nf.IDs,
		Authors: nf.Authors,
		Kinds:   nf.Kinds,
		Tags:    make(map[string][]string),
	}

	if nf.Since != nil {
		since := nf.Since.Time().Unix()
		localFilter.Since = &since
	}
	if nf.Until != nil {
		until := nf.Until.Time().Unix()
		localFilter.Until = &until
	}
	if nf.Limit != 0 {
		limit := nf.Limit
		localFilter.Limit = &limit
	}

	for k, v := range nf.Tags {
		localFilter.Tags[k] = v
	}

	return localFilter
}

func xOnlyToCompressed(pubkey string) (string, error) {
	// Decode the hex-encoded public key.
	pubKeyBytes, err := hex.DecodeString(pubkey)
	if err != nil {
		return "", fmt.Errorf("failed to decode public key: %w", err)
	}

	// Parse the public key.
	parsedPubKey, err := schnorr.ParsePubKey(pubKeyBytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse public key: %w", err)
	}

	// Serialize the public key in compressed format.
	compressedPubKey := parsedPubKey.SerializeCompressed()

	// Return the hex-encoded compressed public key.
	return hex.EncodeToString(compressedPubKey), nil
}