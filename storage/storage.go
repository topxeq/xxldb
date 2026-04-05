// Package storage provides data persistence for xxldb
package storage

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/topxeq/xxldb/types"
)

const (
	// PageSize is the size of a data page
	PageSize = 4096

	// PageHeaderSize is the size of the page header
	PageHeaderSize = 24

	// MaxPageContent is the maximum content size per page
	MaxPageContent = PageSize - PageHeaderSize

	// MetaFileName is the metadata file name
	MetaFileName = "xxldb.meta"

	// WALFileName is the WAL file name
	WALFileName = "xxldb.wal"

	// CheckpointFileName is the checkpoint file name
	CheckpointFileName = "xxldb.chk"

	// DataDirName is the data directory name
	DataDirName = "data"

	// BlobDirName is the blob directory name
	BlobDirName = "blobs"

	// MagicNumber for file identification
	MagicNumber = 0x58584C44 // "XXLD"

	// CurrentVersion is the current storage version
	CurrentVersion = 2
)

// PageType represents the type of a page
type PageType uint8

const (
	PageTypeFree PageType = iota
	PageTypeData
	PageTypeIndex
	PageTypeBlob
	PageTypeMeta
)

// PageHeader represents the header of a page
type PageHeader struct {
	PageID    uint64   // Page identifier
	PageType  PageType // Type of page
	Flags     uint8    // Flags
	FreeSpace uint16   // Free space offset
	ItemCount uint16   // Number of items
	Reserved  uint32   // Reserved for future use
	Checksum  uint32   // CRC32 checksum
}

// Page represents a data page
type Page struct {
	Header  PageHeader
	Content [MaxPageContent]byte
}

// NewPage creates a new page
func NewPage(id uint64, typ PageType) *Page {
	p := &Page{}
	p.Header.PageID = id
	p.Header.PageType = typ
	p.Header.FreeSpace = MaxPageContent
	p.Header.ItemCount = 0
	return p
}

// CalculateChecksum calculates the page checksum
func (p *Page) CalculateChecksum() uint32 {
	data := make([]byte, PageSize)
	binary.LittleEndian.PutUint64(data[0:8], p.Header.PageID)
	data[8] = byte(p.Header.PageType)
	data[9] = p.Header.Flags
	binary.LittleEndian.PutUint16(data[10:12], p.Header.FreeSpace)
	binary.LittleEndian.PutUint16(data[12:14], p.Header.ItemCount)
	binary.LittleEndian.PutUint32(data[14:18], p.Header.Reserved)
	copy(data[PageHeaderSize:], p.Content[:])
	// Zero out checksum field for calculation
	return crc32.ChecksumIEEE(data)
}

// VerifyChecksum verifies the page checksum
func (p *Page) VerifyChecksum() bool {
	return p.Header.Checksum == p.CalculateChecksum()
}

// ToBytes serializes the page to bytes
func (p *Page) ToBytes() []byte {
	data := make([]byte, PageSize)

	// Write header
	binary.LittleEndian.PutUint64(data[0:8], p.Header.PageID)
	data[8] = byte(p.Header.PageType)
	data[9] = p.Header.Flags
	binary.LittleEndian.PutUint16(data[10:12], p.Header.FreeSpace)
	binary.LittleEndian.PutUint16(data[12:14], p.Header.ItemCount)
	binary.LittleEndian.PutUint32(data[14:18], p.Header.Reserved)
	binary.LittleEndian.PutUint32(data[18:22], p.Header.Checksum)
	// 2 bytes padding

	// Write content
	copy(data[PageHeaderSize:], p.Content[:])

	return data
}

