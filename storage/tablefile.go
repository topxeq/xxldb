// Package storage provides table file management for XxLdb
package storage

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/topxeq/xxldb/types"
)

// Table file constants
const (
	TableFileMagic    = "XXLDTBL\x00"
	TableFileVersion  = 1
	TableFileExt      = ".db"
)

// TableFileHeader is the header of a table data file
// Stored in page 0 (first 16KB)
type TableFileHeader struct {
	Magic         [8]byte  // "XXLDTBL\x00"
	Version       uint16   // File format version
	TableID       uint64   // Table ID (matches metadata)
	PageSize      uint16   // Page size (16384)
	PageCount     uint32   // Total pages in file
	RootPage      uint64   // B-tree root page (0 if no index)
	FirstDataPage uint64   // First data page ID
	LastDataPage  uint64   // Last data page ID
	FreePageList  uint64   // Free page list head
	RowCount      uint64   // Total rows in table
	Reserved      [120]byte
	ColumnDefs    []types.ColumnDef // Column definitions (JSON serialized)
	Checksum      uint32            // Header checksum
}

// TableFile manages a table's data file
type TableFile struct {
	mu        sync.RWMutex
	tableName string
	filePath  string
	file      *os.File
	header    TableFileHeader
	storage   *Storage         // Parent storage reference
	cache     *PageCache       // Page cache (shared or dedicated)
	dirty     bool             // Header needs to be written
}

// OpenTableFile opens an existing table file
func OpenTableFile(storage *Storage, tableName string) (*TableFile, error) {
	filePath := filepath.Join(storage.path, DataDirName, tableName+TableFileExt)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("table file does not exist: %s", filePath)
	}

	file, err := os.OpenFile(filePath, os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open table file: %w", err)
	}

	tf := &TableFile{
		tableName: tableName,
		filePath:  filePath,
		file:      file,
		storage:   storage,
		cache:     storage.pageCache,
	}

	// Read header (page 0)
	if err := tf.readHeader(); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to read table header: %w", err)
	}

	return tf, nil
}

// CreateTableFile creates a new table file
func CreateTableFile(storage *Storage, tableName string, tableID uint64, columns []types.ColumnDef) (*TableFile, error) {
	// Ensure data directory exists
	dataDir := filepath.Join(storage.path, DataDirName)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	filePath := filepath.Join(dataDir, tableName+TableFileExt)

	// Check if file already exists
	if _, err := os.Stat(filePath); err == nil {
		return nil, fmt.Errorf("table file already exists: %s", filePath)
	}

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to create table file: %w", err)
	}

	tf := &TableFile{
		tableName: tableName,
		filePath:  filePath,
		file:      file,
		storage:   storage,
		cache:     storage.pageCache,
		dirty:     true,
	}

	// Initialize header
	copy(tf.header.Magic[:], TableFileMagic)
	tf.header.Version = TableFileVersion
	tf.header.TableID = tableID
	tf.header.PageSize = PageSize16KB
	tf.header.PageCount = 1 // Header page
	tf.header.ColumnDefs = columns
	tf.header.RowCount = 0

	// Write header
	if err := tf.writeHeader(); err != nil {
		file.Close()
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to write table header: %w", err)
	}

	return tf, nil
}

// Close closes the table file
func (tf *TableFile) Close() error {
	tf.mu.Lock()
	defer tf.mu.Unlock()

	// Flush dirty pages (use NoLock version since we hold the lock)
	if tf.cache != nil {
		tf.cache.Flush(tf.writePageToFileNoLock)
	}

	// Write header if dirty
	if tf.dirty {
		if err := tf.writeHeader(); err != nil {
			return err
		}
	}

	if tf.file != nil {
		return tf.file.Close()
	}
	return nil
}

