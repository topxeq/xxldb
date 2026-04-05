// Package storage provides data persistence for xxldb
package storage

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hirochachacha/go-smb2"
)

// SMBConfig holds SMB connection configuration
type SMBConfig struct {
	Host       string
	Port       int
	Share      string // Share name (e.g., "shared")
	Username   string
	Password   string
	Domain     string // Optional domain
	DBPath     string // Path within the share
	Timeout    time.Duration
	MaxRetries int
	RetryDelay time.Duration
}

// SMBFS implements FileSystem using SMB/CIFS with short connections
// Each operation connects, executes, and disconnects
// Use Batch() for multiple operations on a single connection
type SMBFS struct {
	basePath string
	config   SMBConfig
	mu       sync.Mutex
	closed   bool
}

// NewSMBFS creates a new SMB filesystem (short connection mode)
func NewSMBFS(basePath string, config SMBConfig) (*SMBFS, error) {
	if config.Port == 0 {
		config.Port = 445
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = 1 * time.Second
	}

	fs := &SMBFS{
		basePath: basePath,
		config:   config,
	}

	// Test connection on creation
	conn, err := fs.connect()
	if err != nil {
		return nil, err
	}
	fs.disconnect(conn)

	return fs, nil
}

// smbConn holds an SMB connection
type smbConn struct {
	session *smb2.Session
	share   *smb2.Share
	tcpConn net.Conn
}

// connect establishes a new SMB connection
func (fs *SMBFS) connect() (*smbConn, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if fs.closed {
		return nil, fmt.Errorf("filesystem is closed")
	}

	// Build SMB dialer
	d := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     fs.config.Username,
			Password: fs.config.Password,
			Domain:   fs.config.Domain,
		},
	}

	addr := net.JoinHostPort(fs.config.Host, strconv.Itoa(fs.config.Port))

	var tcpConn net.Conn
	var session *smb2.Session
	var share *smb2.Share
	var err error

	for i := 0; i <= fs.config.MaxRetries; i++ {
		// Establish TCP connection
		tcpConn, err = net.DialTimeout("tcp", addr, fs.config.Timeout)
		if err != nil {
			if i < fs.config.MaxRetries {
				time.Sleep(fs.config.RetryDelay)
				continue
			}
			return nil, fmt.Errorf("SMB TCP connection failed: %w", err)
		}

		// Dial SMB session
		session, err = d.Dial(tcpConn)
		if err != nil {
			tcpConn.Close()
			if i < fs.config.MaxRetries {
				time.Sleep(fs.config.RetryDelay)
				continue
			}
			return nil, fmt.Errorf("SMB session failed: %w", err)
		}

		// Mount share
		shareName := fmt.Sprintf("\\\\%s\\%s", fs.config.Host, fs.config.Share)
		share, err = session.Mount(shareName)
		if err != nil {
			session.Logoff()
			tcpConn.Close()
			if i < fs.config.MaxRetries {
				time.Sleep(fs.config.RetryDelay)
				continue
			}
			return nil, fmt.Errorf("SMB mount failed: %w", err)
		}

		return &smbConn{session: session, share: share, tcpConn: tcpConn}, nil
	}

	return nil, fmt.Errorf("SMB connection failed after %d retries", fs.config.MaxRetries)
}

// disconnect closes the SMB connection
func (fs *SMBFS) disconnect(conn *smbConn) {
	if conn != nil {
		if conn.share != nil {
			conn.share.Umount()
		}
		if conn.session != nil {
			conn.session.Logoff()
		}
		if conn.tcpConn != nil {
			conn.tcpConn.Close()
		}
	}
}

// withConn executes an operation with a short-lived connection
func (fs *SMBFS) withConn(fn func(*smb2.Share) error) error {
	conn, err := fs.connect()
	if err != nil {
		return err
	}
	defer fs.disconnect(conn)

	return fn(conn.share)
}

// Close closes the filesystem
func (fs *SMBFS) Close() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.closed = true
	return nil
}

// Batch executes multiple operations within a single connection
func (fs *SMBFS) Batch(fn func(BatchFS) error) error {
	conn, err := fs.connect()
	if err != nil {
		return err
	}
	defer fs.disconnect(conn)

	bfs := &smbBatchFS{share: conn.share, fs: fs}
	return fn(bfs)
}