// FromBytes deserializes the page from bytes
func (p *Page) FromBytes(data []byte) error {
	if len(data) != PageSize {
		return fmt.Errorf("invalid page size: %d", len(data))
	}

	// Read header
	p.Header.PageID = binary.LittleEndian.Uint64(data[0:8])
	p.Header.PageType = PageType(data[8])
	p.Header.Flags = data[9]
	p.Header.FreeSpace = binary.LittleEndian.Uint16(data[10:12])
	p.Header.ItemCount = binary.LittleEndian.Uint16(data[12:14])
	p.Header.Reserved = binary.LittleEndian.Uint32(data[14:18])
	p.Header.Checksum = binary.LittleEndian.Uint32(data[18:22])

	// Read content
	copy(p.Content[:], data[PageHeaderSize:])

	return nil
}

// Storage handles data persistence
type Storage struct {
	mu       sync.RWMutex
	path     string
	enabled  bool // false for in-memory mode

	// Metadata
	tables    map[string]*TableInfo
	sequences map[string]int64
	nextID    uint64

	// Data storage
	dataFiles map[string]*os.File   // Table name -> file
	pages     map[uint64]*Page      // Page cache
	rowData   map[string][][]types.Value // Table name -> rows (in-memory storage)
	rowIDs    map[string][]uint64       // Table name -> row IDs

	// WAL
	walFile   *os.File
	walSeq    uint64

	// Checkpoint
	lastCheckpoint time.Time

	// Config
	config Config
}

// Config holds storage configuration
type Config struct {
	SyncInterval   time.Duration // File sync interval
	CheckpointInt  time.Duration // Checkpoint interval
	BufferSize     int           // Page buffer size
	AutoCheckpoint bool          // Enable auto checkpoint
}

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	return Config{
		SyncInterval:   time.Second,
		CheckpointInt:  time.Minute * 5,
		BufferSize:     1000,
		AutoCheckpoint: true,
	}
}

// TableInfo stores table metadata (extends types.TableInfo)
type TableInfo struct {
	types.TableInfo
	DataFile string `json:"data_file"`
}

// NewStorage creates a new storage instance
func NewStorage(path string, inMemory bool) (*Storage, error) {
	s := &Storage{
		path:       path,
		enabled:    !inMemory,
		tables:     make(map[string]*TableInfo),
		sequences:  make(map[string]int64),
		nextID:     1,
		dataFiles:  make(map[string]*os.File),
		pages:      make(map[uint64]*Page),
		rowData:    make(map[string][][]types.Value),
		rowIDs:     make(map[string][]uint64),
		config:     DefaultConfig(),
	}

	if !inMemory && path != "" {
		if err := s.initFileStorage(); err != nil {
			return nil, err
		}
	}

	return s, nil
}

// initFileStorage initializes file-based storage
func (s *Storage) initFileStorage() error {
	// Create directory if not exists
	if err := os.MkdirAll(s.path, 0755); err != nil {
		return fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Create subdirectories
	if err := os.MkdirAll(filepath.Join(s.path, DataDirName), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(s.path, BlobDirName), 0755); err != nil {
		return err
	}

	// Load existing metadata
	if err := s.loadMetadata(); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to load metadata: %w", err)
		}
	}

	// Always try to recover from WAL (if exists)
	if err := s.recoverFromWAL(); err != nil {
		// Non-fatal: WAL might not exist yet
		s.lastCheckpoint = time.Now()
	}

	// Initialize WAL
	if err := s.initWAL(); err != nil {
		return fmt.Errorf("failed to initialize WAL: %w", err)
	}

	return nil
}

// loadMetadata loads metadata from disk
func (s *Storage) loadMetadata() error {
	metaPath := filepath.Join(s.path, MetaFileName)

	data, err := os.ReadFile(metaPath)
	if err != nil {
		return err
	}

	var meta struct {
		Version   uint64                `json:"version"`
		Tables    map[string]*TableInfo `json:"tables"`
		Sequences map[string]int64      `json:"sequences"`
		NextID    uint64                `json:"next_id"`
	}

	if err := json.Unmarshal(data, &meta); err != nil {
		return fmt.Errorf("failed to parse metadata: %w", err)
	}

	s.tables = meta.Tables
	s.sequences = meta.Sequences
	if s.sequences == nil {
		s.sequences = make(map[string]int64)
	}
	s.nextID = meta.NextID
	if s.nextID == 0 {
		s.nextID = 1
	}

	// Initialize row storage for each table
	for name := range s.tables {
		if s.rowData[name] == nil {
			s.rowData[name] = make([][]types.Value, 0)
		}
		if s.rowIDs[name] == nil {
			s.rowIDs[name] = make([]uint64, 0)
		}
	}

	return nil
}