// readHeader reads the file header from page 0
func (tf *TableFile) readHeader() error {
	headerData := make([]byte, PageSize16KB)
	if _, err := tf.file.ReadAt(headerData, 0); err != nil {
		return fmt.Errorf("failed to read header: %w", err)
	}

	// Parse header
	copy(tf.header.Magic[:], headerData[0:8])
	if string(tf.header.Magic[:]) != TableFileMagic {
		return fmt.Errorf("invalid table file magic: %s", string(tf.header.Magic[:]))
	}

	tf.header.Version = binary.LittleEndian.Uint16(headerData[8:10])
	tf.header.TableID = binary.LittleEndian.Uint64(headerData[10:18])
	tf.header.PageSize = binary.LittleEndian.Uint16(headerData[18:20])
	tf.header.PageCount = binary.LittleEndian.Uint32(headerData[20:24])
	tf.header.RootPage = binary.LittleEndian.Uint64(headerData[24:32])
	tf.header.FirstDataPage = binary.LittleEndian.Uint64(headerData[32:40])
	tf.header.LastDataPage = binary.LittleEndian.Uint64(headerData[40:48])
	tf.header.FreePageList = binary.LittleEndian.Uint64(headerData[48:56])
	tf.header.RowCount = binary.LittleEndian.Uint64(headerData[56:64])

	// Read column definitions (stored as JSON after header fields)
	colDefsLen := binary.LittleEndian.Uint16(headerData[64:66])
	if colDefsLen > 0 {
		colDefsData := headerData[66 : 66+colDefsLen]
		if err := json.Unmarshal(colDefsData, &tf.header.ColumnDefs); err != nil {
			return fmt.Errorf("failed to parse column definitions: %w", err)
		}
	}

	return nil
}

// writeHeader writes the file header to page 0
func (tf *TableFile) writeHeader() error {
	headerData := make([]byte, PageSize16KB)

	// Write header fields
	copy(headerData[0:8], tf.header.Magic[:])
	binary.LittleEndian.PutUint16(headerData[8:10], tf.header.Version)
	binary.LittleEndian.PutUint64(headerData[10:18], tf.header.TableID)
	binary.LittleEndian.PutUint16(headerData[18:20], tf.header.PageSize)
	binary.LittleEndian.PutUint32(headerData[20:24], tf.header.PageCount)
	binary.LittleEndian.PutUint64(headerData[24:32], tf.header.RootPage)
	binary.LittleEndian.PutUint64(headerData[32:40], tf.header.FirstDataPage)
	binary.LittleEndian.PutUint64(headerData[40:48], tf.header.LastDataPage)
	binary.LittleEndian.PutUint64(headerData[48:56], tf.header.FreePageList)
	binary.LittleEndian.PutUint64(headerData[56:64], tf.header.RowCount)

	// Serialize column definitions
	if tf.header.ColumnDefs != nil {
		colDefsData, err := json.Marshal(tf.header.ColumnDefs)
		if err != nil {
			return fmt.Errorf("failed to serialize column definitions: %w", err)
		}
		binary.LittleEndian.PutUint16(headerData[64:66], uint16(len(colDefsData)))
		copy(headerData[66:], colDefsData)
	}

	// Write to file
	if _, err := tf.file.WriteAt(headerData, 0); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	tf.dirty = false
	return nil
}

// allocatePage allocates a new page
func (tf *TableFile) allocatePage(pageType PageType) (*PageV2, uint64, error) {
	tf.mu.Lock()
	defer tf.mu.Unlock()
	return tf.allocatePageNoLock(pageType)
}

// allocatePageNoLock allocates a new page without acquiring the lock
func (tf *TableFile) allocatePageNoLock(pageType PageType) (*PageV2, uint64, error) {
	// Check free list first
	if tf.header.FreePageList > 0 {
		// Reuse free page
		freePageID := tf.header.FreePageList
		page, err := tf.readPageNoLock(freePageID)
		if err != nil {
			return nil, 0, err
		}
		// Update free list (stored in RightPage of free page)
		tf.header.FreePageList = page.Header.RightPage
		tf.dirty = true

		// Initialize as new page
		page.Header = PageHeaderV2{
			PageID:    freePageID,
			PageType:  pageType,
			Version:   1,
			FreeSpace: MaxPageContentV2,
		}
		return page, freePageID, nil
	}

	// Allocate new page at end of file
	pageID := uint64(tf.header.PageCount)
	page := NewPageV2(pageID, pageType)

	// Update header
	tf.header.PageCount++
	tf.dirty = true

	// Extend file
	if err := tf.writePageToFileWithCache(page); err != nil {
		return nil, 0, err
	}

	return page, pageID, nil
}

