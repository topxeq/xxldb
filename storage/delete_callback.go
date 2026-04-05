// Package storage provides DeleteRowsWithCallback for FTS support
package storage

import (
	"fmt"
	"time"

	"github.com/topxeq/xxldb/types"
)

// DeleteRowsWithCallback deletes rows matching a condition and calls a callback for each deleted row
// The callback receives the row ID and row data, useful for maintaining FTS indexes
func (s *Storage) DeleteRowsWithCallback(tableName string, condition func([]types.Value) bool, callback func(uint64, []types.Value)) (int64, error) {
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

			// Call callback before deleting
			if callback != nil {
				callback(rowIDs[i], row)
			}

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
