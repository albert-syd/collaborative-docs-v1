package document

import (
	"sync"
	"testing"
	"time"
)

// TestNewDocument verifies document initialization.
func TestNewDocument(t *testing.T) {
	doc := NewDocument()

	if doc == nil {
		t.Fatal("NewDocument() returned nil")
	}

	if doc.content != "" {
		t.Errorf("expected empty content, got: %q", doc.content)
	}

	if doc.version != 0 {
		t.Errorf("expected version 0, got: %d", doc.version)
	}

	if time.Since(doc.lastModified) > time.Second {
		t.Errorf("lastModified is too old: %v", doc.lastModified)
	}
}

// TestSetAndGetContent verifies content operations.
func TestSetAndGetContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantVer int
	}{
		{
			name:    "simple text",
			content: "Hello, World!",
			wantVer: 1,
		},
		{
			name:    "empty string",
			content: "",
			wantVer: 1,
		},
		{
			name:    "unicode",
			content: "Hello 世界 🌍",
			wantVer: 1,
		},
		{
			name:    "multiline",
			content: "Line 1\nLine 2\nLine 3",
			wantVer: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := NewDocument()
			doc.SetContent(tt.content)

			if got := doc.GetContent(); got != tt.content {
				t.Errorf("GetContent() = %q, want %q", got, tt.content)
			}

			if got := doc.GetVersion(); got != tt.wantVer {
				t.Errorf("GetVersion() = %d, want %d", got, tt.wantVer)
			}
		})
	}
}

// TestVersionIncrement verifies version tracking across multiple updates.
func TestVersionIncrement(t *testing.T) {
	tests := []struct {
		name     string
		updates  []string
		wantVers []int
	}{
		{
			name:     "three updates",
			updates:  []string{"First", "Second", "Third"},
			wantVers: []int{1, 2, 3},
		},
		{
			name:     "single update",
			updates:  []string{"Only"},
			wantVers: []int{1},
		},
		{
			name:     "many updates",
			updates:  []string{"A", "B", "C", "D", "E"},
			wantVers: []int{1, 2, 3, 4, 5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := NewDocument()

			if v := doc.GetVersion(); v != 0 {
				t.Errorf("initial version = %d, want 0", v)
			}

			for i, content := range tt.updates {
				doc.SetContent(content)
				if v := doc.GetVersion(); v != tt.wantVers[i] {
					t.Errorf("after update %d: version = %d, want %d", i+1, v, tt.wantVers[i])
				}
			}
		})
	}
}

// TestGetStats verifies document statistics reporting.
func TestGetStats(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		wantVer    int
		wantLength int
	}{
		{
			name:       "simple content",
			content:    "Test content",
			wantVer:    1,
			wantLength: 12,
		},
		{
			name:       "empty content",
			content:    "",
			wantVer:    1,
			wantLength: 0,
		},
		{
			name:       "unicode content",
			content:    "Hello 世界",
			wantVer:    1,
			wantLength: 12, // byte length
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := NewDocument()
			doc.SetContent(tt.content)

			version, lastModified, length := doc.GetStats()

			if version != tt.wantVer {
				t.Errorf("version = %d, want %d", version, tt.wantVer)
			}

			if length != tt.wantLength {
				t.Errorf("length = %d, want %d", length, tt.wantLength)
			}

			if time.Since(lastModified) > time.Second {
				t.Errorf("lastModified is too old: %v", lastModified)
			}
		})
	}
}

// TestConcurrentReads verifies that multiple goroutines can read simultaneously
func TestConcurrentReads(t *testing.T) {
	doc := NewDocument()
	content := "Concurrent read test"
	doc.SetContent(content)

	// Number of concurrent readers
	numReaders := 100
	var wg sync.WaitGroup
	wg.Add(numReaders)

	// Track if any reader got wrong content
	errors := make(chan string, numReaders)

	// Start multiple readers
	for i := 0; i < numReaders; i++ {
		go func() {
			defer wg.Done()
			got := doc.GetContent()
			if got != content {
				errors <- got
			}
		}()
	}

	// Wait for all readers to finish
	wg.Wait()
	close(errors)

	// Check for errors
	for got := range errors {
		t.Errorf("Concurrent read got wrong content: %q, want %q", got, content)
	}
}

// TestConcurrentWrites verifies that concurrent writes don't cause data races
func TestConcurrentWrites(t *testing.T) {
	doc := NewDocument()

	// Number of concurrent writers
	numWriters := 100
	var wg sync.WaitGroup
	wg.Add(numWriters)

	// Each writer writes their ID
	for i := 0; i < numWriters; i++ {
		go func(id int) {
			defer wg.Done()
			doc.SetContent(string(rune('A' + id%26))) // A, B, C, etc.
		}(i)
	}

	// Wait for all writers to finish
	wg.Wait()

	// Version should equal number of writes
	if v := doc.GetVersion(); v != numWriters {
		t.Errorf("After %d writes: version = %d, want %d", numWriters, v, numWriters)
	}

	// Content should be one of the written values (we don't care which due to race)
	content := doc.GetContent()
	if len(content) != 1 || content[0] < 'A' || content[0] > 'Z' {
		t.Errorf("Unexpected content after concurrent writes: %q", content)
	}
}

// TestConcurrentReadWrite verifies that reads and writes can happen simultaneously
func TestConcurrentReadWrite(t *testing.T) {
	doc := NewDocument()
	doc.SetContent("Initial") // This counts as write #1

	var wg sync.WaitGroup
	numOperations := 50

	// Start readers
	wg.Add(numOperations)
	for i := 0; i < numOperations; i++ {
		go func() {
			defer wg.Done()
			_ = doc.GetContent() // Just read, don't check value
		}()
	}

	// Start writers
	wg.Add(numOperations)
	for i := 0; i < numOperations; i++ {
		go func(id int) {
			defer wg.Done()
			doc.SetContent(string(rune('0' + id%10)))
		}(i)
	}

	// Wait for all operations
	wg.Wait()

	// Version should equal number of writes + 1 (for the initial SetContent)
	expectedVersion := numOperations + 1
	if v := doc.GetVersion(); v != expectedVersion {
		t.Errorf("After %d writes: version = %d, want %d", numOperations+1, v, expectedVersion)
	}
}

// TestContentSizes verifies handling of various content sizes.
func TestContentSizes(t *testing.T) {
	tests := []struct {
		name string
		size int
	}{
		{
			name: "empty",
			size: 0,
		},
		{
			name: "small",
			size: 100,
		},
		{
			name: "medium",
			size: 10 * 1024, // 10KB
		},
		{
			name: "large",
			size: 1024 * 1024, // 1MB
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := NewDocument()
			content := string(make([]byte, tt.size))
			doc.SetContent(content)

			got := doc.GetContent()
			if len(got) != tt.size {
				t.Errorf("content length = %d, want %d", len(got), tt.size)
			}

			if v := doc.GetVersion(); v != 1 {
				t.Errorf("version = %d, want 1", v)
			}
		})
	}
}

// BenchmarkGetContent measures read performance
func BenchmarkGetContent(b *testing.B) {
	doc := NewDocument()
	doc.SetContent("Benchmark content")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = doc.GetContent()
	}
}

// BenchmarkSetContent measures write performance
func BenchmarkSetContent(b *testing.B) {
	doc := NewDocument()
	content := "Benchmark content"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		doc.SetContent(content)
	}
}

// BenchmarkConcurrentReads measures concurrent read performance
func BenchmarkConcurrentReads(b *testing.B) {
	doc := NewDocument()
	doc.SetContent("Concurrent benchmark")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = doc.GetContent()
		}
	})
}