// readPage reads a page from the file
func (tf *TableFile) readPage(pageID uint64) (*PageV2, error) {
	// Check cache first
	if tf.cache != nil {
		if page := tf.cache.Get(pageID); page != nil {
			return page, nil
		}
	}

	tf.mu.RLock()
	defer tf.mu.RUnlock()
	return tf.readPageNoLock(pageID)
}

// readPageNoLock reads a page without acquiring the lock
func (tf *TableFile) readPageNoLock(pageID uint64) (*PageV2, error) {
	// Check cache first
	if tf.cache != nil {
		if page := tf.cache.Get(pageID); page != nil {
			return page, nil
		}
	}

	// Read from file
	offset := int64(pageID) * PageSize16KB
	pageData := make([]byte, PageSize16KB)
	if _, err := tf.file.ReadAt(pageData, offset); err != nil {
		return nil, fmt.Errorf("failed to read page %d: %w", pageID, err)
	}

	// Deserialize page
	page := &PageV2{}
	if err := page.FromBytes(pageData); err != nil {
		return nil, fmt.Errorf("failed to parse page %d: %w", pageID, err)
	}

	// Add to cache
	if tf.cache != nil {
		tf.cache.Put(page)
	}

	return page, nil
}

// writePage writes a page to the cache and file
func (tf *TableFile) writePage(page *PageV2) error {
	// Write to file first
	if err := tf.writePageToFileNoLock(page); err != nil {
		return err
	}
	// Update cache
	if tf.cache != nil {
		tf.cache.Put(page)
		// Clear dirty flag since we just wrote to file
		tf.cache.ClearDirty(page.Header.PageID)
	}
	return nil
}

// writePageToFile writes a page directly to the file
// Note: This version does NOT acquire the lock - caller must hold appropriate lock
// Note: This does NOT update the cache - caller should handle that separately if needed
func (tf *TableFile) writePageToFileNoLock(page *PageV2) error {
	page.UpdateChecksum()
	pageData := page.ToBytes()
	offset := int64(page.Header.PageID) * PageSize16KB

	if _, err := tf.file.WriteAt(pageData, offset); err != nil {
		return fmt.Errorf("failed to write page %d: %w", page.Header.PageID, err)
	}

	return nil
}

// writePageToFileWithCache writes a page to file and adds to cache
// Used for new page allocation
func (tf *TableFile) writePageToFileWithCache(page *PageV2) error {
	if err := tf.writePageToFileNoLock(page); err != nil {
		return err
	}
	// Add to cache so subsequent reads find it
	if tf.cache != nil {
		tf.cache.Put(page)
	}
	return nil
}

// writePageToFile writes a page directly to the file (with lock)
func (tf *TableFile) writePageToFile(page *PageV2) error {
	tf.mu.Lock()
	defer tf.mu.Unlock()
	return tf.writePageToFileNoLock(page)
}