// saveMetadata saves metadata to disk
func (s *Storage) saveMetadata() error {
	if !s.enabled {
		return nil
	}

	meta := struct {
		Version   uint64               `json:"version"`
		Tables    map[string]*TableInfo `json:"tables"`
		Sequences map[string]int64     `json:"sequences"`
		NextID    uint64               `json:"next_id"`
	}{
		Version:   CurrentVersion,
		Tables:    s.tables,
		Sequences: s.sequences,
		NextID:    s.nextID,
	}

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	metaPath := filepath.Join(s.path, MetaFileName)
	tempPath := metaPath + ".tmp"

	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	if err := os.Rename(tempPath, metaPath); err != nil {
		return fmt.Errorf("failed to rename metadata file: %w", err)
	}

	return nil
}

// CreateTable creates a new table
func (s *Storage) CreateTable(info *types.TableInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tables[info.Name]; exists {
		return fmt.Errorf("table %s already exists", info.Name)
	}

	// Assign ID
	info.ID = s.nextID
	s.nextID++

	// Create table info
	tableInfo := &TableInfo{
		TableInfo: *info,
		DataFile:  fmt.Sprintf("table_%d.db", info.ID),
	}

	tableInfo.CreatedAt = time.Now()
	tableInfo.UpdatedAt = time.Now()
	tableInfo.RowCount = 0

	s.tables[info.Name] = tableInfo

	// Initialize row data storage for this table
	s.rowData[info.Name] = make([][]types.Value, 0)
	s.rowIDs[info.Name] = make([]uint64, 0)

	// Initialize sequence for auto-increment columns
	for _, col := range info.Columns {
		if col.AutoInc || col.Type == types.TypeSeq {
			s.sequences[info.Name+"_"+col.Name] = 0
		}
	}

	// Log to WAL
	if s.enabled {
		if err := s.writeWAL(WALRecord{
			Type:    WALTypeCreateTable,
			TableID: info.ID,
			Data:    tableInfo,
		}); err != nil {
			return err
		}
		if err := s.saveMetadata(); err != nil {
			return err
		}
	}

	return nil
}

// DropTable drops a table
func (s *Storage) DropTable(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	info, exists := s.tables[name]
	if !exists {
		return fmt.Errorf("table %s does not exist", name)
	}

	// Close data file if open
	if file, ok := s.dataFiles[name]; ok {
		file.Close()
		delete(s.dataFiles, name)
	}

	// Delete data file
	if s.enabled && info.DataFile != "" {
		dataPath := filepath.Join(s.path, DataDirName, info.DataFile)
		os.Remove(dataPath)
	}

	// Clean up sequences
	for _, col := range info.Columns {
		if col.AutoInc || col.Type == types.TypeSeq {
			delete(s.sequences, name+"_"+col.Name)
		}
	}

	// Clean up row data
	delete(s.rowData, name)
	delete(s.rowIDs, name)

	delete(s.tables, name)

	// Log to WAL
	if s.enabled {
		if err := s.writeWAL(WALRecord{
			Type:    WALTypeDropTable,
			TableID: info.ID,
			Data:    name,
		}); err != nil {
			return err
		}
		if err := s.saveMetadata(); err != nil {
			return err
		}
	}

	return nil
}

// GetTableInfo gets table information
func (s *Storage) GetTableInfo(name string) (*types.TableInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	info, exists := s.tables[name]
	if !exists {
		return nil, fmt.Errorf("table %s does not exist", name)
	}

	return &info.TableInfo, nil
}