// smbPath converts a path to SMB format (backslashes)
func (fs *SMBFS) smbPath(path string) string {
	if path == "" || path == "." {
		return ""
	}
	result := strings.ReplaceAll(path, "/", "\\")
	result = strings.TrimPrefix(result, "\\")
	return result
}

// Create creates a new file
func (fs *SMBFS) Create(name string) (File, error) {
	conn, err := fs.connect()
	if err != nil {
		return nil, err
	}

	f, err := conn.share.Create(fs.smbPath(name))
	if err != nil {
		fs.disconnect(conn)
		return nil, err
	}

	return &smbFile{File: f, conn: conn, fs: fs}, nil
}

// Open opens an existing file
func (fs *SMBFS) Open(name string) (File, error) {
	conn, err := fs.connect()
	if err != nil {
		return nil, err
	}

	f, err := conn.share.Open(fs.smbPath(name))
	if err != nil {
		fs.disconnect(conn)
		return nil, err
	}

	return &smbFile{File: f, conn: conn, fs: fs}, nil
}

// OpenFile opens a file with specified flags
func (fs *SMBFS) OpenFile(name string, flag int, perm FileMode) (File, error) {
	conn, err := fs.connect()
	if err != nil {
		return nil, err
	}

	f, err := conn.share.OpenFile(fs.smbPath(name), flag, os.FileMode(perm))
	if err != nil {
		fs.disconnect(conn)
		return nil, err
	}

	return &smbFile{File: f, conn: conn, fs: fs}, nil
}

// MkdirAll creates a directory tree
func (fs *SMBFS) MkdirAll(path string, perm FileMode) error {
	return fs.withConn(func(share *smb2.Share) error {
		parts := strings.Split(strings.Trim(path, "/"), "/")
		current := ""

		for _, part := range parts {
			if part == "" {
				continue
			}
			current = current + "\\" + part
			if err := share.Mkdir(current, os.FileMode(perm)); err != nil {
				if !isSMBExistError(err) {
					if info, err2 := share.Stat(current); err2 == nil && info.IsDir() {
						continue
					}
					return err
				}
			}
		}
		return nil
	})
}

// ReadDir reads directory contents
func (fs *SMBFS) ReadDir(dirname string) ([]DirEntry, error) {
	conn, err := fs.connect()
	if err != nil {
		return nil, err
	}
	defer fs.disconnect(conn)

	infos, err := conn.share.ReadDir(fs.smbPath(dirname))
	if err != nil {
		return nil, err
	}

	result := make([]DirEntry, len(infos))
	for i, info := range infos {
		result[i] = &smbDirEntry{info: info}
	}
	return result, nil
}

// Stat returns file info
func (fs *SMBFS) Stat(name string) (FileInfo, error) {
	conn, err := fs.connect()
	if err != nil {
		return nil, err
	}
	defer fs.disconnect(conn)

	info, err := conn.share.Stat(fs.smbPath(name))
	if err != nil {
		return nil, err
	}
	return &smbFileInfo{FileInfo: info}, nil
}

// Exists checks if a file exists
func (fs *SMBFS) Exists(name string) bool {
	_, err := fs.Stat(name)
	return err == nil
}

// Remove removes a file
func (fs *SMBFS) Remove(name string) error {
	return fs.withConn(func(share *smb2.Share) error {
		return share.Remove(fs.smbPath(name))
	})
}

// Rename renames a file
func (fs *SMBFS) Rename(oldpath, newpath string) error {
	return fs.withConn(func(share *smb2.Share) error {
		return share.Rename(fs.smbPath(oldpath), fs.smbPath(newpath))
	})
}

// ReadFile reads entire file content
func (fs *SMBFS) ReadFile(name string) ([]byte, error) {
	conn, err := fs.connect()
	if err != nil {
		return nil, err
	}
	defer fs.disconnect(conn)

	return conn.share.ReadFile(fs.smbPath(name))
}

// WriteFile writes data to a file
func (fs *SMBFS) WriteFile(name string, data []byte, perm FileMode) error {
	return fs.SafeWriteFile(name, data, perm)
}

// SafeWriteFile writes data safely using temp file + atomic rename
func (fs *SMBFS) SafeWriteFile(path string, data []byte, perm FileMode) error {
	return fs.Batch(func(bfs BatchFS) error {
		tempPath := path + ".tmp." + generateID()

		// Write to temp file
		smbTempPath := fs.smbPath(tempPath)
		if err := fs.withConn(func(share *smb2.Share) error {
			return share.WriteFile(smbTempPath, data, os.FileMode(perm))
		}); err != nil {
			return fmt.Errorf("write failed: %w", err)
		}

		// Atomic rename
		if err := bfs.Rename(tempPath, path); err != nil {
			bfs.Remove(tempPath)
			return fmt.Errorf("rename failed: %w", err)
		}

		return nil
	})
}

