// Package storage provides page cache for XxLdb
package storage

import (
	"container/list"
	"fmt"
	"sync"
)

// PageCacheConfig holds configuration for page cache
type PageCacheConfig struct {
	MaxPages    int   // Maximum number of pages to cache
	WriteBehind bool  // Enable write-behind (dirty pages written asynchronously)
	WriteBatch  int   // Number of dirty pages to write in a batch
}

// DefaultPageCacheConfig returns default cache configuration
func DefaultPageCacheConfig() PageCacheConfig {
	return PageCacheConfig{
		MaxPages:    1000,  // ~16MB with 16KB pages
		WriteBehind: true,
		WriteBatch:  100,
	}
}

// PageCache implements an LRU page cache
type PageCache struct {
	mu       sync.RWMutex
	config   PageCacheConfig

	// Page storage
	pages    map[uint64]*PageV2  // PageID -> Page

	// LRU tracking
	lruList  *list.List          // Doubly linked list for LRU
	lruMap   map[uint64]*list.Element  // PageID -> LRU element

	// Dirty page tracking
	dirty    map[uint64]bool     // PageID -> dirty flag

	// Statistics
	hits     int64
	misses   int64
}

// cacheEntry represents an entry in the LRU list
type cacheEntry struct {
	pageID   uint64
	page     *PageV2
}

// NewPageCache creates a new page cache
func NewPageCache(config PageCacheConfig) *PageCache {
	return &PageCache{
		config:  config,
		pages:   make(map[uint64]*PageV2),
		lruList: list.New(),
		lruMap:  make(map[uint64]*list.Element),
		dirty:   make(map[uint64]bool),
	}
}

// Get retrieves a page from the cache
// Returns nil if page is not in cache
func (c *PageCache) Get(pageID uint64) *PageV2 {
	c.mu.Lock()
	defer c.mu.Unlock()

	page, exists := c.pages[pageID]
	if !exists {
		c.misses++
		return nil
	}

	c.hits++
	c.touchPage(pageID)
	return page
}

// Put adds a page to the cache
// If cache is full, evicts the least recently used page
func (c *PageCache) Put(page *PageV2) error {
	if page == nil {
		return fmt.Errorf("cannot cache nil page")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	pageID := page.Header.PageID

	// Check if page already in cache
	if _, exists := c.pages[pageID]; exists {
		// Update existing page
		c.pages[pageID] = page
		c.touchPage(pageID)
		return nil
	}

	// Check if we need to evict
	for len(c.pages) >= c.config.MaxPages {
		if err := c.evictLRU(); err != nil {
			return fmt.Errorf("failed to evict page: %w", err)
		}
	}

	// Add new page
	c.pages[pageID] = page
	entry := &cacheEntry{pageID: pageID, page: page}
	element := c.lruList.PushFront(entry)
	c.lruMap[pageID] = element

	return nil
}

// MarkDirty marks a page as dirty (modified)
func (c *PageCache) MarkDirty(pageID uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dirty[pageID] = true
}

// IsDirty checks if a page is dirty
func (c *PageCache) IsDirty(pageID uint64) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.dirty[pageID]
}

// ClearDirty clears the dirty flag for a page
func (c *PageCache) ClearDirty(pageID uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.dirty, pageID)
}

// GetDirtyPages returns a list of dirty page IDs
func (c *PageCache) GetDirtyPages() []uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	pageIDs := make([]uint64, 0, len(c.dirty))
	for pageID := range c.dirty {
		pageIDs = append(pageIDs, pageID)
	}
	return pageIDs
}

// Remove removes a page from the cache
func (c *PageCache) Remove(pageID uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if element, exists := c.lruMap[pageID]; exists {
		c.lruList.Remove(element)
		delete(c.lruMap, pageID)
	}

	delete(c.pages, pageID)
	delete(c.dirty, pageID)
}

// Clear clears all pages from the cache
func (c *PageCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.pages = make(map[uint64]*PageV2)
	c.lruList = list.New()
	c.lruMap = make(map[uint64]*list.Element)
	c.dirty = make(map[uint64]bool)
}

// Size returns the number of pages in the cache
func (c *PageCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.pages)
}

// Stats returns cache statistics
func (c *PageCache) Stats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	hitRate := float64(0)
	total := c.hits + c.misses
	if total > 0 {
		hitRate = float64(c.hits) / float64(total)
	}

	return map[string]interface{}{
		"pages":      len(c.pages),
		"max_pages":  c.config.MaxPages,
		"dirty":      len(c.dirty),
		"hits":       c.hits,
		"misses":     c.misses,
		"hit_rate":   hitRate,
		"usage_mb":   len(c.pages) * PageSize16KB / (1024 * 1024),
	}
}

// touchPage moves a page to the front of the LRU list
func (c *PageCache) touchPage(pageID uint64) {
	if element, exists := c.lruMap[pageID]; exists {
		c.lruList.MoveToFront(element)
	}
}

// evictLRU evicts the least recently used page
func (c *PageCache) evictLRU() error {
	// Find LRU page (back of list)
	element := c.lruList.Back()
	if element == nil {
		return fmt.Errorf("no pages to evict")
	}

	entry := element.Value.(*cacheEntry)
	pageID := entry.pageID

	// Check if page is dirty - we should not evict dirty pages
	// In a real implementation, we would flush dirty pages first
	if c.dirty[pageID] {
		// Move to front to give it another chance
		c.lruList.MoveToFront(element)
		return fmt.Errorf("cannot evict dirty page %d", pageID)
	}

	// Remove from cache
	delete(c.pages, pageID)
	delete(c.lruMap, pageID)
	c.lruList.Remove(element)

	return nil
}

// Flush flushes all dirty pages using the provided write function
func (c *PageCache) Flush(writeFunc func(*PageV2) error) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for pageID, isDirty := range c.dirty {
		if !isDirty {
			continue
		}

		page, exists := c.pages[pageID]
		if !exists {
			delete(c.dirty, pageID)
			continue
		}

		if err := writeFunc(page); err != nil {
			return fmt.Errorf("failed to flush page %d: %w", pageID, err)
		}

		delete(c.dirty, pageID)
	}

	return nil
}

// FlushPage flushes a specific page
func (c *PageCache) FlushPage(pageID uint64, writeFunc func(*PageV2) error) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.dirty[pageID] {
		return nil
	}

	page, exists := c.pages[pageID]
	if !exists {
		delete(c.dirty, pageID)
		return nil
	}

	if err := writeFunc(page); err != nil {
		return fmt.Errorf("failed to flush page %d: %w", pageID, err)
	}

	delete(c.dirty, pageID)
	return nil
}

// Resize changes the maximum cache size
func (c *PageCache) Resize(maxPages int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.config.MaxPages = maxPages

	// Evict pages if necessary
	for len(c.pages) > c.config.MaxPages {
		if err := c.evictLRU(); err != nil {
			return err
		}
	}

	return nil
}