// ListTables lists all tables
func (s *Storage) ListTables() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.tables))
	for name := range s.tables {
		names = append(names, name)
	}
	return names
}

// RenameTable renames a table
func (s *Storage) RenameTable(oldName, newName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	info, exists := s.tables[oldName]
	if !exists {
		return fmt.Errorf("table %s does not exist", oldName)
	}

	if _, exists := s.tables[newName]; exists {
		return fmt.Errorf("table %s already exists", newName)
	}

	// Update table info
	info.Name = newName

	// Move in tables map
	delete(s.tables, oldName)
	s.tables[newName] = info

	// Move data file reference
	if file, ok := s.dataFiles[oldName]; ok {
		delete(s.dataFiles, oldName)
		s.dataFiles[newName] = file
	}

	// Move row data
	if data, ok := s.rowData[oldName]; ok {
		delete(s.rowData, oldName)
		s.rowData[newName] = data
	}
	if ids, ok := s.rowIDs[oldName]; ok {
		delete(s.rowIDs, oldName)
		s.rowIDs[newName] = ids
	}

	// Rename sequences
	for _, col := range info.Columns {
		if col.AutoInc || col.Type == types.TypeSeq {
			oldSeqKey := oldName + "_" + col.Name
			newSeqKey := newName + "_" + col.Name
			if val, ok := s.sequences[oldSeqKey]; ok {
				delete(s.sequences, oldSeqKey)
				s.sequences[newSeqKey] = val
			}
		}
	}

	// Log to WAL
	if s.enabled {
		if err := s.writeWAL(WALRecord{
			Type:    WALTypeRenameTable,
			TableID: info.ID,
			Data:    oldName + ":" + newName,
		}); err != nil {
			return err
		}
		if err := s.saveMetadata(); err != nil {
			return err
		}
	}

	return nil
}

// TruncateTable removes all rows from a table but keeps the table structure
func (s *Storage) TruncateTable(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	info, exists := s.tables[name]
	if !exists {
		return fmt.Errorf("table %s does not exist", name)
	}

	// Clear row data
	s.rowData[name] = make([][]types.Value, 0)
	s.rowIDs[name] = make([]uint64, 0)

	// Reset sequences
	for _, col := range info.Columns {
		if col.AutoInc || col.Type == types.TypeSeq {
			s.sequences[name+"_"+col.Name] = 0
		}
	}

	// Reset row count
	info.RowCount = 0
	info.UpdatedAt = time.Now()

	// Log to WAL
	if s.enabled {
		if err := s.writeWAL(WALRecord{
			Type:    WALTypeTruncateTable,
			TableID: info.ID,
			Data:    name,
		}); err != nil {
			return err
		}
		if err := s.saveMetadata(); err != nil {
			return err
		}
	}

	return nil
}

// InsertRow inserts a row into a table
func (s *Storage) InsertRow(tableName string, row []types.Value) (uint64, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	info, exists := s.tables[tableName]
	if !exists {
		return 0, 0, fmt.Errorf("table %s does not exist", tableName)
	}

	// Handle auto-increment
	var lastInsertID int64
	for i, col := range info.Columns {
		if col.AutoInc || col.Type == types.TypeSeq {
			seqKey := tableName + "_" + col.Name
			seq := s.sequences[seqKey] + 1
			s.sequences[seqKey] = seq
			row[i] = types.NewIntValue(seq)
			lastInsertID = seq
		}
	}

	// Assign row ID
	rowID := s.nextID
	s.nextID++

	// Log to WAL
	if s.enabled {
		if err := s.writeWAL(WALRecord{
			Type:    WALTypeInsert,
			TableID: info.ID,
			RowID:   rowID,
			Data:    row,
		}); err != nil {
			return 0, 0, err
		}
	}

		// Store row data
		s.rowData[tableName] = append(s.rowData[tableName], row)
		s.rowIDs[tableName] = append(s.rowIDs[tableName], rowID)

	info.RowCount++
	info.UpdatedAt = time.Now()

	// Save metadata periodically
	if s.enabled {
		if err := s.saveMetadata(); err != nil {
			return 0, 0, err
		}
	}

	return rowID, lastInsertID, nil
}

