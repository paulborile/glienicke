package testutil

import (
	"encoding/hex"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/paul/glienicke/pkg/event"
)

// KeyPair represents a Nostr keypair for testing
type KeyPair struct {
	PrivateKey *btcec.PrivateKey
	PublicKey  *btcec.PublicKey
	PubKeyHex  string
}

// GenerateKeyPair generates a new keypair for testing
func GenerateKeyPair() (*KeyPair, error) {
	privKey, err := btcec.NewPrivateKey()
	if err != nil {
		return nil, err
	}

	pubKey := privKey.PubKey()
	// Nostr uses Schnorr x-only pubkeys (32 bytes - BIP-340)
	pubKeyBytes := schnorr.SerializePubKey(pubKey)

	return &KeyPair{
		PrivateKey: privKey,
		PublicKey:  pubKey,
		PubKeyHex:  hex.EncodeToString(pubKeyBytes),
	}, nil
}

// SignEvent signs an event with the keypair
func (kp *KeyPair) SignEvent(evt *event.Event) error {
	// Set pubkey
	evt.PubKey = kp.PubKeyHex

	// Compute ID
	id, err := evt.ComputeID()
	if err != nil {
		return err
	}
	evt.ID = id

	// Sign the ID
	idBytes, err := hex.DecodeString(id)
	if err != nil {
		return err
	}

	sig, err := schnorr.Sign(kp.PrivateKey, idBytes)
	if err != nil {
		return err
	}

	evt.Sig = hex.EncodeToString(sig.Serialize())
	return nil
}

// NewTestEvent creates a signed test event
func NewTestEvent(kind int, content string, tags [][]string) (*event.Event, *KeyPair, error) {
	kp, err := GenerateKeyPair()
	if err != nil {
		return nil, nil, err
	}

	evt := &event.Event{
		Kind:      kind,
		Content:   content,
		Tags:      tags,
		CreatedAt: 1234567890,
	}

	if err := kp.SignEvent(evt); err != nil {
		return nil, nil, err
	}

	return evt, kp, nil
}

// NewTestEventWithKey creates a signed test event with an existing keypair
func NewTestEventWithKey(kp *KeyPair, kind int, content string, tags [][]string) (*event.Event, error) {
	evt := &event.Event{
		Kind:      kind,
		Content:   content,
		Tags:      tags,
		CreatedAt: 1234567890,
	}

	if err := kp.SignEvent(evt); err != nil {
		return nil, err
	}

	return evt, nil
}

// MustGenerateKeyPair generates a keypair or panics (for test convenience)
func MustGenerateKeyPair() *KeyPair {
	kp, err := GenerateKeyPair()
	if err != nil {
		panic(err)
	}
	return kp
}

// MustNewTestEvent creates a test event or panics (for test convenience)
func MustNewTestEvent(kind int, content string, tags [][]string) (*event.Event, *KeyPair) {
	evt, kp, err := NewTestEvent(kind, content, tags)
	if err != nil {
		panic(err)
	}
	return evt, kp
}
