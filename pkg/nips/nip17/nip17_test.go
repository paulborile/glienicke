package nip17

import (
	"testing"

	"github.com/paul/glienicke/pkg/event"
	"github.com/stretchr/testify/assert"
)

func TestIsPrivateDirectMessage(t *testing.T) {
	evt := &event.Event{Kind: PrivateDirectMessageKind}
	assert.True(t, IsPrivateDirectMessage(evt))

	evt.Kind = 15
	assert.False(t, IsPrivateDirectMessage(evt))
}

func TestIsFileMessage(t *testing.T) {
	evt := &event.Event{Kind: FileMessageKind}
	assert.True(t, IsFileMessage(evt))

	evt.Kind = 14
	assert.False(t, IsFileMessage(evt))
}

func TestGetRecipients(t *testing.T) {
	tests := []struct {
		name           string
		tags           [][]string
		expectedRecips []string
	}{
		{
			name:           "single recipient",
			tags:           [][]string{{"p", "pubkey1"}},
			expectedRecips: []string{"pubkey1"},
		},
		{
			name:           "multiple recipients",
			tags:           [][]string{{"p", "pubkey1"}, {"e", "event1"}, {"p", "pubkey2"}},
			expectedRecips: []string{"pubkey1", "pubkey2"},
		},
		{
			name:           "no recipients",
			tags:           [][]string{{"e", "event1"}, {"t", "tag1"}},
			expectedRecips: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evt := &event.Event{Tags: tt.tags}
			recipients := GetRecipients(evt)
			assert.Equal(t, tt.expectedRecips, recipients)
		})
	}
}

func TestGetReplyTo(t *testing.T) {
	tests := []struct {
		name          string
		tags          [][]string
		expectedReply string
		expectedFound bool
	}{
		{
			name:          "has reply tag",
			tags:          [][]string{{"p", "pubkey1"}, {"e", "event123"}},
			expectedReply: "event123",
			expectedFound: true,
		},
		{
			name:          "no reply tag",
			tags:          [][]string{{"p", "pubkey1"}, {"t", "tag1"}},
			expectedReply: "",
			expectedFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evt := &event.Event{Tags: tt.tags}
			reply, found := GetReplyTo(evt)
			assert.Equal(t, tt.expectedReply, reply)
			assert.Equal(t, tt.expectedFound, found)
		})
	}
}

func TestGetSubject(t *testing.T) {
	tests := []struct {
		name          string
		tags          [][]string
		expectedSubj  string
		expectedFound bool
	}{
		{
			name:          "has subject tag",
			tags:          [][]string{{"p", "pubkey1"}, {"subject", "Hello"}},
			expectedSubj:  "Hello",
			expectedFound: true,
		},
		{
			name:          "no subject tag",
			tags:          [][]string{{"p", "pubkey1"}, {"t", "tag1"}},
			expectedSubj:  "",
			expectedFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evt := &event.Event{Tags: tt.tags}
			subject, found := GetSubject(evt)
			assert.Equal(t, tt.expectedSubj, subject)
			assert.Equal(t, tt.expectedFound, found)
		})
	}
}

