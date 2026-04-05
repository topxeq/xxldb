// Package xxldb provides a lightweight SQL database for Go applications
//
// XxLdb is a lightweight, embedded SQL database written in pure Go.
// It supports common SQL operations without external dependencies.
//
// Features:
//   - Pure Go implementation, no CGO dependencies
//   - Support for common SQL statements: SELECT, INSERT, UPDATE, DELETE, CREATE, DROP
//   - Support for JOIN and UNION operations
//   - Built-in functions for string, numeric, date/time operations
//   - Support for persistent storage
//   - User authentication
//   - Configurable logging
//   - Standard Go SQL driver interface
//   - Support for BLOB and FILE storage
//   - SSH/SFTP remote database access
//
// Basic Usage:
//
//	// Open a database
//	engine, err := xxldb.Open("/path/to/db")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer engine.Close()
//
//	// Execute SQL
//	result, err := engine.Exec("CREATE TABLE users (id SEQ, name VARCHAR(100), age INT)")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Query data
//	rows, err := engine.Query("SELECT * FROM users WHERE age > ?", 18)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer rows.Close()
//
// Using with database/sql:
//
//	import (
//	    "database/sql"
//	    _ "github.com/topxeq/xxldb/driver"
//	)
//
//	db, err := sql.Open("xxldb", "/path/to/database")
//
// SSH Remote Database:
//
//	// Using DSN
//	db, err := sql.Open("xxldb", "ssh://user:password@host:22/path/to/db")
//
//	// Using OpenSSH function
//	engine, err := xxldb.OpenSSH(xxldb.SSHConfig{
//	    Host:     "server.com",
//	    Port:     22,
//	    Username: "admin",
//	    Password: "secret",
//	    DBPath:   "/data/mydb",
//	})
package xxldb

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/topxeq/xxldb/driver"
	"github.com/topxeq/xxldb/executor"
	"github.com/topxeq/xxldb/storage"
	"github.com/topxeq/xxldb/types"
)

// Version information
const (
	Version   = executor.Version
	BuildDate = executor.BuildDate
)

// Re-export types from executor package
type Engine = executor.Engine
type Result = executor.Result
type Config = executor.Config

// Re-export types from types package
type Value = types.Value
type DataType = types.DataType
type ColumnDef = types.ColumnDef
type TableInfo = types.TableInfo

// DataType constants
const (
	TypeUnknown  = types.TypeUnknown
	TypeNull     = types.TypeNull
	TypeSeq      = types.TypeSeq
	TypeInt      = types.TypeInt
	TypeFloat    = types.TypeFloat
	TypeChar     = types.TypeChar
	TypeVarchar  = types.TypeVarchar
	TypeText     = types.TypeText
	TypeDate     = types.TypeDate
	TypeTime     = types.TypeTime
	TypeDatetime = types.TypeDatetime
	TypeBlob     = types.TypeBlob
	TypeFile     = types.TypeFile
)

// FileSystem is an interface for file system operations
// Can be used to provide custom storage backends (e.g., SFTP, memory, etc.)
type FileSystem = storage.FileSystem

// SSHConfig holds SSH connection parameters for remote database access
type SSHConfig struct {
	Host       string        // SSH server hostname or IP
	Port       int           // SSH server port (default: 22)
	Username   string        // SSH username
	Password   string        // SSH password (optional if using key)
	PrivateKey string        // Path to SSH private key file (optional if using password)
	Passphrase string        // Passphrase for encrypted private key (optional)
	Timeout    time.Duration // Connection timeout (default: 30s)
	DBPath     string        // Database path on remote server
}

// Open opens a database at the specified path
func Open(path string) (*Engine, error) {
	return executor.NewEngine(path, false)
}

// OpenInMemory opens an in-memory database
func OpenInMemory() (*Engine, error) {
	return executor.NewEngine("", true)
}

// OpenWithConfig opens a database with the specified configuration
func OpenWithConfig(config Config) (*Engine, error) {
	return executor.NewEngineWithConfig(config)
}

// NewEngine creates a new database engine (alias for Open)
func NewEngine(path string, inMemory bool) (*Engine, error) {
	return executor.NewEngine(path, inMemory)
}

// OpenSSH opens a remote database via SSH/SFTP
//
// Example:
//
//	engine, err := xxldb.OpenSSH(xxldb.SSHConfig{
//	    Host:     "server.com",
//	    Port:     22,
//	    Username: "admin",
//	    Password: "secret",
//	    DBPath:   "/data/mydb",
//	})
func OpenSSH(config SSHConfig) (*Engine, error) {
	if config.Port == 0 {
		config.Port = 22
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	sftpConfig := storage.SFTPConfig{
		Host:       config.Host,
		Port:       config.Port,
		Username:   config.Username,
		Password:   config.Password,
		PrivateKey: config.PrivateKey,
		Passphrase: config.Passphrase,
		Timeout:    config.Timeout,
		MaxRetries: 5,
		RetryDelay: 2e9,
	}

	fs, err := storage.NewSFTPFS(config.DBPath, sftpConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH connection: %w", err)
	}

	return executor.NewEngineWithFS(config.DBPath, false, fs)
}

// OpenWithFS opens a database with a custom filesystem
// This allows using custom storage backends like SFTP, memory, etc.
//
// Example:
//
//	// Using local filesystem
//	fs := storage.NewLocalFS("/path/to/db")
//	engine, err := xxldb.OpenWithFS("/path/to/db", fs)
//
//	// Using SFTP filesystem
//	sftpFS, _ := storage.NewSFTPFS("/remote/db", sftpConfig)
//	engine, err := xxldb.OpenWithFS("/remote/db", sftpFS)
func OpenWithFS(path string, fs FileSystem) (*Engine, error) {
	return executor.NewEngineWithFS(path, false, fs)
}

// OpenDSN opens a database using a DSN string
// Supported DSN formats:
//   - :memory: - in-memory database
//   - /path/to/db - local file database
//   - ssh://user:pass@host:port/path/to/db - SSH with password
//   - ssh://user@host:port/path/to/db?private_key=/path/to/key - SSH with key
//
// Note: This function uses database/sql internally. For direct Engine access,
// use Open, OpenSSH, or OpenWithFS instead.
//
// Example:
//
//	db, err := xxldb.OpenDSN("ssh://admin:secret@server.com:22/data/mydb")
func OpenDSN(dsn string) (*sql.DB, error) {
	return sql.Open("xxldb", dsn)
}

// NewValue creates a new Value from any Go value
func NewValue(data interface{}) Value {
	return types.NewValue(data)
}

// NullValue creates a null value
func NullValue() Value {
	return types.NewNullValue()
}

// IntValue creates an integer value
func IntValue(n int64) Value {
	return types.NewIntValue(n)
}

// FloatValue creates a float value
func FloatValue(f float64) Value {
	return types.NewFloatValue(f)
}

// StringValue creates a string value
func StringValue(s string) Value {
	return types.NewStringValue(s)
}

// BoolValue creates a boolean value
func BoolValue(b bool) Value {
	return types.NewBoolValue(b)
}

// DateValue creates a date value
func DateValue(t interface{}) Value {
	return types.NewValue(t)
}

// BlobValue creates a blob value
func BlobValue(data []byte) Value {
	return types.NewBlobValue(data)
}
