// Package storage provides data persistence for xxldb
package storage

import (
	"os"
	"path/filepath"
	"strings"
)

// LocalFS implements FileSystem using the local os package
type LocalFS struct {
	basePath string
}

// NewLocalFS creates a new local filesystem
func NewLocalFS(basePath string) *LocalFS {
	return &LocalFS{basePath: basePath}
}

// Close closes the filesystem (no-op for local filesystem)
func (fs *LocalFS) Close() error {
	return nil
}

// Create creates a new file
func (fs *LocalFS) Create(name string) (File, error) {
	f, err := os.Create(name)
	if err != nil {
		return nil, err
	}
	return &localFile{File: f}, nil
}

// Open opens an existing file
func (fs *LocalFS) Open(name string) (File, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	return &localFile{File: f}, nil
}

// OpenFile opens a file with specified flags and permissions
func (fs *LocalFS) OpenFile(name string, flag int, perm FileMode) (File, error) {
	f, err := os.OpenFile(name, flag, os.FileMode(perm))
	if err != nil {
		return nil, err
	}
	return &localFile{File: f}, nil
}

// MkdirAll creates a directory tree
func (fs *LocalFS) MkdirAll(path string, perm FileMode) error {
	return os.MkdirAll(path, os.FileMode(perm))
}

// ReadDir reads directory contents
func (fs *LocalFS) ReadDir(dirname string) ([]DirEntry, error) {
	entries, err := os.ReadDir(dirname)
	if err != nil {
		return nil, err
	}
	result := make([]DirEntry, len(entries))
	for i, e := range entries {
		result[i] = &localDirEntry{DirEntry: e}
	}
	return result, nil
}

// Stat returns file info
func (fs *LocalFS) Stat(name string) (FileInfo, error) {
	info, err := os.Stat(name)
	if err != nil {
		return nil, err
	}
	return &localFileInfo{FileInfo: info}, nil
}

// Exists checks if a file exists
func (fs *LocalFS) Exists(name string) bool {
	_, err := os.Stat(name)
	return err == nil
}

// Remove removes a file
func (fs *LocalFS) Remove(name string) error {
	return os.Remove(name)
}

// Rename renames a file
func (fs *LocalFS) Rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

// ReadFile reads entire file content
func (fs *LocalFS) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

// WriteFile writes data to a file
func (fs *LocalFS) WriteFile(name string, data []byte, perm FileMode) error {
	return os.WriteFile(name, data, os.FileMode(perm))
}

// Join joins path elements
func (fs *LocalFS) Join(elem ...string) string {
	return filepath.Join(elem...)
}

// Dir returns the directory part of a path
func (fs *LocalFS) Dir(path string) string {
	return filepath.Dir(path)
}

// Base returns the base name of a path
func (fs *LocalFS) Base(path string) string {
	return filepath.Base(path)
}

// Abs returns an absolute path
func (fs *LocalFS) Abs(path string) (string, error) {
	return filepath.Abs(path)
}

// TempFile creates a temporary file
func (fs *LocalFS) TempFile(dir, pattern string) (File, error) {
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return nil, err
	}
	return &localFile{File: f}, nil
}

// RemoveAll removes a directory tree
func (fs *LocalFS) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

// Walk walks the file tree
func (fs *LocalFS) Walk(root string, walkFn WalkFunc) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return walkFn(path, nil, err)
		}
		return walkFn(path, &localFileInfo{FileInfo: info}, nil)
	})
}

// localFile wraps os.File to implement File interface
type localFile struct {
	*os.File
}

func (f *localFile) Stat() (FileInfo, error) {
	info, err := f.File.Stat()
	if err != nil {
		return nil, err
	}
	return &localFileInfo{FileInfo: info}, nil
}

func (f *localFile) Readdir(n int) ([]FileInfo, error) {
	infos, err := f.File.Readdir(n)
	if err != nil {
		return nil, err
	}
	result := make([]FileInfo, len(infos))
	for i, info := range infos {
		result[i] = &localFileInfo{FileInfo: info}
	}
	return result, nil
}

// localFileInfo wraps os.FileInfo to implement FileInfo interface
type localFileInfo struct {
	os.FileInfo
}

func (i *localFileInfo) Mode() FileMode {
	return FileMode(i.FileInfo.Mode())
}

// localDirEntry wraps os.DirEntry to implement DirEntry interface
type localDirEntry struct {
	os.DirEntry
}

func (e *localDirEntry) Type() FileMode {
	return FileMode(e.DirEntry.Type())
}

func (e *localDirEntry) Info() (FileInfo, error) {
	info, err := e.DirEntry.Info()
	if err != nil {
		return nil, err
	}
	return &localFileInfo{FileInfo: info}, nil
}

// Ensure LocalFS implements FileSystem interface
var _ FileSystem = (*LocalFS)(nil)

// SafeWriteFile writes data safely using temp file + atomic rename
func (fs *LocalFS) SafeWriteFile(path string, data []byte, perm FileMode) error {
	// Create temp file in same directory for atomic rename
	dir := fs.Dir(path)
	tempFile, err := fs.TempFile(dir, ".tmp."+fs.Base(path)+".")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()

	// Ensure cleanup on failure
	success := false
	defer func() {
		if !success {
			fs.Remove(tempPath)
		}
	}()

	// Write data
	if _, err := tempFile.Write(data); err != nil {
		tempFile.Close()
		return err
	}

	// Sync to ensure data is persisted
	if err := tempFile.Sync(); err != nil {
		tempFile.Close()
		return err
	}
	tempFile.Close()

	// Set permissions
	if err := os.Chmod(tempPath, os.FileMode(perm)); err != nil {
		return err
	}

	// Atomic rename
	if err := fs.Rename(tempPath, path); err != nil {
		return err
	}

	success = true
	return nil
}

// IsTempFile checks if a file is a temporary file (starts with .tmp.)
func IsTempFile(name string) bool {
	base := filepath.Base(name)
	return strings.HasPrefix(base, ".tmp.")
}
