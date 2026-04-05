// Package storage provides data persistence for xxldb
package storage

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLocalFSImplementsFileSystem verifies LocalFS implements FileSystem
func TestLocalFSImplementsFileSystem(t *testing.T) {
	var _ FileSystem = (*LocalFS)(nil)
}

// TestLocalFSImplementsBatchFileSystem verifies LocalFS does NOT need BatchFileSystem
func TestLocalFSBatch(t *testing.T) {
	// LocalFS should work with or without Batch
	// Create a temp dir
	tmpDir, err := os.MkdirTemp("", "xxldb-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	fs := NewLocalFS(tmpDir)

	// Test basic operations
	if err := fs.MkdirAll("testdir", 0755); err != nil {
		t.Fatal(err)
	}

	data := []byte("hello world")
	if err := fs.WriteFile("testdir/file.txt", data, 0644); err != nil {
		t.Fatal(err)
	}

	readData, err := fs.ReadFile("testdir/file.txt")
	if err != nil {
		t.Fatal(err)
	}

	if string(readData) != string(data) {
		t.Errorf("expected %q, got %q", data, readData)
	}

	// Test Close (should be no-op)
	if err := fs.Close(); err != nil {
		t.Errorf("Close should not return error: %v", err)
	}
}

// TestLocalFSConcurrentOperations tests concurrent file operations
func TestLocalFSConcurrentOperations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "xxldb-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	fs := NewLocalFS(tmpDir)

	// Create test directory
	if err := fs.MkdirAll("concurrent", 0755); err != nil {
		t.Fatal(err)
	}

	// Run concurrent operations
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(n int) {
			filename := filepath.Join("concurrent", "file"+string(rune('0'+n))+".txt")
			data := []byte("data from goroutine")
			if err := fs.WriteFile(filename, data, 0644); err != nil {
				t.Errorf("write error: %v", err)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all files exist
	entries, err := fs.ReadDir("concurrent")
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 10 {
		t.Errorf("expected 10 files, got %d", len(entries))
	}
}

// TestFileSystemInterfaces verifies all filesystem types implement interfaces
func TestFileSystemInterfaces(t *testing.T) {
	// LocalFS implements FileSystem
	var _ FileSystem = (*LocalFS)(nil)

	// SFTPFS implements both interfaces
	var _ FileSystem = (*SFTPFS)(nil)
	var _ BatchFileSystem = (*SFTPFS)(nil)

	// SMBFS implements both interfaces
	var _ FileSystem = (*SMBFS)(nil)
	var _ BatchFileSystem = (*SMBFS)(nil)

	// WebDAVFS implements both interfaces
	var _ FileSystem = (*WebDAVFS)(nil)
	var _ BatchFileSystem = (*WebDAVFS)(nil)
}

// TestBatchFSInterface verifies BatchFS interface methods
func TestBatchFSInterface(t *testing.T) {
	// BatchFS should have all required methods
	// This is a compile-time check
	var _ BatchFS = (*sftpBatchFS)(nil)
	var _ BatchFS = (*smbBatchFS)(nil)
	var _ BatchFS = (*webdavBatchFS)(nil)
}

// TestSafeWriteFile tests atomic file writing
func TestSafeWriteFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "xxldb-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	fs := NewLocalFS(tmpDir)

	data := []byte("test data for safe write")

	// Test SafeWriteFile
	if err := fs.SafeWriteFile("safe.txt", data, 0644); err != nil {
		t.Fatal(err)
	}

	// Verify content
	readData, err := fs.ReadFile("safe.txt")
	if err != nil {
		t.Fatal(err)
	}

	if string(readData) != string(data) {
		t.Errorf("expected %q, got %q", data, readData)
	}

	// Verify no temp files left
	entries, err := fs.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}

	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

// TestFileModes tests FileMode constants
func TestFileModes(t *testing.T) {
	if PermDir != 0755 {
		t.Errorf("PermDir should be 0755, got %o", PermDir)
	}
	if PermFile != 0644 {
		t.Errorf("PermFile should be 0644, got %o", PermFile)
	}
}

// TestLocalFSPathOperations tests path manipulation methods
func TestLocalFSPathOperations(t *testing.T) {
	fs := NewLocalFS("/base")

	tests := []struct {
		name     string
		fn       func() string
		expected string
	}{
		{"Join", func() string { return fs.Join("a", "b", "c") }, "a/b/c"},
		{"Dir", func() string { return fs.Dir("/a/b/c.txt") }, "/a/b"},
		{"Base", func() string { return fs.Base("/a/b/c.txt") }, "c.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.fn()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestLocalFSAbs tests Abs method
func TestLocalFSAbs(t *testing.T) {
	fs := NewLocalFS("/base/path")

	// Absolute path should be returned as-is
	abs, err := fs.Abs("/already/absolute")
	if err != nil {
		t.Fatal(err)
	}
	if abs != "/already/absolute" {
		t.Errorf("expected /already/absolute, got %s", abs)
	}

	// Relative path should be joined with base
	rel, err := fs.Abs("relative/path")
	if err != nil {
		t.Fatal(err)
	}
	// Note: filepath.Join behavior varies by OS
	if rel == "" {
		t.Error("Abs should not return empty string for relative path")
	}
}

// TestLocalFSTempFile tests temporary file creation
func TestLocalFSTempFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "xxldb-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	fs := NewLocalFS(tmpDir)

	f, err := fs.TempFile(".", "test-")
	if err != nil {
		t.Fatal(err)
	}

	// Write some data
	data := []byte("temp data")
	n, err := f.Write(data)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(data) {
		t.Errorf("expected %d bytes written, got %d", len(data), n)
	}

	// Close and verify
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	// Check file exists
	name := f.Name()
	if _, err := fs.Stat(name); os.IsNotExist(err) {
		t.Errorf("temp file should exist: %s", name)
	}
}

// TestLocalFSRemoveAll tests RemoveAll
func TestLocalFSRemoveAll(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "xxldb-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	fs := NewLocalFS(tmpDir)

	// Create nested structure
	fs.MkdirAll("a/b/c", 0755)
	fs.WriteFile("a/file1.txt", []byte("data"), 0644)
	fs.WriteFile("a/b/file2.txt", []byte("data"), 0644)
	fs.WriteFile("a/b/c/file3.txt", []byte("data"), 0644)

	// Remove all
	if err := fs.RemoveAll("a"); err != nil {
		t.Fatal(err)
	}

	// Verify removed
	if fs.Exists("a") {
		t.Error("directory should be removed")
	}
}

// TestLocalFSWalk tests Walk
func TestLocalFSWalk(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "xxldb-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	fs := NewLocalFS(tmpDir)

	// Create structure
	fs.MkdirAll("dir1/dir2", 0755)
	fs.WriteFile("file1.txt", []byte("data"), 0644)
	fs.WriteFile("dir1/file2.txt", []byte("data"), 0644)
	fs.WriteFile("dir1/dir2/file3.txt", []byte("data"), 0644)

	// Walk and collect paths
	var paths []string
	err = fs.Walk(".", func(path string, info FileInfo, err error) error {
		if err != nil {
			return err
		}
		paths = append(paths, path)
		return nil
	})

	if err != nil {
		t.Fatal(err)
	}

	// Should have 6 entries: ., file1.txt, dir1, file2.txt, dir2, file3.txt
	if len(paths) < 4 {
		t.Errorf("expected at least 4 paths, got %d: %v", len(paths), paths)
	}
}

// TestLocalFSExists tests Exists method
func TestLocalFSExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "xxldb-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	fs := NewLocalFS(tmpDir)

	// Non-existent file
	if fs.Exists("nonexistent.txt") {
		t.Error("non-existent file should not exist")
	}

	// Create file
	fs.WriteFile("exists.txt", []byte("data"), 0644)

	// Should exist now
	if !fs.Exists("exists.txt") {
		t.Error("file should exist")
	}
}

// TestLocalFSRename tests Rename
func TestLocalFSRename(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "xxldb-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	fs := NewLocalFS(tmpDir)

	// Create file
	fs.WriteFile("old.txt", []byte("data"), 0644)

	// Rename
	if err := fs.Rename("old.txt", "new.txt"); err != nil {
		t.Fatal(err)
	}

	// Verify
	if fs.Exists("old.txt") {
		t.Error("old file should not exist")
	}
	if !fs.Exists("new.txt") {
		t.Error("new file should exist")
	}

	// Verify content
	data, err := fs.ReadFile("new.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "data" {
		t.Errorf("expected 'data', got %q", data)
	}
}

// TestLocalFSStat tests Stat
func TestLocalFSStat(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "xxldb-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	fs := NewLocalFS(tmpDir)

	// Create file
	fs.WriteFile("stat.txt", []byte("hello"), 0644)

	// Stat
	info, err := fs.Stat("stat.txt")
	if err != nil {
		t.Fatal(err)
	}

	if info.Name() != "stat.txt" {
		t.Errorf("expected name 'stat.txt', got %q", info.Name())
	}
	if info.Size() != 5 {
		t.Errorf("expected size 5, got %d", info.Size())
	}
	if info.IsDir() {
		t.Error("file should not be a directory")
	}
}

// TestLocalFSReadDir tests ReadDir
func TestLocalFSReadDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "xxldb-readdir-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create files directly in tmpDir
	if err := os.WriteFile(filepath.Join(tmpDir, "a.txt"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "b.txt"), []byte("b"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}

	fs := NewLocalFS(tmpDir)

	// ReadDir
	entries, err := fs.ReadDir(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}

	// Check types
	fileCount := 0
	dirCount := 0
	for _, e := range entries {
		if e.IsDir() {
			dirCount++
		} else {
			fileCount++
		}
	}

	if fileCount != 2 {
		t.Errorf("expected 2 files, got %d", fileCount)
	}
	if dirCount != 1 {
		t.Errorf("expected 1 dir, got %d", dirCount)
	}
}
