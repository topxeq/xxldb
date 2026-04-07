// Package storage provides data persistence for xxldb
package storage

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/topxeq/xxldb/storage/crypto"
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

// Errors
var (
	ErrEncryptionKeyRequired = fmt.Errorf("encryption key required - database is encrypted")
)

// Storage handles data persistence
type Storage struct {
	mu       sync.RWMutex
	path     string
	enabled  bool // false for in-memory mode

	// FileSystem abstraction
	fs       FileSystem

	// Metadata
	tables    map[string]*TableInfo
	sequences map[string]int64
	nextID    uint64

	// Data storage
	dataFiles map[string]File        // Table name -> file (legacy)
	tableFiles map[string]*TableFile // Table name -> TableFile (new page-based)
	pageCache  *PageCache            // LRU page cache
	rowData   map[string][][]types.Value // Table name -> rows (in-memory storage, legacy)
	rowIDs    map[string][]uint64        // Table name -> row IDs (legacy)

	// WAL
	walFile   *os.File
	walSeq    uint64

	// Checkpoint
	lastCheckpoint time.Time

	// Config
	config Config

	// Auth configuration (persisted)
	authConfig map[string]interface{}

	// Full-text search indexes: "table.column" -> index info
	ftsIndexes map[string]*FTSIndexInfo

	// Encryption
	encrypted bool
	encryptor *crypto.Encryptor
	metadataPendingPassword bool // True if metadata is encrypted but not yet loaded

	// Skip save for bulk imports
	skipSave bool
}

// FTSIndexInfo stores full-text index metadata
type FTSIndexInfo struct {
	TableName  string `json:"table_name"`
	ColumnName string `json:"column_name"`
	IndexName  string `json:"index_name"`
}

// Config holds storage configuration
type Config struct {
	SyncInterval   time.Duration // File sync interval
	CheckpointInt  time.Duration // Checkpoint interval
	BufferSize     int           // Page buffer size
	AutoCheckpoint bool          // Enable auto checkpoint
	BlobThreshold  int64         // Size threshold for external blob storage (default: 64KB, 0 = always inline)
}

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	return Config{
		SyncInterval:   time.Second,
		CheckpointInt:  time.Minute * 5,
		BufferSize:     1000,
		AutoCheckpoint: true,
		BlobThreshold:  64 * 1024, // 64KB - blobs larger than this are stored in separate files
	}
}

// TableInfo stores table metadata (extends types.TableInfo)
type TableInfo struct {
	types.TableInfo
	DataFile string `json:"data_file"`
}

// NewStorage creates a new storage instance
func NewStorage(path string, inMemory bool) (*Storage, error) {
	return NewStorageWithFS(path, inMemory, nil)
}

// NewStorageWithFS creates a new storage instance with custom filesystem
func NewStorageWithFS(path string, inMemory bool, fs FileSystem) (*Storage, error) {
	s := &Storage{
		path:       path,
		enabled:    !inMemory,
		tables:     make(map[string]*TableInfo),
		sequences:  make(map[string]int64),
		nextID:     1,
		dataFiles:  make(map[string]File),
		tableFiles: make(map[string]*TableFile),
		pageCache:  NewPageCache(DefaultPageCacheConfig()),
		rowData:    make(map[string][][]types.Value),
		rowIDs:     make(map[string][]uint64),
		config:     DefaultConfig(),
		ftsIndexes: make(map[string]*FTSIndexInfo),
		authConfig: make(map[string]interface{}),
	}

	if !inMemory && path != "" {
		// Use provided filesystem or create local one
		if fs == nil {
			s.fs = NewLocalFS(path)
		} else {
			s.fs = fs
		}

		if err := s.initFileStorage(); err != nil {
			return nil, err
		}
	}

	return s, nil
}

