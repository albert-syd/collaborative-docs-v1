package operations

import "fmt"

// Apply executes an operation on a document and returns the result.
func Apply(doc string, op *Operation) (string, error) {
	if op == nil {
		return "", fmt.Errorf("operation cannot be nil")
	}

	if err := op.Validate(); err != nil {
		return "", fmt.Errorf("invalid operation: %w", err)
	}

	switch op.Type {
	case OpInsert:
		return applyInsert(doc, op)

	case OpDelete:
		return applyDelete(doc, op)

	case OpRetain:
		return doc, nil

	default:
		return "", fmt.Errorf("unknown operation type: %s", op.Type)
	}
}

// applyInsert inserts text at the specified position.
func applyInsert(doc string, op *Operation) (string, error) {
	docLen := len(doc)

	if op.Position < 0 || op.Position > docLen {
		return "", fmt.Errorf("insert position %d out of range [0, %d]", op.Position, docLen)
	}

	result := doc[:op.Position] + op.Text + doc[op.Position:]
	return result, nil
}

// applyDelete removes text at the specified position.
func applyDelete(doc string, op *Operation) (string, error) {
	docLen := len(doc)
	deleteLen := op.Length()

	if op.Position < 0 || op.Position >= docLen {
		return "", fmt.Errorf("delete position %d out of range [0, %d)", op.Position, docLen)
	}

	if op.Position+deleteLen > docLen {
		return "", fmt.Errorf("delete range [%d, %d) exceeds document length %d",
			op.Position, op.Position+deleteLen, docLen)
	}

	actualText := doc[op.Position : op.Position+deleteLen]
	if actualText != op.Text {
		return "", fmt.Errorf("delete text mismatch: expected '%s', found '%s'",
			op.Text, actualText)
	}

	result := doc[:op.Position] + doc[op.Position+deleteLen:]
	return result, nil
}

// ApplyAll applies a sequence of operations to a document.
func ApplyAll(doc string, ops []*Operation) (string, error) {
	result := doc
	for i, op := range ops {
		newDoc, err := Apply(result, op)
		if err != nil {
			return "", fmt.Errorf("failed to apply operation %d: %w", i, err)
		}
		result = newDoc
	}
	return result, nil
}