// Close closes the storage
func (s *Storage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Close all data files
	for _, file := range s.dataFiles {
		file.Close()
	}

	// Close WAL
	if s.walFile != nil {
		s.walFile.Close()
	}

	// Save metadata
	if s.enabled {
		if err := s.saveMetadata(); err != nil {
			return err
		}
	}

	return nil
}

// Backup creates a backup of the database
func (s *Storage) Backup(backupPath string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.enabled {
		// For in-memory, save to the specified path
		tmpStorage, err := NewStorage(backupPath, false)
		if err != nil {
			return err
		}
		defer tmpStorage.Close()

		// Copy all tables
		for name, info := range s.tables {
			if err := tmpStorage.CreateTable(&info.TableInfo); err != nil {
				return err
			}
			_ = name
		}

		return nil
	}

	// Create backup directory
	if err := os.MkdirAll(backupPath, 0755); err != nil {
		return err
	}

	// Copy files
	files := []string{MetaFileName, WALFileName}
	for _, f := range files {
		src := filepath.Join(s.path, f)
		dst := filepath.Join(backupPath, f)
		if err := copyFile(src, dst); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	// Copy data directory
	dataSrc := filepath.Join(s.path, DataDirName)
	dataDst := filepath.Join(backupPath, DataDirName)
	if err := copyDir(dataSrc, dataDst); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

// copyFile copies a file
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// copyDir copies a directory
func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// GetRows gets all rows from a table
func (s *Storage) GetRows(tableName string) ([][]types.Value, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, exists := s.tables[tableName]; !exists {
		return nil, fmt.Errorf("table %s does not exist", tableName)
	}

	rows, exists := s.rowData[tableName]
	if !exists {
		return [][]types.Value{}, nil
	}

	// Return a copy to avoid race conditions
	result := make([][]types.Value, len(rows))
	for i, row := range rows {
		result[i] = make([]types.Value, len(row))
		copy(result[i], row)
	}

	return result, nil
}

// UpdateRows updates rows matching a condition
func (s *Storage) UpdateRows(tableName string, updates map[int]types.Value, condition func([]types.Value) bool) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	info, exists := s.tables[tableName]
	if !exists {
		return 0, fmt.Errorf("table %s does not exist", tableName)
	}

	rows, exists := s.rowData[tableName]
	if !exists {
		return 0, nil
	}

	var count int64
	for i, row := range rows {
		if condition(row) {
			for colIdx, val := range updates {
				if colIdx < len(row) {
					rows[i][colIdx] = val
				}
			}
			count++

			// Log to WAL
			if s.enabled {
				if err := s.writeWAL(WALRecord{
					Type:    WALTypeUpdate,
					TableID: info.ID,
					RowID:   s.rowIDs[tableName][i],
					Data:    rows[i],
					OldData: row,
				}); err != nil {
					return 0, err
				}
			}
		}
	}

	if count > 0 && s.enabled {
		if err := s.saveMetadata(); err != nil {
			return 0, err
		}
	}

	return count, nil
}

// DeleteRows deletes rows matching a condition
func (s *Storage) DeleteRows(tableName string, condition func([]types.Value) bool) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	info, exists := s.tables[tableName]
	if !exists {
		return 0, fmt.Errorf("table %s does not exist", tableName)
	}

	rows, exists := s.rowData[tableName]
	if !exists {
		return 0, nil
	}

	rowIDs := s.rowIDs[tableName]
	var newRows [][]types.Value
	var newRowIDs []uint64
	var count int64

	for i, row := range rows {
		if condition(row) {
			count++

			// Log to WAL
			if s.enabled {
				if err := s.writeWAL(WALRecord{
					Type:    WALTypeDelete,
					TableID: info.ID,
					RowID:   rowIDs[i],
					Data:    row,
				}); err != nil {
					return 0, err
				}
			}
		} else {
			newRows = append(newRows, row)
			newRowIDs = append(newRowIDs, rowIDs[i])
		}
	}

	if count > 0 {
		s.rowData[tableName] = newRows
		s.rowIDs[tableName] = newRowIDs
		info.RowCount -= count
		info.UpdatedAt = time.Now()

		if s.enabled {
			if err := s.saveMetadata(); err != nil {
				return 0, err
			}
		}
	}

	return count, nil
}