// InsertRow inserts a row into the table
// Note: This method does NOT acquire locks internally. The caller (Storage) is responsible for locking.
func (tf *TableFile) InsertRow(row []types.Value) (uint64, error) {
	// Generate row ID
	rowID := tf.storage.nextID
	tf.storage.nextID++

	// Encode row
	rowData := EncodeRow(rowID, row)
	rowSize := len(rowData)

	// Check if row fits in a page
	if rowSize > int(MaxPageContentV2)-100 { // Leave room for headers
		// Row too large, need overflow page
		return 0, fmt.Errorf("row too large (%d bytes), overflow not yet implemented", rowSize)
	}

	// Find a page with enough space (no lock needed, caller holds lock)
	var targetPage *PageV2

	// Check last data page first
	if tf.header.LastDataPage > 0 {
		page, err := tf.readPageNoLock(tf.header.LastDataPage)
		if err != nil {
			return 0, fmt.Errorf("failed to read last data page: %w", err)
		}
		if page.CanInsert(rowSize) {
			targetPage = page
		}
	}

	// Need to allocate new page
	if targetPage == nil {
		page, pageID, err := tf.allocatePageNoLock(PageTypeData)
		if err != nil {
			return 0, fmt.Errorf("failed to allocate page: %w", err)
		}
		targetPage = page

		// Update page list
		if tf.header.FirstDataPage == 0 {
			tf.header.FirstDataPage = pageID
		}
		if tf.header.LastDataPage > 0 {
			// Link to previous last page
			lastPage, err := tf.readPageNoLock(tf.header.LastDataPage)
			if err == nil {
				lastPage.Header.RightPage = pageID
				tf.writePage(lastPage)
			}
		}
		tf.header.LastDataPage = pageID
		tf.dirty = true
	}

	// Insert row into page
	if _, err := InsertRowToPage(targetPage, rowID, row); err != nil {
		return 0, fmt.Errorf("failed to insert row into page: %w", err)
	}

	// Write page
	if err := tf.writePage(targetPage); err != nil {
		return 0, fmt.Errorf("failed to write page: %w", err)
	}

	// Update row count
	tf.header.RowCount++
	tf.dirty = true

	return rowID, nil
}

// GetRow retrieves a row by row ID
func (tf *TableFile) GetRow(rowID uint64) ([]types.Value, error) {
	tf.mu.RLock()
	defer tf.mu.RUnlock()

	// Scan all data pages to find the row
	for pageID := tf.header.FirstDataPage; pageID > 0; {
		page, err := tf.readPage(pageID)
		if err != nil {
			return nil, fmt.Errorf("failed to read page %d: %w", pageID, err)
		}

		// Search page for row
		for i := 0; i < int(page.Header.CellCount); i++ {
			cellData, err := GetCellData(page, i)
			if err != nil {
				continue
			}

			decodedRowID, _, err := DecodeRow(cellData, tf.header.ColumnDefs)
			if err != nil {
				continue
			}

			if decodedRowID == rowID {
				_, row, err := GetRowFromPage(page, i, tf.header.ColumnDefs)
				if err != nil {
					return nil, err
				}
				return row, nil
			}
		}

		// Move to next page
		pageID = page.Header.RightPage
	}

	return nil, fmt.Errorf("row %d not found", rowID)
}

// DeleteRow deletes a row by row ID
func (tf *TableFile) DeleteRow(rowID uint64) error {
	tf.mu.Lock()
	defer tf.mu.Unlock()

	// Scan all data pages to find the row
	for pageID := tf.header.FirstDataPage; pageID > 0; {
		page, err := tf.readPage(pageID)
		if err != nil {
			return fmt.Errorf("failed to read page %d: %w", pageID, err)
		}

		// Search page for row
		for i := 0; i < int(page.Header.CellCount); i++ {
			cellOffset := page.GetCellPointer(i)
			if cellOffset == 0 {
				continue // Deleted cell
			}

			cellData, err := GetCellData(page, i)
			if err != nil {
				continue
			}

			decodedRowID, _, err := DecodeRow(cellData, tf.header.ColumnDefs)
			if err != nil {
				continue
			}

			if decodedRowID == rowID {
				// Found the row, delete it
				if err := DeleteRowFromPage(page, i); err != nil {
					return fmt.Errorf("failed to delete row from page: %w", err)
				}

				// Write page
				if err := tf.writePage(page); err != nil {
					return fmt.Errorf("failed to write page: %w", err)
				}

				// Update row count
				tf.header.RowCount--
				tf.dirty = true

				return nil
			}
		}

		// Move to next page
		pageID = page.Header.RightPage
	}

	return fmt.Errorf("row %d not found", rowID)
}

