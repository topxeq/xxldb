// Package fts provides full-text search functionality tests
package fts

import (
	"testing"
)

func TestSimpleTokenizer(t *testing.T) {
	tokenizer := &SimpleTokenizer{}

	tests := []struct {
		input    string
		expected []string
	}{
		{"Hello World", []string{"hello", "world"}},
		{"Hello, World!", []string{"hello", "world"}},
		{"test123 abc", []string{"test123", "abc"}},
		{"  multiple   spaces  ", []string{"multiple", "spaces"}},
		{"", []string(nil)},
		{"   ", []string(nil)},
		{"中文测试", []string{"中文测试"}},
		{"Hello 世界", []string{"hello", "世界"}},
	}

	for _, tt := range tests {
		tokens := tokenizer.Tokenize(tt.input)
		if len(tokens) != len(tt.expected) {
			t.Errorf("Tokenize(%q) = %v, want %v", tt.input, tokens, tt.expected)
			continue
		}
		for i, tok := range tokens {
			if tok != tt.expected[i] {
				t.Errorf("Tokenize(%q)[%d] = %q, want %q", tt.input, i, tok, tt.expected[i])
			}
		}
	}
}

func TestInvertedIndex(t *testing.T) {
	tokenizer := &SimpleTokenizer{}
	idx := NewInvertedIndex(tokenizer)

	// Test indexing documents
	err := idx.IndexDocument(1, "Hello World")
	if err != nil {
		t.Fatalf("IndexDocument failed: %v", err)
	}

	err = idx.IndexDocument(2, "Hello Go")
	if err != nil {
		t.Fatalf("IndexDocument failed: %v", err)
	}

	err = idx.IndexDocument(3, "World of Programming")
	if err != nil {
		t.Fatalf("IndexDocument failed: %v", err)
	}

	// Test search
	results, err := idx.Search("Hello", 10, 0)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Search 'Hello' returned %d results, want 2", len(results))
	}

	results, err = idx.Search("World", 10, 0)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Search 'World' returned %d results, want 2", len(results))
	}

	results, err = idx.Search("Go", 10, 0)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Search 'Go' returned %d results, want 1", len(results))
	}

	// Test AND search
	results, err = idx.Search("Hello World", 10, 0)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Search 'Hello World' (AND) returned %d results, want 1", len(results))
	}

	// Test delete
	err = idx.DeleteDocument(1)
	if err != nil {
		t.Fatalf("DeleteDocument failed: %v", err)
	}

	results, err = idx.Search("Hello", 10, 0)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Search 'Hello' after delete returned %d results, want 1", len(results))
	}

	// Test update
	err = idx.UpdateDocument(2, "Hello Golang Programming")
	if err != nil {
		t.Fatalf("UpdateDocument failed: %v", err)
	}

	results, err = idx.Search("Golang", 10, 0)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Search 'Golang' after update returned %d results, want 1", len(results))
	}

	// Test stats
	stats := idx.Stats()
	if stats.DocumentCount != 2 {
		t.Errorf("Stats.DocumentCount = %d, want 2", stats.DocumentCount)
	}
}

func TestManager(t *testing.T) {
	m := NewManager()

	// Create index
	err := m.CreateIndex("articles", "content", nil)
	if err != nil {
		t.Fatalf("CreateIndex failed: %v", err)
	}

	// Check if index exists
	if !m.HasIndex("articles", "content") {
		t.Error("HasIndex returned false, want true")
	}

	// List indexes
	indexes := m.ListIndexes()
	if len(indexes) != 1 {
		t.Errorf("ListIndexes returned %d indexes, want 1", len(indexes))
	}

	// Index documents
	err = m.IndexDocument("articles", "content", 1, "Hello World")
	if err != nil {
		t.Fatalf("IndexDocument failed: %v", err)
	}

	err = m.IndexDocument("articles", "content", 2, "Hello Go")
	if err != nil {
		t.Fatalf("IndexDocument failed: %v", err)
	}

	// Search
	results, err := m.Search("articles", "content", "Hello", 10, 0)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Search returned %d results, want 2", len(results))
	}

	// Delete document
	err = m.DeleteDocument("articles", "content", 1)
	if err != nil {
		t.Fatalf("DeleteDocument failed: %v", err)
	}

	results, err = m.Search("articles", "content", "Hello", 10, 0)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Search after delete returned %d results, want 1", len(results))
	}

	// Drop index
	err = m.DropIndex("articles", "content")
	if err != nil {
		t.Fatalf("DropIndex failed: %v", err)
	}

	if m.HasIndex("articles", "content") {
		t.Error("HasIndex returned true after drop, want false")
	}
}

func TestDuplicateIndex(t *testing.T) {
	m := NewManager()

	err := m.CreateIndex("test", "col1", nil)
	if err != nil {
		t.Fatalf("First CreateIndex failed: %v", err)
	}

	// Try to create duplicate index
	err = m.CreateIndex("test", "col1", nil)
	if err == nil {
		t.Error("CreateIndex should fail for duplicate index")
	}
}

func TestDropNonExistentIndex(t *testing.T) {
	m := NewManager()

	err := m.DropIndex("nonexistent", "col1")
	if err == nil {
		t.Error("DropIndex should fail for non-existent index")
	}
}

func TestSearchNonExistentIndex(t *testing.T) {
	m := NewManager()

	_, err := m.Search("nonexistent", "col1", "query", 10, 0)
	if err == nil {
		t.Error("Search should fail for non-existent index")
	}
}

func TestSearchPagination(t *testing.T) {
	tokenizer := &SimpleTokenizer{}
	idx := NewInvertedIndex(tokenizer)

	// Index multiple documents
	for i := 1; i <= 20; i++ {
		err := idx.IndexDocument(uint64(i), "Hello World")
		if err != nil {
			t.Fatalf("IndexDocument failed: %v", err)
		}
	}

	// Test pagination
	results, err := idx.Search("Hello World", 5, 0)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("Search with limit 5 returned %d results, want 5", len(results))
	}

	results, err = idx.Search("Hello World", 5, 5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("Search with offset 5, limit 5 returned %d results, want 5", len(results))
	}

	// Test offset beyond results
	results, err = idx.Search("Hello World", 5, 100)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Search with offset 100 returned %d results, want 0", len(results))
	}
}

func TestNoMatchSearch(t *testing.T) {
	tokenizer := &SimpleTokenizer{}
	idx := NewInvertedIndex(tokenizer)

	err := idx.IndexDocument(1, "Hello World")
	if err != nil {
		t.Fatalf("IndexDocument failed: %v", err)
	}

	results, err := idx.Search("NonExistent", 10, 0)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Search for non-existent term returned %d results, want 0", len(results))
	}
}
