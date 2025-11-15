package operations

import (
	"encoding/json"
	"fmt"
)

// OpType represents the type of operation
type OpType string

const (
	OpInsert OpType = "insert" // Insert text at position
	OpDelete OpType = "delete" // Delete text at position
	OpRetain OpType = "retain" // Retain text (for composition)
)

// Operation represents a text editing operation in OT.
// Operations can be transformed against each other for conflict resolution.
type Operation struct {
	Type     OpType `json:"type"`
	Position int    `json:"position"`
	Text     string `json:"text,omitempty"`
	Version  int    `json:"version"`
}

// NewInsertOp creates a new insert operation.
func NewInsertOp(position int, text string, version int) *Operation {
	return &Operation{
		Type:     OpInsert,
		Position: position,
		Text:     text,
		Version:  version,
	}
}

// NewDeleteOp creates a new delete operation.
func NewDeleteOp(position int, text string, version int) *Operation {
	return &Operation{
		Type:     OpDelete,
		Position: position,
		Text:     text,
		Version:  version,
	}
}

// String returns a human-readable representation of the operation.
func (op *Operation) String() string {
	switch op.Type {
	case OpInsert:
		return fmt.Sprintf("Insert('%s' at %d, v%d)", op.Text, op.Position, op.Version)
	case OpDelete:
		return fmt.Sprintf("Delete('%s' at %d, v%d)", op.Text, op.Position, op.Version)
	case OpRetain:
		return fmt.Sprintf("Retain(%d chars at %d, v%d)", len(op.Text), op.Position, op.Version)
	default:
		return fmt.Sprintf("Unknown operation")
	}
}

// ToJSON converts the operation to JSON.
func (op *Operation) ToJSON() (string, error) {
	data, err := json.Marshal(op)
	if err != nil {
		return "", fmt.Errorf("failed to marshal operation: %w", err)
	}
	return string(data), nil
}

// FromJSON creates an operation from JSON.
func FromJSON(jsonStr string) (*Operation, error) {
	var op Operation
	if err := json.Unmarshal([]byte(jsonStr), &op); err != nil {
		return nil, fmt.Errorf("failed to unmarshal operation: %w", err)
	}
	return &op, nil
}

// Validate checks if the operation is valid.
func (op *Operation) Validate() error {
	if op.Position < 0 {
		return fmt.Errorf("invalid position: %d (must be >= 0)", op.Position)
	}

	switch op.Type {
	case OpInsert:
		if op.Text == "" {
			return fmt.Errorf("insert operation must have non-empty text")
		}
	case OpDelete:
		if op.Text == "" {
			return fmt.Errorf("delete operation must have non-empty text")
		}
	case OpRetain:
		// Retain is valid with empty text
	default:
		return fmt.Errorf("unknown operation type: %s", op.Type)
	}

	if op.Version < 0 {
		return fmt.Errorf("invalid version: %d (must be >= 0)", op.Version)
	}

	return nil
}

// Length returns the number of characters affected by this operation.
func (op *Operation) Length() int {
	return len(op.Text)
}