// Join joins path elements
func (fs *SMBFS) Join(elem ...string) string {
	return strings.Join(elem, "/")
}

// Dir returns the directory part of a path
func (fs *SMBFS) Dir(path string) string {
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return path[:idx]
	}
	return "."
}

// Base returns the base name of a path
func (fs *SMBFS) Base(path string) string {
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return path[idx+1:]
	}
	return path
}

// Abs returns an absolute path
func (fs *SMBFS) Abs(path string) (string, error) {
	if strings.HasPrefix(path, "/") {
		return path, nil
	}
	return fs.Join(fs.basePath, path), nil
}

// TempFile creates a temporary file
func (fs *SMBFS) TempFile(dir, pattern string) (File, error) {
	name := pattern + generateID()
	path := fs.Join(dir, name)
	return fs.Create(path)
}

// RemoveAll removes a directory tree
func (fs *SMBFS) RemoveAll(path string) error {
	return fs.withConn(func(share *smb2.Share) error {
		return share.RemoveAll(fs.smbPath(path))
	})
}

// Walk walks the file tree
func (fs *SMBFS) Walk(root string, walkFn WalkFunc) error {
	return fs.withConn(func(share *smb2.Share) error {
		return fs.walkRecursive(share, fs.smbPath(root), root, walkFn)
	})
}

// walkRecursive recursively walks the file tree
func (fs *SMBFS) walkRecursive(share *smb2.Share, smbPath, displayPath string, walkFn WalkFunc) error {
	info, err := share.Stat(smbPath)
	if err != nil {
		return walkFn(displayPath, nil, err)
	}

	if !info.IsDir() {
		return walkFn(displayPath, &smbFileInfo{FileInfo: info}, nil)
	}

	if err := walkFn(displayPath, &smbFileInfo{FileInfo: info}, nil); err != nil {
		return err
	}

	entries, err := share.ReadDir(smbPath)
	if err != nil {
		return walkFn(displayPath, &smbFileInfo{FileInfo: info}, err)
	}

	for _, entry := range entries {
		childSMBPath := smbPath + "\\" + entry.Name()
		childDisplayPath := displayPath + "/" + entry.Name()

		if err := fs.walkRecursive(share, childSMBPath, childDisplayPath, walkFn); err != nil {
			if err == filepath.SkipDir {
				continue
			}
			return err
		}
	}

	return nil
}

// isSMBExistError checks if error is "already exists"
func isSMBExistError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "exists") ||
		strings.Contains(msg, "STATUS_OBJECT_NAME_COLLISION") ||
		strings.Contains(msg, "STATUS_OBJECT_NAME_EXISTS")
}

// smbFile wraps smb2.File with connection management
type smbFile struct {
	*smb2.File
	conn *smbConn
	fs   *SMBFS
}

func (f *smbFile) Close() error {
	err := f.File.Close()
	f.fs.disconnect(f.conn)
	return err
}

func (f *smbFile) Stat() (FileInfo, error) {
	info, err := f.File.Stat()
	if err != nil {
		return nil, err
	}
	return &smbFileInfo{FileInfo: info}, nil
}

func (f *smbFile) Readdir(n int) ([]FileInfo, error) {
	infos, err := f.File.Readdir(n)
	if err != nil {
		return nil, err
	}
	result := make([]FileInfo, len(infos))
	for i, info := range infos {
		result[i] = &smbFileInfo{FileInfo: info}
	}
	return result, nil
}

// smbFileInfo wraps os.FileInfo
type smbFileInfo struct {
	os.FileInfo
}

func (i *smbFileInfo) Mode() FileMode {
	return FileMode(i.FileInfo.Mode())
}

// smbDirEntry wraps os.FileInfo as DirEntry
type smbDirEntry struct {
	info os.FileInfo
}

func (e *smbDirEntry) Name() string {
	return e.info.Name()
}

func (e *smbDirEntry) IsDir() bool {
	return e.info.IsDir()
}

func (e *smbDirEntry) Type() FileMode {
	return FileMode(e.info.Mode().Type())
}

