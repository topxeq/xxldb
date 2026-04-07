// Package storage provides migration from V1 to V2 storage format
package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/topxeq/xxldb/types"
)

// Storage version constants
const (
	VersionV1 = 2 // Old format: all data in meta file
	VersionV2 = 3 // New format: per-table page files
)

// Migration handles data migration from V1 to V2
type Migration struct {
	storage *Storage
}

// NewMigration creates a new migration handler
func NewMigration(storage *Storage) *Migration {
	return &Migration{storage: storage}
}

// NeedsMigration checks if the database needs migration from V1 to V2
func (m *Migration) NeedsMigration() bool {
	if !m.storage.enabled || m.storage.fs == nil {
		return false
	}

	metaPath := m.storage.fs.Join(m.storage.path, MetaFileName)
	data, err := m.storage.fs.ReadFile(metaPath)
	if err != nil {
		return false
	}

	// Check for encrypted data
	if len(data) > 8 && string(data[:8]) == "XXLDBENC" {
		// Need to decrypt first - migration will be handled after password is set
		return false
	}

	var meta struct {
		Version uint64                  `json:"version"`
		Rows    map[string]interface{} `json:"rows,omitempty"`
	}

	if err := json.Unmarshal(data, &meta); err != nil {
		return false
	}

	// V1 with rows data needs migration
	return meta.Version <= VersionV1 && meta.Rows != nil && len(meta.Rows) > 0
}

// Migrate performs the migration from V1 to V2
func (m *Migration) Migrate() error {
	m.storage.mu.Lock()
	defer m.storage.mu.Unlock()

	fmt.Println("Migrating database from V1 to V2 format...")

	// Read the old metadata file
	metaPath := m.storage.fs.Join(m.storage.path, MetaFileName)
	data, err := m.storage.fs.ReadFile(metaPath)
	if err != nil {
		return fmt.Errorf("failed to read metadata: %w", err)
	}

	// Check if encrypted
	if m.storage.encrypted && m.storage.encryptor != nil {
		data, err = m.storage.encryptor.DecryptFile(data)
		if err != nil {
			return fmt.Errorf("failed to decrypt metadata: %w", err)
		}
	}

	// Parse old format
	var oldMeta struct {
		Version   uint64                 `json:"version"`
		Tables    map[string]*TableInfo  `json:"tables"`
		Sequences map[string]int64       `json:"sequences"`
		NextID    uint64                 `json:"next_id"`
		Auth      map[string]interface{} `json:"auth,omitempty"`
		Encrypted bool                   `json:"encrypted,omitempty"`
		Rows      map[string]interface{} `json:"rows,omitempty"`
	}

	if err := json.Unmarshal(data, &oldMeta); err != nil {
		return fmt.Errorf("failed to parse metadata: %w", err)
	}

	// Backup old metadata
	backupPath := metaPath + ".v1.bak"
	if err := m.storage.fs.WriteFile(backupPath, data, 0644); err != nil {
		return fmt.Errorf("failed to backup old metadata: %w", err)
	}
	fmt.Printf("  Backed up old metadata to: %s\n", backupPath)

	// Migrate each table's data
	for tableName, tableInfo := range oldMeta.Tables {
		fmt.Printf("  Migrating table: %s\n", tableName)

		// Create table data file
		tf, err := CreateTableFile(m.storage, tableName, tableInfo.ID, tableInfo.Columns)
		if err != nil {
			return fmt.Errorf("failed to create table file for %s: %w", tableName, err)
		}

		// Migrate rows
		rowsData, hasRows := oldMeta.Rows[tableName]
		if hasRows {
			rows, err := m.parseRowsData(rowsData, tableInfo)
			if err != nil {
				tf.Close()
				return fmt.Errorf("failed to parse rows for %s: %w", tableName, err)
			}

			for _, row := range rows {
				if _, err := tf.InsertRow(row); err != nil {
					tf.Close()
					return fmt.Errorf("failed to insert row into %s: %w", tableName, err)
				}
			}

			fmt.Printf("    Migrated %d rows\n", len(rows))
		}

		// Flush and close
		if err := tf.Flush(); err != nil {
			tf.Close()
			return fmt.Errorf("failed to flush table %s: %w", tableName, err)
		}
		tf.Close()
	}

	// Update metadata version and remove rows
	oldMeta.Version = VersionV2
	oldMeta.Rows = nil // Remove rows from metadata

	// Save new metadata
	newData, err := json.Marshal(oldMeta)
	if err != nil {
		return fmt.Errorf("failed to marshal new metadata: %w", err)
	}

	// Re-encrypt if needed
	if m.storage.encrypted && m.storage.encryptor != nil {
		encrypted, err := m.storage.encryptor.EncryptFile(newData)
		if err != nil {
			return fmt.Errorf("failed to encrypt new metadata: %w", err)
		}
		newData = encrypted
	}

	if err := m.storage.fs.WriteFile(metaPath, newData, 0644); err != nil {
		return fmt.Errorf("failed to write new metadata: %w", err)
	}

	fmt.Println("Migration completed successfully!")
	return nil
}

