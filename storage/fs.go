// Package storage provides data persistence for xxldb
package storage

import (
	"io"
	"time"
)

// FileSystem defines file system operations needed by Storage
// This abstraction allows for different backends (local, SFTP, etc.)
type FileSystem interface {
	// File operations
	Create(name string) (File, error)
	Open(name string) (File, error)
	OpenFile(name string, flag int, perm FileMode) (File, error)

	// Directory operations
	MkdirAll(path string, perm FileMode) error
	ReadDir(dirname string) ([]DirEntry, error)

	// File info
	Stat(name string) (FileInfo, error)
	Exists(name string) bool
	Remove(name string) error
	Rename(oldpath, newpath string) error

	// Read/Write helpers
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm FileMode) error

	// Path operations
	Join(elem ...string) string
	Dir(path string) string
	Base(path string) string
	Abs(path string) (string, error)

	// Temp file for atomic writes
	TempFile(dir, pattern string) (File, error)

	// Cleanup
	RemoveAll(path string) error

	// Walk directory tree
	Walk(root string, walkFn WalkFunc) error

	// Close closes the filesystem connection
	Close() error
}

// BatchFileSystem extends FileSystem with batch operation support
// Batch allows multiple operations to be performed within a single connection
// This is useful for remote filesystems (SFTP, SMB, WebDAV) to avoid
// connection overhead for each operation
type BatchFileSystem interface {
	FileSystem

	// Batch executes multiple operations within a single connection
	// The provided function receives a connected filesystem that can be used
	// for multiple operations. The connection is managed by the implementation.
	// Example:
	//   fs.Batch(func(bfs BatchFS) error {
	//       bfs.MkdirAll("dir", 0755)
	//       bfs.WriteFile("dir/file.txt", data, 0644)
	//       return nil
	//   })
	Batch(fn func(BatchFS) error) error
}

// BatchFS provides the operations available within a batch context
// Operations are performed on a maintained connection
type BatchFS interface {
	// File operations
	Create(name string) (File, error)
	Open(name string) (File, error)
	OpenFile(name string, flag int, perm FileMode) (File, error)

	// Directory operations
	MkdirAll(path string, perm FileMode) error
	ReadDir(dirname string) ([]DirEntry, error)

	// File info
	Stat(name string) (FileInfo, error)
	Exists(name string) bool
	Remove(name string) error
	Rename(oldpath, newpath string) error

	// Read/Write helpers
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm FileMode) error

	// Path operations
	Join(elem ...string) string
	Dir(path string) string
	Base(path string) string

	// Cleanup
	RemoveAll(path string) error
}

// File represents an open file
type File interface {
	io.Reader
	io.Writer
	io.Closer
	io.Seeker

	Name() string
	Stat() (FileInfo, error)
	Sync() error
	Truncate(size int64) error
	Readdir(n int) ([]FileInfo, error)
}

// FileInfo represents file metadata
type FileInfo interface {
	Name() string       // base name of the file
	Size() int64        // length in bytes
	Mode() FileMode     // file mode bits
	ModTime() time.Time // modification time
	IsDir() bool        // abbreviation for Mode().IsDir()
}

// FileMode represents file mode bits
type FileMode uint32

const (
	ModeDir       FileMode = 1 << (32 - 1 - iota) // d: is a directory
	ModeAppend                                     // a: append-only
	ModeExclusive                                  // l: exclusive use
	ModeTemporary                                  // T: temporary file
	ModeSymlink                                    // L: symbolic link
	ModeDevice                                     // D: device file
	ModeNamedPipe                                  // p: named pipe (FIFO)
	ModeSocket                                     // S: Unix domain socket
	ModeSetuid                                     // u: setuid
	ModeSetgid                                     // g: setgid
	ModeCharDevice                                 // c: Unix character device
	ModeSticky                                     // t: sticky

	ModeType = ModeDir | ModeSymlink | ModeNamedPipe | ModeSocket | ModeDevice

	ModePerm FileMode = 0777 // Unix permission bits
)

// DirEntry represents a directory entry
type DirEntry interface {
	Name() string
	IsDir() bool
	Type() FileMode
	Info() (FileInfo, error)
}

// WalkFunc is the type of function called by Walk to visit each file or directory
type WalkFunc func(path string, info FileInfo, err error) error

// FileMode constants
const (
	PermDir  FileMode = 0755
	PermFile FileMode = 0644
)

// OpenFile flags
const (
	FlagO_RDONLY int = 0
	FlagO_WRONLY int = 1
	FlagO_RDWR   int = 2
	FlagO_APPEND int = 0400
	FlagO_CREATE int = 0100
	FlagO_EXCL   int = 0200
	FlagO_SYNC   int = 01000
	FlagO_TRUNC  int = 01000
)
