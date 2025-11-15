package operations

import (
	"testing"
)

// TestOperationCreation verifies operation constructor functions.
func TestOperationCreation(t *testing.T) {
	tests := []struct {
		name     string
		createOp func() *Operation
		wantType OpType
		wantPos  int
		wantText string
		wantVer  int
	}{
		{
			name:     "insert operation",
			createOp: func() *Operation { return NewInsertOp(5, "hello", 1) },
			wantType: OpInsert,
			wantPos:  5,
			wantText: "hello",
			wantVer:  1,
		},
		{
			name:     "delete operation",
			createOp: func() *Operation { return NewDeleteOp(3, "world", 2) },
			wantType: OpDelete,
			wantPos:  3,
			wantText: "world",
			wantVer:  2,
		},
		{
			name:     "insert at position 0",
			createOp: func() *Operation { return NewInsertOp(0, "start", 0) },
			wantType: OpInsert,
			wantPos:  0,
			wantText: "start",
			wantVer:  0,
		},
		{
			name:     "delete single character",
			createOp: func() *Operation { return NewDeleteOp(10, "x", 5) },
			wantType: OpDelete,
			wantPos:  10,
			wantText: "x",
			wantVer:  5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := tt.createOp()

			if op.Type != tt.wantType {
				t.Errorf("Type = %s, want %s", op.Type, tt.wantType)
			}
			if op.Position != tt.wantPos {
				t.Errorf("Position = %d, want %d", op.Position, tt.wantPos)
			}
			if op.Text != tt.wantText {
				t.Errorf("Text = %q, want %q", op.Text, tt.wantText)
			}
			if op.Version != tt.wantVer {
				t.Errorf("Version = %d, want %d", op.Version, tt.wantVer)
			}
		})
	}
}

