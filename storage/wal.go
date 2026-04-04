// Package storage provides WAL (Write-Ahead Logging) support
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

// WAL record types
const (
	WALTypeBegin WALType = iota
	WALTypeInsert
	WALTypeUpdate
	WALTypeDelete
	WALTypeCreateTable
	WALTypeDropTable
	WALTypeCommit
	WALTypeRollback
	WALTypeCheckpoint
)

// WALType represents the type of WAL record
type WALType uint8

// String returns the string representation
func (t WALType) String() string {
	switch t {
	case WALTypeBegin:
		return "BEGIN"
	case WALTypeInsert:
		return "INSERT"
	case WALTypeUpdate:
		return "UPDATE"
	case WALTypeDelete:
		return "DELETE"
	case WALTypeCreateTable:
		return "CREATE_TABLE"
	case WALTypeDropTable:
		return "DROP_TABLE"
	case WALTypeCommit:
		return "COMMIT"
	case WALTypeRollback:
		return "ROLLBACK"
	case WALTypeCheckpoint:
		return "CHECKPOINT"
	default:
		return "UNKNOWN"
	}
}

// WALRecord represents a WAL record
type WALRecord struct {
	LSN       uint64      `json:"lsn"`
	TxnID     uint64      `json:"txn_id"`
	Type      WALType     `json:"type"`
	TableID   uint64      `json:"table_id"`
	RowID     uint64      `json:"row_id"`
	Timestamp int64       `json:"timestamp"`
	Data      interface{} `json:"data,omitempty"`
	OldData   interface{} `json:"old_data,omitempty"`
}

// WALHeader is the header for each WAL record on disk
type WALHeader struct {
	LSN      uint64
	Length   uint32
	Checksum uint32
}

// WAL manages Write-Ahead Logging
type WAL struct {
	mu       sync.Mutex
	file     *os.File
	path     string
	lsn      uint64
	enabled  bool
}

// NewWAL creates a new WAL instance
func NewWAL(path string) (*WAL, error) {
	wal := &WAL{
		path:    path,
		lsn:     0,
		enabled: path != "",
	}

	if !wal.enabled {
		return wal, nil
	}

	// Open or create WAL file
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open WAL file: %w", err)
	}

	wal.file = file

	// Read the last LSN
	if err := wal.readLastLSN(); err != nil {
		// Non-fatal, start from 0
		wal.lsn = 0
	}

	return wal, nil
}

// readLastLSN reads the last LSN from the WAL file
func (w *WAL) readLastLSN() error {
	if w.file == nil {
		return nil
	}

	// Seek to beginning
	if _, err := w.file.Seek(0, 0); err != nil {
		return err
	}

	var maxLSN uint64

	for {
		header := WALHeader{}
		if err := binary.Read(w.file, binary.LittleEndian, &header); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		// Skip record data
		if _, err := w.file.Seek(int64(header.Length), 1); err != nil {
			break
		}

		if header.LSN > maxLSN {
			maxLSN = header.LSN
		}
	}

	w.lsn = maxLSN
	return nil
}

// Write writes a record to the WAL
func (w *WAL) Write(record WALRecord) (uint64, error) {
	if !w.enabled || w.file == nil {
		return 0, nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Assign LSN
	w.lsn++
	record.LSN = w.lsn
	record.Timestamp = time.Now().UnixNano()

	// Serialize record
	data, err := json.Marshal(record)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal WAL record: %w", err)
	}

	// Create header
	header := WALHeader{
		LSN:      record.LSN,
		Length:   uint32(len(data)),
		Checksum: crc32.ChecksumIEEE(data),
	}

	// Write header
	if err := binary.Write(w.file, binary.LittleEndian, &header); err != nil {
		return 0, fmt.Errorf("failed to write WAL header: %w", err)
	}

	// Write data
	if _, err := w.file.Write(data); err != nil {
		return 0, fmt.Errorf("failed to write WAL data: %w", err)
	}

	// Sync to disk
	if err := w.file.Sync(); err != nil {
		return 0, fmt.Errorf("failed to sync WAL: %w", err)
	}

	return record.LSN, nil
}

