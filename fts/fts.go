// Package fts provides full-text search functionality for xxldb
package fts

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/go-ego/gse"
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

// FTSConfig defines configuration for full-text index
type FTSConfig struct {
	Levels       []int              `json:"levels"`        // 分词级别 [1,2,3]
	MinTermLen   int                `json:"min_term_len"`  // 最小词长度
	MaxTermLen   int                `json:"max_term_len"`  // 最大词长度
	WeightL1     float64            `json:"weight_l1"`     // Level 1权重（原始词）
	WeightL2     float64            `json:"weight_l2"`     // Level 2权重（拆分词）
	WeightL3     float64            `json:"weight_l3"`     // Level 3权重（单字）
	StorePos     bool               `json:"store_pos"`     // 存储位置信息
	CustomDict   map[string]bool    `json:"custom_dict"`   // 自定义词典
	Synonyms     map[string][]string `json:"synonyms"`     // 同义词映射
}

// DefaultFTSConfig returns default FTS configuration
func DefaultFTSConfig() *FTSConfig {
	return &FTSConfig{
		Levels:     []int{1},  // 默认只用Level 1
		MinTermLen: 1,
		MaxTermLen: 50,
		WeightL1:   1.0,
		WeightL2:   0.7,
		WeightL3:   0.3,
		StorePos:   false,
	}
}

// TokenWithLevel represents a token with level information
type TokenWithLevel struct {
	Term      string
	Level     int
	Position  int
	Weight    float64
	SourceTerm string
}

// Tokenizer defines the interface for text tokenization
type Tokenizer interface {
	Tokenize(text string) []string
}

