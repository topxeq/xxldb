// Package storage provides data persistence for xxldb
package storage

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/studio-b12/gowebdav"
)

// WebDAVConfig holds WebDAV connection configuration
type WebDAVConfig struct {
	URL        string // WebDAV server URL (e.g., "https://cloud.example.com/dav")
	Username   string
	Password   string
	DBPath     string // Path on WebDAV server
	Timeout    time.Duration
	MaxRetries int
	RetryDelay time.Duration
}

// WebDAVFS implements FileSystem using WebDAV (short connection mode)
// Each operation is a separate HTTP request
// Use Batch() to indicate multiple operations will be performed (for optimization hints)
type WebDAVFS struct {
	client   *gowebdav.Client
	basePath string
	config   WebDAVConfig
	mu       sync.Mutex
	closed   bool
}

// NewWebDAVFS creates a new WebDAV filesystem
func NewWebDAVFS(basePath string, config WebDAVConfig) (*WebDAVFS, error) {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = 1 * time.Second
	}

	client := gowebdav.NewClient(config.URL, config.Username, config.Password)

	fs := &WebDAVFS{
		client:   client,
		basePath: basePath,
		config:   config,
	}

	// Test connection
	if err := fs.testConnection(); err != nil {
		return nil, fmt.Errorf("WebDAV connection test failed: %w", err)
	}

	return fs, nil
}

// testConnection tests the WebDAV connection
func (fs *WebDAVFS) testConnection() error {
	_, err := fs.client.ReadDir(fs.webdavPath(""))
	if err != nil {
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "Not Found") {
			return fs.client.MkdirAll(fs.webdavPath(""), 0755)
		}
		return err
	}
	return nil
}

// webdavPath returns the full path on WebDAV server
func (fs *WebDAVFS) webdavPath(name string) string {
	if fs.basePath == "" {
		return "/" + name
	}
	return path.Join(fs.basePath, name)
}

// withRetry executes an operation with retry logic
func (fs *WebDAVFS) withRetry(op func() error) error {
	var lastErr error
	for i := 0; i <= fs.config.MaxRetries; i++ {
		err := op()
		if err == nil {
			return nil
		}
		lastErr = err
		if !fs.isRetryable(err) {
			return err
		}
		if i < fs.config.MaxRetries {
			time.Sleep(fs.config.RetryDelay)
		}
	}
	return lastErr
}

// isRetryable checks if an error is retryable
func (fs *WebDAVFS) isRetryable(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection") ||
		strings.Contains(errStr, "503") ||
		strings.Contains(errStr, "502") ||
		strings.Contains(errStr, "network")
}

// Close closes the filesystem (no persistent connection)
func (fs *WebDAVFS) Close() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.closed = true
	return nil
}

// Batch executes multiple operations
// For WebDAV, this is primarily for API consistency since each operation is already independent
// However, it can be useful for grouping related operations
func (fs *WebDAVFS) Batch(fn func(BatchFS) error) error {
	bfs := &webdavBatchFS{fs: fs}
	return fn(bfs)
}

// Create creates a new file
func (fs *WebDAVFS) Create(name string) (File, error) {
	webdavPath := fs.webdavPath(name)
	tempPath := webdavPath + ".tmp"

	return &webdavFile{
		fs:        fs,
		path:      webdavPath,
		tempPath:  tempPath,
		writeMode: true,
		buffer:    &bytes.Buffer{},
	}, nil
}

// Open opens an existing file
func (fs *WebDAVFS) Open(name string) (File, error) {
	webdavPath := fs.webdavPath(name)

	var data []byte
	err := fs.withRetry(func() error {
		var err error
		data, err = fs.client.Read(webdavPath)
		return err
	})
	if err != nil {
		return nil, err
	}

	return &webdavFile{
		fs:        fs,
		path:      webdavPath,
		writeMode: false,
		reader:    bytes.NewReader(data),
		size:      int64(len(data)),
	}, nil
}

// OpenFile opens a file with specified flags and permissions
func (fs *WebDAVFS) OpenFile(name string, flag int, perm FileMode) (File, error) {
	webdavPath := fs.webdavPath(name)
	exists := fs.Exists(name)

	if flag&os.O_CREATE != 0 && !exists {
		return fs.Create(name)
	}

	if flag&os.O_APPEND != 0 {
		var existingData []byte
		err := fs.withRetry(func() error {
			var err error
			existingData, err = fs.client.Read(webdavPath)
			return err
		})
		if err != nil {
			return nil, err
		}

		return &webdavFile{
			fs:        fs,
			path:      webdavPath,
			tempPath:  webdavPath + ".tmp",
			writeMode: true,
			buffer:    bytes.NewBuffer(existingData),
		}, nil
	}

	return fs.Open(name)
}

// MkdirAll creates a directory tree
func (fs *WebDAVFS) MkdirAll(path string, perm FileMode) error {
	webdavPath := fs.webdavPath(path)
	return fs.withRetry(func() error {
		return fs.client.MkdirAll(webdavPath, os.FileMode(perm))
	})
}

