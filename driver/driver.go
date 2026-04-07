// Package driver provides a Go SQL driver for xxldb
package driver

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"github.com/topxeq/xxldb/executor"
	"github.com/topxeq/xxldb/storage"
	"github.com/topxeq/xxldb/types"
)

func init() {
	sql.Register("xxldb", &xxldbDriver{})
}

// Driver name
const DriverName = "xxldb"

// BlobValue wraps []byte to indicate it should be treated as BLOB data
type BlobValue struct {
	Data []byte
}

// ImageValue wraps []byte to indicate it should be treated as IMAGE data
type ImageValue struct {
	Data []byte
}

// NewBlob creates a BlobValue for use in SQL queries
func NewBlob(data []byte) BlobValue {
	return BlobValue{Data: data}
}

// NewImage creates an ImageValue for use in SQL queries
func NewImage(data []byte) ImageValue {
	return ImageValue{Data: data}
}

// BlobFromReader creates a BlobValue from an io.Reader
func BlobFromReader(r io.Reader) (BlobValue, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return BlobValue{}, fmt.Errorf("failed to read blob data: %w", err)
	}
	return BlobValue{Data: data}, nil
}

// ImageFromReader creates an ImageValue from an io.Reader
func ImageFromReader(r io.Reader) (ImageValue, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return ImageValue{}, fmt.Errorf("failed to read image data: %w", err)
	}
	return ImageValue{Data: data}, nil
}

// xxldbDriver implements driver.Driver
type xxldbDriver struct{}

// Open opens a new connection to xxldb
func (d *xxldbDriver) Open(name string) (driver.Conn, error) {
	var engine *executor.Engine
	var err error

	// Parse DSN
	dsn, err := ParseDSN(name)
	if err != nil {
		return nil, err
	}

	// Validate DSN
	if err := dsn.Validate(); err != nil {
		return nil, err
	}

	// Build config with encryption password
	config := executor.Config{
		Path:            dsn.Path,
		InMemory:        dsn.InMemory,
		EncryptPassword: dsn.EncryptPassword,
	}

	if dsn.InMemory {
		engine, err = executor.NewEngineWithConfig(config)
	} else if dsn.SSH != nil {
		// Create SFTP filesystem for remote access
		fs, err := createSFTPFileSystem(dsn.Path, dsn.SSH)
		if err != nil {
			return nil, fmt.Errorf("failed to create SSH connection: %w", err)
		}

		// Create storage with custom filesystem
		engine, err = executor.NewEngineWithFS(dsn.Path, false, fs)
	} else if dsn.SMB != nil {
		// Create SMB filesystem for remote access
		fs, err := createSMBFileSystem(dsn.Path, dsn.SMB)
		if err != nil {
			return nil, fmt.Errorf("failed to create SMB connection: %w", err)
		}

		// Create storage with custom filesystem
		engine, err = executor.NewEngineWithFS(dsn.Path, false, fs)
	} else if dsn.WebDAV != nil {
		// Create WebDAV filesystem for remote access
		fs, err := createWebDAVFileSystem(dsn.Path, dsn.WebDAV)
		if err != nil {
			return nil, fmt.Errorf("failed to create WebDAV connection: %w", err)
		}

		// Create storage with custom filesystem
		engine, err = executor.NewEngineWithFS(dsn.Path, false, fs)
	} else {
		engine, err = executor.NewEngineWithConfig(config)
	}

	if err != nil {
		return nil, err
	}

	return &xxldbConn{engine: engine}, nil
}

// createSFTPFileSystem creates an SFTP filesystem from SSH config
func createSFTPFileSystem(dbPath string, ssh *SSHConfig) (storage.FileSystem, error) {
	config := storage.SFTPConfig{
		Host:       ssh.Host,
		Port:       ssh.Port,
		Username:   ssh.Username,
		Password:   ssh.Password,
		PrivateKey: ssh.PrivateKey,
		Passphrase: ssh.Passphrase,
		Timeout:    ssh.Timeout,
		MaxRetries: 5,
		RetryDelay: 2e9, // 2 seconds in nanoseconds
	}

	return storage.NewSFTPFS(dbPath, config)
}