// ReadBlob reads a blob from storage
func (s *Storage) ReadBlob(blobID uint64) ([]byte, error) {
	if !s.enabled {
		return nil, fmt.Errorf("blob storage not available in memory mode")
	}

	blobPath := s.getBlobPath(blobID)
	return os.ReadFile(blobPath)
}

// WriteBlob writes a blob to storage
func (s *Storage) WriteBlob(data []byte) (uint64, error) {
	if !s.enabled {
		return 0, fmt.Errorf("blob storage not available in memory mode")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	blobID := s.nextID
	s.nextID++

	blobPath := s.getBlobPath(blobID)
	blobDir := filepath.Dir(blobPath)

	// Create bucket directory if not exists
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		return 0, err
	}

	if err := os.WriteFile(blobPath, data, 0644); err != nil {
		return 0, err
	}

	return blobID, nil
}

// DeleteBlob deletes a blob from storage
func (s *Storage) DeleteBlob(blobID uint64) error {
	if !s.enabled {
		return nil
	}

	blobPath := s.getBlobPath(blobID)
	return os.Remove(blobPath)
}

// getBlobPath returns the bucketed path for a blob
// Format: blobs/XX/YY/blob_ID.bin where XX and YY are derived from ID
func (s *Storage) getBlobPath(blobID uint64) string {
	// Convert ID to 8-digit zero-padded string
	idStr := fmt.Sprintf("%08d", blobID)

	// Use first 4 digits for two levels of buckets
	// e.g., blob_12345678 → blobs/12/34/blob_12345678.bin
	bucket1 := idStr[0:2]
	bucket2 := idStr[2:4]

	return filepath.Join(s.path, BlobDirName, bucket1, bucket2, fmt.Sprintf("blob_%d.bin", blobID))
}

// ImportFile imports a file into the database
func (s *Storage) ImportFile(filePath string) (uint64, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to read file: %w", err)
	}
	return s.WriteBlob(data)
}

// ExportFile exports a blob to a file
func (s *Storage) ExportFile(blobID uint64, filePath string) error {
	data, err := s.ReadBlob(blobID)
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}

// GetSequence gets the current value of a sequence
func (s *Storage) GetSequence(name string) int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sequences[name]
}

// SetSequence sets the value of a sequence
func (s *Storage) SetSequence(name string, value int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sequences[name] = value
}

// Stats returns storage statistics
func (s *Storage) Stats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"tables":     len(s.tables),
		"sequences":  len(s.sequences),
		"next_id":    s.nextID,
		"page_cache": len(s.pages),
	}
}

// Reader returns an io.Reader for a blob
func (s *Storage) Reader(blobID uint64) (io.ReadCloser, error) {
	if !s.enabled {
		return nil, fmt.Errorf("blob storage not available in memory mode")
	}

	blobPath := s.getBlobPath(blobID)
	return os.Open(blobPath)
}

// Writer returns an io.Writer for a blob
func (s *Storage) Writer() (io.WriteCloser, uint64, error) {
	if !s.enabled {
		return nil, 0, fmt.Errorf("blob storage not available in memory mode")
	}

	s.mu.Lock()
	blobID := s.nextID
	s.nextID++
	s.mu.Unlock()

	blobPath := s.getBlobPath(blobID)
	blobDir := filepath.Dir(blobPath)

	// Create bucket directory if not exists
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		return nil, 0, err
	}

	file, err := os.Create(blobPath)
	if err != nil {
		return nil, 0, err
	}

	return file, blobID, nil
}
