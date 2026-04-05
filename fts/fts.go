// Package fts provides full-text search functionality for xxldb
package fts

import (
	"sort"
	"strings"
	"sync"

	"github.com/topxeq/xxldb/types"
)

// SearchResult represents a search result
type SearchResult struct {
	RowID      uint64
	Score      float64
	Highlights []string
}

// IndexStats represents index statistics
type IndexStats struct {
	DocumentCount int64
	TermCount     int64
	IndexSize     int64
}

// Tokenizer defines the interface for text tokenization
type Tokenizer interface {
	Tokenize(text string) []string
}

// FullTextIndexer defines the interface for full-text indexing
type FullTextIndexer interface {
	// IndexDocument indexes a document
	IndexDocument(rowID uint64, content string) error

	// DeleteDocument removes a document from the index
	DeleteDocument(rowID uint64) error

	// UpdateDocument updates an indexed document
	UpdateDocument(rowID uint64, content string) error

	// Search performs a full-text search
	Search(query string, limit, offset int) ([]SearchResult, error)

	// Stats returns index statistics
	Stats() IndexStats

	// Close closes the indexer
	Close() error
}

// SimpleTokenizer is a basic tokenizer
type SimpleTokenizer struct{}

// Tokenize splits text into tokens
func (t *SimpleTokenizer) Tokenize(text string) []string {
	// Convert to lowercase
	text = strings.ToLower(text)

	// Split by whitespace and punctuation
	var tokens []string
	var current strings.Builder

	for _, r := range text {
		if isLetterOrDigit(r) {
			current.WriteRune(r)
		} else {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

func isLetterOrDigit(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r >= 128
}

// InvertedIndex implements a simple inverted index
type InvertedIndex struct {
	mu        sync.RWMutex
	tokenizer Tokenizer

	// term -> postings list (row IDs)
	index map[string][]uint64

	// rowID -> document terms (for deletion)
	documents map[uint64]map[string]bool

	// rowID -> original content
	contents map[uint64]string

	// statistics
	docCount int64
	termCount int64
}

// NewInvertedIndex creates a new inverted index
func NewInvertedIndex(tokenizer Tokenizer) *InvertedIndex {
	if tokenizer == nil {
		tokenizer = &SimpleTokenizer{}
	}

	return &InvertedIndex{
		tokenizer: tokenizer,
		index:     make(map[string][]uint64),
		documents: make(map[uint64]map[string]bool),
		contents:  make(map[uint64]string),
	}
}

// IndexDocument indexes a document
func (idx *InvertedIndex) IndexDocument(rowID uint64, content string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Check if document already exists
	if _, exists := idx.documents[rowID]; exists {
		// Remove old index entries
		idx.removeDocumentLocked(rowID)
	}

	// Tokenize
	tokens := idx.tokenizer.Tokenize(content)
	if len(tokens) == 0 {
		return nil
	}

	// Build term set for this document
	termSet := make(map[string]bool)
	for _, token := range tokens {
		termSet[token] = true
	}

	// Add to inverted index
	for term := range termSet {
		// Check if rowID already in postings
		postings := idx.index[term]
		if !containsRowID(postings, rowID) {
			idx.index[term] = append(postings, rowID)
		}
	}

	// Store document info
	idx.documents[rowID] = termSet
	idx.contents[rowID] = content
	idx.docCount++
	idx.termCount = int64(len(idx.index))

	return nil
}

// DeleteDocument removes a document from the index
func (idx *InvertedIndex) DeleteDocument(rowID uint64) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	return idx.removeDocumentLocked(rowID)
}

func (idx *InvertedIndex) removeDocumentLocked(rowID uint64) error {
	termSet, exists := idx.documents[rowID]
	if !exists {
		return nil
	}

	// Remove from inverted index
	for term := range termSet {
		postings := idx.index[term]
		idx.index[term] = removeRowID(postings, rowID)
		if len(idx.index[term]) == 0 {
			delete(idx.index, term)
		}
	}

	// Remove document info
	delete(idx.documents, rowID)
	delete(idx.contents, rowID)
	idx.docCount--
	idx.termCount = int64(len(idx.index))

	return nil
}

// UpdateDocument updates an indexed document
func (idx *InvertedIndex) UpdateDocument(rowID uint64, content string) error {
	return idx.IndexDocument(rowID, content)
}

// Search performs a full-text search
func (idx *InvertedIndex) Search(query string, limit, offset int) ([]SearchResult, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	// Tokenize query
	queryTerms := idx.tokenizer.Tokenize(query)
	if len(queryTerms) == 0 {
		return nil, nil
	}

	// Find documents containing all query terms (AND search)
	var candidateRows map[uint64]bool

	for _, term := range queryTerms {
		postings, exists := idx.index[term]
		if !exists {
			// Term not found, no results
			return nil, nil
		}

		termRows := make(map[uint64]bool)
		for _, rowID := range postings {
			termRows[rowID] = true
		}

		if candidateRows == nil {
			candidateRows = termRows
		} else {
			// Intersection
			for rowID := range candidateRows {
				if !termRows[rowID] {
					delete(candidateRows, rowID)
				}
			}
		}

		if len(candidateRows) == 0 {
			return nil, nil
		}
	}

	// Calculate scores (TF-IDF simplified)
	results := make([]SearchResult, 0, len(candidateRows))
	for rowID := range candidateRows {
		score := idx.calculateScore(rowID, queryTerms)
		results = append(results, SearchResult{
			RowID: rowID,
			Score: score,
		})
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Apply offset and limit
	if offset >= len(results) {
		return nil, nil
	}

	end := offset + limit
	if end > len(results) {
		end = len(results)
	}

	return results[offset:end], nil
}

// calculateScore calculates a simplified TF-IDF score
func (idx *InvertedIndex) calculateScore(rowID uint64, queryTerms []string) float64 {
	termSet := idx.documents[rowID]
	if termSet == nil {
		return 0
	}

	var score float64
	docLen := len(termSet)
	totalDocs := idx.docCount
	if totalDocs == 0 {
		totalDocs = 1
	}

	for _, term := range queryTerms {
		// Term frequency in document
		tf := 1.0
		if docLen > 0 {
			tf = 1.0 / float64(docLen)
		}

		// Inverse document frequency
		postings := idx.index[term]
		df := float64(len(postings))
		idf := 1.0
		if df > 0 {
			idf = float64(totalDocs) / df
		}

		score += tf * idf
	}

	return score
}

// Stats returns index statistics
func (idx *InvertedIndex) Stats() IndexStats {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return IndexStats{
		DocumentCount: idx.docCount,
		TermCount:     idx.termCount,
	}
}

// Close closes the index
func (idx *InvertedIndex) Close() error {
	return nil
}

// Helper functions

func containsRowID(postings []uint64, rowID uint64) bool {
	for _, id := range postings {
		if id == rowID {
			return true
		}
	}
	return false
}

func removeRowID(postings []uint64, rowID uint64) []uint64 {
	for i, id := range postings {
		if id == rowID {
			return append(postings[:i], postings[i+1:]...)
		}
	}
	return postings
}

// Manager manages full-text indexes for multiple tables/columns
type Manager struct {
	mu     sync.RWMutex
	indexes map[string]FullTextIndexer // "table.column" -> indexer
}

// NewManager creates a new FTS manager
func NewManager() *Manager {
	return &Manager{
		indexes: make(map[string]FullTextIndexer),
	}
}

// CreateIndex creates a full-text index for a table/column
func (m *Manager) CreateIndex(tableName, columnName string, indexer FullTextIndexer) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := tableName + "." + columnName
	if _, exists := m.indexes[key]; exists {
		return types.NewError(types.ErrDuplicateKey, "full-text index already exists: %s", key)
	}

	if indexer == nil {
		indexer = NewInvertedIndex(nil)
	}

	m.indexes[key] = indexer
	return nil
}

// DropIndex drops a full-text index
func (m *Manager) DropIndex(tableName, columnName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := tableName + "." + columnName
	indexer, exists := m.indexes[key]
	if !exists {
		return types.NewError(types.ErrNotFound, "full-text index not found: %s", key)
	}

	indexer.Close()
	delete(m.indexes, key)
	return nil
}

// GetIndex gets a full-text index
func (m *Manager) GetIndex(tableName, columnName string) FullTextIndexer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.indexes[tableName+"."+columnName]
}

// HasIndex checks if an index exists
func (m *Manager) HasIndex(tableName, columnName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.indexes[tableName+"."+columnName]
	return exists
}

// IndexDocument indexes a document in the specified index
func (m *Manager) IndexDocument(tableName, columnName string, rowID uint64, content string) error {
	m.mu.RLock()
	indexer, exists := m.indexes[tableName+"."+columnName]
	m.mu.RUnlock()

	if !exists {
		return nil // No index, skip silently
	}

	return indexer.IndexDocument(rowID, content)
}

// DeleteDocument removes a document from the index
func (m *Manager) DeleteDocument(tableName, columnName string, rowID uint64) error {
	m.mu.RLock()
	indexer, exists := m.indexes[tableName+"."+columnName]
	m.mu.RUnlock()

	if !exists {
		return nil
	}

	return indexer.DeleteDocument(rowID)
}

// Search performs a full-text search
func (m *Manager) Search(tableName, columnName, query string, limit, offset int) ([]SearchResult, error) {
	m.mu.RLock()
	indexer, exists := m.indexes[tableName+"."+columnName]
	m.mu.RUnlock()

	if !exists {
		return nil, types.NewError(types.ErrNotFound, "full-text index not found: %s.%s", tableName, columnName)
	}

	return indexer.Search(query, limit, offset)
}

// Close closes all indexes
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, indexer := range m.indexes {
		indexer.Close()
	}

	m.indexes = make(map[string]FullTextIndexer)
	return nil
}

// ListIndexes returns all index keys
func (m *Manager) ListIndexes() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := make([]string, 0, len(m.indexes))
	for key := range m.indexes {
		keys = append(keys, key)
	}
	return keys
}
