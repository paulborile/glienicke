package nip28

import (
	"fmt"
	"strings"

	"github.com/paul/glienicke/pkg/event"
	"github.com/paul/glienicke/pkg/storage"
)

const (
	KindChannelCreate   = 40
	KindChannelMetadata = 41
	KindChannelMessage  = 42
	KindChannelHide     = 43
	KindChannelMute     = 44
)

type Processor struct{}

func New() *Processor {
	return &Processor{}
}

func (p *Processor) Process(evt *event.Event, store storage.Store) error {
	switch evt.Kind {
	case KindChannelCreate:
		return ValidateChannelCreate(evt)
	case KindChannelMetadata:
		return ValidateChannelMetadata(evt)
	case KindChannelMessage:
		return ValidateChannelMessage(evt)
	case KindChannelHide:
		return ValidateChannelHide(evt)
	case KindChannelMute:
		return ValidateChannelMute(evt)
	default:
		return fmt.Errorf("unsupported NIP-28 kind: %d", evt.Kind)
	}
}

func IsNIP28Event(evt *event.Event) bool {
	switch evt.Kind {
	case KindChannelCreate, KindChannelMetadata, KindChannelMessage,
		KindChannelHide, KindChannelMute:
		return true
	}
	return false
}

func ValidateChannelCreate(evt *event.Event) error {
	if evt.Kind != KindChannelCreate {
		return fmt.Errorf("not a channel create event: kind %d", evt.Kind)
	}

	if evt.Content == "" {
		return fmt.Errorf("channel name cannot be empty")
	}

	if len(evt.Content) > 256 {
		return fmt.Errorf("channel name too long: max 256 characters")
	}

	channelID := getChannelID(evt)
	if channelID == "" {
		return fmt.Errorf("missing or invalid channel id in tags")
	}

	if evt.Sig != "" {
		return evt.VerifySignature()
	}
	return nil
}

func ValidateChannelMetadata(evt *event.Event) error {
	if evt.Kind != KindChannelMetadata {
		return fmt.Errorf("not a channel metadata event: kind %d", evt.Kind)
	}

	if evt.Content == "" && len(evt.Tags) == 0 {
		return fmt.Errorf("channel metadata cannot be empty")
	}

	channelID := getChannelID(evt)
	if channelID == "" {
		return fmt.Errorf("missing or invalid channel id in tags")
	}

	if evt.Sig != "" {
		return evt.VerifySignature()
	}
	return nil
}

func ValidateChannelMessage(evt *event.Event) error {
	if evt.Kind != KindChannelMessage {
		return fmt.Errorf("not a channel message event: kind %d", evt.Kind)
	}

	if strings.TrimSpace(evt.Content) == "" {
		return fmt.Errorf("message content cannot be empty")
	}

	if len(evt.Content) > 16384 {
		return fmt.Errorf("message too long: max 16384 characters")
	}

	channelID := getChannelID(evt)
	if channelID == "" {
		return fmt.Errorf("missing or invalid channel id in tags")
	}

	if evt.Sig != "" {
		return evt.VerifySignature()
	}
	return nil
}

func ValidateChannelHide(evt *event.Event) error {
	if evt.Kind != KindChannelHide {
		return fmt.Errorf("not a channel hide event: kind %d", evt.Kind)
	}

	if strings.TrimSpace(evt.Content) == "" {
		return fmt.Errorf("hide reason cannot be empty")
	}

	eventID := getEventID(evt)
	if eventID == "" {
		return fmt.Errorf("missing event id in tags")
	}

	if evt.Sig != "" {
		return evt.VerifySignature()
	}
	return nil
}

func ValidateChannelMute(evt *event.Event) error {
	if evt.Kind != KindChannelMute {
		return fmt.Errorf("not a channel mute event: kind %d", evt.Kind)
	}

	pubkey := getPubkey(evt)
	if pubkey == "" {
		return fmt.Errorf("missing pubkey in tags")
	}

	if evt.Sig != "" {
		return evt.VerifySignature()
	}
	return nil
}

func getChannelID(evt *event.Event) string {
	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "channel_id" {
			return tag[1]
		}
	}
	return ""
}

func getEventID(evt *event.Event) string {
	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "e" {
			return tag[1]
		}
	}
	return ""
}

func getPubkey(evt *event.Event) string {
	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "p" {
			return tag[1]
		}
	}
	return ""
}

type Channel struct {
	ID          string
	Name        string
	Description string
	Picture     string
	CreatedAt   int64
	UpdatedAt   int64
}

func ParseChannelMetadata(evt *event.Event) (*Channel, error) {
	if evt.Kind != KindChannelMetadata && evt.Kind != KindChannelCreate {
		return nil, fmt.Errorf("not a channel metadata or create event")
	}

	channel := &Channel{
		ID:          getChannelID(evt),
		CreatedAt:   evt.CreatedAt,
		UpdatedAt:   evt.CreatedAt,
		Name:        evt.Content,
		Description: "",
		Picture:     "",
	}

	for _, tag := range evt.Tags {
		if len(tag) >= 2 {
			switch tag[0] {
			case "name":
				channel.Name = tag[1]
			case "description":
				channel.Description = tag[1]
			case "picture":
				channel.Picture = tag[1]
			}
		}
	}

	return channel, nil
}

func ParseChannelMessage(evt *event.Event) (string, error) {
	if evt.Kind != KindChannelMessage {
		return "", fmt.Errorf("not a channel message event")
	}
	return evt.Content, nil
}

func IsReplaceableKind(kind int) bool {
	return kind == KindChannelCreate || kind == KindChannelMetadata
}