func (e *smbDirEntry) Info() (FileInfo, error) {
	return &smbFileInfo{FileInfo: e.info}, nil
}

// WithContext returns a context-aware wrapper
func (fs *SMBFS) WithContext(ctx context.Context) *SMBFSContext {
	return &SMBFSContext{fs: fs, ctx: ctx}
}

// SMBFSContext wraps SMBFS with context
type SMBFSContext struct {
	fs  *SMBFS
	ctx context.Context
}

// BatchFS implementation for smbBatchFS

type smbBatchFS struct {
	share *smb2.Share
	fs    *SMBFS
}

func (b *smbBatchFS) Create(name string) (File, error) {
	f, err := b.share.Create(b.fs.smbPath(name))
	if err != nil {
		return nil, err
	}
	return &smbBatchFile{File: f}, nil
}

func (b *smbBatchFS) Open(name string) (File, error) {
	f, err := b.share.Open(b.fs.smbPath(name))
	if err != nil {
		return nil, err
	}
	return &smbBatchFile{File: f}, nil
}

func (b *smbBatchFS) OpenFile(name string, flag int, perm FileMode) (File, error) {
	f, err := b.share.OpenFile(b.fs.smbPath(name), flag, os.FileMode(perm))
	if err != nil {
		return nil, err
	}
	return &smbBatchFile{File: f}, nil
}

func (b *smbBatchFS) MkdirAll(path string, perm FileMode) error {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	current := ""
	for _, part := range parts {
		if part == "" {
			continue
		}
		current = current + "\\" + part
		if err := b.share.Mkdir(current, os.FileMode(perm)); err != nil {
			if !isSMBExistError(err) {
				if info, err2 := b.share.Stat(current); err2 == nil && info.IsDir() {
					continue
				}
				return err
			}
		}
	}
	return nil
}

func (b *smbBatchFS) ReadDir(dirname string) ([]DirEntry, error) {
	infos, err := b.share.ReadDir(b.fs.smbPath(dirname))
	if err != nil {
		return nil, err
	}
	result := make([]DirEntry, len(infos))
	for i, info := range infos {
		result[i] = &smbDirEntry{info: info}
	}
	return result, nil
}

func (b *smbBatchFS) Stat(name string) (FileInfo, error) {
	info, err := b.share.Stat(b.fs.smbPath(name))
	if err != nil {
		return nil, err
	}
	return &smbFileInfo{FileInfo: info}, nil
}

func (b *smbBatchFS) Exists(name string) bool {
	_, err := b.share.Stat(b.fs.smbPath(name))
	return err == nil
}

func (b *smbBatchFS) Remove(name string) error {
	return b.share.Remove(b.fs.smbPath(name))
}

func (b *smbBatchFS) Rename(oldpath, newpath string) error {
	return b.share.Rename(b.fs.smbPath(oldpath), b.fs.smbPath(newpath))
}

func (b *smbBatchFS) ReadFile(name string) ([]byte, error) {
	return b.share.ReadFile(b.fs.smbPath(name))
}

func (b *smbBatchFS) WriteFile(name string, data []byte, perm FileMode) error {
	return b.share.WriteFile(b.fs.smbPath(name), data, os.FileMode(perm))
}

func (b *smbBatchFS) Join(elem ...string) string {
	return strings.Join(elem, "/")
}

func (b *smbBatchFS) Dir(path string) string {
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return path[:idx]
	}
	return "."
}

func (b *smbBatchFS) Base(path string) string {
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return path[idx+1:]
	}
	return path
}

func (b *smbBatchFS) RemoveAll(path string) error {
	return b.share.RemoveAll(b.fs.smbPath(path))
}

// smbBatchFile is a file within a batch context (doesn't close connection)
type smbBatchFile struct {
	*smb2.File
}

func (f *smbBatchFile) Stat() (FileInfo, error) {
	info, err := f.File.Stat()
	if err != nil {
		return nil, err
	}
	return &smbFileInfo{FileInfo: info}, nil
}

func (f *smbBatchFile) Readdir(n int) ([]FileInfo, error) {
	infos, err := f.File.Readdir(n)
	if err != nil {
		return nil, err
	}
	result := make([]FileInfo, len(infos))
	for i, info := range infos {
		result[i] = &smbFileInfo{FileInfo: info}
	}
	return result, nil
}

// Ensure SMBFS implements interfaces
var _ FileSystem = (*SMBFS)(nil)
var _ BatchFileSystem = (*SMBFS)(nil)
