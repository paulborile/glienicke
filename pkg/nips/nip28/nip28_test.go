package nip28

import (
	"testing"

	"github.com/paul/glienicke/pkg/event"
	"github.com/stretchr/testify/assert"
)

func TestIsNIP28Event(t *testing.T) {
	tests := []struct {
		name     string
		kind     int
		expected bool
	}{
		{"channel create", KindChannelCreate, true},
		{"channel metadata", KindChannelMetadata, true},
		{"channel message", KindChannelMessage, true},
		{"channel hide", KindChannelHide, true},
		{"channel mute", KindChannelMute, true},
		{"regular text note", 1, false},
		{"metadata", 0, false},
		{"reaction", 7, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evt := &event.Event{Kind: tt.kind}
			result := IsNIP28Event(evt)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateChannelCreate(t *testing.T) {
	t.Run("valid channel create", func(t *testing.T) {
		evt := &event.Event{
			Kind:    KindChannelCreate,
			Content: "Test Channel",
			Tags:    [][]string{{"channel_id", "test-channel-123"}},
		}
		err := ValidateChannelCreate(evt)
		assert.NoError(t, err)
	})

	t.Run("wrong kind", func(t *testing.T) {
		evt := &event.Event{Kind: KindChannelMessage}
		err := ValidateChannelCreate(evt)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not a channel create event")
	})

	t.Run("empty content", func(t *testing.T) {
		evt := &event.Event{
			Kind:    KindChannelCreate,
			Content: "",
			Tags:    [][]string{{"channel_id", "test-channel"}},
		}
		err := ValidateChannelCreate(evt)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")
	})

	t.Run("name too long", func(t *testing.T) {
		evt := &event.Event{
			Kind:    KindChannelCreate,
			Content: string(make([]byte, 257)),
			Tags:    [][]string{{"channel_id", "test-channel"}},
		}
		err := ValidateChannelCreate(evt)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "too long")
	})

	t.Run("missing channel_id", func(t *testing.T) {
		evt := &event.Event{
			Kind:    KindChannelCreate,
			Content: "Test Channel",
			Tags:    [][]string{},
		}
		err := ValidateChannelCreate(evt)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "channel id")
	})
}

func TestValidateChannelMessage(t *testing.T) {
	t.Run("valid message", func(t *testing.T) {
		evt := &event.Event{
			Kind:    KindChannelMessage,
			Content: "Hello, world!",
			Tags:    [][]string{{"channel_id", "test-channel"}},
		}
		err := ValidateChannelMessage(evt)
		assert.NoError(t, err)
	})

	t.Run("empty content", func(t *testing.T) {
		evt := &event.Event{
			Kind:    KindChannelMessage,
			Content: "",
			Tags:    [][]string{{"channel_id", "test-channel"}},
		}
		err := ValidateChannelMessage(evt)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")
	})

	t.Run("content too long", func(t *testing.T) {
		evt := &event.Event{
			Kind:    KindChannelMessage,
			Content: string(make([]byte, 16385)),
			Tags:    [][]string{{"channel_id", "test-channel"}},
		}
		err := ValidateChannelMessage(evt)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "too long")
	})

	t.Run("missing channel_id", func(t *testing.T) {
		evt := &event.Event{
			Kind:    KindChannelMessage,
			Content: "Hello",
		}
		err := ValidateChannelMessage(evt)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "channel id")
	})
}

func TestValidateChannelMetadata(t *testing.T) {
	t.Run("valid metadata", func(t *testing.T) {
		evt := &event.Event{
			Kind:    KindChannelMetadata,
			Content: "Updated Name",
			Tags:    [][]string{{"channel_id", "test-channel"}},
		}
		err := ValidateChannelMetadata(evt)
		assert.NoError(t, err)
	})

	t.Run("empty content but has tags", func(t *testing.T) {
		evt := &event.Event{
			Kind:    KindChannelMetadata,
			Content: "",
			Tags:    [][]string{{"channel_id", "test-channel"}, {"name", "New Name"}},
		}
		err := ValidateChannelMetadata(evt)
		assert.NoError(t, err)
	})

	t.Run("missing channel_id", func(t *testing.T) {
		evt := &event.Event{
			Kind:    KindChannelMetadata,
			Content: "Name",
		}
		err := ValidateChannelMetadata(evt)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "channel id")
	})
}