// MultiLevelTokenizer defines interface for multi-level tokenization
type MultiLevelTokenizer interface {
	Tokenizer
	TokenizeWithLevels(text string, config *FTSConfig) []TokenWithLevel
	TokenizeWithLevelsForIndex(text string, config *FTSConfig, forIndex bool) []TokenWithLevel
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

	// SearchWithLevel performs a full-text search with specific level filter
	SearchWithLevel(query string, level int, limit, offset int) ([]SearchResult, error)

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

// ChineseTokenizer uses gse for Chinese word segmentation
type ChineseTokenizer struct {
	seg gse.Segmenter
}

// global segmenter instance (lazy initialized)
var (
	globalSegmenter     gse.Segmenter
	globalSegmenterErr  error
	globalSegmenterOnce sync.Once
)

// NewChineseTokenizer creates a new Chinese tokenizer
func NewChineseTokenizer() *ChineseTokenizer {
	globalSegmenterOnce.Do(func() {
		globalSegmenter, globalSegmenterErr = gse.NewEmbed()
	})
	return &ChineseTokenizer{seg: globalSegmenter}
}

// Tokenize splits text into tokens using Chinese word segmentation
func (t *ChineseTokenizer) Tokenize(text string) []string {
	if globalSegmenterErr != nil {
		// Fallback to simple tokenizer if gse failed to initialize
		return (&SimpleTokenizer{}).Tokenize(text)
	}

	// Use gse for segmentation (cut mode with HMM for better accuracy)
	tokens := t.seg.Cut(text, true)

	// Convert to lowercase and filter empty tokens
	result := make([]string, 0, len(tokens))
	for _, token := range tokens {
		token = strings.TrimSpace(strings.ToLower(token))
		if token != "" && len(token) > 0 {
			result = append(result, token)
		}
	}

	return result
}

// TokenizeWithLevels performs multi-level tokenization with custom dict and synonyms support
func (t *ChineseTokenizer) TokenizeWithLevels(text string, config *FTSConfig) []TokenWithLevel {
	return t.TokenizeWithLevelsForIndex(text, config, false)
}

// TokenizeWithLevelsForIndex performs multi-level tokenization with optional synonym expansion for indexing
func (t *ChineseTokenizer) TokenizeWithLevelsForIndex(text string, config *FTSConfig, forIndex bool) []TokenWithLevel {
	if globalSegmenterErr != nil {
		return nil
	}

	// Get base tokens
	baseTokens := t.seg.Cut(text, true)

	// Convert to lowercase and clean
	cleanedTokens := make([]string, 0, len(baseTokens))
	for _, token := range baseTokens {
		token = strings.TrimSpace(strings.ToLower(token))
		if token != "" {
			cleanedTokens = append(cleanedTokens, token)
		}
	}

	// Apply custom dictionary - merge custom terms
	if config != nil && len(config.CustomDict) > 0 {
		// Find custom dictionary terms in text
		lowerText := strings.ToLower(text)
		for customTerm := range config.CustomDict {
			if strings.Contains(lowerText, strings.ToLower(customTerm)) {
				// Add custom term if not already present
				found := false
				for _, t := range cleanedTokens {
					if t == strings.ToLower(customTerm) {
						found = true
						break
					}
				}
				if !found {
					cleanedTokens = append(cleanedTokens, strings.ToLower(customTerm))
				}
			}
		}
	}

	results := make([]TokenWithLevel, 0)
	pos := 0

	for _, token := range cleanedTokens {
		runes := []rune(token)
		runeLen := len(runes)

		// Level 1: 原始词
		if t.hasLevel(config, 1) {
			weight := config.WeightL1
			if weight == 0 {
				weight = 1.0
			}
			results = append(results, TokenWithLevel{
				Term:       token,
				Level:      1,
				Position:   pos,
				Weight:     weight,
				SourceTerm: token,
			})

			// Add synonyms ONLY for indexing, not for searching
			// This allows searching "写代码" to find documents containing "编程"
			if forIndex && config != nil && len(config.Synonyms) > 0 {
				if syns, ok := config.Synonyms[token]; ok {
					for _, syn := range syns {
						synLower := strings.ToLower(syn)
						results = append(results, TokenWithLevel{
							Term:       synLower,
							Level:      1,
							Position:   pos,
							Weight:     weight * 0.9, // Slightly lower weight for synonyms
							SourceTerm: token,
						})
					}
				}
			}
		}

		// Level 2: 拆分>=2字的词
		if t.hasLevel(config, 2) && runeLen >= 2 {
			subTerms := t.splitTerm(token)
			weight := config.WeightL2
			if weight == 0 {
				weight = 0.7
			}
			for _, sub := range subTerms {
				if len([]rune(sub)) >= config.MinTermLen {
					results = append(results, TokenWithLevel{
						Term:       sub,
						Level:      2,
						Position:   pos,
						Weight:     weight,
						SourceTerm: token,
					})
				}
			}
		}

		// Level 3: 单字拆分（可选）
		if t.hasLevel(config, 3) && runeLen >= 2 {
			weight := config.WeightL3
			if weight == 0 {
				weight = 0.3
			}
			for i, r := range runes {
				char := string(r)
				results = append(results, TokenWithLevel{
					Term:       char,
					Level:      3,
					Position:   pos + i,
					Weight:     weight,
					SourceTerm: token,
				})
			}
		}

		pos++
	}

	return results
}

// hasLevel checks if a level is enabled in config
func (t *ChineseTokenizer) hasLevel(config *FTSConfig, level int) bool {
	if config == nil || len(config.Levels) == 0 {
		return level == 1 // 默认只启用Level 1
	}
	for _, l := range config.Levels {
		if l == level {
			return true
		}
	}
	return false
}

// splitTerm splits a combined term into sub-terms
// 例如: "编程语言" -> ["编程", "语言"]
func (t *ChineseTokenizer) splitTerm(term string) []string {
	runes := []rune(term)
	runeLen := len(runes)

	if runeLen < 2 {
		return nil
	}

	results := make([]string, 0)

	// 策略1: 尝试2+2拆分 (对于4字词)
	if runeLen == 4 {
		left := string(runes[:2])
		right := string(runes[2:])
		results = append(results, left, right)
		return results
	}

	// 策略2: 对于>4字的词，尝试按常见边界拆分
	// 先尝试2字词开头
	if runeLen > 4 {
		// 尝试前2字+剩余
		left := string(runes[:2])
		right := string(runes[2:])
		if len([]rune(right)) >= 2 {
			results = append(results, left)
			// 剩余部分继续拆分
			subResults := t.splitTerm(right)
			results = append(results, subResults...)
			return results
		}
	}

	// 策略3: 对于3字词，尝试1+2或2+1
	if runeLen == 3 {
		// 优先2+1
		results = append(results, string(runes[:2]), string(runes[2:]))
		return results
	}

	// 策略4: 按每2字一组拆分
	for i := 0; i < runeLen; i += 2 {
		end := i + 2
		if end > runeLen {
			end = runeLen
		}
		if end - i >= 1 {
			results = append(results, string(runes[i:end]))
		}
	}

	return results
}

// TermPosition represents position info for a term in a document
type TermPosition struct {
	RowID     uint64
	Position  int
	Level     int
	SourceTerm string
}

// InvertedIndex implements a simple inverted index with multi-level support
type InvertedIndex struct {
	mu        sync.RWMutex
	tokenizer Tokenizer
	config    *FTSConfig // 多级分词配置

	// term -> postings list (row IDs)
	index map[string][]uint64

	// term -> level weights (for scoring)
	termWeights map[string]float64

	// term -> positions (for highlighting)
	termPositions map[string][]TermPosition

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
		tokenizer = NewChineseTokenizer()
	}

	return &InvertedIndex{
		tokenizer:     tokenizer,
		config:        DefaultFTSConfig(),
		index:         make(map[string][]uint64),
		termWeights:   make(map[string]float64),
		termPositions: make(map[string][]TermPosition),
		documents:     make(map[uint64]map[string]bool),
		contents:      make(map[uint64]string),
	}
}

