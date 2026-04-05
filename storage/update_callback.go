// Package storage provides UpdateRowsWithCallback for FTS support
package storage

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/topxeq/xxldb/types"
)

// UpdateRowsWithCallback updates rows matching a condition and calls a callback with the updated data
// The callback receives the row ID and the new row data, useful for maintaining FTS indexes
func (s *Storage) UpdateRowsWithCallback(tableName string, updateFunc func([]types.Value) map[int]types.Value, condition func([]types.Value) bool, callback func(uint64, []types.Value)) (int64, error) {
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

		// Handle CHAR type: pad with spaces to fixed length
		if col.Type == types.TypeChar && col.Length > 0 && !val.IsNull {
			strVal := val.ToString()
			if utf8.RuneCountInString(strVal) > col.Length {
				return val, fmt.Errorf("value too long for column '%s': max length %d, got %d", col.Name, col.Length, utf8.RuneCountInString(strVal))
			}
			// Pad with trailing spaces to fixed length
			if len(strVal) < col.Length {
				padded := strVal + strings.Repeat(" ", col.Length-len(strVal))
				return types.NewStringValue(padded), nil
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

			// Call callback after update
			if callback != nil {
				callback(s.rowIDs[tableName][i], rows[i])
			}

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