// createSMBFileSystem creates an SMB filesystem from SMB config
func createSMBFileSystem(dbPath string, smb *SMBConfig) (storage.FileSystem, error) {
	config := storage.SMBConfig{
		Host:     smb.Host,
		Port:     smb.Port,
		Share:    smb.Share,
		Username: smb.Username,
		Password: smb.Password,
		Domain:   smb.Domain,
		Timeout:  smb.Timeout,
	}

	return storage.NewSMBFS(dbPath, config)
}

// createWebDAVFileSystem creates a WebDAV filesystem from WebDAV config
func createWebDAVFileSystem(dbPath string, webdav *WebDAVConfig) (storage.FileSystem, error) {
	config := storage.WebDAVConfig{
		URL:      webdav.URL,
		Username: webdav.Username,
		Password: webdav.Password,
		Timeout:  webdav.Timeout,
	}

	return storage.NewWebDAVFS(dbPath, config)
}

// xxldbConn implements driver.Conn
type xxldbConn struct {
	engine *executor.Engine
	closed bool
}

// Prepare prepares a statement
func (c *xxldbConn) Prepare(query string) (driver.Stmt, error) {
	if c.closed {
		return nil, driver.ErrBadConn
	}
	return &xxldbStmt{conn: c, query: query}, nil
}

// Close closes the connection
func (c *xxldbConn) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true
	return c.engine.Close()
}

// Begin starts a transaction (not supported)
func (c *xxldbConn) Begin() (driver.Tx, error) {
	return nil, fmt.Errorf("transactions not supported")
}

// xxldbStmt implements driver.Stmt
type xxldbStmt struct {
	conn  *xxldbConn
	query string
}

// Close closes the statement
func (s *xxldbStmt) Close() error {
	return nil
}

// NumInput returns the number of placeholders
func (s *xxldbStmt) NumInput() int {
	return -1 // We don't know in advance
}

// Exec executes a statement
func (s *xxldbStmt) Exec(args []driver.Value) (driver.Result, error) {
	query := s.replacePlaceholders(s.query, args)
	result, err := s.conn.engine.Execute(query)
	if err != nil {
		return nil, err
	}
	return &xxldbResult{result: result}, nil
}

// Query executes a query
func (s *xxldbStmt) Query(args []driver.Value) (driver.Rows, error) {
	query := s.replacePlaceholders(s.query, args)
	result, err := s.conn.engine.Execute(query)
	if err != nil {
		return nil, err
	}
	return &xxldbRows{result: result}, nil
}

// replacePlaceholders replaces ? placeholders with values
func (s *xxldbStmt) replacePlaceholders(query string, args []driver.Value) string {
	result := query
	for _, arg := range args {
		var replacement string
		switch v := arg.(type) {
		case string:
			replacement = fmt.Sprintf("'%s'", strings.ReplaceAll(v, "'", "''"))
		case int, int64, int32, int16, int8:
			replacement = fmt.Sprintf("%d", v)
		case uint, uint64, uint32, uint16, uint8:
			replacement = fmt.Sprintf("%d", v)
		case float64, float32:
			replacement = fmt.Sprintf("%v", v)
		case bool:
			if v {
				replacement = "1"
			} else {
				replacement = "0"
			}
		case nil:
			replacement = "NULL"
		case BlobValue:
			// Use BLOB_FROM_HEX function for BLOB data
			hexStr := hex.EncodeToString(v.Data)
			replacement = fmt.Sprintf("BLOB_FROM_HEX('%s')", hexStr)
		case ImageValue:
			// Use IMAGE_FROM_HEX function for IMAGE data
			hexStr := hex.EncodeToString(v.Data)
			replacement = fmt.Sprintf("IMAGE_FROM_HEX('%s')", hexStr)
		case []byte:
			// Default []byte treated as BLOB
			hexStr := hex.EncodeToString(v)
			replacement = fmt.Sprintf("BLOB_FROM_HEX('%s')", hexStr)
		default:
			replacement = fmt.Sprintf("'%v'", v)
		}
		result = strings.Replace(result, "?", replacement, 1)
	}
	return result
}

// xxldbResult implements driver.Result
type xxldbResult struct {
	result *executor.Result
}

// LastInsertId returns the last insert ID
func (r *xxldbResult) LastInsertId() (int64, error) {
	return r.result.LastInsertID, nil
}