// ReadDir reads directory contents
func (fs *WebDAVFS) ReadDir(dirname string) ([]DirEntry, error) {
	webdavPath := fs.webdavPath(dirname)

	var files []os.FileInfo
	err := fs.withRetry(func() error {
		var err error
		files, err = fs.client.ReadDir(webdavPath)
		return err
	})
	if err != nil {
		return nil, err
	}

	result := make([]DirEntry, len(files))
	for i, f := range files {
		result[i] = &webdavDirEntry{info: f}
	}
	return result, nil
}

// Stat returns file info
func (fs *WebDAVFS) Stat(name string) (FileInfo, error) {
	webdavPath := fs.webdavPath(name)

	var info os.FileInfo
	err := fs.withRetry(func() error {
		var err error
		info, err = fs.client.Stat(webdavPath)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &webdavFileInfo{FileInfo: info}, nil
}

// Exists checks if a file exists
func (fs *WebDAVFS) Exists(name string) bool {
	_, err := fs.Stat(name)
	return err == nil
}

// Remove removes a file
func (fs *WebDAVFS) Remove(name string) error {
	webdavPath := fs.webdavPath(name)
	return fs.withRetry(func() error {
		return fs.client.Remove(webdavPath)
	})
}

// Rename renames a file
func (fs *WebDAVFS) Rename(oldpath, newpath string) error {
	oldWebDAV := fs.webdavPath(oldpath)
	newWebDAV := fs.webdavPath(newpath)
	return fs.withRetry(func() error {
		return fs.client.Rename(oldWebDAV, newWebDAV, true)
	})
}

// ReadFile reads entire file content
func (fs *WebDAVFS) ReadFile(name string) ([]byte, error) {
	webdavPath := fs.webdavPath(name)

	var data []byte
	err := fs.withRetry(func() error {
		var err error
		data, err = fs.client.Read(webdavPath)
		return err
	})
	if err != nil {
		return nil, err
	}
	return data, nil
}

// WriteFile writes data to a file
func (fs *WebDAVFS) WriteFile(name string, data []byte, perm FileMode) error {
	return fs.SafeWriteFile(name, data, perm)
}

// SafeWriteFile writes data safely using temp file + atomic rename
func (fs *WebDAVFS) SafeWriteFile(path string, data []byte, perm FileMode) error {
	webdavPath := fs.webdavPath(path)
	tempPath := webdavPath + ".tmp." + generateID()

	// Write to temp file
	err := fs.withRetry(func() error {
		return fs.client.Write(tempPath, data, os.FileMode(perm))
	})
	if err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	// Ensure cleanup on failure
	success := false
	defer func() {
		if !success {
			fs.client.Remove(tempPath)
		}
	}()

	// Rename to final path
	err = fs.withRetry(func() error {
		return fs.client.Rename(tempPath, webdavPath, true)
	})
	if err != nil {
		return fmt.Errorf("rename failed: %w", err)
	}

	success = true
	return nil
}

// Join joins path elements
func (fs *WebDAVFS) Join(elem ...string) string {
	return path.Join(elem...)
}

// Dir returns the directory part of a path
func (fs *WebDAVFS) Dir(path string) string {
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return path[:idx]
	}
	return "."
}

// Base returns the base name of a path
func (fs *WebDAVFS) Base(path string) string {
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return path[idx+1:]
	}
	return path
}

// Abs returns an absolute path
func (fs *WebDAVFS) Abs(path string) (string, error) {
	if strings.HasPrefix(path, "/") {
		return path, nil
	}
	return fs.Join(fs.basePath, path), nil
}

// TempFile creates a temporary file
func (fs *WebDAVFS) TempFile(dir, pattern string) (File, error) {
	name := pattern + generateID()
	path := fs.Join(dir, name)
	return fs.Create(path)
}

// RemoveAll removes a directory tree
func (fs *WebDAVFS) RemoveAll(path string) error {
	webdavPath := fs.webdavPath(path)
	return fs.withRetry(func() error {
		return fs.client.RemoveAll(webdavPath)
	})
}

// Walk walks the file tree
func (fs *WebDAVFS) Walk(root string, walkFn WalkFunc) error {
	webdavRoot := fs.webdavPath(root)
	return fs.walkDir(webdavRoot, walkFn)
}

// walkDir recursively walks a directory
func (fs *WebDAVFS) walkDir(dir string, walkFn WalkFunc) error {
	files, err := fs.client.ReadDir(dir)
	if err != nil {
		return walkFn(dir, nil, err)
	}

	for _, f := range files {
		fullPath := path.Join(dir, f.Name())

		info := &webdavFileInfo{FileInfo: f}
		if err := walkFn(fullPath, info, nil); err != nil {
			return err
		}

		if f.IsDir() {
			if err := fs.walkDir(fullPath, walkFn); err != nil {
				return err
			}
		}
	}
	return nil
}

