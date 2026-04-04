// Package driver provides a Go SQL driver for xxldb
package driver

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"strings"

	"github.com/topxeq/xxldb/executor"
	"github.com/topxeq/xxldb/types"
)

func init() {
	sql.Register("xxldb", &xxldbDriver{})
}

// Driver name
const DriverName = "xxldb"

// xxldbDriver implements driver.Driver
type xxldbDriver struct{}

// Open opens a new connection to xxldb
func (d *xxldbDriver) Open(name string) (driver.Conn, error) {
	var engine *executor.Engine
	var err error

	if strings.HasSuffix(name, ":memory:") {
		engine, err = executor.NewEngine("", true)
	} else {
		// Remove prefix if present
		path := name
		if strings.HasPrefix(path, "file:") {
			path = strings.TrimPrefix(path, "file:")
		}
		engine, err = executor.NewEngine(path, false)
	}

	if err != nil {
		return nil, err
	}

	return &xxldbConn{engine: engine}, nil
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
		case []byte:
			// For BLOB, use hex representation or string
			replacement = fmt.Sprintf("'%s'", string(v))
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