func TestValidateRumor(t *testing.T) {
	tests := []struct {
		name        string
		evt         *event.Event
		expectError bool
		errorType   string
	}{
		{
			name: "valid private message",
			evt: &event.Event{
				Kind:    PrivateDirectMessageKind,
				Content: "Hello",
				Tags:    [][]string{{"p", "pubkey1"}},
			},
			expectError: false,
		},
		{
			name: "valid file message",
			evt: &event.Event{
				Kind: FileMessageKind,
				Tags: [][]string{{"p", "pubkey1"}},
			},
			expectError: false,
		},
		{
			name: "invalid kind",
			evt: &event.Event{
				Kind:    1,
				Content: "Hello",
				Tags:    [][]string{{"p", "pubkey1"}},
			},
			expectError: true,
			errorType:   "invalid_kind",
		},
		{
			name: "missing recipient",
			evt: &event.Event{
				Kind:    PrivateDirectMessageKind,
				Content: "Hello",
				Tags:    [][]string{{"t", "tag1"}},
			},
			expectError: true,
			errorType:   "missing_recipient",
		},
		{
			name: "empty content for private message",
			evt: &event.Event{
				Kind:    PrivateDirectMessageKind,
				Content: "",
				Tags:    [][]string{{"p", "pubkey1"}},
			},
			expectError: true,
			errorType:   "empty_content",
		},
		{
			name: "signed rumor (should not be signed)",
			evt: &event.Event{
				Kind:    PrivateDirectMessageKind,
				Content: "Hello",
				Tags:    [][]string{{"p", "pubkey1"}},
				Sig:     "signature",
			},
			expectError: true,
			errorType:   "signed_rumor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRumor(tt.evt)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorType != "" {
					assert.IsType(t, &ValidationError{}, err)
					assert.Equal(t, tt.errorType, err.(*ValidationError).Kind)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreatePrivateDirectMessage(t *testing.T) {
	senderPubKey := "sender123"
	content := "Hello, world!"
	recipients := []string{"recipient1", "recipient2"}
	replyTo := "event456"

	rumor := CreatePrivateDirectMessage(senderPubKey, content, recipients, &replyTo)

	assert.Equal(t, senderPubKey, rumor.PubKey)
	assert.Equal(t, PrivateDirectMessageKind, rumor.Kind)
	assert.Equal(t, content, rumor.Content)

	recips := GetRecipients(rumor)
	assert.Equal(t, recipients, recips)

	reply, found := GetReplyTo(rumor)
	assert.True(t, found)
	assert.Equal(t, replyTo, reply)
}

func TestCreatePrivateDirectMessageWithoutReply(t *testing.T) {
	senderPubKey := "sender123"
	content := "Hello, world!"
	recipients := []string{"recipient1"}

	rumor := CreatePrivateDirectMessage(senderPubKey, content, recipients, nil)

	assert.Equal(t, senderPubKey, rumor.PubKey)
	assert.Equal(t, PrivateDirectMessageKind, rumor.Kind)
	assert.Equal(t, content, rumor.Content)

	recips := GetRecipients(rumor)
	assert.Equal(t, recipients, recips)

	_, found := GetReplyTo(rumor)
	assert.False(t, found)
}

func TestCreateFileMessage(t *testing.T) {
	senderPubKey := "sender123"
	content := "file content"
	fileName := "test.txt"
	mimeType := "text/plain"
	recipients := []string{"recipient1"}

	rumor := CreateFileMessage(senderPubKey, content, fileName, mimeType, recipients)

	assert.Equal(t, senderPubKey, rumor.PubKey)
	assert.Equal(t, FileMessageKind, rumor.Kind)
	assert.Equal(t, content, rumor.Content)

	recips := GetRecipients(rumor)
	assert.Equal(t, recipients, recips)

	// Check file metadata tags
	var foundFileName, foundMimeType bool
	for _, tag := range rumor.Tags {
		if len(tag) >= 2 && tag[0] == "filename" && tag[1] == fileName {
			foundFileName = true
		}
		if len(tag) >= 2 && tag[0] == "mimetype" && tag[1] == mimeType {
			foundMimeType = true
		}
	}
	assert.True(t, foundFileName)
	assert.True(t, foundMimeType)
}

func TestExportImportRumor(t *testing.T) {
	original := &event.Event{
		PubKey:    "sender123",
		Kind:      PrivateDirectMessageKind,
		Content:   "Hello",
		Tags:      [][]string{{"p", "recipient1"}},
		CreatedAt: 1234567890,
	}

	// Export rumor
	data, err := ExportRumor(original)
	assert.NoError(t, err)
	assert.NotEmpty(t, data)

	// Import rumor
	imported, err := ImportRumor(data)
	assert.NoError(t, err)
	assert.Equal(t, original.PubKey, imported.PubKey)
	assert.Equal(t, original.Kind, imported.Kind)
	assert.Equal(t, original.Content, imported.Content)
	assert.Equal(t, original.CreatedAt, imported.CreatedAt)
	assert.Equal(t, original.Tags, imported.Tags)
}

func TestImportInvalidRumor(t *testing.T) {
	invalidData := []byte("not json")

	_, err := ImportRumor(invalidData)
	assert.Error(t, err)
	assert.IsType(t, &ValidationError{}, err)
	assert.Equal(t, "invalid_json", err.(*ValidationError).Kind)
}