// webdavFile implements File interface for WebDAV
type webdavFile struct {
	fs        *WebDAVFS
	path      string
	tempPath  string
	writeMode bool
	buffer    *bytes.Buffer
	reader    *bytes.Reader
	size      int64
	offset    int64
	closed    bool
}

func (f *webdavFile) Name() string {
	return f.path
}

func (f *webdavFile) Read(p []byte) (n int, err error) {
	if f.writeMode || f.reader == nil {
		return 0, io.EOF
	}
	return f.reader.Read(p)
}

func (f *webdavFile) Write(p []byte) (n int, err error) {
	if !f.writeMode {
		return 0, fmt.Errorf("file not open for writing")
	}
	return f.buffer.Write(p)
}

func (f *webdavFile) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true

	if f.writeMode && f.buffer != nil {
		data := f.buffer.Bytes()
		err := f.fs.withRetry(func() error {
			return f.fs.client.Write(f.tempPath, data, 0644)
		})
		if err != nil {
			return err
		}

		err = f.fs.withRetry(func() error {
			return f.fs.client.Rename(f.tempPath, f.path, true)
		})
		if err != nil {
			f.fs.client.Remove(f.tempPath)
			return err
		}
	}

	return nil
}

func (f *webdavFile) Seek(offset int64, whence int) (int64, error) {
	if f.writeMode {
		return 0, fmt.Errorf("seek not supported in write mode")
	}
	if f.reader == nil {
		return 0, io.EOF
	}
	return f.reader.Seek(offset, whence)
}

func (f *webdavFile) Stat() (FileInfo, error) {
	return f.fs.Stat(f.path)
}

func (f *webdavFile) Sync() error {
	return nil
}

func (f *webdavFile) Truncate(size int64) error {
	if !f.writeMode {
		return fmt.Errorf("file not open for writing")
	}
	if f.buffer == nil {
		return nil
	}

	data := f.buffer.Bytes()
	if size > int64(len(data)) {
		padding := make([]byte, size-int64(len(data)))
		f.buffer.Write(padding)
	} else {
		f.buffer = bytes.NewBuffer(data[:size])
	}
	return nil
}

func (f *webdavFile) Readdir(n int) ([]FileInfo, error) {
	return nil, fmt.Errorf("Readdir not supported on WebDAV file")
}

// webdavFileInfo wraps os.FileInfo
type webdavFileInfo struct {
	os.FileInfo
}

func (i *webdavFileInfo) Mode() FileMode {
	return FileMode(i.FileInfo.Mode())
}

// webdavDirEntry wraps directory entry
type webdavDirEntry struct {
	info os.FileInfo
}

func (e *webdavDirEntry) Name() string {
	return e.info.Name()
}

func (e *webdavDirEntry) IsDir() bool {
	return e.info.IsDir()
}

func (e *webdavDirEntry) Type() FileMode {
	return FileMode(e.info.Mode())
}

func (e *webdavDirEntry) Info() (FileInfo, error) {
	return &webdavFileInfo{FileInfo: e.info}, nil
}

// BatchFS implementation for webdavBatchFS

type webdavBatchFS struct {
	fs *WebDAVFS
}

func (b *webdavBatchFS) Create(name string) (File, error) {
	return b.fs.Create(name)
}

func (b *webdavBatchFS) Open(name string) (File, error) {
	return b.fs.Open(name)
}

func (b *webdavBatchFS) OpenFile(name string, flag int, perm FileMode) (File, error) {
	return b.fs.OpenFile(name, flag, perm)
}

func (b *webdavBatchFS) MkdirAll(path string, perm FileMode) error {
	return b.fs.MkdirAll(path, perm)
}

func (b *webdavBatchFS) ReadDir(dirname string) ([]DirEntry, error) {
	return b.fs.ReadDir(dirname)
}

func (b *webdavBatchFS) Stat(name string) (FileInfo, error) {
	return b.fs.Stat(name)
}

func (b *webdavBatchFS) Exists(name string) bool {
	return b.fs.Exists(name)
}

func (b *webdavBatchFS) Remove(name string) error {
	return b.fs.Remove(name)
}

func (b *webdavBatchFS) Rename(oldpath, newpath string) error {
	return b.fs.Rename(oldpath, newpath)
}

func (b *webdavBatchFS) ReadFile(name string) ([]byte, error) {
	return b.fs.ReadFile(name)
}

func (b *webdavBatchFS) WriteFile(name string, data []byte, perm FileMode) error {
	return b.fs.WriteFile(name, data, perm)
}

func (b *webdavBatchFS) Join(elem ...string) string {
	return b.fs.Join(elem...)
}

func (b *webdavBatchFS) Dir(path string) string {
	return b.fs.Dir(path)
}

func (b *webdavBatchFS) Base(path string) string {
	return b.fs.Base(path)
}

func (b *webdavBatchFS) RemoveAll(path string) error {
	return b.fs.RemoveAll(path)
}

// Ensure WebDAVFS implements interfaces
var _ FileSystem = (*WebDAVFS)(nil)
var _ BatchFileSystem = (*WebDAVFS)(nil)