// parseRowsData parses row data from V1 format
func (m *Migration) parseRowsData(rowsData interface{}, tableInfo *TableInfo) ([][]types.Value, error) {
	var rows [][]types.Value

	dataSlice, ok := rowsData.([]interface{})
	if !ok {
		return nil, fmt.Errorf("rows data is not an array")
	}

	for _, rowData := range dataSlice {
		rowSlice, ok := rowData.([]interface{})
		if !ok {
			continue
		}

		if len(rowSlice) < 2 {
			continue
		}

		// Format: [rowID, [col1, col2, ...]]
		valuesSlice, ok := rowSlice[1].([]interface{})
		if !ok {
			continue
		}

		row := make([]types.Value, len(valuesSlice))
		for i, v := range valuesSlice {
			row[i] = m.parseValue(v, tableInfo, i)
		}
		rows = append(rows, row)
	}

	return rows, nil
}

// parseValue parses a value from V1 format
func (m *Migration) parseValue(v interface{}, tableInfo *TableInfo, colIndex int) types.Value {
	if v == nil {
		return types.NewNullValue()
	}

	// Check for blob ref format: [1, id, size]
	if arr, ok := v.([]interface{}); ok && len(arr) == 3 {
		if marker, ok := arr[0].(float64); ok && marker == 1 {
			if id, ok := arr[1].(float64); ok {
				if size, ok := arr[2].(float64); ok {
					return types.NewBlobRefValue(uint64(id), int64(size))
				}
			}
		}
	}

	// Get column type if available
	var colType types.DataType
	if tableInfo != nil && colIndex < len(tableInfo.Columns) {
		colType = tableInfo.Columns[colIndex].Type
	}

	// Convert based on type
	switch val := v.(type) {
	case string:
		return types.NewStringValue(val)
	case float64:
		// JSON numbers are always float64
		if colType == types.TypeInt || colType == types.TypeSeq {
			return types.NewIntValue(int64(val))
		}
		if colType == types.TypeFloat {
			return types.NewFloatValue(val)
		}
		// Auto-detect: if it's a whole number, treat as int
		if val == float64(int64(val)) {
			return types.NewIntValue(int64(val))
		}
		return types.NewFloatValue(val)
	case int64:
		return types.NewIntValue(val)
	case int:
		return types.NewIntValue(int64(val))
	case bool:
		return types.NewBoolValue(val)
	case []byte:
		return types.NewBlobValue(val)
	default:
		return types.NewStringValue(fmt.Sprintf("%v", v))
	}
}

// AutoMigrate automatically detects and performs migration if needed
func (s *Storage) AutoMigrate() error {
	migration := NewMigration(s)
	if !migration.NeedsMigration() {
		return nil
	}

	return migration.Migrate()
}

// GetTableFile returns a TableFile for the given table, creating if necessary
func (s *Storage) GetTableFile(tableName string) (*TableFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if already loaded
	if tf, exists := s.tableFiles[tableName]; exists {
		return tf, nil
	}

	// Get table info
	tableInfo, exists := s.tables[tableName]
	if !exists {
		return nil, fmt.Errorf("table %s does not exist", tableName)
	}

	// Check if data file exists
	dataPath := filepath.Join(s.path, DataDirName, tableName+TableFileExt)
	if _, err := os.Stat(dataPath); err == nil {
		// Open existing file
		tf, err := OpenTableFile(s, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to open table file: %w", err)
		}
		s.tableFiles[tableName] = tf
		return tf, nil
	}

	// Create new file
	tf, err := CreateTableFile(s, tableName, tableInfo.ID, tableInfo.Columns)
	if err != nil {
		return nil, fmt.Errorf("failed to create table file: %w", err)
	}
	s.tableFiles[tableName] = tf
	return tf, nil
}

// CloseTableFile closes a table file
func (s *Storage) CloseTableFile(tableName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tf, exists := s.tableFiles[tableName]
	if !exists {
		return nil
	}

	delete(s.tableFiles, tableName)
	return tf.Close()
}

// CloseAllTableFiles closes all table files
func (s *Storage) CloseAllTableFiles() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var lastErr error
	for name, tf := range s.tableFiles {
		if err := tf.Close(); err != nil {
			lastErr = err
		}
		delete(s.tableFiles, name)
	}
	return lastErr
}

// FlushAll flushes all table files and the page cache
func (s *Storage) FlushAll() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Flush page cache
	if s.pageCache != nil {
		if err := s.pageCache.Flush(func(page *PageV2) error {
			// Find which table file owns this page
			for _, tf := range s.tableFiles {
				if page.Header.PageID < uint64(tf.header.PageCount) {
					return tf.writePageToFile(page)
				}
			}
			return fmt.Errorf("cannot find owner for page %d", page.Header.PageID)
		}); err != nil {
			return err
		}
	}

	// Flush all table files
	for _, tf := range s.tableFiles {
		if err := tf.Flush(); err != nil {
			return err
		}
	}

	return nil
}

// GetPageCacheStats returns page cache statistics
func (s *Storage) GetPageCacheStats() map[string]interface{} {
	if s.pageCache == nil {
		return nil
	}
	return s.pageCache.Stats()
}