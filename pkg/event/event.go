package event

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

// Event represents a Nostr event as defined in NIP-01
type Event struct {
	ID        string     `json:"id"`
	PubKey    string     `json:"pubkey"`
	CreatedAt int64      `json:"created_at"`
	Kind      int        `json:"kind"`
	Tags      [][]string `json:"tags"`
	Content   string     `json:"content"`
	Sig       string     `json:"sig"`
}

// Filter represents a subscription filter as defined in NIP-01
type Filter struct {
	IDs     []string `json:"ids,omitempty"`
	Authors []string `json:"authors,omitempty"`
	Kinds   []int    `json:"kinds,omitempty"`
	Tags    map[string][]string
	Since   *int64 `json:"since,omitempty"`
	Until   *int64 `json:"until,omitempty"`
	Limit   *int   `json:"limit,omitempty"`
	Search  string `json:"search,omitempty"`
}

// UnmarshalJSON implements a custom unmarshaler for Filter
func (f *Filter) UnmarshalJSON(data []byte) error {
	// Use a temporary struct to unmarshal known fields
	type Alias Filter
	aux := &struct {
		*Alias
	}{Alias: (*Alias)(f)}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	// Now, unmarshal into a map to find generic tags
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	if f.Tags == nil {
		f.Tags = make(map[string][]string)
	}

	for key, value := range m {
		if len(key) > 1 && key[0] == '#' {
			tagName := key[1:]
			var tagValues []string
			if err := json.Unmarshal(value, &tagValues); err != nil {
				return fmt.Errorf("invalid tag value for %s: %w", key, err)
			}
			f.Tags[tagName] = tagValues
		}
	}

	return nil
}

// Validate checks if the event is valid according to NIP-01
func (e *Event) Validate() error {
	// Check required fields
	if e.PubKey == "" {
		return fmt.Errorf("missing pubkey")
	}
	if e.Sig == "" {
		return fmt.Errorf("missing signature")
	}
	if e.Kind < 0 {
		return fmt.Errorf("invalid kind")
	}

	// Verify ID matches the computed hash
	computedID, err := e.ComputeID()
	if err != nil {
		return fmt.Errorf("failed to compute ID: %w", err)
	}
	if e.ID != computedID {
		return fmt.Errorf("ID does not match computed hash")
	}

	// Verify signature
	if err := e.VerifySignature(); err != nil {
		return fmt.Errorf("invalid signature: %w", err)
	}

	return nil
}

// ComputeID computes the event ID according to NIP-01
func (e *Event) ComputeID() (string, error) {
	// Serialize event data for hashing
	serialized, err := e.Serialize()
	if err != nil {
		return "", err
	}

	// Compute SHA256 hash
	hash := sha256.Sum256([]byte(serialized))
	return hex.EncodeToString(hash[:]), nil
}

// Serialize creates the canonical serialization for ID computation
func (e *Event) Serialize() (string, error) {
	// NIP-01 format: [0,<pubkey>,<created_at>,<kind>,<tags>,<content>]
	data := []interface{}{
		0,
		e.PubKey,
		e.CreatedAt,
		e.Kind,
		e.Tags,
		e.Content,
	}

	serialized, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to serialize event: %w", err)
	}

	return string(serialized), nil
}

// VerifySignature verifies the Schnorr signature
func (e *Event) VerifySignature() error {
	// Decode pubkey (32 bytes x-only format used by Nostr/BIP-340)
	pubKeyBytes, err := hex.DecodeString(e.PubKey)
	if err != nil {
		return fmt.Errorf("invalid pubkey hex: %w", err)
	}
	if len(pubKeyBytes) != 32 {
		return fmt.Errorf("pubkey must be 32 bytes")
	}

	// Parse as Schnorr x-only pubkey (BIP-340)
	pubKey, err := schnorr.ParsePubKey(pubKeyBytes)
	if err != nil {
		return fmt.Errorf("invalid pubkey: %w", err)
	}

	// Decode signature
	sigBytes, err := hex.DecodeString(e.Sig)
	if err != nil {
		return fmt.Errorf("invalid signature hex: %w", err)
	}
	if len(sigBytes) != 64 {
		return fmt.Errorf("signature must be 64 bytes")
	}

	sig, err := schnorr.ParseSignature(sigBytes)
	if err != nil {
		return fmt.Errorf("invalid signature: %w", err)
	}

	// Decode event ID (the message that was signed)
	idBytes, err := hex.DecodeString(e.ID)
	if err != nil {
		return fmt.Errorf("invalid ID hex: %w", err)
	}

	// Verify signature
	if !sig.Verify(idBytes, pubKey) {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}

// Matches checks if the event matches the given filter
func (e *Event) Matches(f *Filter) bool {
	// Check IDs
	if len(f.IDs) > 0 {
		match := false
		for _, id := range f.IDs {
			if matchesPrefix(e.ID, id) {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}

	// Check authors
	if len(f.Authors) > 0 {
		match := false
		for _, author := range f.Authors {
			if matchesPrefix(e.PubKey, author) {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}

	// Check kinds
	if len(f.Kinds) > 0 {
		match := false
		for _, kind := range f.Kinds {
			if e.Kind == kind {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}

	// Check time range
	if f.Since != nil && e.CreatedAt < *f.Since {
		return false
	}
	if f.Until != nil && e.CreatedAt > *f.Until {
		return false
	}

	// Check tags
	for tagName, filterValues := range f.Tags {
		found := false
		for _, filterValue := range filterValues {
			// Check if the event has this tag with any of the filter values
			if e.hasTag(tagName, filterValue) {
				found = true
				break
			}
		}
		if !found {
			return false // No match for this tag name
		}
	}

	return true
}

// hasTag checks if the event has a tag with the given name and value
func (e *Event) hasTag(name, value string) bool {
	for _, tag := range e.Tags {
		if len(tag) >= 2 && tag[0] == name {
			if matchesPrefix(tag[1], value) {
				return true
			}
		}
	}
	return false
}

// matchesPrefix checks if target starts with prefix (supports prefix matching)
func matchesPrefix(target, prefix string) bool {
	if len(prefix) > len(target) {
		return false
	}
	return target[:len(prefix)] == prefix
}

// GetTagValues returns all values for a given tag name
func (e *Event) GetTagValues(tagName string) []string {
	var values []string
	for _, tag := range e.Tags {
		if len(tag) >= 2 && tag[0] == tagName {
			values = append(values, tag[1])
		}
	}
	return values
}

// IsExpired checks if the event has expired based on NIP-40
func (e *Event) IsExpired() bool {
	expirations := e.GetTagValues("expiration")
	if len(expirations) == 0 {
		return false
	}

	// Parse the first expiration timestamp
	var expiration int64
	_, err := fmt.Sscanf(expirations[0], "%d", &expiration)
	if err != nil {
		return false
	}

	return time.Now().Unix() > expiration
}

// IsDeleted checks if this is a deletion event (kind 5)
func (e *Event) IsDeleted() bool {
	return e.Kind == 5
}

// GetDeletedEventIDs returns the event IDs this deletion event refers to
func (e *Event) GetDeletedEventIDs() []string {
	if !e.IsDeleted() {
		return nil
	}
	return e.GetTagValues("e")
}