// NewInvertedIndexWithConfig creates a new inverted index with custom config
func NewInvertedIndexWithConfig(tokenizer Tokenizer, config *FTSConfig) *InvertedIndex {
	if tokenizer == nil {
		tokenizer = NewChineseTokenizer()
	}
	if config == nil {
		config = DefaultFTSConfig()
	}

	return &InvertedIndex{
		tokenizer:     tokenizer,
		config:        config,
		index:         make(map[string][]uint64),
		termWeights:   make(map[string]float64),
		termPositions: make(map[string][]TermPosition),
		documents:     make(map[uint64]map[string]bool),
		contents:      make(map[uint64]string),
	}
}

// SetConfig sets the FTS configuration
func (idx *InvertedIndex) SetConfig(config *FTSConfig) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.config = config
}

// GetConfig returns the current FTS configuration
func (idx *InvertedIndex) GetConfig() *FTSConfig {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.config
}

// IndexDocument indexes a document with multi-level support
func (idx *InvertedIndex) IndexDocument(rowID uint64, content string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Check if document already exists
	if _, exists := idx.documents[rowID]; exists {
		// Remove old index entries
		idx.removeDocumentLocked(rowID)
	}

	// Use multi-level tokenization if configured
	var termSet map[string]bool
	var termWeightsLocal map[string]float64
	var tokensWithLevels []TokenWithLevel

	if mlTokenizer, ok := idx.tokenizer.(MultiLevelTokenizer); ok && len(idx.config.Levels) > 1 {
		// Multi-level tokenization for indexing (with synonym expansion)
		tokensWithLevels = mlTokenizer.TokenizeWithLevelsForIndex(content, idx.config, true)
		if len(tokensWithLevels) == 0 {
			return nil
		}

		termSet = make(map[string]bool)
		termWeightsLocal = make(map[string]float64)

		for _, tw := range tokensWithLevels {
			termSet[tw.Term] = true
			// Store the maximum weight for each term
			if existing, exists := termWeightsLocal[tw.Term]; !exists || tw.Weight > existing {
				termWeightsLocal[tw.Term] = tw.Weight
			}
			// Also store in global term weights (use max)
			if existing, exists := idx.termWeights[tw.Term]; !exists || tw.Weight > existing {
				idx.termWeights[tw.Term] = tw.Weight
			}

			// Store position info if enabled
			if idx.config.StorePos {
				pos := TermPosition{
					RowID:      rowID,
					Position:   tw.Position,
					Level:      tw.Level,
					SourceTerm: tw.SourceTerm,
				}
				idx.termPositions[tw.Term] = append(idx.termPositions[tw.Term], pos)
			}
		}
	} else {
		// Standard single-level tokenization
		tokens := idx.tokenizer.Tokenize(content)
		if len(tokens) == 0 {
			return nil
		}

		termSet = make(map[string]bool)
		for pos, token := range tokens {
			termSet[token] = true
			// Default weight for level 1
			idx.termWeights[token] = idx.config.WeightL1
			if idx.termWeights[token] == 0 {
				idx.termWeights[token] = 1.0
			}

			// Store position info if enabled
			if idx.config.StorePos {
				posInfo := TermPosition{
					RowID:     rowID,
					Position:  pos,
					Level:     1,
					SourceTerm: token,
				}
				idx.termPositions[token] = append(idx.termPositions[token], posInfo)
			}
		}
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

	// Remove from inverted index and positions
	for term := range termSet {
		postings := idx.index[term]
		idx.index[term] = removeRowID(postings, rowID)
		if len(idx.index[term]) == 0 {
			delete(idx.index, term)
		}

		// Remove position info
		if positions, exists := idx.termPositions[term]; exists {
			newPositions := make([]TermPosition, 0)
			for _, pos := range positions {
				if pos.RowID != rowID {
					newPositions = append(newPositions, pos)
				}
			}
			if len(newPositions) == 0 {
				delete(idx.termPositions, term)
			} else {
				idx.termPositions[term] = newPositions
			}
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

// Search performs a full-text search with multi-level support
func (idx *InvertedIndex) Search(query string, limit, offset int) ([]SearchResult, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	// Tokenize query - also use multi-level for query
	var queryTerms []string
	var queryWeights map[string]float64

	if mlTokenizer, ok := idx.tokenizer.(MultiLevelTokenizer); ok && len(idx.config.Levels) > 1 {
		tokensWithLevels := mlTokenizer.TokenizeWithLevelsForIndex(query, idx.config, false)
		if len(tokensWithLevels) == 0 {
			return nil, nil
		}
		queryTerms = make([]string, 0, len(tokensWithLevels))
		queryWeights = make(map[string]float64)
		for _, tw := range tokensWithLevels {
			queryTerms = append(queryTerms, tw.Term)
			if existing, exists := queryWeights[tw.Term]; !exists || tw.Weight > existing {
				queryWeights[tw.Term] = tw.Weight
			}
		}
	} else {
		queryTerms = idx.tokenizer.Tokenize(query)
		if len(queryTerms) == 0 {
			return nil, nil
		}
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

	// Calculate scores with level weights
	results := make([]SearchResult, 0, len(candidateRows))
	for rowID := range candidateRows {
		score := idx.calculateScoreWithLevels(rowID, queryTerms, queryWeights)
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

// SearchWithLevel performs a full-text search with specific level filter
func (idx *InvertedIndex) SearchWithLevel(query string, level int, limit, offset int) ([]SearchResult, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	// If level is 0, use regular search (all levels)
	if level == 0 {
		// Copy the logic from Search but without re-acquiring lock
		var queryTerms []string
		var queryWeights map[string]float64

		if mlTokenizer, ok := idx.tokenizer.(MultiLevelTokenizer); ok && len(idx.config.Levels) > 1 {
			// Tokenize for searching (without synonym expansion)
			tokensWithLevels := mlTokenizer.TokenizeWithLevelsForIndex(query, idx.config, false)
			if len(tokensWithLevels) == 0 {
				return nil, nil
			}
			queryTerms = make([]string, 0, len(tokensWithLevels))
			queryWeights = make(map[string]float64)
			for _, tw := range tokensWithLevels {
				queryTerms = append(queryTerms, tw.Term)
				if existing, exists := queryWeights[tw.Term]; !exists || tw.Weight > existing {
					queryWeights[tw.Term] = tw.Weight
				}
			}
		} else {
			queryTerms = idx.tokenizer.Tokenize(query)
			if len(queryTerms) == 0 {
				return nil, nil
			}
		}

		// Find documents containing all query terms
		var candidateRows map[uint64]bool
		for _, term := range queryTerms {
			postings, exists := idx.index[term]
			if !exists {
				return nil, nil
			}
			termRows := make(map[uint64]bool)
			for _, rowID := range postings {
				termRows[rowID] = true
			}
			if candidateRows == nil {
				candidateRows = termRows
			} else {
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

		results := make([]SearchResult, 0, len(candidateRows))
		for rowID := range candidateRows {
			score := idx.calculateScoreWithLevels(rowID, queryTerms, queryWeights)
			results = append(results, SearchResult{RowID: rowID, Score: score})
		}

		sort.Slice(results, func(i, j int) bool {
			return results[i].Score > results[j].Score
		})

		if offset >= len(results) {
			return nil, nil
		}
		end := offset + limit
		if end > len(results) {
			end = len(results)
		}
		return results[offset:end], nil
	}

	// Specific level search - filter by term positions
	var queryTerms []string
	if mlTokenizer, ok := idx.tokenizer.(MultiLevelTokenizer); ok {
		tokensWithLevels := mlTokenizer.TokenizeWithLevelsForIndex(query, idx.config, false)
		for _, tw := range tokensWithLevels {
			if tw.Level == level {
				queryTerms = append(queryTerms, tw.Term)
			}
		}
	} else {
		if level == 1 {
			queryTerms = idx.tokenizer.Tokenize(query)
		} else {
			return nil, nil // No terms at this level
		}
	}

	if len(queryTerms) == 0 {
		return nil, nil
	}

	// Find documents containing all query terms at the specified level
	var candidateRows map[uint64]bool
	for _, term := range queryTerms {
		// Check if term has positions at the specified level
		positions, exists := idx.termPositions[term]
		if !exists {
			// Fallback to regular index
			postings, exists := idx.index[term]
			if !exists {
				return nil, nil
			}
			termRows := make(map[uint64]bool)
			for _, rowID := range postings {
				termRows[rowID] = true
			}
			if candidateRows == nil {
				candidateRows = termRows
			} else {
				for rowID := range candidateRows {
					if !termRows[rowID] {
						delete(candidateRows, rowID)
					}
				}
			}
		} else {
			// Filter by level
			termRows := make(map[uint64]bool)
			for _, pos := range positions {
				if pos.Level == level {
					termRows[pos.RowID] = true
				}
			}
			if len(termRows) == 0 {
				return nil, nil
			}
			if candidateRows == nil {
				candidateRows = termRows
			} else {
				for rowID := range candidateRows {
					if !termRows[rowID] {
						delete(candidateRows, rowID)
					}
				}
			}
		}

		if len(candidateRows) == 0 {
			return nil, nil
		}
	}

	// Calculate scores
	results := make([]SearchResult, 0, len(candidateRows))
	queryWeights := make(map[string]float64)
	for _, term := range queryTerms {
		queryWeights[term] = idx.termWeights[term]
	}

	for rowID := range candidateRows {
		score := idx.calculateScoreWithLevels(rowID, queryTerms, queryWeights)
		results = append(results, SearchResult{RowID: rowID, Score: score})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if offset >= len(results) {
		return nil, nil
	}
	end := offset + limit
	if end > len(results) {
		end = len(results)
	}
	return results[offset:end], nil
}

// calculateScoreWithLevels calculates score with multi-level weights
func (idx *InvertedIndex) calculateScoreWithLevels(rowID uint64, queryTerms []string, queryWeights map[string]float64) float64 {
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
		if !termSet[term] {
			continue
		}

		// Get term weight (from index)
		termWeight := idx.termWeights[term]
		if termWeight == 0 {
			termWeight = 1.0
		}

		// Get query term weight
		queryWeight := 1.0
		if queryWeights != nil {
			if qw, exists := queryWeights[term]; exists {
				queryWeight = qw
			}
		}

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

		// Combine score with weights
		// Higher weight for exact matches (Level 1) vs partial matches (Level 2)
		score += tf * idf * termWeight * queryWeight
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

// GetTermPositions returns positions for a term in a specific document
func (idx *InvertedIndex) GetTermPositions(term string, rowID uint64) []TermPosition {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	positions, exists := idx.termPositions[term]
	if !exists {
		return nil
	}

	result := make([]TermPosition, 0)
	for _, pos := range positions {
		if pos.RowID == rowID {
			result = append(result, pos)
		}
	}
	return result
}

// Highlight highlights matching terms in content
func (idx *InvertedIndex) Highlight(rowID uint64, query string, prefix, suffix string) string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	content, exists := idx.contents[rowID]
	if !exists {
		return ""
	}

	// Get query terms
	var queryTerms []string
	if mlTokenizer, ok := idx.tokenizer.(MultiLevelTokenizer); ok && len(idx.config.Levels) > 1 {
		tokensWithLevels := mlTokenizer.TokenizeWithLevelsForIndex(query, idx.config, false)
		queryTerms = make([]string, 0, len(tokensWithLevels))
		for _, tw := range tokensWithLevels {
			queryTerms = append(queryTerms, tw.Term)
		}
	} else {
		queryTerms = idx.tokenizer.Tokenize(query)
	}

	if len(queryTerms) == 0 {
		return content
	}

	// Find all positions to highlight
	type highlightRange struct {
		start int
		end   int
	}
	ranges := make([]highlightRange, 0)

	for _, term := range queryTerms {
		positions, exists := idx.termPositions[term]
		if !exists {
			continue
		}

		for _, pos := range positions {
			if pos.RowID != rowID {
				continue
			}

			// Find the term in content
			lowerContent := strings.ToLower(content)
			lowerTerm := strings.ToLower(term)

			// Search for term occurrences
			searchStart := 0
			for {
				idx := strings.Index(lowerContent[searchStart:], lowerTerm)
				if idx == -1 {
					break
				}
				actualIdx := searchStart + idx
				ranges = append(ranges, highlightRange{start: actualIdx, end: actualIdx + len(term)})
				searchStart = actualIdx + len(term)
			}
		}
	}

	if len(ranges) == 0 {
		return content
	}

	// Sort and merge overlapping ranges
	sort.Slice(ranges, func(i, j int) bool {
		return ranges[i].start < ranges[j].start
	})

	merged := make([]highlightRange, 0)
	for _, r := range ranges {
		if len(merged) == 0 {
			merged = append(merged, r)
		} else {
			last := &merged[len(merged)-1]
			if r.start <= last.end {
				if r.end > last.end {
					last.end = r.end
				}
			} else {
				merged = append(merged, r)
			}
		}
	}

	// Build highlighted string
	var result strings.Builder
	lastEnd := 0
	for _, r := range merged {
		result.WriteString(content[lastEnd:r.start])
		result.WriteString(prefix)
		result.WriteString(content[r.start:r.end])
		result.WriteString(suffix)
		lastEnd = r.end
	}
	result.WriteString(content[lastEnd:])

	return result.String()
}

// SaveToFile saves the index to a file
func (idx *InvertedIndex) SaveToFile(path string) error {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	data := struct {
		Index        map[string][]uint64     `json:"index"`
		Documents    map[uint64][]string     `json:"documents"` // Convert map[string]bool to []string for JSON
		Contents     map[uint64]string       `json:"contents"`
		DocCount     int64                   `json:"doc_count"`
		TermCount    int64                   `json:"term_count"`
		TermWeights  map[string]float64      `json:"term_weights,omitempty"`
		TermPositions map[string][]TermPosition `json:"term_positions,omitempty"`
		Config       *FTSConfig              `json:"config,omitempty"`
	}{
		Index:        idx.index,
		Documents:    make(map[uint64][]string),
		Contents:     idx.contents,
		DocCount:     idx.docCount,
		TermCount:    idx.termCount,
		TermWeights:  idx.termWeights,
		TermPositions: idx.termPositions,
		Config:       idx.config,
	}

	// Convert documents map
	for rowID, termSet := range idx.documents {
		terms := make([]string, 0, len(termSet))
		for term := range termSet {
			terms = append(terms, term)
		}
		data.Documents[rowID] = terms
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// Compress the data
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(jsonData); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}

	return os.WriteFile(path, buf.Bytes(), 0644)
}

// LoadFromFile loads the index from a file
func (idx *InvertedIndex) LoadFromFile(path string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	compressedData, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Decompress
	gz, err := gzip.NewReader(bytes.NewReader(compressedData))
	if err != nil {
		return err
	}
	jsonData, err := io.ReadAll(gz)
	if err != nil {
		return err
	}
	gz.Close()

	var data struct {
		Index        map[string][]uint64    `json:"index"`
		Documents    map[uint64][]string    `json:"documents"`
		Contents     map[uint64]string      `json:"contents"`
		DocCount     int64                  `json:"doc_count"`
		TermCount    int64                  `json:"term_count"`
		TermWeights  map[string]float64     `json:"term_weights,omitempty"`
		TermPositions map[string][]TermPosition `json:"term_positions,omitempty"`
		Config       *FTSConfig             `json:"config,omitempty"`
	}

	if err := json.Unmarshal(jsonData, &data); err != nil {
		return err
	}

	idx.index = data.Index
	if idx.index == nil {
		idx.index = make(map[string][]uint64)
	}
	idx.contents = data.Contents
	if idx.contents == nil {
		idx.contents = make(map[uint64]string)
	}
	idx.docCount = data.DocCount
	idx.termCount = data.TermCount

	// Load term weights
	if data.TermWeights != nil {
		idx.termWeights = data.TermWeights
	} else {
		idx.termWeights = make(map[string]float64)
	}

	// Load term positions
	if data.TermPositions != nil {
		idx.termPositions = data.TermPositions
	} else {
		idx.termPositions = make(map[string][]TermPosition)
	}

	// Load config if present
	if data.Config != nil {
		idx.config = data.Config
	} else if idx.config == nil {
		idx.config = DefaultFTSConfig()
	}

	// Convert documents back
	idx.documents = make(map[uint64]map[string]bool)
	for rowID, terms := range data.Documents {
		termSet := make(map[string]bool)
		for _, term := range terms {
			termSet[term] = true
		}
		idx.documents[rowID] = termSet
	}

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
	mu       sync.RWMutex
	indexes  map[string]FullTextIndexer // "table.column" -> indexer
	configs  map[string]*FTSConfig      // "table.column" -> config
}

// NewManager creates a new FTS manager
func NewManager() *Manager {
	return &Manager{
		indexes: make(map[string]FullTextIndexer),
		configs: make(map[string]*FTSConfig),
	}
}

// CreateIndex creates a full-text index for a table/column
func (m *Manager) CreateIndex(tableName, columnName string, indexer FullTextIndexer) error {
	return m.CreateIndexWithConfig(tableName, columnName, indexer, nil)
}

// CreateIndexWithConfig creates a full-text index with custom config
func (m *Manager) CreateIndexWithConfig(tableName, columnName string, indexer FullTextIndexer, config *FTSConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := tableName + "." + columnName
	if _, exists := m.indexes[key]; exists {
		return types.NewError(types.ErrDuplicateKey, "full-text index already exists: %s", key)
	}

	if indexer == nil {
		if config != nil {
			indexer = NewInvertedIndexWithConfig(nil, config)
		} else {
			indexer = NewInvertedIndex(nil)
		}
	} else if invIdx, ok := indexer.(*InvertedIndex); ok && config != nil {
		// Apply config to existing indexer
		invIdx.SetConfig(config)
	}

	m.indexes[key] = indexer
	m.configs[key] = config
	return nil
}

// GetIndexConfig gets the FTS config for a specific index
func (m *Manager) GetIndexConfig(tableName, columnName string) *FTSConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.configs[tableName+"."+columnName]
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

	key := tableName + "." + columnName
	// Check exact match first
	if indexer, exists := m.indexes[key]; exists {
		return indexer
	}

	// Try case-insensitive match
	lowerKey := strings.ToLower(key)
	for k, indexer := range m.indexes {
		if strings.ToLower(k) == lowerKey {
			return indexer
		}
	}
	return nil
}

// HasIndex checks if an index exists
func (m *Manager) HasIndex(tableName, columnName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check exact match first
	key := tableName + "." + columnName
	if _, exists := m.indexes[key]; exists {
		return true
	}

	// Try case-insensitive match
	lowerKey := strings.ToLower(key)
	for k := range m.indexes {
		if strings.ToLower(k) == lowerKey {
			return true
		}
	}
	return false
}

// IndexDocument indexes a document in the specified index
func (m *Manager) IndexDocument(tableName, columnName string, rowID uint64, content string) error {
	m.mu.RLock()
	key := tableName + "." + columnName
	indexer, exists := m.indexes[key]
	if !exists {
		// Try case-insensitive match
		lowerKey := strings.ToLower(key)
		for k, idx := range m.indexes {
			if strings.ToLower(k) == lowerKey {
				indexer = idx
				exists = true
				break
			}
		}
	}
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
	defer m.mu.RUnlock()

	key := tableName + "." + columnName
	indexer, exists := m.indexes[key]
	if !exists {
		// Try case-insensitive match
		lowerKey := strings.ToLower(key)
		for k, idx := range m.indexes {
			if strings.ToLower(k) == lowerKey {
				indexer = idx
				exists = true
				break
			}
		}
	}

	if !exists {
		return nil, types.NewError(types.ErrNotFound, "full-text index not found: %s.%s", tableName, columnName)
	}

	return indexer.Search(query, limit, offset)
}

// SearchWithLevel performs a full-text search with level filter
func (m *Manager) SearchWithLevel(tableName, columnName, query string, level int, limit, offset int) ([]SearchResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := tableName + "." + columnName
	indexer, exists := m.indexes[key]
	if !exists {
		// Try case-insensitive match
		lowerKey := strings.ToLower(key)
		for k, idx := range m.indexes {
			if strings.ToLower(k) == lowerKey {
				indexer = idx
				exists = true
				break
			}
		}
	}

	if !exists {
		return nil, types.NewError(types.ErrNotFound, "full-text index not found: %s.%s", tableName, columnName)
	}

	return indexer.SearchWithLevel(query, level, limit, offset)
}

// Highlight highlights matching terms in content
func (m *Manager) Highlight(tableName, columnName string, rowID uint64, query string, prefix, suffix string) string {
	m.mu.RLock()
	key := tableName + "." + columnName
	indexer, exists := m.indexes[key]
	if !exists {
		// Try case-insensitive match
		lowerKey := strings.ToLower(key)
		for k, idx := range m.indexes {
			if strings.ToLower(k) == lowerKey {
				indexer = idx
				exists = true
				break
			}
		}
	}
	m.mu.RUnlock()

	if !exists {
		return ""
	}

	if invIdx, ok := indexer.(*InvertedIndex); ok {
		return invIdx.Highlight(rowID, query, prefix, suffix)
	}

	return ""
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