// RowsAffected returns the number of rows affected
func (r *xxldbResult) RowsAffected() (int64, error) {
	return r.result.RowsAffected, nil
}

// xxldbRows implements driver.Rows
type xxldbRows struct {
	result   *executor.Result
	rowIndex int
	closed   bool
}

// Columns returns column names
func (r *xxldbRows) Columns() []string {
	return r.result.Columns
}

// Close closes the rows
func (r *xxldbRows) Close() error {
	r.closed = true
	return nil
}

// Next fetches the next row
func (r *xxldbRows) Next(dest []driver.Value) error {
	if r.closed || r.rowIndex >= len(r.result.Rows) {
		return io.EOF
	}

	row := r.result.Rows[r.rowIndex]
	for i, val := range row.Data {
		if i < len(dest) {
			if val.IsNull {
				dest[i] = nil
			} else {
				dest[i] = val.Data
			}
		}
	}

	r.rowIndex++
	return nil
}

// Additional interfaces for better compatibility

// CheckNamedValue implements driver.NamedValueChecker
func (s *xxldbStmt) CheckNamedValue(nv *driver.NamedValue) error {
	return nil // Accept any value
}

// QueryContext implements driver.StmtQueryContext
func (s *xxldbStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	values := make([]driver.Value, len(args))
	for i, nv := range args {
		values[i] = nv.Value
	}
	return s.Query(values)
}

// ExecContext implements driver.StmtExecContext
func (s *xxldbStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	values := make([]driver.Value, len(args))
	for i, nv := range args {
		values[i] = nv.Value
	}
	return s.Exec(values)
}

// BeginTx implements driver.ConnBeginTx
func (c *xxldbConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	return nil, fmt.Errorf("transactions not supported")
}

// PrepareContext implements driver.ConnPrepareContext
func (c *xxldbConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	return c.Prepare(query)
}

// Ping implements driver.Pinger
func (c *xxldbConn) Ping(ctx context.Context) error {
	if c.closed {
		return driver.ErrBadConn
	}
	return nil
}

// ResetSession implements driver.SessionResetter
func (c *xxldbConn) ResetSession(ctx context.Context) error {
	if c.closed {
		return driver.ErrBadConn
	}
	return nil
}

// IsValid implements driver.Validator
func (c *xxldbConn) IsValid() bool {
	return !c.closed
}

// Helper functions for external use

// Open opens a database connection
func Open(path string) (*sql.DB, error) {
	return sql.Open(DriverName, path)
}

// OpenInMemory opens an in-memory database
func OpenInMemory() (*sql.DB, error) {
	return sql.Open(DriverName, ":memory:")
}

// OpenFile opens a file-based database
func OpenFile(path string) (*sql.DB, error) {
	return sql.Open(DriverName, path)
}

// ConvertValue converts a types.Value to driver.Value
func ConvertValue(v types.Value) driver.Value {
	if v.IsNull {
		return nil
	}
	return v.Data
}

// ConvertToValue converts a driver.Value to types.Value
func ConvertToValue(v driver.Value) types.Value {
	if v == nil {
		return types.NewNullValue()
	}
	return types.NewValue(v)
}

// Value implements driver.Valuer for BlobValue
func (b BlobValue) Value() (driver.Value, error) {
	return b, nil // Return self, handled in replacePlaceholders
}

// Value implements driver.Valuer for ImageValue
func (i ImageValue) Value() (driver.Value, error) {
	return i, nil // Return self, handled in replacePlaceholders
}

// ConvertBlobToValue converts []byte to a types.Value for BLOB column
func ConvertBlobToValue(data []byte) types.Value {
	return types.NewBlobValue(data)
}

// ConvertImageToValue converts []byte to a types.Value for IMAGE column
func ConvertImageToValue(data []byte) types.Value {
	return types.NewImageValue(data)
}

// ConvertBlobFromReader creates a types.Value for BLOB column from io.Reader
func ConvertBlobFromReader(r io.Reader) (types.Value, error) {
	return types.NewBlobValueFromReader(r)
}

// ConvertImageFromReader creates a types.Value for IMAGE column from io.Reader
func ConvertImageFromReader(r io.Reader) (types.Value, error) {
	return types.NewImageValueFromReader(r)
}