// UpdateRow updates a row by row ID
func (tf *TableFile) UpdateRow(rowID uint64, row []types.Value) error {
	tf.mu.Lock()
	defer tf.mu.Unlock()

	// Scan all data pages to find the row
	for pageID := tf.header.FirstDataPage; pageID > 0; {
		page, err := tf.readPage(pageID)
		if err != nil {
			return fmt.Errorf("failed to read page %d: %w", pageID, err)
		}

		// Search page for row
		for i := 0; i < int(page.Header.CellCount); i++ {
			cellOffset := page.GetCellPointer(i)
			if cellOffset == 0 {
				continue // Deleted cell
			}

			cellData, err := GetCellData(page, i)
			if err != nil {
				continue
			}

			decodedRowID, _, err := DecodeRow(cellData, tf.header.ColumnDefs)
			if err != nil {
				continue
			}

			if decodedRowID == rowID {
				// Found the row
				newRowData := EncodeRow(rowID, row)
				newRowSize := len(newRowData)

				// Delete old row
				if err := DeleteRowFromPage(page, i); err != nil {
					return fmt.Errorf("failed to delete old row: %w", err)
				}

				// Check if new row fits in page
				if page.CanInsert(newRowSize) {
					// Insert new row
					if _, err := InsertRowToPage(page, rowID, row); err != nil {
						return fmt.Errorf("failed to insert updated row: %w", err)
					}
				} else {
					// Need to insert in different page
					// Find or allocate a new page
					var targetPage *PageV2
					if tf.header.LastDataPage > 0 && tf.header.LastDataPage != pageID {
						lastPage, err := tf.readPage(tf.header.LastDataPage)
						if err == nil && lastPage.CanInsert(newRowSize) {
							targetPage = lastPage
						}
					}

					if targetPage == nil {
						newPage, newPageID, err := tf.allocatePage(PageTypeData)
						if err != nil {
							return fmt.Errorf("failed to allocate new page: %w", err)
						}
						// Link pages
						page.Header.RightPage = newPageID
						tf.header.LastDataPage = newPageID
						tf.dirty = true
						targetPage = newPage
					}

					if _, err := InsertRowToPage(targetPage, rowID, row); err != nil {
						return fmt.Errorf("failed to insert updated row in new page: %w", err)
					}
					tf.writePage(targetPage)
				}

				// Write page
				if err := tf.writePage(page); err != nil {
					return fmt.Errorf("failed to write page: %w", err)
				}

				return nil
			}
		}

		// Move to next page
		pageID = page.Header.RightPage
	}

	return fmt.Errorf("row %d not found", rowID)
}

// ScanRows returns an iterator over all rows
func (tf *TableFile) ScanRows() *RowIterator {
	return &RowIterator{
		tf:       tf,
		pageID:   tf.header.FirstDataPage,
		cellIdx:  0,
		page:     nil,
	}
}

// RowIterator iterates over rows in a table
type RowIterator struct {
	tf      *TableFile
	pageID  uint64
	cellIdx int
	page    *PageV2
	err     error
	started bool // Whether we've returned the first row
}

// Next advances to the next row
func (it *RowIterator) Next() bool {
	if it.err != nil {
		return false
	}

	// Load first page if needed
	if it.page == nil && it.pageID > 0 {
		it.page, it.err = it.tf.readPage(it.pageID)
		if it.err != nil {
			return false
		}
		it.cellIdx = 0
	}

	// Find next valid cell
	for {
		if it.page == nil {
			return false
		}

		// If not started yet, check current cell (don't advance)
		// If already started, advance to next cell first
		if it.started {
			it.cellIdx++
		} else {
			it.started = true
		}

		// Skip deleted cells
		for it.cellIdx < int(it.page.Header.CellCount) {
			if it.page.GetCellPointer(it.cellIdx) > 0 {
				return true
			}
			it.cellIdx++
		}

		// Move to next page
		it.pageID = it.page.Header.RightPage
		if it.pageID == 0 {
			it.page = nil
			return false
		}

		it.page, it.err = it.tf.readPage(it.pageID)
		if it.err != nil {
			return false
		}
		it.cellIdx = 0
		it.started = false // Reset for new page
	}
}

// Row returns the current row
func (it *RowIterator) Row() (uint64, []types.Value, error) {
	if it.page == nil || it.err != nil {
		return 0, nil, it.err
	}

	return GetRowFromPage(it.page, it.cellIdx, it.tf.header.ColumnDefs)
}

