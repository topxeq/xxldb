// Package storage provides data persistence for xxldb
package storage

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// TransferStatus represents the status of a transfer
type TransferStatus string

const (
	TransferStatusPending   TransferStatus = "pending"
	TransferStatusUploading TransferStatus = "uploading"
	TransferStatusComplete  TransferStatus = "complete"
	TransferStatusFailed    TransferStatus = "failed"
)

// TransferInfo tracks file transfer progress for recovery
type TransferInfo struct {
	ID          string         `json:"id"`
	SourcePath  string         `json:"source_path,omitempty"`
	TargetPath  string         `json:"target_path"`
	TempPath    string         `json:"temp_path"`
	TotalSize   int64          `json:"total_size"`
	Transferred int64          `json:"transferred"`
	Checksum    string         `json:"checksum,omitempty"`
	Status      TransferStatus `json:"status"`
	StartedAt   time.Time      `json:"started_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	Error       string         `json:"error,omitempty"`
	Retries     int            `json:"retries"`
}

// TransferState tracks file transfer progress for recovery
type TransferState struct {
	mu        sync.RWMutex
	transfers map[string]*TransferInfo
	stateFile string
	fs        FileSystem
}

// NewTransferState creates a transfer state tracker
func NewTransferState(fs FileSystem, stateFile string) *TransferState {
	ts := &TransferState{
		transfers: make(map[string]*TransferInfo),
		stateFile: stateFile,
		fs:        fs,
	}
	ts.load()
	return ts
}

// load loads transfer state from file
func (ts *TransferState) load() {
	if ts.fs == nil || ts.stateFile == "" {
		return
	}

	data, err := ts.fs.ReadFile(ts.stateFile)
	if err != nil {
		return // No existing state file
	}

	var transfers []*TransferInfo
	if err := json.Unmarshal(data, &transfers); err != nil {
		return
	}

	for _, t := range transfers {
		ts.transfers[t.ID] = t
	}
}

// save saves transfer state to file
func (ts *TransferState) save() {
	if ts.fs == nil || ts.stateFile == "" {
		return
	}

	transfers := make([]*TransferInfo, 0, len(ts.transfers))
	for _, t := range ts.transfers {
		transfers = append(transfers, t)
	}

	data, err := json.MarshalIndent(transfers, "", "  ")
	if err != nil {
		return
	}

	ts.fs.WriteFile(ts.stateFile, data, PermFile)
}

// BeginTransfer starts tracking a new transfer
func (ts *TransferState) BeginTransfer(id, source, target, temp string, totalSize int64) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.transfers[id] = &TransferInfo{
		ID:          id,
		SourcePath:  source,
		TargetPath:  target,
		TempPath:    temp,
		TotalSize:   totalSize,
		Status:      TransferStatusPending,
		StartedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	ts.save()
}

// UpdateProgress updates transfer progress
func (ts *TransferState) UpdateProgress(id string, transferred int64) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if info, ok := ts.transfers[id]; ok {
		info.Transferred = transferred
		info.Status = TransferStatusUploading
		info.UpdatedAt = time.Now()
		// Don't save on every progress update for performance
	}
}

// CompleteTransfer marks transfer as complete
func (ts *TransferState) CompleteTransfer(id string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if info, ok := ts.transfers[id]; ok {
		info.Status = TransferStatusComplete
		info.UpdatedAt = time.Now()
		delete(ts.transfers, id) // Remove completed transfers
		ts.save()
	}
}

// FailTransfer marks transfer as failed
func (ts *TransferState) FailTransfer(id string, err error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if info, ok := ts.transfers[id]; ok {
		info.Status = TransferStatusFailed
		info.Error = err.Error()
		info.UpdatedAt = time.Now()
		ts.save()
	}
}

// GetTransfer returns transfer info by ID
func (ts *TransferState) GetTransfer(id string) *TransferInfo {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.transfers[id]
}

// GetPendingTransfers returns incomplete transfers for recovery
func (ts *TransferState) GetPendingTransfers() []*TransferInfo {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	var pending []*TransferInfo
	for _, info := range ts.transfers {
		if info.Status != TransferStatusComplete {
			pending = append(pending, info)
		}
	}
	return pending
}

// CleanupIncomplete removes temporary files from failed/incomplete transfers
func (ts *TransferState) CleanupIncomplete(fs FileSystem) []error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	var errors []error
	for id, info := range ts.transfers {
		if info.Status == TransferStatusFailed || info.Status == TransferStatusPending {
			// Remove temporary file
			if info.TempPath != "" {
				if err := fs.Remove(info.TempPath); err != nil {
					errors = append(errors, fmt.Errorf("failed to remove temp file %s: %w", info.TempPath, err))
				}
			}
			delete(ts.transfers, id)
		}
	}
	ts.save()
	return errors
}

// CleanupOldTransfers removes transfers older than the specified duration
func (ts *TransferState) CleanupOldTransfers(maxAge time.Duration) int {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	cleaned := 0
	now := time.Now()
	for id, info := range ts.transfers {
		if now.Sub(info.StartedAt) > maxAge {
			delete(ts.transfers, id)
			cleaned++
		}
	}
	if cleaned > 0 {
		ts.save()
	}
	return cleaned
}

// IncrementRetry increments retry count for a transfer
func (ts *TransferState) IncrementRetry(id string) int {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if info, ok := ts.transfers[id]; ok {
		info.Retries++
		info.UpdatedAt = time.Now()
		ts.save()
		return info.Retries
	}
	return 0
}

// CanRetry checks if a transfer can be retried
func (ts *TransferState) CanRetry(id string, maxRetries int) bool {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	if info, ok := ts.transfers[id]; ok {
		return info.Retries < maxRetries
	}
	return false
}

// ResumeInfo contains information needed to resume a transfer
type ResumeInfo struct {
	Offset   int64
	TempPath string
}

// GetResumeInfo returns resume information for a pending transfer
func (ts *TransferState) GetResumeInfo(id string) *ResumeInfo {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	if info, ok := ts.transfers[id]; ok {
		if info.Status == TransferStatusUploading && info.Transferred > 0 {
			return &ResumeInfo{
				Offset:   info.Transferred,
				TempPath: info.TempPath,
			}
		}
	}
	return nil
}

// Count returns the number of tracked transfers
func (ts *TransferState) Count() int {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return len(ts.transfers)
}

// CountByStatus returns the number of transfers with a specific status
func (ts *TransferState) CountByStatus(status TransferStatus) int {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	count := 0
	for _, info := range ts.transfers {
		if info.Status == status {
			count++
		}
	}
	return count
}