// ReadAll reads all records from the WAL
func (w *WAL) ReadAll() ([]WALRecord, error) {
	if !w.enabled || w.file == nil {
		return nil, nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Seek to beginning
	if _, err := w.file.Seek(0, 0); err != nil {
		return nil, err
	}

	var records []WALRecord

	for {
		header := WALHeader{}
		if err := binary.Read(w.file, binary.LittleEndian, &header); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		// Read record data
		data := make([]byte, header.Length)
		if _, err := io.ReadFull(w.file, data); err != nil {
			return nil, err
		}

		// Verify checksum
		if crc32.ChecksumIEEE(data) != header.Checksum {
			// Corrupted record, stop reading
			break
		}

		// Unmarshal record
		var record WALRecord
		if err := json.Unmarshal(data, &record); err != nil {
			return nil, fmt.Errorf("failed to unmarshal WAL record: %w", err)
		}

		records = append(records, record)
	}

	return records, nil
}

// Truncate truncates the WAL file
func (w *WAL) Truncate() error {
	if !w.enabled || w.file == nil {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.file.Truncate(0); err != nil {
		return err
	}

	if _, err := w.file.Seek(0, 0); err != nil {
		return err
	}

	w.lsn = 0
	return nil
}

// Close closes the WAL file
func (w *WAL) Close() error {
	if w.file == nil {
		return nil
	}
	return w.file.Close()
}

// CurrentLSN returns the current LSN
func (w *WAL) CurrentLSN() uint64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.lsn
}

// initWAL initializes the WAL for storage
func (s *Storage) initWAL() error {
	if !s.enabled {
		return nil
	}

	walPath := filepath.Join(s.path, WALFileName)
	_, err := NewWAL(walPath)
	if err != nil {
		return err
	}

	// We'll use a simpler approach: just open the file
	walFile, err := os.OpenFile(walPath, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		return err
	}

	s.walFile = walFile
	return nil
}

// writeWAL writes a record to the WAL
func (s *Storage) writeWAL(record WALRecord) error {
	if !s.enabled || s.walFile == nil {
		return nil
	}

	s.walSeq++
	record.LSN = s.walSeq
	record.Timestamp = time.Now().UnixNano()

	// Serialize record
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal WAL record: %w", err)
	}

	// Write header
	header := WALHeader{
		LSN:      record.LSN,
		Length:   uint32(len(data)),
		Checksum: crc32.ChecksumIEEE(data),
	}

	if err := binary.Write(s.walFile, binary.LittleEndian, &header); err != nil {
		return err
	}

	if _, err := s.walFile.Write(data); err != nil {
		return err
	}

	return s.walFile.Sync()
}

// recoverFromWAL recovers the database from WAL
func (s *Storage) recoverFromWAL() error {
	if !s.enabled {
		return nil
	}

	walPath := filepath.Join(s.path, WALFileName)

	wal, err := NewWAL(walPath)
	if err != nil {
		return err
	}
	defer wal.Close()

	records, err := wal.ReadAll()
	if err != nil {
		return err
	}

	// Replay records
	for _, record := range records {
		switch record.Type {
		case WALTypeCreateTable:
			if info, ok := record.Data.(*TableInfo); ok {
				s.tables[info.Name] = info
				// Initialize row storage for this table
				if s.rowData == nil {
					s.rowData = make(map[string][][]types.Value)
				}
				if s.rowIDs == nil {
					s.rowIDs = make(map[string][]uint64)
				}
				s.rowData[info.Name] = make([][]types.Value, 0)
				s.rowIDs[info.Name] = make([]uint64, 0)
			} else {
				// Try to unmarshal from map
				if dataMap, ok := record.Data.(map[string]interface{}); ok {
					info := &TableInfo{}
					if name, ok := dataMap["name"].(string); ok {
						info.Name = name
					}
					if id, ok := dataMap["id"].(float64); ok {
						info.ID = uint64(id)
					}
					s.tables[info.Name] = info
					if s.rowData == nil {
						s.rowData = make(map[string][][]types.Value)
					}
					if s.rowIDs == nil {
						s.rowIDs = make(map[string][]uint64)
					}
					s.rowData[info.Name] = make([][]types.Value, 0)
					s.rowIDs[info.Name] = make([]uint64, 0)
				}
			}
		case WALTypeDropTable:
			if name, ok := record.Data.(string); ok {
				delete(s.tables, name)
				delete(s.rowData, name)
				delete(s.rowIDs, name)
			}
		case WALTypeInsert:
			// Find table name from table ID
			var tableName string
			for name, info := range s.tables {
				if info.ID == record.TableID {
					tableName = name
					break
				}
			}
			if tableName != "" {
				// Convert data to []types.Value
				if vals, ok := record.Data.([]interface{}); ok {
					row := make([]types.Value, len(vals))
					for i, v := range vals {
						row[i] = interfaceToValue(v)
					}
					s.rowData[tableName] = append(s.rowData[tableName], row)
					s.rowIDs[tableName] = append(s.rowIDs[tableName], record.RowID)
				}
			}
		case WALTypeUpdate:
			// Find table name from table ID
			var tableName string
			for name, info := range s.tables {
				if info.ID == record.TableID {
					tableName = name
					break
				}
			}
			if tableName != "" {
				// Find and update the row
				for i, rowID := range s.rowIDs[tableName] {
					if rowID == record.RowID {
						if vals, ok := record.Data.([]interface{}); ok {
							row := make([]types.Value, len(vals))
							for j, v := range vals {
								row[j] = interfaceToValue(v)
							}
							s.rowData[tableName][i] = row
						}
						break
					}
				}
			}
		case WALTypeDelete:
			// Find table name from table ID
			var tableName string
			for name, info := range s.tables {
				if info.ID == record.TableID {
					tableName = name
					break
				}
			}
			if tableName != "" {
				// Find and delete the row
				for i, rowID := range s.rowIDs[tableName] {
					if rowID == record.RowID {
						s.rowData[tableName] = append(s.rowData[tableName][:i], s.rowData[tableName][i+1:]...)
						s.rowIDs[tableName] = append(s.rowIDs[tableName][:i], s.rowIDs[tableName][i+1:]...)
						break
					}
				}
			}
		}

		if record.LSN > s.walSeq {
			s.walSeq = record.LSN
		}
	}

	return nil
}

