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
//	for rows.Next() {
//	    var id int64
//	    var name string
//	    var age int
//	    if err := rows.Scan(&id, &name, &age); err != nil {
//	        log.Fatal(err)
//	    }
//	    fmt.Printf("ID: %d, Name: %s, Age: %d\n", id, name, age)
//	}
//
// Using with database/sql:
//
//	import (
//	    "database/sql"
//	    _ "github.com/topxeq/xxldb/driver"
//	)
//
//	db, err := sql.Open("xxldb", "/path/to/database")
package xxldb

import (
	"github.com/topxeq/xxldb/executor"
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