// Err returns any error encountered
func (it *RowIterator) Err() error {
	return it.err
}

// PageIDDebug returns the current page ID for debugging
func (it *RowIterator) PageIDDebug() uint64 {
	return it.pageID
}

// GetAllRows returns all rows in the table
func (tf *TableFile) GetAllRows() ([][]types.Value, error) {
	var rows [][]types.Value
	iter := tf.ScanRows()

	for iter.Next() {
		_, row, err := iter.Row()
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}

	return rows, nil
}

// GetAllRowsWithIDs returns all rows with their IDs
func (tf *TableFile) GetAllRowsWithIDs() ([]RowWithID, error) {
	var rows []RowWithID
	iter := tf.ScanRows()

	for iter.Next() {
		id, row, err := iter.Row()
		if err != nil {
			return nil, err
		}
		rows = append(rows, RowWithID{ID: id, Row: row})
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}

	return rows, nil
}

// GetRowCount returns the number of rows in the table
func (tf *TableFile) GetRowCount() uint64 {
	tf.mu.RLock()
	defer tf.mu.RUnlock()
	return tf.header.RowCount
}

// GetColumnDefs returns the column definitions
func (tf *TableFile) GetColumnDefs() []types.ColumnDef {
	return tf.header.ColumnDefs
}

// Flush flushes all dirty pages to disk
func (tf *TableFile) Flush() error {
	tf.mu.Lock()
	defer tf.mu.Unlock()

	// Flush dirty pages from cache
	// Use writePageToFileNoLock since we already hold the lock
	if tf.cache != nil {
		if err := tf.cache.Flush(tf.writePageToFileNoLock); err != nil {
			return err
		}
	}

	// Write header if dirty
	if tf.dirty {
		if err := tf.writeHeader(); err != nil {
			return err
		}
	}

	// Sync file
	return tf.file.Sync()
}

// FilePath returns the file path
func (tf *TableFile) FilePath() string {
	return tf.filePath
}

// PageSize returns the page size
func (tf *TableFile) PageSize() int {
	return PageSize16KB
}

// PageCount returns the number of pages
func (tf *TableFile) PageCount() uint32 {
	tf.mu.RLock()
	defer tf.mu.RUnlock()
	return tf.header.PageCount
}

// Clear removes all rows from the table
func (tf *TableFile) Clear() error {
	tf.mu.Lock()
	defer tf.mu.Unlock()

	// Reset header
	tf.header.FirstDataPage = 0
	tf.header.LastDataPage = 0
	tf.header.RowCount = 0
	tf.header.FreePageList = 0
	tf.dirty = true

	// Clear cache
	if tf.cache != nil {
		tf.cache.Clear()
	}

	// Truncate file to just header page
	if tf.file != nil {
		if err := tf.file.Truncate(PageSize16KB); err != nil {
			return err
		}
		tf.header.PageCount = 1
	}

	return nil
}

// GetFirstDataPage returns the first data page ID (for debugging)
func (tf *TableFile) GetFirstDataPage() uint64 {
	tf.mu.RLock()
	defer tf.mu.RUnlock()
	return tf.header.FirstDataPage
}

// GetLastDataPage returns the last data page ID (for debugging)
func (tf *TableFile) GetLastDataPage() uint64 {
	tf.mu.RLock()
	defer tf.mu.RUnlock()
	return tf.header.LastDataPage
}

// ReadPageDebug reads a page for debugging (public wrapper for readPage)
func (tf *TableFile) ReadPageDebug(pageID uint64) (*PageV2, error) {
	return tf.readPage(pageID)
}

// CacheSize returns the number of pages in cache (for debugging)
func (tf *TableFile) CacheSize() int {
	if tf.cache == nil {
		return 0
	}
	return tf.cache.Size()
}

// CacheStats returns cache statistics (for debugging)
func (tf *TableFile) CacheStats() map[string]interface{} {
	if tf.cache == nil {
		return nil
	}
	return tf.cache.Stats()
}