// TestOperationValidation tests operation validation
func TestOperationValidation(t *testing.T) {
	tests := []struct {
		name    string
		op      *Operation
		wantErr bool
	}{
		{
			name:    "valid insert",
			op:      NewInsertOp(0, "test", 1),
			wantErr: false,
		},
		{
			name:    "valid delete",
			op:      NewDeleteOp(5, "abc", 1),
			wantErr: false,
		},
		{
			name:    "invalid negative position",
			op:      &Operation{Type: OpInsert, Position: -1, Text: "x", Version: 1},
			wantErr: true,
		},
		{
			name:    "invalid empty insert",
			op:      &Operation{Type: OpInsert, Position: 0, Text: "", Version: 1},
			wantErr: true,
		},
		{
			name:    "invalid empty delete",
			op:      &Operation{Type: OpDelete, Position: 0, Text: "", Version: 1},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.op.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestApplyInsert tests applying insert operations
func TestApplyInsert(t *testing.T) {
	tests := []struct {
		name    string
		doc     string
		op      *Operation
		want    string
		wantErr bool
	}{
		{
			name:    "insert at beginning",
			doc:     "world",
			op:      NewInsertOp(0, "hello ", 1),
			want:    "hello world",
			wantErr: false,
		},
		{
			name:    "insert at end",
			doc:     "hello",
			op:      NewInsertOp(5, " world", 1),
			want:    "hello world",
			wantErr: false,
		},
		{
			name:    "insert in middle",
			doc:     "helo",
			op:      NewInsertOp(3, "l", 1),
			want:    "hello",
			wantErr: false,
		},
		{
			name:    "insert in empty doc",
			doc:     "",
			op:      NewInsertOp(0, "first", 1),
			want:    "first",
			wantErr: false,
		},
		{
			name:    "insert out of range",
			doc:     "test",
			op:      NewInsertOp(10, "x", 1),
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Apply(tt.doc, tt.op)
			if (err != nil) != tt.wantErr {
				t.Errorf("Apply() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Apply() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestApplyDelete tests applying delete operations
func TestApplyDelete(t *testing.T) {
	tests := []struct {
		name    string
		doc     string
		op      *Operation
		want    string
		wantErr bool
	}{
		{
			name:    "delete from beginning",
			doc:     "hello world",
			op:      NewDeleteOp(0, "hello ", 1),
			want:    "world",
			wantErr: false,
		},
		{
			name:    "delete from end",
			doc:     "hello world",
			op:      NewDeleteOp(5, " world", 1),
			want:    "hello",
			wantErr: false,
		},
		{
			name:    "delete from middle",
			doc:     "hello",
			op:      NewDeleteOp(1, "ell", 1),
			want:    "ho",
			wantErr: false,
		},
		{
			name:    "delete entire doc",
			doc:     "test",
			op:      NewDeleteOp(0, "test", 1),
			want:    "",
			wantErr: false,
		},
		{
			name:    "delete with wrong text",
			doc:     "hello",
			op:      NewDeleteOp(0, "world", 1),
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Apply(tt.doc, tt.op)
			if (err != nil) != tt.wantErr {
				t.Errorf("Apply() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Apply() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestTransformInsertInsert tests transforming two insert operations
func TestTransformInsertInsert(t *testing.T) {
	tests := []struct {
		name      string
		doc       string
		op1       *Operation
		op2       *Operation
		wantDoc   string
		wantErr   bool
	}{
		{
			name:    "inserts at different positions",
			doc:     "ac",
			op1:     NewInsertOp(1, "b", 1),  // insert 'b' at 1 -> "abc"
			op2:     NewInsertOp(2, "d", 1),  // insert 'd' at 2 -> "acd"
			wantDoc: "abcd",
			wantErr: false,
		},
		{
			name:    "inserts at same position",
			doc:     "ac",
			op1:     NewInsertOp(1, "x", 1),
			op2:     NewInsertOp(1, "y", 1),
			wantDoc: "axyc", // tie-breaking: op2 goes after op1
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Transform operations
			op1Prime, op2Prime, err := Transform(tt.op1, tt.op2)
			if (err != nil) != tt.wantErr {
				t.Errorf("Transform() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			// Apply in both orders and verify they converge
			// Order 1: apply op1, then op2'
			doc1, err := Apply(tt.doc, tt.op1)
			if err != nil {
				t.Errorf("Failed to apply op1: %v", err)
				return
			}
			result1, err := Apply(doc1, op2Prime)
			if err != nil {
				t.Errorf("Failed to apply op2': %v", err)
				return
			}

			// Order 2: apply op2, then op1'
			doc2, err := Apply(tt.doc, tt.op2)
			if err != nil {
				t.Errorf("Failed to apply op2: %v", err)
				return
			}
			result2, err := Apply(doc2, op1Prime)
			if err != nil {
				t.Errorf("Failed to apply op1': %v", err)
				return
			}

			// Both orders should produce the same result
			if result1 != result2 {
				t.Errorf("Results don't converge: %v != %v", result1, result2)
			}

			if result1 != tt.wantDoc {
				t.Errorf("Result = %v, want %v", result1, tt.wantDoc)
			}
		})
	}
}

// TestTransformInsertDelete tests transforming insert vs delete
func TestTransformInsertDelete(t *testing.T) {
	tests := []struct {
		name    string
		doc     string
		insert  *Operation
		delete  *Operation
		wantDoc string
	}{
		{
			name:    "insert before delete",
			doc:     "abcd",
			insert:  NewInsertOp(1, "X", 1), // insert X at 1 -> "aXbcd"
			delete:  NewDeleteOp(2, "cd", 1), // delete "cd" at 2 -> "ab"
			wantDoc: "aXb",
		},
		{
			name:    "insert after delete",
			doc:     "abcd",
			insert:  NewInsertOp(3, "X", 1),  // insert X at 3 -> "abcXd"
			delete:  NewDeleteOp(0, "ab", 1), // delete "ab" at 0 -> "cd"
			wantDoc: "cXd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Transform operations
			insertPrime, deletePrime, err := Transform(tt.insert, tt.delete)
			if err != nil {
				t.Errorf("Transform() error = %v", err)
				return
			}

			// Apply in both orders and verify convergence
			// Order 1: insert, then delete'
			doc1, _ := Apply(tt.doc, tt.insert)
			result1, _ := Apply(doc1, deletePrime)

			// Order 2: delete, then insert'
			doc2, _ := Apply(tt.doc, tt.delete)
			result2, _ := Apply(doc2, insertPrime)

			if result1 != result2 {
				t.Errorf("Results don't converge: '%v' != '%v'", result1, result2)
			}

			if result1 != tt.wantDoc {
				t.Errorf("Result = '%v', want '%v'", result1, tt.wantDoc)
			}
		})
	}
}

// TestTransformDeleteDelete tests transforming two delete operations
func TestTransformDeleteDelete(t *testing.T) {
	tests := []struct {
		name    string
		doc     string
		op1     *Operation
		op2     *Operation
		wantDoc string
	}{
		{
			name:    "non-overlapping deletes",
			doc:     "abcdef",
			op1:     NewDeleteOp(0, "ab", 1), // delete "ab" -> "cdef"
			op2:     NewDeleteOp(4, "ef", 1), // delete "ef" -> "abcd"
			wantDoc: "cd",
		},
		{
			name:    "identical deletes",
			doc:     "abcd",
			op1:     NewDeleteOp(1, "bc", 1),
			op2:     NewDeleteOp(1, "bc", 1),
			wantDoc: "ad",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Transform operations
			op1Prime, op2Prime, err := Transform(tt.op1, tt.op2)
			if err != nil {
				t.Errorf("Transform() error = %v", err)
				return
			}

			// Apply in both orders and verify convergence
			// Order 1: op1, then op2'
			doc1, err1 := Apply(tt.doc, tt.op1)
			if err1 != nil {
				t.Errorf("Failed to apply op1: %v", err1)
				return
			}

			var result1 string
			if op2Prime.Text != "" {
				result1, err1 = Apply(doc1, op2Prime)
				if err1 != nil {
					t.Errorf("Failed to apply op2': %v", err1)
					return
				}
			} else {
				result1 = doc1 // op2' is a no-op
			}

			// Order 2: op2, then op1'
			doc2, err2 := Apply(tt.doc, tt.op2)
			if err2 != nil {
				t.Errorf("Failed to apply op2: %v", err2)
				return
			}

			var result2 string
			if op1Prime.Text != "" {
				result2, err2 = Apply(doc2, op1Prime)
				if err2 != nil {
					t.Errorf("Failed to apply op1': %v", err2)
					return
				}
			} else {
				result2 = doc2 // op1' is a no-op
			}

			if result1 != result2 {
				t.Errorf("Results don't converge: '%v' != '%v'", result1, result2)
			}

			if result1 != tt.wantDoc {
				t.Errorf("Result = '%v', want '%v'", result1, tt.wantDoc)
			}
		})
	}
}

// TestJSONSerialization tests JSON encoding/decoding
func TestJSONSerialization(t *testing.T) {
	op := NewInsertOp(5, "hello", 3)

	// Serialize to JSON
	jsonStr, err := op.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error = %v", err)
	}

	// Deserialize from JSON
	op2, err := FromJSON(jsonStr)
	if err != nil {
		t.Fatalf("FromJSON() error = %v", err)
	}

	// Compare
	if op2.Type != op.Type || op2.Position != op.Position ||
		op2.Text != op.Text || op2.Version != op.Version {
		t.Errorf("Deserialized operation doesn't match: got %+v, want %+v", op2, op)
	}
}
