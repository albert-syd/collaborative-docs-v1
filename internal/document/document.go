package document

import (
	"collaborative-docs/internal/operations"
	"sync"
	"time"
)

// Document represents thread-safe shared document state.
// It tracks content, version number, and last modification time.
type Document struct {
	content      string
	version      int
	lastModified time.Time
	mu           sync.RWMutex
}

// NewDocument creates a new empty document.
func NewDocument() *Document {
	return &Document{
		content:      "",
		version:      0,
		lastModified: time.Now(),
	}
}

// GetContent returns the current document content.
func (d *Document) GetContent() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.content
}

// SetContent updates the document content and increments the version.
func (d *Document) SetContent(content string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.content = content
	d.version++
	d.lastModified = time.Now()
}

// GetVersion returns the current version number.
func (d *Document) GetVersion() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.version
}

// GetStats returns document version, last modified time, and content length.
func (d *Document) GetStats() (version int, lastModified time.Time, length int) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.version, d.lastModified, len(d.content)
}

// ApplyOperation applies an OT operation and returns the new content and version.
func (d *Document) ApplyOperation(op *operations.Operation) (string, int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	newContent, err := operations.Apply(d.content, op)
	if err != nil {
		return "", d.version, err
	}

	d.content = newContent
	d.version++
	d.lastModified = time.Now()

	return newContent, d.version, nil
}

// GetContentAndVersion atomically returns both content and version.
func (d *Document) GetContentAndVersion() (string, int) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.content, d.version
}
