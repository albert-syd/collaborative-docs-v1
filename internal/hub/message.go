package hub

import (
	"collaborative-docs/internal/operations"
	"encoding/json"
	"fmt"
)

// MessageType represents the kind of message being sent
type MessageType string

const (
	MsgTypeContent   MessageType = "content"    // Full content update
	MsgTypeOperation MessageType = "operation"  // OT operation
	MsgTypeUserCount MessageType = "user_count" // System message for user count
)

// Message represents the WebSocket protocol for exchanging
// document content, operations, or system notifications.
type Message struct {
	Type       MessageType            `json:"type"`
	DocumentID string                 `json:"document_id,omitempty"`
	Content    string                 `json:"content,omitempty"`
	Operation  *operations.Operation  `json:"operation,omitempty"`
	UserCount  int                    `json:"user_count,omitempty"`
}

// NewContentMessage creates a message with full content.
func NewContentMessage(content string) *Message {
	return &Message{
		Type:    MsgTypeContent,
		Content: content,
	}
}

// NewOperationMessage creates a message with an OT operation.
func NewOperationMessage(op *operations.Operation) *Message {
	return &Message{
		Type:      MsgTypeOperation,
		Operation: op,
	}
}

// NewUserCountMessage creates a user count system message.
func NewUserCountMessage(count int) *Message {
	return &Message{
		Type:      MsgTypeUserCount,
		UserCount: count,
	}
}

// ToJSON serializes the message to JSON.
func (m *Message) ToJSON() (string, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("failed to marshal message: %w", err)
	}
	return string(data), nil
}

// ToBytes serializes the message to JSON bytes.
func (m *Message) ToBytes() ([]byte, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}
	return data, nil
}

// MessageFromJSON deserializes a message from JSON.
func MessageFromJSON(jsonStr string) (*Message, error) {
	var msg Message
	if err := json.Unmarshal([]byte(jsonStr), &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}
	return &msg, nil
}

// MessageFromBytes deserializes a message from JSON bytes.
func MessageFromBytes(data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}
	return &msg, nil
}

// IsLegacyContent checks if the data is a legacy (non-JSON) content message.
func IsLegacyContent(data []byte) bool {
	var test map[string]interface{}
	err := json.Unmarshal(data, &test)
	return err != nil
}

// HandleLegacyContent wraps legacy plain-text content in a Message.
func HandleLegacyContent(data []byte) *Message {
	return &Message{
		Type:    MsgTypeContent,
		Content: string(data),
	}
}
