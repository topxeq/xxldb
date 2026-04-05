// Package storage provides data persistence for xxldb
// +build !nosftp

package storage

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SFTPConfig holds SFTP connection configuration
type SFTPConfig struct {
	Host           string
	Port           int
	Username       string
	Password       string
	PrivateKey     string // Path to private key file
	PrivateKeyPEM  string // Private key content (PEM format)
	Passphrase     string // Key passphrase
	Timeout        time.Duration
	MaxRetries     int
	RetryDelay     time.Duration
	ConnectTimeout time.Duration
}

// SFTPFS implements FileSystem using SFTP with short connections
// Each operation connects, executes, and disconnects
// Use Batch() for multiple operations on a single connection
type SFTPFS struct {
	basePath string
	config   SFTPConfig
	mu       sync.Mutex
	closed   bool
}

// NewSFTPFS creates a new SFTP filesystem (short connection mode)
func NewSFTPFS(basePath string, config SFTPConfig) (*SFTPFS, error) {
	if config.Port == 0 {
		config.Port = 22
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.ConnectTimeout == 0 {
		config.ConnectTimeout = 10 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = 1 * time.Second
	}

	fs := &SFTPFS{
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

// connect establishes a new SSH+SFTP connection
func (fs *SFTPFS) connect() (*sftpConn, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if fs.closed {
		return nil, fmt.Errorf("filesystem is closed")
	}

	// Build SSH config
	sshConfig := &ssh.ClientConfig{
		User:            fs.config.Username,
		Auth:            []ssh.AuthMethod{},
		Timeout:         fs.config.ConnectTimeout,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// Add authentication methods
	if fs.config.Password != "" {
		sshConfig.Auth = append(sshConfig.Auth, ssh.Password(fs.config.Password))
	}

	if fs.config.PrivateKeyPEM != "" || fs.config.PrivateKey != "" {
		signer, err := fs.loadPrivateKey()
		if err != nil {
			return nil, fmt.Errorf("failed to load private key: %w", err)
		}
		sshConfig.Auth = append(sshConfig.Auth, ssh.PublicKeys(signer))
	}

	if len(sshConfig.Auth) == 0 {
		return nil, fmt.Errorf("no authentication method provided")
	}

	// Connect with retry
	addr := net.JoinHostPort(fs.config.Host, strconv.Itoa(fs.config.Port))

	var sshClient *ssh.Client
	var sftpClient *sftp.Client
	var err error

	for i := 0; i <= fs.config.MaxRetries; i++ {
		sshClient, err = ssh.Dial("tcp", addr, sshConfig)
		if err == nil {
			sftpClient, err = sftp.NewClient(sshClient)
			if err == nil {
				return &sftpConn{ssh: sshClient, sftp: sftpClient}, nil
			}
			sshClient.Close()
		}
		if i < fs.config.MaxRetries {
			time.Sleep(fs.config.RetryDelay)
		}
	}

	return nil, fmt.Errorf("SFTP connection failed after %d retries: %w", fs.config.MaxRetries, err)
}

// disconnect closes the connection
func (fs *SFTPFS) disconnect(conn *sftpConn) {
	if conn != nil {
		if conn.sftp != nil {
			conn.sftp.Close()
		}
		if conn.ssh != nil {
			conn.ssh.Close()
		}
	}
}

// sftpConn holds an SSH+SFTP connection
type sftpConn struct {
	ssh  *ssh.Client
	sftp *sftp.Client
}

// loadPrivateKey loads and parses the private key
func (fs *SFTPFS) loadPrivateKey() (ssh.Signer, error) {
	var keyData []byte
	var err error

	if fs.config.PrivateKeyPEM != "" {
		keyData = []byte(fs.config.PrivateKeyPEM)
	} else if fs.config.PrivateKey != "" {
		keyPath := fs.config.PrivateKey
		if strings.HasPrefix(keyPath, "~/") {
			home, _ := os.UserHomeDir()
			keyPath = filepath.Join(home, keyPath[2:])
		}
		keyData, err = os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read private key file: %w", err)
		}
	} else {
		return nil, fmt.Errorf("no private key provided")
	}

	if fs.config.Passphrase != "" {
		return ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(fs.config.Passphrase))
	}
	return ssh.ParsePrivateKey(keyData)
}

// withConn executes an operation with a short-lived connection
func (fs *SFTPFS) withConn(fn func(*sftp.Client) error) error {
	conn, err := fs.connect()
	if err != nil {
		return err
	}
	defer fs.disconnect(conn)

	return fn(conn.sftp)
}

// Close closes the filesystem
func (fs *SFTPFS) Close() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.closed = true
	return nil
}

// Batch executes multiple operations within a single connection
func (fs *SFTPFS) Batch(fn func(BatchFS) error) error {
	conn, err := fs.connect()
	if err != nil {
		return err
	}
	defer fs.disconnect(conn)

	bfs := &sftpBatchFS{conn: conn, fs: fs}
	return fn(bfs)
}

// sftpBatchFS implements BatchFS for batch operations
type sftpBatchFS struct {
	conn *sftpConn
	fs   *SFTPFS
}

// Create creates a new file
func (fs *SFTPFS) Create(name string) (File, error) {
	conn, err := fs.connect()
	if err != nil {
		return nil, err
	}

	f, err := conn.sftp.Create(name)
	if err != nil {
		fs.disconnect(conn)
		return nil, err
	}

	return &sftpFile{File: f, conn: conn, fs: fs}, nil
}

// Open opens an existing file
func (fs *SFTPFS) Open(name string) (File, error) {
	conn, err := fs.connect()
	if err != nil {
		return nil, err
	}

	f, err := conn.sftp.Open(name)
	if err != nil {
		fs.disconnect(conn)
		return nil, err
	}

	return &sftpFile{File: f, conn: conn, fs: fs}, nil
}

// OpenFile opens a file with specified flags
func (fs *SFTPFS) OpenFile(name string, flag int, perm FileMode) (File, error) {
	conn, err := fs.connect()
	if err != nil {
		return nil, err
	}

	f, err := conn.sftp.OpenFile(name, flag)
	if err != nil {
		fs.disconnect(conn)
		return nil, err
	}

	return &sftpFile{File: f, conn: conn, fs: fs}, nil
}

// MkdirAll creates a directory tree
func (fs *SFTPFS) MkdirAll(path string, perm FileMode) error {
	return fs.withConn(func(client *sftp.Client) error {
		parts := strings.Split(path, "/")
		current := ""

		for _, part := range parts {
			if part == "" {
				continue
			}
			current += "/" + part
			if err := client.Mkdir(current); err != nil {
				if !isAlreadyExistsError(err) {
					if info, err2 := client.Stat(current); err2 == nil && info.IsDir() {
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
func (fs *SFTPFS) ReadDir(dirname string) ([]DirEntry, error) {
	conn, err := fs.connect()
	if err != nil {
		return nil, err
	}
	defer fs.disconnect(conn)

	entries, err := conn.sftp.ReadDir(dirname)
	if err != nil {
		return nil, err
	}

	result := make([]DirEntry, len(entries))
	for i, e := range entries {
		result[i] = &sftpDirEntry{FileInfo: e}
	}
	return result, nil
}

// Stat returns file info
func (fs *SFTPFS) Stat(name string) (FileInfo, error) {
	conn, err := fs.connect()
	if err != nil {
		return nil, err
	}
	defer fs.disconnect(conn)

	info, err := conn.sftp.Stat(name)
	if err != nil {
		return nil, err
	}
	return &sftpFileInfo{FileInfo: info}, nil
}

// Exists checks if a file exists
func (fs *SFTPFS) Exists(name string) bool {
	_, err := fs.Stat(name)
	return err == nil
}

// Remove removes a file
func (fs *SFTPFS) Remove(name string) error {
	return fs.withConn(func(client *sftp.Client) error {
		return client.Remove(name)
	})
}

// Rename renames a file
func (fs *SFTPFS) Rename(oldpath, newpath string) error {
	return fs.withConn(func(client *sftp.Client) error {
		return client.Rename(oldpath, newpath)
	})
}

// ReadFile reads entire file content
func (fs *SFTPFS) ReadFile(name string) ([]byte, error) {
	conn, err := fs.connect()
	if err != nil {
		return nil, err
	}
	defer fs.disconnect(conn)

	f, err := conn.sftp.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

// WriteFile writes data to a file
func (fs *SFTPFS) WriteFile(name string, data []byte, perm FileMode) error {
	return fs.SafeWriteFile(name, data, perm)
}

// SafeWriteFile writes data safely using temp file + atomic rename
func (fs *SFTPFS) SafeWriteFile(path string, data []byte, perm FileMode) error {
	return fs.Batch(func(bfs BatchFS) error {
		tempPath := path + ".tmp." + generateID()

		// Create temp file
		f, err := bfs.Create(tempPath)
		if err != nil {
			return fmt.Errorf("create temp file failed: %w", err)
		}

		// Write data
		written, err := f.Write(data)
		if err != nil {
			f.Close()
			bfs.Remove(tempPath)
			return fmt.Errorf("write failed: %w", err)
		}

		if written != len(data) {
			f.Close()
			bfs.Remove(tempPath)
			return fmt.Errorf("incomplete write: %d of %d bytes", written, len(data))
		}

		f.Close()

		// Atomic rename
		if err := bfs.Rename(tempPath, path); err != nil {
			bfs.Remove(tempPath)
			return fmt.Errorf("rename failed: %w", err)
		}

		return nil
	})
}

// Join joins path elements
func (fs *SFTPFS) Join(elem ...string) string {
	return filepath.Join(elem...)
}

// Dir returns the directory part of a path
func (fs *SFTPFS) Dir(path string) string {
	return filepath.Dir(path)
}

// Base returns the base name of a path
func (fs *SFTPFS) Base(path string) string {
	return filepath.Base(path)
}

// Abs returns an absolute path
func (fs *SFTPFS) Abs(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}
	return filepath.Join(fs.basePath, path), nil
}

// TempFile creates a temporary file
func (fs *SFTPFS) TempFile(dir, pattern string) (File, error) {
	name := pattern + generateID()
	path := filepath.Join(dir, name)
	return fs.Create(path)
}

// RemoveAll removes a directory tree
func (fs *SFTPFS) RemoveAll(path string) error {
	return fs.withConn(func(client *sftp.Client) error {
		walker := client.Walk(path)
		var paths []string
		for walker.Step() {
			if err := walker.Err(); err != nil {
				return err
			}
			paths = append(paths, walker.Path())
		}

		for i := len(paths) - 1; i >= 0; i-- {
			if err := client.Remove(paths[i]); err != nil {
				return err
			}
		}
		return nil
	})
}

// Walk walks the file tree
func (fs *SFTPFS) Walk(root string, walkFn WalkFunc) error {
	return fs.withConn(func(client *sftp.Client) error {
		walker := client.Walk(root)
		for walker.Step() {
			if err := walker.Err(); err != nil {
				if err := walkFn(walker.Path(), nil, err); err != nil {
					return err
				}
				continue
			}

			info := &sftpFileInfo{FileInfo: walker.Stat()}
			if err := walkFn(walker.Path(), info, nil); err != nil {
				return err
			}
		}
		return nil
	})
}

// generateID generates a unique ID
func generateID() string {
	var b [8]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// isAlreadyExistsError checks if error is "already exists"
func isAlreadyExistsError(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "exists") ||
		strings.Contains(err.Error(), "file exists"))
}

// sftpFile wraps sftp.File with connection management
type sftpFile struct {
	*sftp.File
	conn *sftpConn
	fs   *SFTPFS
}

func (f *sftpFile) Close() error {
	err := f.File.Close()
	f.fs.disconnect(f.conn)
	return err
}

func (f *sftpFile) Stat() (FileInfo, error) {
	info, err := f.File.Stat()
	if err != nil {
		return nil, err
	}
	return &sftpFileInfo{FileInfo: info}, nil
}

func (f *sftpFile) Readdir(n int) ([]FileInfo, error) {
	return nil, fmt.Errorf("Readdir not supported on SFTP file, use filesystem ReadDir instead")
}

// sftpFileInfo wraps os.FileInfo
type sftpFileInfo struct {
	os.FileInfo
}

func (i *sftpFileInfo) Mode() FileMode {
	return FileMode(i.FileInfo.Mode())
}

// sftpDirEntry wraps os.FileInfo as DirEntry
type sftpDirEntry struct {
	os.FileInfo
}

func (e *sftpDirEntry) Name() string {
	return e.FileInfo.Name()
}

func (e *sftpDirEntry) IsDir() bool {
	return e.FileInfo.IsDir()
}

func (e *sftpDirEntry) Type() FileMode {
	return FileMode(e.FileInfo.Mode())
}

func (e *sftpDirEntry) Info() (FileInfo, error) {
	return &sftpFileInfo{FileInfo: e.FileInfo}, nil
}

// BatchFS implementation for sftpBatchFS

func (b *sftpBatchFS) Create(name string) (File, error) {
	f, err := b.conn.sftp.Create(name)
	if err != nil {
		return nil, err
	}
	return &sftpBatchFile{File: f}, nil
}

func (b *sftpBatchFS) Open(name string) (File, error) {
	f, err := b.conn.sftp.Open(name)
	if err != nil {
		return nil, err
	}
	return &sftpBatchFile{File: f}, nil
}

func (b *sftpBatchFS) OpenFile(name string, flag int, perm FileMode) (File, error) {
	f, err := b.conn.sftp.OpenFile(name, flag)
	if err != nil {
		return nil, err
	}
	return &sftpBatchFile{File: f}, nil
}

func (b *sftpBatchFS) MkdirAll(path string, perm FileMode) error {
	parts := strings.Split(path, "/")
	current := ""
	for _, part := range parts {
		if part == "" {
			continue
		}
		current += "/" + part
		if err := b.conn.sftp.Mkdir(current); err != nil {
			if !isAlreadyExistsError(err) {
				if info, err2 := b.conn.sftp.Stat(current); err2 == nil && info.IsDir() {
					continue
				}
				return err
			}
		}
	}
	return nil
}

func (b *sftpBatchFS) ReadDir(dirname string) ([]DirEntry, error) {
	entries, err := b.conn.sftp.ReadDir(dirname)
	if err != nil {
		return nil, err
	}
	result := make([]DirEntry, len(entries))
	for i, e := range entries {
		result[i] = &sftpDirEntry{FileInfo: e}
	}
	return result, nil
}

func (b *sftpBatchFS) Stat(name string) (FileInfo, error) {
	info, err := b.conn.sftp.Stat(name)
	if err != nil {
		return nil, err
	}
	return &sftpFileInfo{FileInfo: info}, nil
}

func (b *sftpBatchFS) Exists(name string) bool {
	_, err := b.conn.sftp.Stat(name)
	return err == nil
}

func (b *sftpBatchFS) Remove(name string) error {
	return b.conn.sftp.Remove(name)
}

func (b *sftpBatchFS) Rename(oldpath, newpath string) error {
	return b.conn.sftp.Rename(oldpath, newpath)
}

func (b *sftpBatchFS) ReadFile(name string) ([]byte, error) {
	f, err := b.conn.sftp.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func (b *sftpBatchFS) WriteFile(name string, data []byte, perm FileMode) error {
	f, err := b.conn.sftp.Create(name)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	f.Close()
	return err
}

func (b *sftpBatchFS) Join(elem ...string) string {
	return filepath.Join(elem...)
}

func (b *sftpBatchFS) Dir(path string) string {
	return filepath.Dir(path)
}

func (b *sftpBatchFS) Base(path string) string {
	return filepath.Base(path)
}

func (b *sftpBatchFS) RemoveAll(path string) error {
	walker := b.conn.sftp.Walk(path)
	var paths []string
	for walker.Step() {
		if err := walker.Err(); err != nil {
			return err
		}
		paths = append(paths, walker.Path())
	}
	for i := len(paths) - 1; i >= 0; i-- {
		if err := b.conn.sftp.Remove(paths[i]); err != nil {
			return err
		}
	}
	return nil
}

// sftpBatchFile is a file within a batch context (doesn't close connection)
type sftpBatchFile struct {
	*sftp.File
}

func (f *sftpBatchFile) Stat() (FileInfo, error) {
	info, err := f.File.Stat()
	if err != nil {
		return nil, err
	}
	return &sftpFileInfo{FileInfo: info}, nil
}

func (f *sftpBatchFile) Readdir(n int) ([]FileInfo, error) {
	return nil, fmt.Errorf("Readdir not supported")
}

// Ensure SFTPFS implements interfaces
var _ FileSystem = (*SFTPFS)(nil)
var _ BatchFileSystem = (*SFTPFS)(nil)