// initFileStorage initializes file-based storage
func (s *Storage) initFileStorage() error {
	// Create directory if not exists
	if err := s.fs.MkdirAll(s.path, PermDir); err != nil {
		return fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Create subdirectories
	if err := s.fs.MkdirAll(s.fs.Join(s.path, DataDirName), PermDir); err != nil {
		return err
	}
	if err := s.fs.MkdirAll(s.fs.Join(s.path, BlobDirName), PermDir); err != nil {
		return err
	}

	// Check if metadata file is encrypted before loading
	metaPath := s.fs.Join(s.path, MetaFileName)
	if data, err := s.fs.ReadFile(metaPath); err == nil {
		if crypto.IsEncryptedFile(data) {
			// Metadata is encrypted, don't load yet - wait for password
			s.encrypted = true
			s.metadataPendingPassword = true
			// Don't recover from WAL yet - will do after password is set
		} else {
			// Not encrypted, load normally
			if err := s.loadMetadata(); err != nil {
				// Non-fatal: metadata might be corrupted
			}
			// Recover from WAL
			if err := s.recoverFromWAL(); err != nil {
				// Non-fatal: WAL might not exist yet
				s.lastCheckpoint = time.Now()
			}
		}
	}

	// Initialize WAL
	if err := s.initWAL(); err != nil {
		return fmt.Errorf("failed to initialize WAL: %w", err)
	}

	return nil
}

// loadMetadata loads metadata from disk
func (s *Storage) loadMetadata() error {
	if s.fs == nil {
		return nil
	}

	metaPath := s.fs.Join(s.path, MetaFileName)

	data, err := s.fs.ReadFile(metaPath)
	if err != nil {
		return err
	}

	// Check if metadata is encrypted
	if crypto.IsEncryptedFile(data) {
		s.encrypted = true
		// If no encryptor set, we need password to decrypt
		if s.encryptor == nil {
			return ErrEncryptionKeyRequired
		}
		// Decrypt data
		data, err = s.encryptor.DecryptFile(data)
		if err != nil {
			return fmt.Errorf("failed to decrypt metadata: %w", err)
		}
	}

	var meta struct {
		Version   uint64                 `json:"version"`
		Tables    map[string]*TableInfo  `json:"tables"`
		Sequences map[string]int64       `json:"sequences"`
		NextID    uint64                 `json:"next_id"`
		Auth      map[string]interface{} `json:"auth,omitempty"`
		Encrypted bool                   `json:"encrypted,omitempty"`
		Rows      map[string]interface{} `json:"rows,omitempty"`
	}

	if err := json.Unmarshal(data, &meta); err != nil {
		return fmt.Errorf("failed to parse metadata: %w", err)
	}

	// Metadata successfully loaded
	s.metadataPendingPassword = false

	s.tables = meta.Tables
	s.sequences = meta.Sequences
	if s.sequences == nil {
		s.sequences = make(map[string]int64)
	}
	s.nextID = meta.NextID
	if s.nextID == 0 {
		s.nextID = 1
	}
	if meta.Auth != nil {
		s.authConfig = meta.Auth
	}
	if meta.Encrypted {
		s.encrypted = true
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

	// Restore row data if present (support both old and new format)
	if meta.Rows != nil {
		for tableName, tableRowsInterface := range meta.Rows {
			tableInfo := s.tables[tableName]
			if tableRows, ok := tableRowsInterface.([]interface{}); ok {
				for _, rowInterface := range tableRows {
					// New compact format: [rowID, [rawVal1, rawVal2, ...]]
					if rowSlice, ok := rowInterface.([]interface{}); ok && len(rowSlice) == 2 {
						if id, ok := rowSlice[0].(float64); ok {
							rowID := uint64(id)
							if rowDataSlice, ok := rowSlice[1].([]interface{}); ok {
								row := make([]types.Value, len(rowDataSlice))
								for i, v := range rowDataSlice {
									// Convert raw value to types.Value using table schema
									row[i] = rawToValue(v, tableInfo, i)
								}
								s.rowData[tableName] = append(s.rowData[tableName], row)
								s.rowIDs[tableName] = append(s.rowIDs[tableName], rowID)
								continue
							}
						}
					}
					// Old format: {"_id": rowID, "_data": [...]}
					if rowMap, ok := rowInterface.(map[string]interface{}); ok {
						var rowID uint64
						if id, ok := rowMap["_id"].(float64); ok {
							rowID = uint64(id)
						}
						if rowDataInterface, ok := rowMap["_data"]; ok {
							if rowDataSlice, ok := rowDataInterface.([]interface{}); ok {
								row := make([]types.Value, len(rowDataSlice))
								for i, v := range rowDataSlice {
									row[i] = interfaceToValue(v)
								}
								s.rowData[tableName] = append(s.rowData[tableName], row)
								s.rowIDs[tableName] = append(s.rowIDs[tableName], rowID)
							}
						}
					}
				}
			}
		}
	}

	return nil
}

// LoadAndVerifyMetadata loads and verifies metadata (used to verify encryption password)
func (s *Storage) LoadAndVerifyMetadata() error {
	if err := s.loadMetadata(); err != nil {
		return err
	}
	// After loading encrypted metadata, also recover from WAL
	if err := s.recoverFromWAL(); err != nil {
		// Non-fatal: WAL might not exist
		s.lastCheckpoint = time.Now()
	}
	return nil
}

// saveMetadata saves metadata to disk
func (s *Storage) saveMetadata() error {
	if !s.enabled || s.fs == nil {
		return nil
	}

	// Skip if in bulk import mode
	if s.skipSave {
		return nil
	}

	// Prepare row data for serialization (compact format)
	// Store raw values only, reconstruct types.Value from table schema on load
	rowsData := make(map[string]interface{})
	for tableName, rows := range s.rowData {
		rowIDs := s.rowIDs[tableName]
		// Use ultra-compact format: [[rowID, [rawVal1, rawVal2, ...]], ...]
		// For null: store null marker as special value
		tableRows := make([]interface{}, len(rows))
		for i, row := range rows {
			// Convert each value to raw data (null represented as nil)
			rowValues := make([]interface{}, len(row))
			for j, v := range row {
				if v.IsNull {
					rowValues[j] = nil
				} else if v.BlobRef != nil {
					// Store blob ref compactly: [1, id, size]
					rowValues[j] = []interface{}{1, v.BlobRef.ID, v.BlobRef.Size}
				} else {
					rowValues[j] = v.Data
				}
			}
			tableRows[i] = []interface{}{rowIDs[i], rowValues}
		}
		rowsData[tableName] = tableRows
	}

	meta := struct {
		Version   uint64                 `json:"version"`
		Tables    map[string]*TableInfo  `json:"tables"`
		Sequences map[string]int64       `json:"sequences"`
		NextID    uint64                 `json:"next_id"`
		Auth      map[string]interface{} `json:"auth,omitempty"`
		Encrypted bool                   `json:"encrypted,omitempty"`
		Rows      map[string]interface{} `json:"rows,omitempty"`
	}{
		Version:   CurrentVersion,
		Tables:    s.tables,
		Sequences: s.sequences,
		NextID:    s.nextID,
		Auth:      s.authConfig,
		Encrypted: s.encrypted,
		Rows:      rowsData,
	}

	// Use compact JSON (no indent) to reduce size
	data, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Encrypt if encryption is enabled
	if s.encrypted && s.encryptor != nil {
		encryptedData, err := s.encryptor.EncryptFile(data)
		if err != nil {
			return fmt.Errorf("failed to encrypt metadata: %w", err)
		}
		data = encryptedData
	}

	metaPath := s.fs.Join(s.path, MetaFileName)

	// Use safe write (temp file + atomic rename)
	if lfs, ok := s.fs.(*LocalFS); ok {
		return lfs.SafeWriteFile(metaPath, data, PermFile)
	}

	// Generic safe write
	tempPath := metaPath + ".tmp." + generateID()

	if err := s.fs.WriteFile(tempPath, data, PermFile); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	if err := s.fs.Rename(tempPath, metaPath); err != nil {
		s.fs.Remove(tempPath)
		return fmt.Errorf("failed to rename metadata file: %w", err)
	}

	return nil
}

// SetSkipSave enables or disables skip-save mode for bulk imports
// When enabled, saveMetadata() is a no-op until disabled
func (s *Storage) SetSkipSave(skip bool) {
	s.skipSave = skip
}

// ForceSave forces a save even if skipSave is enabled, and truncates WAL
func (s *Storage) ForceSave() error {
	// Temporarily disable skipSave
	oldSkip := s.skipSave
	s.skipSave = false
	defer func() { s.skipSave = oldSkip }()

	// Now save metadata
	if err := s.saveMetadata(); err != nil {
		return err
	}

	// Truncate WAL after successful save (all data is now in metadata)
	if s.walFile != nil {
		if err := s.walFile.Truncate(0); err != nil {
			return fmt.Errorf("failed to truncate WAL: %w", err)
		}
		if _, err := s.walFile.Seek(0, 0); err != nil {
			return fmt.Errorf("failed to reset WAL: %w", err)
		}
		s.walSeq = 0
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

	// Initialize sequence for auto-increment columns
	for _, col := range info.Columns {
		if col.AutoInc || col.Type == types.TypeSeq {
			s.sequences[info.Name+"_"+col.Name] = 0
		}
	}

	// Create table file for V2 format (if persistent storage)
	if s.enabled {
		// Ensure data directory exists
		dataDir := filepath.Join(s.path, DataDirName)
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			return fmt.Errorf("failed to create data directory: %w", err)
		}

		// Create table file
		tf, err := CreateTableFile(s, info.Name, info.ID, info.Columns)
		if err != nil {
			return fmt.Errorf("failed to create table file: %w", err)
		}
		s.tableFiles[info.Name] = tf

		// Log to WAL
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
	} else {
		// In-memory mode: use legacy storage
		s.rowData[info.Name] = make([][]types.Value, 0)
		s.rowIDs[info.Name] = make([]uint64, 0)
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
		dataPath := s.fs.Join(s.path, DataDirName, info.DataFile)
		s.fs.Remove(dataPath)
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

	// Handle V2 format (table files)
	if tf, ok := s.tableFiles[oldName]; ok {
		delete(s.tableFiles, oldName)
		s.tableFiles[newName] = tf
		// Rename the physical file
		oldPath := filepath.Join(s.path, DataDirName, oldName+TableFileExt)
		newPath := filepath.Join(s.path, DataDirName, newName+TableFileExt)
		if _, err := os.Stat(oldPath); err == nil {
			os.Rename(oldPath, newPath)
		}
	}

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

	// Check if using V2 format (page-based storage)
	if tf, exists := s.tableFiles[name]; exists {
		// Clear the table file
		if err := tf.Clear(); err != nil {
			return err
		}
		// Reset sequences
		for _, col := range info.Columns {
			if col.AutoInc || col.Type == types.TypeSeq {
				s.sequences[name+"_"+col.Name] = 0
			}
		}
		info.RowCount = 0
		info.UpdatedAt = time.Now()
		return nil
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

	// Validate and process column values
	for i, col := range info.Columns {
		if i >= len(row) {
			continue
		}

		// Handle CHAR type: pad with spaces to fixed length
		if col.Type == types.TypeChar && col.Length > 0 && !row[i].IsNull {
			strVal := row[i].ToString()
			runeCount := utf8.RuneCountInString(strVal)
			if runeCount > col.Length {
				return 0, 0, fmt.Errorf("value too long for column '%s': max length %d, got %d", col.Name, col.Length, runeCount)
			}
			// Pad with trailing spaces to fixed length (by characters, not bytes)
			if runeCount < col.Length {
				runes := []rune(strVal)
				paddedRunes := append(runes, []rune(strings.Repeat(" ", col.Length-runeCount))...)
				row[i] = types.NewStringValue(string(paddedRunes))
			}
		}

		// Validate VARCHAR length constraint (no padding, just validation)
		if col.Type == types.TypeVarchar && col.Length > 0 {
			strVal := row[i].ToString()
			if utf8.RuneCountInString(strVal) > col.Length {
				return 0, 0, fmt.Errorf("value too long for column '%s': max length %d, got %d", col.Name, col.Length, utf8.RuneCountInString(strVal))
			}
		}

		// Validate NOT NULL constraint
		if !col.Nullable && row[i].IsNull && !col.AutoInc && col.Type != types.TypeSeq {
			return 0, 0, fmt.Errorf("column '%s' cannot be null", col.Name)
		}

		// Handle large BLOBs - store externally if above threshold
		if s.enabled && s.config.BlobThreshold > 0 && (col.Type == types.TypeBlob || col.Type == types.TypeImage) {
			row[i] = s.processBlobValue(row[i])
		}
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

	// Check if using V2 format (page-based storage)
	if tf, exists := s.tableFiles[tableName]; exists {
		// V2 format: use TableFile
		insertedRowID, err := tf.InsertRow(row)
		if err != nil {
			return 0, 0, err
		}
		info.RowCount++
		info.UpdatedAt = time.Now()
		return insertedRowID, lastInsertID, nil
	}

	// Check if data file exists for V2 format
	dataPath := filepath.Join(s.path, DataDirName, tableName+TableFileExt)
	if _, err := os.Stat(dataPath); err == nil {
		// Open table file and use V2 format
		tf, err := s.getTableFileLocked(tableName)
		if err == nil {
			insertedRowID, err := tf.InsertRow(row)
			if err != nil {
				return 0, 0, err
			}
			info.RowCount++
			info.UpdatedAt = time.Now()
			return insertedRowID, lastInsertID, nil
		}
	}

	// V1 format: use in-memory storage
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

// getTableFileLocked gets or creates a TableFile while already holding the lock
func (s *Storage) getTableFileLocked(tableName string) (*TableFile, error) {
	if tf, exists := s.tableFiles[tableName]; exists {
		return tf, nil
	}

	tableInfo, exists := s.tables[tableName]
	if !exists {
		return nil, fmt.Errorf("table %s does not exist", tableName)
	}

	dataPath := filepath.Join(s.path, DataDirName, tableName+TableFileExt)
	if _, err := os.Stat(dataPath); err == nil {
		tf, err := OpenTableFile(s, tableName)
		if err != nil {
			return nil, err
		}
		s.tableFiles[tableName] = tf
		return tf, nil
	}

	// Create new table file
	tf, err := CreateTableFile(s, tableName, tableInfo.ID, tableInfo.Columns)
	if err != nil {
		return nil, err
	}
	s.tableFiles[tableName] = tf
	return tf, nil
}

// processBlobValue checks if a blob value should be stored externally
// and returns either the original value or a blob reference
func (s *Storage) processBlobValue(val types.Value) types.Value {
	if val.IsNull {
		return val
	}

	data, ok := val.Data.([]byte)
	if !ok {
		return val
	}

	// Check if blob size exceeds threshold
	if int64(len(data)) > s.config.BlobThreshold {
		// Store blob externally
		blobID := s.nextID
		s.nextID++

		blobPath := s.getBlobPath(blobID)
		blobDir := s.fs.Dir(blobPath)

		// Create bucket directory if not exists
		if err := s.fs.MkdirAll(blobDir, PermDir); err != nil {
			// If we can't create the directory, store inline
			return val
		}

		// Encrypt blob data if encryption is enabled
		writeData := data
		if s.encrypted && s.encryptor != nil {
			encrypted, err := s.encryptor.Encrypt(data)
			if err != nil {
				// If encryption fails, store inline
				return val
			}
			writeData = encrypted
		}

		if err := s.fs.WriteFile(blobPath, writeData, PermFile); err != nil {
			// If we can't write the file, store inline
			return val
		}

		// Return a blob reference instead of the data
		return types.NewBlobRefValue(blobID, int64(len(data)))
	}

	return val
}

// GetDataPath returns the data path
func (s *Storage) GetDataPath() string {
	return s.path
}

// Close closes the storage
func (s *Storage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Flush and close all table files
	for name, tf := range s.tableFiles {
		tf.Flush()
		tf.Close()
		delete(s.tableFiles, name)
	}

	// Close all data files (legacy)
	for _, file := range s.dataFiles {
		file.Close()
	}

	// Save metadata and truncate WAL
	// Only save if metadata was successfully loaded (not just detected as encrypted)
	if s.enabled && !s.metadataPendingPassword {
		if err := s.saveMetadata(); err != nil {
			return err
		}
		// Truncate WAL after saving metadata
		if s.walFile != nil {
			s.walFile.Truncate(0)
			s.walFile.Seek(0, 0)
		}
	}

	// Close WAL
	if s.walFile != nil {
		s.walFile.Close()
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

	// Check if using V2 format (page-based storage)
	if tf, exists := s.tableFiles[tableName]; exists {
		rows, err := tf.GetAllRows()
		if err != nil {
			return nil, err
		}
		// Resolve blob references
		result := make([][]types.Value, len(rows))
		for i, row := range rows {
			result[i] = make([]types.Value, len(row))
			for j, val := range row {
				result[i][j] = s.resolveBlobRef(val)
			}
		}
		return result, nil
	}

	// Check if data file exists for V2 format
	dataPath := filepath.Join(s.path, DataDirName, tableName+TableFileExt)
	if _, err := os.Stat(dataPath); err == nil {
		// Open table file and use V2 format
		tf, err := OpenTableFile(s, tableName)
		if err == nil {
			s.tableFiles[tableName] = tf
			rows, err := tf.GetAllRows()
			if err != nil {
				return nil, err
			}
			// Resolve blob references
			result := make([][]types.Value, len(rows))
			for i, row := range rows {
				result[i] = make([]types.Value, len(row))
				for j, val := range row {
					result[i][j] = s.resolveBlobRef(val)
				}
			}
			return result, nil
		}
	}

	// V1 format: use in-memory storage
	rows, exists := s.rowData[tableName]
	if !exists {
		return [][]types.Value{}, nil
	}

	// Return a copy to avoid race conditions
	result := make([][]types.Value, len(rows))
	for i, row := range rows {
		result[i] = make([]types.Value, len(row))
		for j, val := range row {
			// Resolve blob references
			result[i][j] = s.resolveBlobRef(val)
		}
	}

	return result, nil
}

// RowWithID represents a row with its ID
type RowWithID struct {
	ID   uint64
	Row  []types.Value
}

// GetRowsWithIDs gets all rows from a table with their IDs
func (s *Storage) GetRowsWithIDs(tableName string) ([]RowWithID, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, exists := s.tables[tableName]; !exists {
		return nil, fmt.Errorf("table %s does not exist", tableName)
	}

	rows, exists := s.rowData[tableName]
	if !exists {
		return []RowWithID{}, nil
	}

	rowIDs := s.rowIDs[tableName]
	result := make([]RowWithID, len(rows))
	for i, row := range rows {
		result[i] = RowWithID{
			ID:  rowIDs[i],
			Row: make([]types.Value, len(row)),
		}
		for j, val := range row {
			result[i].Row[j] = s.resolveBlobRef(val)
		}
	}

	return result, nil
}

// resolveBlobRef resolves a blob reference to actual data
// If the value is not a blob reference, returns the value as-is
func (s *Storage) resolveBlobRef(val types.Value) types.Value {
	if !val.IsBlobRef() || val.BlobRef == nil {
		return val
	}

	// Load blob data from external storage
	data, err := s.ReadBlob(val.BlobRef.ID)
	if err != nil {
		// If we can't load the blob, return the reference
		return val
	}

	// Return a new value with the actual data
	return types.Value{
		Type:    val.Type,
		Data:    data,
		BlobRef: val.BlobRef, // Keep the reference for size info
	}
}

// GetRowsWithoutBlobRefs gets all rows without resolving blob references
// This is more efficient when you only need to check row structure or non-blob columns
func (s *Storage) GetRowsWithoutBlobRefs(tableName string) ([][]types.Value, error) {
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

	// Check if using V2 format (page-based storage)
	if tf, exists := s.tableFiles[tableName]; exists {
		var count int64
		// Get all rows, find matching ones, update them
		rows, err := tf.GetAllRows()
		if err != nil {
			return 0, err
		}
		for _, row := range rows {
			if condition(row) {
				// Apply updates
				for colIdx, val := range updates {
					if colIdx < len(row) {
						row[colIdx] = val
					}
				}
				count++
			}
		}
		// For V2, we need to rewrite the table
		// Simple approach: clear and re-insert all rows
		if count > 0 {
			// Get current page ID assignment
			// Clear the table file and re-insert
			tf.Clear()
			for _, row := range rows {
				_, err := tf.InsertRow(row)
				if err != nil {
					return 0, err
				}
			}
			info.RowCount = int64(len(rows))
			info.UpdatedAt = time.Now()
		}
		return count, nil
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

// UpdateRowsWithFunc updates rows matching a condition using a function to compute updates
// This allows expressions like "SET counter = counter + 1" to work correctly
func (s *Storage) UpdateRowsWithFunc(tableName string, updateFunc func([]types.Value) map[int]types.Value, condition func([]types.Value) bool) (int64, error) {
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

	// Helper function to validate and process a value against column constraints
	processValue := func(colIdx int, val types.Value) (types.Value, error) {
		if colIdx >= len(info.Columns) {
			return val, nil
		}
		col := info.Columns[colIdx]

		// Handle CHAR type: pad with spaces to fixed length (by characters, not bytes)
		if col.Type == types.TypeChar && col.Length > 0 && !val.IsNull {
			strVal := val.ToString()
			runeCount := utf8.RuneCountInString(strVal)
			if runeCount > col.Length {
				return val, fmt.Errorf("value too long for column '%s': max length %d, got %d", col.Name, col.Length, runeCount)
			}
			// Pad with trailing spaces to fixed length
			if runeCount < col.Length {
				runes := []rune(strVal)
				paddedRunes := append(runes, []rune(strings.Repeat(" ", col.Length-runeCount))...)
				return types.NewStringValue(string(paddedRunes)), nil
			}
		}

		// Validate VARCHAR length constraint
		if col.Type == types.TypeVarchar && col.Length > 0 {
			strVal := val.ToString()
			if utf8.RuneCountInString(strVal) > col.Length {
				return val, fmt.Errorf("value too long for column '%s': max length %d, got %d", col.Name, col.Length, utf8.RuneCountInString(strVal))
			}
		}

		// Validate NOT NULL constraint
		if !col.Nullable && val.IsNull {
			return val, fmt.Errorf("column '%s' cannot be null", col.Name)
		}

		return val, nil
	}

	var count int64
	for i, row := range rows {
		if condition(row) {
			// Compute updates based on current row values
			updates := updateFunc(row)
			for colIdx, val := range updates {
				if colIdx < len(row) {
					// Validate and process (including CHAR padding)
					processedVal, err := processValue(colIdx, val)
					if err != nil {
						return 0, err
					}
					rows[i][colIdx] = processedVal
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

	// Check if using V2 format (page-based storage)
	if tf, exists := s.tableFiles[tableName]; exists {
		var count int64
		// Get all rows, filter out matching ones
		rows, err := tf.GetAllRows()
		if err != nil {
			return 0, err
		}
		var newRows [][]types.Value
		for _, row := range rows {
			if condition(row) {
				count++
			} else {
				newRows = append(newRows, row)
			}
		}
		if count > 0 {
			// Clear and re-insert remaining rows
			tf.Clear()
			for _, row := range newRows {
				_, err := tf.InsertRow(row)
				if err != nil {
					return 0, err
				}
			}
			info.RowCount = int64(len(newRows))
			info.UpdatedAt = time.Now()
		}
		return count, nil
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
	data, err := s.fs.ReadFile(blobPath)
	if err != nil {
		return nil, err
	}

	// Decrypt if encryption is enabled
	if s.encrypted && s.encryptor != nil {
		decrypted, err := s.encryptor.Decrypt(data)
		if err != nil {
			// If decryption fails, the data might not be encrypted (legacy)
			// Return original data
			return data, nil
		}
		return decrypted, nil
	}

	return data, nil
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
	blobDir := s.fs.Dir(blobPath)

	// Create bucket directory if not exists
	if err := s.fs.MkdirAll(blobDir, PermDir); err != nil {
		return 0, err
	}

	// Encrypt if encryption is enabled
	writeData := data
	if s.encrypted && s.encryptor != nil {
		encrypted, err := s.encryptor.Encrypt(data)
		if err != nil {
			return 0, fmt.Errorf("failed to encrypt blob: %w", err)
		}
		writeData = encrypted
	}

	if err := s.fs.WriteFile(blobPath, writeData, PermFile); err != nil {
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
	return s.fs.Remove(blobPath)
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

	return s.fs.Join(s.path, BlobDirName, bucket1, bucket2, fmt.Sprintf("blob_%d.bin", blobID))
}

// ImportFile imports a file into the database
func (s *Storage) ImportFile(filePath string) (uint64, error) {
	// ImportFile reads from local filesystem and writes to database storage
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
	// ExportFile writes to local filesystem
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
		"page_cache": 0, // Will be implemented with PageCache
	}
}

// GetConfig returns the current storage configuration
func (s *Storage) GetConfig() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// SetConfig updates the storage configuration
func (s *Storage) SetConfig(config Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = config
}

// GetAuthConfig returns the stored auth configuration
func (s *Storage) GetAuthConfig() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.authConfig == nil {
		return nil
	}
	// Return a copy
	result := make(map[string]interface{})
	for k, v := range s.authConfig {
		result[k] = v
	}
	return result
}

// SetAuthConfig updates the auth configuration and saves metadata
func (s *Storage) SetAuthConfig(authConfig map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.authConfig = authConfig
	return s.saveMetadata()
}

// IsEncrypted returns whether the storage is encrypted
func (s *Storage) IsEncrypted() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.encrypted
}

// SetEncryption enables or disables encryption
func (s *Storage) SetEncryption(password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if password == "" {
		// Disable encryption
		s.encrypted = false
		s.encryptor = nil
		return nil
	}

	// Generate salt for new encryption
	salt, err := crypto.GenerateSalt()
	if err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}

	// Create encryptor
	encryptor, err := crypto.NewEncryptor(password, salt)
	if err != nil {
		return fmt.Errorf("failed to create encryptor: %w", err)
	}

	s.encrypted = true
	s.encryptor = encryptor

	// If metadata file exists and is encrypted, load it
	if s.fs != nil {
		metaPath := s.fs.Join(s.path, MetaFileName)
		if data, err := s.fs.ReadFile(metaPath); err == nil {
			if crypto.IsEncryptedFile(data) {
				// Load existing encrypted metadata
				if err := s.loadMetadata(); err != nil {
					return fmt.Errorf("failed to load metadata: %w", err)
				}
				// Recover from WAL
				if err := s.recoverFromWAL(); err != nil {
					// Non-fatal
				}
				return nil
			}
		}
	}

	// Save metadata with encryption enabled (new database)
	if s.enabled {
		return s.saveMetadata()
	}
	return nil
}

// SetEncryptionWithSalt enables encryption with existing salt (for opening encrypted DB)
func (s *Storage) SetEncryptionWithSalt(password string, salt []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if password == "" {
		return fmt.Errorf("password required for encrypted database")
	}

	encryptor, err := crypto.NewEncryptor(password, salt)
	if err != nil {
		return fmt.Errorf("failed to create encryptor: %w", err)
	}

	s.encrypted = true
	s.encryptor = encryptor
	return nil
}

// GetEncryptionSalt returns the salt used for encryption
func (s *Storage) GetEncryptionSalt() []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.encryptor == nil {
		return nil
	}
	return s.encryptor.GetSalt()
}

// encryptData encrypts data if encryption is enabled
func (s *Storage) encryptData(data []byte) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.encrypted || s.encryptor == nil {
		return data, nil
	}

	return s.encryptor.EncryptFile(data)
}

// decryptData decrypts data if encryption is enabled
func (s *Storage) decryptData(data []byte) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.encrypted || s.encryptor == nil {
		return data, nil
	}

	// Check if data is encrypted
	if !crypto.IsEncryptedFile(data) {
		return data, nil
	}

	return s.encryptor.DecryptFile(data)
}

// ChangePassword changes the encryption password
func (s *Storage) ChangePassword(oldPassword, newPassword string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Verify old password if currently encrypted
	if s.encrypted && s.encryptor != nil {
		// Try to decrypt metadata to verify password
		metaPath := s.fs.Join(s.path, MetaFileName)
		data, err := s.fs.ReadFile(metaPath)
		if err == nil && crypto.IsEncryptedFile(data) {
			// Verify old password can decrypt
			// TXDEF doesn't use salt, pass nil
			oldEncryptor, err := crypto.NewEncryptor(oldPassword, nil)
			if err != nil {
				return fmt.Errorf("invalid old password")
			}
			_, err = oldEncryptor.DecryptFile(data)
			if err != nil {
				return fmt.Errorf("invalid old password")
			}
		}
	}

	// Set new password
	if newPassword == "" {
		s.encrypted = false
		s.encryptor = nil
	} else {
		// TXDEF doesn't use salt, pass nil
		encryptor, err := crypto.NewEncryptor(newPassword, nil)
		if err != nil {
			return fmt.Errorf("failed to create encryptor: %w", err)
		}
		s.encrypted = true
		s.encryptor = encryptor
	}

	// Re-save all data with new encryption
	if err := s.saveMetadata(); err != nil {
		return err
	}

	// Truncate WAL to avoid issues with old encryption
	// All data should be in metadata after saveMetadata()
	if s.walFile != nil {
		if err := s.walFile.Truncate(0); err != nil {
			return fmt.Errorf("failed to truncate WAL: %w", err)
		}
		if _, err := s.walFile.Seek(0, 0); err != nil {
			return fmt.Errorf("failed to seek WAL: %w", err)
		}
		s.walSeq = 0
	}

	return nil
}

// CreateFTSIndex creates a full-text search index
func (s *Storage) CreateFTSIndex(tableName, columnName, indexName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if table exists
	if _, exists := s.tables[tableName]; !exists {
		return fmt.Errorf("table %s does not exist", tableName)
	}

	// Check if column exists and is text type
	tableInfo := s.tables[tableName]
	colExists := false
	for _, col := range tableInfo.Columns {
		if strings.EqualFold(col.Name, columnName) {
			if col.Type.IsString() || col.Type == types.TypeText {
				colExists = true
			} else {
				return fmt.Errorf("column %s is not a text column", columnName)
			}
			break
		}
	}
	if !colExists {
		return fmt.Errorf("column %s does not exist in table %s", columnName, tableName)
	}

	// Check if index already exists
	key := tableName + "." + columnName
	if _, exists := s.ftsIndexes[key]; exists {
		return fmt.Errorf("full-text index already exists on %s.%s", tableName, columnName)
	}

	// Create index info
	s.ftsIndexes[key] = &FTSIndexInfo{
		TableName:  tableName,
		ColumnName: columnName,
		IndexName:  indexName,
	}

	// Save metadata
	if s.enabled {
		if err := s.saveMetadata(); err != nil {
			delete(s.ftsIndexes, key)
			return err
		}
	}

	return nil
}

// DropFTSIndex drops a full-text search index
func (s *Storage) DropFTSIndex(tableName, columnName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := tableName + "." + columnName
	if _, exists := s.ftsIndexes[key]; !exists {
		return fmt.Errorf("full-text index does not exist on %s.%s", tableName, columnName)
	}

	delete(s.ftsIndexes, key)

	if s.enabled {
		if err := s.saveMetadata(); err != nil {
			return err
		}
	}

	return nil
}

// GetFTSIndexes returns all FTS indexes
func (s *Storage) GetFTSIndexes() map[string]*FTSIndexInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*FTSIndexInfo)
	for k, v := range s.ftsIndexes {
		result[k] = v
	}
	return result
}

// HasFTSIndex checks if an FTS index exists
func (s *Storage) HasFTSIndex(tableName, columnName string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, exists := s.ftsIndexes[tableName+"."+columnName]
	return exists
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

// IsDatabaseEncrypted checks if a database at the given path is encrypted
func IsDatabaseEncrypted(path string) bool {
	metaPath := filepath.Join(path, MetaFileName)
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return false
	}
	return crypto.IsEncryptedFile(data)
}

// GetEncryptionSalt reads the salt from an encrypted database
// Note: TXDEF encryption doesn't use salt, so this returns nil for compatibility
func GetEncryptionSalt(path string) ([]byte, error) {
	metaPath := filepath.Join(path, MetaFileName)
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, err
	}

	if !crypto.IsEncryptedFile(data) {
		return nil, fmt.Errorf("database is not encrypted")
	}

	// TXDEF doesn't use salt - return nil for compatibility
	return nil, nil
}