func TestValidateChannelHide(t *testing.T) {
	t.Run("valid hide", func(t *testing.T) {
		evt := &event.Event{
			Kind:    KindChannelHide,
			Content: "Spam",
			Tags:    [][]string{{"e", "event-id-to-hide"}},
		}
		err := ValidateChannelHide(evt)
		assert.NoError(t, err)
	})

	t.Run("missing event id", func(t *testing.T) {
		evt := &event.Event{
			Kind:    KindChannelHide,
			Content: "Spam",
		}
		err := ValidateChannelHide(evt)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "event id")
	})
}

func TestValidateChannelMute(t *testing.T) {
	t.Run("valid mute", func(t *testing.T) {
		evt := &event.Event{
			Kind:    KindChannelMute,
			Content: "Muted",
			Tags:    [][]string{{"p", "muted-pubkey"}},
		}
		err := ValidateChannelMute(evt)
		assert.NoError(t, err)
	})

	t.Run("missing pubkey", func(t *testing.T) {
		evt := &event.Event{
			Kind:    KindChannelMute,
			Content: "Muted",
		}
		err := ValidateChannelMute(evt)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "pubkey")
	})
}

func TestGetChannelID(t *testing.T) {
	evt := &event.Event{
		Tags: [][]string{
			{"channel_id", "my-channel"},
			{"other", "tag"},
		},
	}
	id := getChannelID(evt)
	assert.Equal(t, "my-channel", id)
}

func TestGetEventID(t *testing.T) {
	evt := &event.Event{
		Tags: [][]string{
			{"e", "event-123"},
			{"p", "pubkey-456"},
		},
	}
	id := getEventID(evt)
	assert.Equal(t, "event-123", id)
}

func TestGetPubkey(t *testing.T) {
	evt := &event.Event{
		Tags: [][]string{
			{"p", "pubkey-456"},
			{"e", "event-123"},
		},
	}
	pubkey := getPubkey(evt)
	assert.Equal(t, "pubkey-456", pubkey)
}

func TestIsReplaceableKind(t *testing.T) {
	assert.True(t, IsReplaceableKind(KindChannelCreate))
	assert.True(t, IsReplaceableKind(KindChannelMetadata))
	assert.False(t, IsReplaceableKind(KindChannelMessage))
	assert.False(t, IsReplaceableKind(KindChannelHide))
	assert.False(t, IsReplaceableKind(KindChannelMute))
}

func TestParseChannelMetadata(t *testing.T) {
	t.Run("parse from metadata event", func(t *testing.T) {
		evt := &event.Event{
			Kind:    KindChannelMetadata,
			Content: "Default Name",
			Tags: [][]string{
				{"channel_id", "channel-123"},
				{"name", "Updated Name"},
				{"description", "A test channel"},
				{"picture", "https://example.com/image.png"},
			},
			CreatedAt: 1234567890,
		}

		channel, err := ParseChannelMetadata(evt)
		assert.NoError(t, err)
		assert.Equal(t, "channel-123", channel.ID)
		assert.Equal(t, "Updated Name", channel.Name)
		assert.Equal(t, "A test channel", channel.Description)
		assert.Equal(t, "https://example.com/image.png", channel.Picture)
	})

	t.Run("parse from create event uses content as name", func(t *testing.T) {
		evt := &event.Event{
			Kind:      KindChannelCreate,
			Content:   "My Channel",
			Tags:      [][]string{{"channel_id", "channel-456"}},
			CreatedAt: 1234567890,
		}

		channel, err := ParseChannelMetadata(evt)
		assert.NoError(t, err)
		assert.Equal(t, "channel-456", channel.ID)
		assert.Equal(t, "My Channel", channel.Name)
	})
}

func TestNIP28Kinds(t *testing.T) {
	assert.Equal(t, 40, KindChannelCreate)
	assert.Equal(t, 41, KindChannelMetadata)
	assert.Equal(t, 42, KindChannelMessage)
	assert.Equal(t, 43, KindChannelHide)
	assert.Equal(t, 44, KindChannelMute)
}
