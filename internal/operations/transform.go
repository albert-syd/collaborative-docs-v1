package operations

import "fmt"

// Transform adjusts two concurrent operations created against the same
// document version so they can be applied sequentially without conflict.
//
// Given operations op1 and op2 at the same version, Transform returns
// transformed versions (op1', op2') such that:
//   apply(op2', apply(op1', doc)) == apply(op1, apply(op2, doc))
//
// This is the core algorithm of Operational Transformation.
func Transform(op1, op2 *Operation) (*Operation, *Operation, error) {
	if op1 == nil || op2 == nil {
		return nil, nil, fmt.Errorf("operations cannot be nil")
	}

	if err := op1.Validate(); err != nil {
		return nil, nil, fmt.Errorf("op1 invalid: %w", err)
	}
	if err := op2.Validate(); err != nil {
		return nil, nil, fmt.Errorf("op2 invalid: %w", err)
	}

	op1Prime := &Operation{
		Type:     op1.Type,
		Position: op1.Position,
		Text:     op1.Text,
		Version:  op1.Version + 1,
	}
	op2Prime := &Operation{
		Type:     op2.Type,
		Position: op2.Position,
		Text:     op2.Text,
		Version:  op2.Version + 1,
	}

	switch {
	case op1.Type == OpInsert && op2.Type == OpInsert:
		transformInsertInsert(op1Prime, op2Prime)
	case op1.Type == OpInsert && op2.Type == OpDelete:
		transformInsertDelete(op1Prime, op2Prime)
	case op1.Type == OpDelete && op2.Type == OpInsert:
		transformDeleteInsert(op1Prime, op2Prime)
	case op1.Type == OpDelete && op2.Type == OpDelete:
		transformDeleteDelete(op1Prime, op2Prime)
	default:
		return nil, nil, fmt.Errorf("unsupported operation type combination: %s vs %s", op1.Type, op2.Type)
	}

	return op1Prime, op2Prime, nil
}

// transformInsertInsert adjusts two concurrent insert operations.
// When inserting at the same position, op2 gets right-bias priority.
func transformInsertInsert(op1, op2 *Operation) {
	if op1.Position < op2.Position {
		op2.Position += op1.Length()
	} else if op1.Position > op2.Position {
		op1.Position += op2.Length()
	} else {
		op2.Position += op1.Length()
	}
}

// transformInsertDelete adjusts insert and delete operations.
// Inserts at or before delete position shift the delete right.
func transformInsertDelete(insert, delete *Operation) {
	if insert.Position <= delete.Position {
		delete.Position += insert.Length()
	} else if insert.Position > delete.Position+delete.Length() {
		insert.Position -= delete.Length()
	} else {
		insert.Position = delete.Position
	}
}

// transformDeleteInsert is the inverse of transformInsertDelete.
func transformDeleteInsert(delete, insert *Operation) {
	transformInsertDelete(insert, delete)
}

// transformDeleteDelete handles two concurrent delete operations.
// Adjusts positions for non-overlapping deletes and handles overlaps
// by marking redundant portions as empty.
func transformDeleteDelete(op1, op2 *Operation) {
	op1Start := op1.Position
	op1End := op1.Position + op1.Length()
	op2Start := op2.Position
	op2End := op2.Position + op2.Length()

	if op1End <= op2Start {
		op2.Position -= op1.Length()
		return
	}

	if op2End <= op1Start {
		op1.Position -= op2.Length()
		return
	}

	if op1Start == op2Start && op1End == op2End {
		op1.Text = ""
		op2.Text = ""
		return
	}

	if op2Start < op1Start {
		overlap := min(op1End, op2End) - op1Start
		op1.Position = op2Start
		if overlap > 0 && overlap < op1.Length() {
			op1.Text = op1.Text[overlap:]
		} else if overlap >= op1.Length() {
			op1.Text = ""
		}
	} else {
		overlap := min(op1End, op2End) - op2Start
		op2.Position = op1Start
		if overlap > 0 && overlap < op2.Length() {
			op2.Text = op2.Text[overlap:]
		} else if overlap >= op2.Length() {
			op2.Text = ""
		}
	}
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