// interfaceToValue converts an interface{} to types.Value
func interfaceToValue(v interface{}) types.Value {
	if v == nil {
		return types.NewNullValue()
	}

	// Check if it's a map (from JSON unmarshaling of types.Value)
	if m, ok := v.(map[string]interface{}); ok {
		// This might be a serialized types.Value
		if data, hasData := m["data"]; hasData {
			if isNull, hasIsNull := m["is_null"]; hasIsNull {
				if isNull == true {
					return types.NewNullValue()
				}
			}

			if typeStr, hasType := m["type"]; hasType {
				// Use the type info to properly convert
				switch typeStr {
				case "INT", "SEQ":
					if f, ok := data.(float64); ok {
						return types.NewIntValue(int64(f))
					}
				case "FLOAT":
					if f, ok := data.(float64); ok {
						return types.NewFloatValue(f)
					}
				case "VARCHAR", "CHAR", "TEXT":
					if s, ok := data.(string); ok {
						return types.NewStringValue(s)
					}
				case "BLOB":
					if s, ok := data.(string); ok {
						return types.NewBlobValue([]byte(s))
					}
					if b, ok := data.([]interface{}); ok {
						bytes := make([]byte, len(b))
						for i, bb := range b {
							if f, ok := bb.(float64); ok {
								bytes[i] = byte(f)
							}
						}
						return types.NewBlobValue(bytes)
					}
				}
			}

			// Fallback: try to convert data directly
			return types.NewValue(data)
		}
	}

	switch val := v.(type) {
	case string:
		return types.NewStringValue(val)
	case float64:
		// JSON numbers are always float64
		// Try to determine if it's an integer
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
	case []interface{}:
		// Could be a nested array
		return types.NewNullValue()
	default:
		return types.NewStringValue(fmt.Sprintf("%v", v))
	}
}

// Checkpoint creates a checkpoint
func (s *Storage) Checkpoint() error {
	if !s.enabled {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Save metadata
	if err := s.saveMetadata(); err != nil {
		return err
	}

	// Write checkpoint record
	if err := s.writeWAL(WALRecord{
		Type: WALTypeCheckpoint,
	}); err != nil {
		return err
	}

	// Truncate WAL
	if s.walFile != nil {
		if err := s.walFile.Truncate(0); err != nil {
			return err
		}
		if _, err := s.walFile.Seek(0, 0); err != nil {
			return err
		}
		s.walSeq = 0
	}

	s.lastCheckpoint = time.Now()

	return nil
}

// BeginTxn begins a transaction
func (s *Storage) BeginTxn() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.walSeq++
	txnID := s.walSeq

	_ = txnID // We'll use this for transaction tracking

	return txnID
}

// CommitTxn commits a transaction
func (s *Storage) CommitTxn(txnID uint64) error {
	return s.writeWAL(WALRecord{
		TxnID: txnID,
		Type:  WALTypeCommit,
	})
}

// RollbackTxn rolls back a transaction
func (s *Storage) RollbackTxn(txnID uint64) error {
	return s.writeWAL(WALRecord{
		TxnID: txnID,
		Type:  WALTypeRollback,
	})
}

// Transaction represents a database transaction
type Transaction struct {
	id      uint64
	storage *Storage
	changes []WALRecord
}

// ID returns the transaction ID
func (t *Transaction) ID() uint64 {
	return t.id
}

// Commit commits the transaction
func (t *Transaction) Commit() error {
	return t.storage.CommitTxn(t.id)
}

// Rollback rolls back the transaction
func (t *Transaction) Rollback() error {
	return t.storage.RollbackTxn(t.id)
}
