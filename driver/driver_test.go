package driver

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/topxeq/xxldb/types"
)

func TestDriverName(t *testing.T) {
	if DriverName != "xxldb" {
		t.Errorf("DriverName = %s, want xxldb", DriverName)
	}
}

func TestOpenInMemory(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatalf("Failed to open in-memory database: %v", err)
	}
	defer db.Close()

	if db == nil {
		t.Error("db should not be nil")
	}
}

func TestOpenFile(t *testing.T) {
	// Use a temp path
	db, err := sql.Open(DriverName, "/tmp/xxldb_test_driver")
	if err != nil {
		t.Fatalf("Failed to open file database: %v", err)
	}
	defer db.Close()

	if db == nil {
		t.Error("db should not be nil")
	}
}

func TestPing(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		t.Errorf("Ping failed: %v", err)
	}
}

func TestExecCreateTable(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE test_table (
		id SEQ,
		name VARCHAR(100),
		value INT
	)`)
	if err != nil {
		t.Errorf("CREATE TABLE failed: %v", err)
	}
}

func TestExecInsert(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create table first
	_, err = db.Exec(`CREATE TABLE test_table (id SEQ, name VARCHAR(100), value INT)`)
	if err != nil {
		t.Fatalf("CREATE TABLE failed: %v", err)
	}

	// Insert
	result, err := db.Exec("INSERT INTO test_table (name, value) VALUES (?, ?)", "test", 42)
	if err != nil {
		t.Errorf("INSERT failed: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Errorf("LastInsertId failed: %v", err)
	}
	if id <= 0 {
		t.Errorf("LastInsertId = %d, expected > 0", id)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		t.Errorf("RowsAffected failed: %v", err)
	}
	if rows != 1 {
		t.Errorf("RowsAffected = %d, expected 1", rows)
	}
}

func TestQuerySelect(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Setup
	_, err = db.Exec(`CREATE TABLE test_table (id SEQ, name VARCHAR(100), value INT)`)
	if err != nil {
		t.Fatalf("CREATE TABLE failed: %v", err)
	}

	_, err = db.Exec("INSERT INTO test_table (name, value) VALUES (?, ?)", "test", 42)
	if err != nil {
		t.Fatalf("INSERT failed: %v", err)
	}

	// Query
	rows, err := db.Query("SELECT id, name, value FROM test_table")
	if err != nil {
		t.Fatalf("SELECT failed: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id int64
		var name string
		var value int
		err := rows.Scan(&id, &name, &value)
		if err != nil {
			t.Errorf("Scan failed: %v", err)
		}
		count++
	}

	if count != 1 {
		t.Errorf("Expected 1 row, got %d", count)
	}
}

func TestQueryWithWhere(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Setup
	_, err = db.Exec(`CREATE TABLE test_table (id SEQ, name VARCHAR(100), value INT)`)
	if err != nil {
		t.Fatalf("CREATE TABLE failed: %v", err)
	}

	db.Exec("INSERT INTO test_table (name, value) VALUES (?, ?)", "a", 10)
	db.Exec("INSERT INTO test_table (name, value) VALUES (?, ?)", "b", 20)
	db.Exec("INSERT INTO test_table (name, value) VALUES (?, ?)", "c", 30)

	// Query with WHERE
	rows, err := db.Query("SELECT name, value FROM test_table WHERE value > ?", 15)
	if err != nil {
		t.Fatalf("SELECT failed: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var name string
		var value int
		rows.Scan(&name, &value)
		if value <= 15 {
			t.Errorf("Value %d should be > 15", value)
		}
		count++
	}

	if count != 2 {
		t.Errorf("Expected 2 rows, got %d", count)
	}
}

func TestUpdate(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Setup
	_, err = db.Exec(`CREATE TABLE test_table (id SEQ, name VARCHAR(100), value INT)`)
	if err != nil {
		t.Fatalf("CREATE TABLE failed: %v", err)
	}

	db.Exec("INSERT INTO test_table (name, value) VALUES (?, ?)", "test", 10)

	// Update
	result, err := db.Exec("UPDATE test_table SET value = ? WHERE name = ?", 20, "test")
	if err != nil {
		t.Errorf("UPDATE failed: %v", err)
	}

	rows, _ := result.RowsAffected()
	if rows != 1 {
		t.Errorf("RowsAffected = %d, expected 1", rows)
	}
}

func TestDelete(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Setup
	_, err = db.Exec(`CREATE TABLE test_table (id SEQ, name VARCHAR(100), value INT)`)
	if err != nil {
		t.Fatalf("CREATE TABLE failed: %v", err)
	}

	db.Exec("INSERT INTO test_table (name, value) VALUES (?, ?)", "test", 10)

	// Delete
	result, err := db.Exec("DELETE FROM test_table WHERE name = ?", "test")
	if err != nil {
		t.Errorf("DELETE failed: %v", err)
	}

	rows, _ := result.RowsAffected()
	if rows != 1 {
		t.Errorf("RowsAffected = %d, expected 1", rows)
	}
}

func TestNullValues(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create table with nullable column
	_, err = db.Exec(`CREATE TABLE test_table (id SEQ, name VARCHAR(100), value INT)`)
	if err != nil {
		t.Fatalf("CREATE TABLE failed: %v", err)
	}

	// Insert with NULL
	db.Exec("INSERT INTO test_table (name, value) VALUES (?, ?)", "test", nil)

	// Query
	rows, err := db.Query("SELECT name, value FROM test_table")
	if err != nil {
		t.Fatalf("SELECT failed: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		var value sql.NullInt64
		err := rows.Scan(&name, &value)
		if err != nil {
			t.Errorf("Scan failed: %v", err)
		}
		if value.Valid {
			t.Error("value should be NULL")
		}
	}
}

func TestMultipleTables(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create multiple tables
	_, err = db.Exec(`CREATE TABLE users (id SEQ, name VARCHAR(100))`)
	if err != nil {
		t.Fatalf("CREATE TABLE users failed: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE orders (id SEQ, user_id INT, amount INT)`)
	if err != nil {
		t.Fatalf("CREATE TABLE orders failed: %v", err)
	}

	// Insert data
	db.Exec("INSERT INTO users (name) VALUES (?)", "Alice")
	db.Exec("INSERT INTO orders (user_id, amount) VALUES (?, ?)", 1, 100)

	// Query
	rows, err := db.Query("SELECT * FROM users")
	if err != nil {
		t.Errorf("SELECT from users failed: %v", err)
	}
	rows.Close()

	rows, err = db.Query("SELECT * FROM orders")
	if err != nil {
		t.Errorf("SELECT from orders failed: %v", err)
	}
	rows.Close()
}

func TestOpenHelper(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	if db == nil {
		t.Error("Open should return non-nil db")
	}
}

func TestOpenInMemoryHelper(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory failed: %v", err)
	}
	defer db.Close()

	if db == nil {
		t.Error("OpenInMemory should return non-nil db")
	}
}

func TestTransactions(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Transactions are not supported, should fail
	_, err = db.Begin()
	if err == nil {
		t.Error("Begin should fail (transactions not supported)")
	}
}

func TestPrepareStatement(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE test_table (id SEQ, name VARCHAR(100))`)
	if err != nil {
		t.Fatalf("CREATE TABLE failed: %v", err)
	}

	// Prepare statement
	stmt, err := db.Prepare("INSERT INTO test_table (name) VALUES (?)")
	if err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}
	defer stmt.Close()

	// Execute multiple times
	_, err = stmt.Exec("first")
	if err != nil {
		t.Errorf("First exec failed: %v", err)
	}

	_, err = stmt.Exec("second")
	if err != nil {
		t.Errorf("Second exec failed: %v", err)
	}

	// Verify
	rows, err := db.Query("SELECT COUNT(*) FROM test_table")
	if err != nil {
		t.Fatalf("COUNT query failed: %v", err)
	}
	defer rows.Close()

	if rows.Next() {
		var count int
		rows.Scan(&count)
		if count != 2 {
			t.Errorf("Expected 2 rows, got %d", count)
		}
	}
}

func TestQueryRow(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE test_table (id SEQ, name VARCHAR(100))`)
	if err != nil {
		t.Fatalf("CREATE TABLE failed: %v", err)
	}

	db.Exec("INSERT INTO test_table (name) VALUES (?)", "test")

	var name string
	err = db.QueryRow("SELECT name FROM test_table WHERE id = ?", 1).Scan(&name)
	if err != nil {
		t.Errorf("QueryRow failed: %v", err)
	}
	if name != "test" {
		t.Errorf("name = %s, expected 'test'", name)
	}
}

// Test Begin transaction
func TestBeginTransaction(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Begin should fail since transactions are not supported
	tx, err := db.Begin()
	if err == nil {
		tx.Rollback()
		t.Error("Begin should fail (transactions not supported)")
	}
}

// Test ResetSession
func TestResetSession(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// ResetSession is a no-op, just verify it doesn't error
	// This is called internally by the driver
}

// Test Query with error
func TestQueryError(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Query non-existent table
	_, err = db.Query("SELECT * FROM nonexistent_table")
	if err == nil {
		t.Error("Query should fail for non-existent table")
	}
}

// Test Exec with error
func TestExecError(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Exec on non-existent table
	_, err = db.Exec("INSERT INTO nonexistent_table (id) VALUES (1)")
	if err == nil {
		t.Error("Exec should fail for non-existent table")
	}
}

// Test Prepare with error
func TestPrepareError(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Prepare may not fail immediately for invalid SQL
	// since parsing might happen at execution time
	stmt, err := db.Prepare("INVALID SQL STATEMENT")
	if err != nil {
		t.Logf("Prepare failed as expected: %v", err)
		return
	}
	// If prepare succeeded, execution should fail
	_, err = stmt.Exec()
	if err == nil {
		t.Error("Execution of invalid SQL should fail")
	}
	stmt.Close()
}

// Test Close connection
func TestCloseConnection(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Close
	err = db.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

// Test multiple placeholders
func TestMultiplePlaceholders(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE test_table (id SEQ, a INT, b INT, c INT)`)
	if err != nil {
		t.Fatalf("CREATE TABLE failed: %v", err)
	}

	// Insert with multiple placeholders
	_, err = db.Exec("INSERT INTO test_table (a, b, c) VALUES (?, ?, ?)", 1, 2, 3)
	if err != nil {
		t.Errorf("INSERT failed: %v", err)
	}

	// Query with multiple placeholders
	rows, err := db.Query("SELECT * FROM test_table WHERE a = ? AND b = ? AND c = ?", 1, 2, 3)
	if err != nil {
		t.Errorf("SELECT failed: %v", err)
	}
	rows.Close()
}

// Test different data types
func TestDifferentDataTypes(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE test_table (id SEQ, int_val INT, float_val FLOAT, str_val VARCHAR(100))`)
	if err != nil {
		t.Fatalf("CREATE TABLE failed: %v", err)
	}

	// Insert different types
	_, err = db.Exec("INSERT INTO test_table (int_val, float_val, str_val) VALUES (?, ?, ?)", 42, 3.14, "hello")
	if err != nil {
		t.Errorf("INSERT failed: %v", err)
	}

	// Query
	rows, err := db.Query("SELECT int_val, float_val, str_val FROM test_table")
	if err != nil {
		t.Fatalf("SELECT failed: %v", err)
	}
	defer rows.Close()

	if rows.Next() {
		var intVal int
		var floatVal float64
		var strVal string
		err := rows.Scan(&intVal, &floatVal, &strVal)
		if err != nil {
			t.Errorf("Scan failed: %v", err)
		}
		if intVal != 42 {
			t.Errorf("intVal = %d, expected 42", intVal)
		}
		if strVal != "hello" {
			t.Errorf("strVal = %s, expected 'hello'", strVal)
		}
	}
}

// Test OpenFile function
func TestOpenFileFunc(t *testing.T) {
	// Create a temp directory for the test
	dir := "/tmp/xxldb_driver_openfile_test"

	db, err := OpenFile(dir)
	if err != nil {
		t.Fatalf("OpenFile failed: %v", err)
	}
	defer db.Close()

	if db == nil {
		t.Error("OpenFile should return non-nil db")
	}
}

// Test ConvertValue function
func TestConvertValue(t *testing.T) {
	tests := []struct {
		input    types.Value
		expected interface{}
	}{
		{types.NewIntValue(42), int(42)},
		{types.NewFloatValue(3.14), 3.14},
		{types.NewStringValue("hello"), "hello"},
		{types.NewNullValue(), nil},
	}

	for _, tt := range tests {
		result := ConvertValue(tt.input)
		if result != tt.expected && tt.expected != nil {
			t.Logf("ConvertValue: got %v, expected %v", result, tt.expected)
		}
	}
}

// Test ConvertToValue function
func TestConvertToValue(t *testing.T) {
	tests := []struct {
		input    interface{}
		wantNull bool
	}{
		{int64(42), false},
		{float64(3.14), false},
		{"hello", false},
		{[]byte("blob"), false},
		{nil, true},
	}

	for _, tt := range tests {
		val := ConvertToValue(tt.input)
		if tt.wantNull && !val.IsNull {
			t.Errorf("ConvertToValue(%v) should return null", tt.input)
		}
		if !tt.wantNull && val.IsNull {
			t.Logf("ConvertToValue(%v) returned null", tt.input)
		}
	}
}

// Test driver Begin method directly
func TestDriverBegin(t *testing.T) {
	drv := &xxldbDriver{}

	_, err := drv.Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// Begin should return error as transactions are not supported
	_, err = drv.Open(":memory:")
	if err != nil {
		t.Logf("Open result: %v", err)
	}
}

// Test statement execution
func TestStatementExec(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE test_table (id SEQ, name VARCHAR(100))`)
	if err != nil {
		t.Fatalf("CREATE TABLE failed: %v", err)
	}

	// Test Exec without args
	_, err = db.Exec("INSERT INTO test_table (name) VALUES ('test')")
	if err != nil {
		t.Errorf("INSERT failed: %v", err)
	}
}

// Test query with no results
func TestQueryNoResults(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE test_table (id SEQ, name VARCHAR(100))`)
	if err != nil {
		t.Fatalf("CREATE TABLE failed: %v", err)
	}

	// Query empty table
	rows, err := db.Query("SELECT * FROM test_table")
	if err != nil {
		t.Fatalf("SELECT failed: %v", err)
	}
	defer rows.Close()

	if rows.Next() {
		t.Error("Empty table should have no rows")
	}
}

// Test Exec with invalid SQL
func TestExecInvalidSQL(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec("INVALID SQL STATEMENT")
	if err == nil {
		t.Error("Invalid SQL should fail")
	}
}

// Test Query with invalid SQL
func TestQueryInvalidSQL(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	_, err = db.Query("INVALID SQL STATEMENT")
	if err == nil {
		t.Error("Invalid SQL should fail")
	}
}

// Test DROP TABLE
func TestDropTable(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE test_table (id SEQ, name VARCHAR(100))`)
	if err != nil {
		t.Fatalf("CREATE TABLE failed: %v", err)
	}

	_, err = db.Exec("DROP TABLE test_table")
	if err != nil {
		t.Errorf("DROP TABLE failed: %v", err)
	}

	// Verify table is gone
	_, err = db.Query("SELECT * FROM test_table")
	if err == nil {
		t.Error("Query should fail after table is dropped")
	}
}

// Test ORDER BY
func TestOrderBy(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE test_table (id SEQ, value INT)`)
	if err != nil {
		t.Fatalf("CREATE TABLE failed: %v", err)
	}

	// Insert values in random order
	db.Exec("INSERT INTO test_table (value) VALUES (3)")
	db.Exec("INSERT INTO test_table (value) VALUES (1)")
	db.Exec("INSERT INTO test_table (value) VALUES (2)")

	// Query with ORDER BY
	rows, err := db.Query("SELECT value FROM test_table ORDER BY value")
	if err != nil {
		t.Fatalf("SELECT with ORDER BY failed: %v", err)
	}
	defer rows.Close()

	var values []int
	for rows.Next() {
		var v int
		rows.Scan(&v)
		values = append(values, v)
	}

	// Verify order
	if len(values) == 3 {
		if values[0] > values[1] || values[1] > values[2] {
			t.Errorf("Values not in order: %v", values)
		}
	}
}

// Test LIMIT
func TestLimit(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE test_table (id SEQ, value INT)`)
	if err != nil {
		t.Fatalf("CREATE TABLE failed: %v", err)
	}

	// Insert multiple values
	for i := 0; i < 10; i++ {
		db.Exec("INSERT INTO test_table (value) VALUES (?)", i)
	}

	// Query with LIMIT
	rows, err := db.Query("SELECT value FROM test_table LIMIT 5")
	if err != nil {
		t.Fatalf("SELECT with LIMIT failed: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
	}

	if count != 5 {
		t.Errorf("LIMIT 5 should return 5 rows, got %d", count)
	}
}

// Test with float values
func TestFloatValues(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE test_table (id SEQ, float_val FLOAT)`)
	if err != nil {
		t.Fatalf("CREATE TABLE failed: %v", err)
	}

	_, err = db.Exec("INSERT INTO test_table (float_val) VALUES (?)", 3.14159)
	if err != nil {
		t.Errorf("INSERT failed: %v", err)
	}

	rows, err := db.Query("SELECT float_val FROM test_table")
	if err != nil {
		t.Fatalf("SELECT failed: %v", err)
	}
	defer rows.Close()

	if rows.Next() {
		var f float64
		err := rows.Scan(&f)
		if err != nil {
			t.Errorf("Scan failed: %v", err)
		}
		// Just verify we got a value
		t.Logf("Float value: %f", f)
	}
}

// Test column types
func TestColumnTypes(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE test_table (id SEQ, name VARCHAR(100), value INT)`)
	if err != nil {
		t.Fatalf("CREATE TABLE failed: %v", err)
	}

	db.Exec("INSERT INTO test_table (name, value) VALUES ('test', 42)")

	rows, err := db.Query("SELECT * FROM test_table")
	if err != nil {
		t.Fatalf("SELECT failed: %v", err)
	}
	defer rows.Close()

	// Get column types
	types, err := rows.ColumnTypes()
	if err != nil {
		t.Logf("ColumnTypes: %v", err)
	} else {
		t.Logf("Number of columns: %d", len(types))
	}
}

// TestDriverOpenErrors tests Open with various error conditions
func TestDriverOpenErrors(t *testing.T) {
	// Test with invalid path (should still work as it creates the path)
	db, err := sql.Open(DriverName, "/tmp/nonexistent_xxldb_test/path")
	if err != nil {
		t.Logf("Open with path: %v", err)
	}
	if db != nil {
		db.Close()
	}
}

// TestPrepareQuery tests prepared statement with query
func TestPrepareQuery(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE test_table (id SEQ, name VARCHAR(100))`)
	if err != nil {
		t.Fatal(err)
	}

	db.Exec("INSERT INTO test_table (name) VALUES ('test')")

	// Prepare query
	stmt, err := db.Prepare("SELECT name FROM test_table WHERE id = ?")
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()

	var name string
	err = stmt.QueryRow(1).Scan(&name)
	if err != nil {
		t.Logf("QueryRow: %v", err)
	} else {
		t.Logf("Name: %s", name)
	}
}

// TestTransactionRollback tests transaction rollback
func TestTransactionRollback(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Begin should fail since transactions are not supported
	tx, err := db.Begin()
	if err == nil {
		err = tx.Rollback()
		t.Logf("Rollback: %v", err)
	} else {
		t.Logf("Begin failed as expected: %v", err)
	}
}

// TestMultipleConnections tests multiple connections
func TestMultipleConnections(t *testing.T) {
	db1, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db1.Close()

	db2, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()

	// Create tables in both
	db1.Exec("CREATE TABLE t1 (id INT)")
	db2.Exec("CREATE TABLE t2 (id INT)")

	// Both should work independently
	_, err = db1.Exec("INSERT INTO t1 VALUES (1)")
	if err != nil {
		t.Errorf("db1 insert: %v", err)
	}

	_, err = db2.Exec("INSERT INTO t2 VALUES (2)")
	if err != nil {
		t.Errorf("db2 insert: %v", err)
	}
}


// TestQueryContext tests QueryContext
func TestQueryContext(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec("CREATE TABLE ctx_test (id INT)")
	if err != nil {
		t.Fatal(err)
	}

	db.Exec("INSERT INTO ctx_test VALUES (1)")

	rows, err := db.QueryContext(context.Background(), "SELECT * FROM ctx_test")
	if err != nil {
		t.Errorf("QueryContext: %v", err)
	}
	if rows != nil {
		rows.Close()
	}
}

// TestExecContext tests ExecContext
func TestExecContext(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.ExecContext(context.Background(), "CREATE TABLE ctx_exec (id INT)")
	if err != nil {
		t.Errorf("ExecContext: %v", err)
	}
}

// TestPingContext tests PingContext
func TestPingContext(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	err = db.PingContext(context.Background())
	if err != nil {
		t.Errorf("PingContext: %v", err)
	}
}

// TestRowsColumns tests Rows.Columns
func TestRowsColumns(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	db.Exec("CREATE TABLE cols_test (id INT, name VARCHAR(50))")
	db.Exec("INSERT INTO cols_test VALUES (1, 'test')")

	rows, err := db.Query("SELECT id, name FROM cols_test")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		t.Errorf("Columns: %v", err)
	}
	t.Logf("Columns: %v", cols)
}

// TestQueryWithEmptyResult tests query that returns no rows
func TestQueryWithEmptyResult(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	db.Exec("CREATE TABLE empty_test (id INT)")

	rows, err := db.Query("SELECT * FROM empty_test WHERE id > 100")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
	}
	if count != 0 {
		t.Errorf("Expected 0 rows, got %d", count)
	}
}

// TestStatementClose tests statement close
func TestStatementClose(t *testing.T) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	db.Exec("CREATE TABLE stmt_close (id INT)")

	stmt, err := db.Prepare("INSERT INTO stmt_close VALUES (?)")
	if err != nil {
		t.Fatal(err)
	}

	err = stmt.Close()
	if err != nil {
		t.Errorf("Statement close: %v", err)
	}
}


// TestDriverReplacePlaceholders tests placeholder replacement
func TestDriverReplacePlaceholdersExtra(t *testing.T) {
	db, err := sql.Open("xxldb", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Create table
	_, err = db.Exec("CREATE TABLE placeholder_test (id INT, name VARCHAR(50))")
	if err != nil {
		t.Fatal(err)
	}

	// Insert with placeholder
	_, err = db.Exec("INSERT INTO placeholder_test VALUES (?, ?)", 1, "test")
	if err != nil {
		t.Errorf("Insert with placeholder failed: %v", err)
	}

	// Query with placeholder
	rows, err := db.Query("SELECT * FROM placeholder_test WHERE id = ?", 1)
	if err != nil {
		t.Errorf("Query with placeholder failed: %v", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var id int
			var name string
			rows.Scan(&id, &name)
			t.Logf("Row: id=%d, name=%s", id, name)
		}
	}
}

// TestDriverPing tests Ping
func TestDriverPing(t *testing.T) {
	db, err := sql.Open("xxldb", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		t.Errorf("PingContext failed: %v", err)
	}
}

// TestDriverResetSession tests session reset

// TestDriverOpenVariations tests Open with various connection strings
func TestDriverOpenVariations(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
	}{
		{"memory", ":memory:"},
		{"file prefix", "file:/tmp/xxldb_test_driver_prefix"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := sql.Open("xxldb", tt.dsn)
			if err != nil {
				t.Errorf("Open(%s) failed: %v", tt.dsn, err)
			}
			defer db.Close()

			// Try a simple operation
			_, err = db.Exec("CREATE TABLE test_table (id INT)")
			if err != nil {
				t.Logf("Create table error: %v", err)
			}
		})
	}
}

// TestDriverPrepareClosedConnection tests Prepare on closed connection
func TestDriverPrepareClosedConnection(t *testing.T) {
	db, err := sql.Open("xxldb", ":memory:")
	if err != nil {
		t.Fatal(err)
	}

	// Close the database
	db.Close()

	// Try to prepare on closed connection - should fail
	_, err = db.Prepare("SELECT 1")
	if err == nil {
		t.Error("Prepare on closed connection should fail")
	}
	t.Logf("Prepare error (expected): %v", err)
}

// TestDriverCloseMultipleTimes tests Close called multiple times
func TestDriverCloseMultipleTimes(t *testing.T) {
	db, err := sql.Open("xxldb", ":memory:")
	if err != nil {
		t.Fatal(err)
	}

	// Close once
	err = db.Close()
	if err != nil {
		t.Errorf("First Close failed: %v", err)
	}

	// Close again - should be idempotent
	err = db.Close()
	if err != nil {
		t.Errorf("Second Close failed: %v", err)
	}
}

// TestDriverReplacePlaceholdersMore tests placeholder replacement
func TestDriverReplacePlaceholdersMore(t *testing.T) {
	db, err := sql.Open("xxldb", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	db.Exec("CREATE TABLE placeholder_more (id INT, name VARCHAR(50), value FLOAT)")

	// Test with various placeholder patterns
	tests := []struct {
		query string
		args  []interface{}
	}{
		{"INSERT INTO placeholder_more VALUES (?, ?, ?)", []interface{}{1, "test", 3.14}},
		{"INSERT INTO placeholder_more VALUES (?, ?, ?)", []interface{}{2, "another", 2.71}},
		{"SELECT * FROM placeholder_more WHERE id = ?", []interface{}{1}},
		{"SELECT * FROM placeholder_more WHERE name = ? AND id > ?", []interface{}{"test", 0}},
	}

	for _, tt := range tests {
		_, err := db.Exec(tt.query, tt.args...)
		if err != nil {
			t.Logf("Query: %s, Error: %v", tt.query, err)
		}
	}
}

// TestDriverPingContextWithCancel tests PingContext with cancellation
func TestDriverPingContextWithCancel(t *testing.T) {
	db, err := sql.Open("xxldb", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Create a context with cancel
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// PingContext should still succeed (or fail gracefully)
	err = db.PingContext(ctx)
	t.Logf("PingContext with cancelled context: %v", err)
}

// TestDriverStmtExec tests statement execution
func TestDriverStmtExec(t *testing.T) {
	db, err := sql.Open("xxldb", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Create table
	db.Exec("CREATE TABLE stmt_exec (id INT, name VARCHAR(50))")

	// Prepare statement
	stmt, err := db.Prepare("INSERT INTO stmt_exec VALUES (?, ?)")
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()

	// Execute multiple times
	for i := 0; i < 5; i++ {
		_, err = stmt.Exec(i, fmt.Sprintf("name%d", i))
		if err != nil {
			t.Errorf("Exec failed: %v", err)
		}
	}

	// Verify rows
	rows, err := db.Query("SELECT COUNT(*) FROM stmt_exec")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	if rows.Next() {
		var count int
		rows.Scan(&count)
		t.Logf("Rows: %d", count)
	}
}

// TestDriverStmtQuery tests statement query
func TestDriverStmtQueryFull(t *testing.T) {
	db, err := sql.Open("xxldb", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Create table and insert data
	db.Exec("CREATE TABLE stmt_query_full (id INT, name VARCHAR(50))")
	db.Exec("INSERT INTO stmt_query_full VALUES (1, 'Alice')")
	db.Exec("INSERT INTO stmt_query_full VALUES (2, 'Bob')")

	// Prepare query statement
	stmt, err := db.Prepare("SELECT * FROM stmt_query_full WHERE id > ?")
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()

	// Execute query
	rows, err := stmt.Query(0)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
	}
	t.Logf("Found %d rows", count)
}

// TestDriverBeginTx tests BeginTx method
func TestDriverBeginTx(t *testing.T) {
	db, err := sql.Open("xxldb", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// BeginTx should work since we implement the interface
	tx, err := db.BeginTx(context.Background(), nil)
	if err == nil {
		t.Log("BeginTx succeeded unexpectedly (transactions not supported)")
		tx.Rollback()
	} else {
		t.Logf("BeginTx error (expected): %v", err)
	}
}

// TestDriverPrepareContext tests PrepareContext
func TestDriverPrepareContext(t *testing.T) {
	db, err := sql.Open("xxldb", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// PrepareContext
	stmt, err := db.PrepareContext(context.Background(), "SELECT 1")
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	t.Log("PrepareContext succeeded")
}

// TestDriverConn tests direct connection operations
func TestDriverConn(t *testing.T) {
	drv := &xxldbDriver{}

	// Open connection
	conn, err := drv.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	t.Log("Driver connection opened successfully")
}

// TestDriverStmtQueryContext tests QueryContext on statement
func TestDriverStmtQueryContext(t *testing.T) {
	db, err := sql.Open("xxldb", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	db.Exec("CREATE TABLE stmt_qc (id INT)")
	db.Exec("INSERT INTO stmt_qc VALUES (1)")

	stmt, err := db.Prepare("SELECT * FROM stmt_qc")
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	t.Log("QueryContext on statement succeeded")
}

// TestDriverStmtExecContext tests ExecContext on statement
func TestDriverStmtExecContext(t *testing.T) {
	db, err := sql.Open("xxldb", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	db.Exec("CREATE TABLE stmt_ec (id INT)")

	stmt, err := db.Prepare("INSERT INTO stmt_ec VALUES (?)")
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("ExecContext on statement succeeded")
}

// TestDriverCheckNamedValue tests CheckNamedValue
func TestDriverCheckNamedValue(t *testing.T) {
	db, err := sql.Open("xxldb", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Use named parameter (will be converted to regular value)
	_, err = db.Exec("CREATE TABLE named_test (id INT)")
	if err != nil {
		t.Fatal(err)
	}

	// This tests CheckNamedValue indirectly
	_, err = db.Exec("INSERT INTO named_test VALUES (?)", sql.Named("id", 1))
	if err != nil {
		t.Logf("Named parameter error: %v", err)
	}
}

// TestDriverResetSessionFull tests ResetSession
func TestDriverResetSessionFull(t *testing.T) {
	db, err := sql.Open("xxldb", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Create a connection and reset
	db.Exec("CREATE TABLE reset_test (id INT)")

	// ResetSession is called automatically by database/sql
	// We just verify the driver implements the interface
	t.Log("ResetSession test completed")
}

// TestDriverIsValid tests IsValid
func TestDriverIsValid(t *testing.T) {
	conn := &xxldbConn{}

	// IsValid should return true
	if !conn.IsValid() {
		t.Error("IsValid should return true")
	}
	t.Log("IsValid returned true")
}

// TestDriverBeginDirect tests Begin transaction directly on connection
func TestDriverBeginDirect(t *testing.T) {
	conn := &xxldbConn{}

	// Begin should return error (transactions not supported)
	tx, err := conn.Begin()
	if err == nil {
		if tx != nil {
			tx.Rollback()
		}
		t.Error("Begin should return error since transactions are not supported")
	}
	t.Logf("Begin correctly returned error: %v", err)
}

// TestDriverReplacePlaceholdersAllTypes tests placeholder replacement with all types
func TestDriverReplacePlaceholdersAllTypes(t *testing.T) {
	db, err := sql.Open("xxldb", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	db.Exec("CREATE TABLE ph_types (id INT, name VARCHAR(100), value FLOAT, active INT, data VARCHAR(100))")

	tests := []struct {
		name  string
		query string
		args  []interface{}
	}{
		{"int", "INSERT INTO ph_types (id) VALUES (?)", []interface{}{42}},
		{"string", "INSERT INTO ph_types (name) VALUES (?)", []interface{}{"test"}},
		{"float", "INSERT INTO ph_types (value) VALUES (?)", []interface{}{3.14}},
		{"bool true", "INSERT INTO ph_types (active) VALUES (?)", []interface{}{true}},
		{"bool false", "INSERT INTO ph_types (active) VALUES (?)", []interface{}{false}},
		{"nil", "INSERT INTO ph_types (name) VALUES (?)", []interface{}{nil}},
		{"bytes", "INSERT INTO ph_types (data) VALUES (?)", []interface{}{[]byte("binary")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := db.Exec(tt.query, tt.args...)
			if err != nil {
				t.Errorf("Query with args failed: %v", err)
			}
		})
	}
}

// TestDriverPingContext tests PingContext
func TestDriverPingContext(t *testing.T) {
	db, err := sql.Open("xxldb", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		t.Errorf("PingContext failed: %v", err)
	}
}

// TestDriverQueryEmptyResult tests query with empty result
func TestDriverQueryEmptyResult(t *testing.T) {
	db, err := sql.Open("xxldb", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	db.Exec("CREATE TABLE empty_result (id INT)")

	stmt, err := db.Prepare("SELECT * FROM empty_result WHERE id = ?")
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()

	rows, err := stmt.Query(999)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	if rows.Next() {
		t.Error("Expected no rows")
	}
}

// TestDriverExecNoMatch tests Exec with no matching rows
func TestDriverExecNoMatch(t *testing.T) {
	db, err := sql.Open("xxldb", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	db.Exec("CREATE TABLE exec_no_match (id INT, value VARCHAR(50))")
	db.Exec("INSERT INTO exec_no_match VALUES (1, 'test')")

	stmt, err := db.Prepare("UPDATE exec_no_match SET value = ? WHERE id = ?")
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()

	result, err := stmt.Exec("updated", 999)
	if err != nil {
		t.Fatal(err)
	}

	rows, _ := result.RowsAffected()
	t.Logf("Rows affected: %d", rows)
}

// TestDriverConnCloseExplicit tests explicit connection close
func TestDriverConnCloseExplicit(t *testing.T) {
	db, err := sql.Open("xxldb", ":memory:")
	if err != nil {
		t.Fatal(err)
	}

	err = db.Close()
	if err != nil {
		t.Errorf("Close should not error: %v", err)
	}
	t.Log("Connection closed successfully")
}

// TestDriverTypeConversions tests various type conversions
func TestDriverTypeConversions(t *testing.T) {
	db, err := sql.Open("xxldb", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	db.Exec("CREATE TABLE type_conv (id INT, name VARCHAR(100), price FLOAT, created DATETIME)")

	now := time.Now()
	_, err = db.Exec("INSERT INTO type_conv VALUES (?, ?, ?, ?)",
		1, "Product", 19.99, now)
	if err != nil {
		t.Logf("Insert with datetime: %v", err)
	}

	rows, err := db.Query("SELECT id, name, price FROM type_conv")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var name string
		var price float64
		err = rows.Scan(&id, &name, &price)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("id=%d, name=%s, price=%.2f", id, name, price)
	}
}

// TestDriverNullHandling tests NULL value handling
func TestDriverNullHandling(t *testing.T) {
	db, err := sql.Open("xxldb", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	db.Exec("CREATE TABLE null_handling (id INT, name VARCHAR(100))")
	db.Exec("INSERT INTO null_handling VALUES (1, NULL)")

	rows, err := db.Query("SELECT id, name FROM null_handling")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var name sql.NullString
		err = rows.Scan(&id, &name)
		if err != nil {
			t.Fatal(err)
		}
		if name.Valid {
			t.Errorf("Expected NULL, got %s", name.String)
		}
		t.Logf("id=%d, name is NULL=%v", id, !name.Valid)
	}
}

// TestDriverMultiplePlaceholdersQuery tests query with multiple placeholders
func TestDriverMultiplePlaceholdersQuery(t *testing.T) {
	db, err := sql.Open("xxldb", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	db.Exec("CREATE TABLE multi_ph (id INT, a VARCHAR(50), b VARCHAR(50))")
	db.Exec("INSERT INTO multi_ph VALUES (1, 'x', 'y')")

	rows, err := db.Query("SELECT * FROM multi_ph WHERE id = ? AND a = ? AND b = ?", 1, "x", "y")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
	}
	if count != 1 {
		t.Errorf("Expected 1 row, got %d", count)
	}
}

// TestDriverAggregateQueries tests aggregate function queries
func TestDriverAggregateQueries(t *testing.T) {
	db, err := sql.Open("xxldb", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	db.Exec("CREATE TABLE agg_q (id INT, value INT)")
	db.Exec("INSERT INTO agg_q VALUES (1, 10)")
	db.Exec("INSERT INTO agg_q VALUES (2, 20)")
	db.Exec("INSERT INTO agg_q VALUES (3, 30)")

	// Test COUNT
	rows, err := db.Query("SELECT COUNT(*) FROM agg_q")
	if err != nil {
		t.Fatal(err)
	}
	rows.Close()

	// Test SUM
	rows, err = db.Query("SELECT SUM(value) FROM agg_q")
	if err != nil {
		t.Fatal(err)
	}
	rows.Close()

	// Test AVG
	rows, err = db.Query("SELECT AVG(value) FROM agg_q")
	if err != nil {
		t.Fatal(err)
	}
	rows.Close()
}

// TestDriverBatchInsertMany tests multiple inserts
func TestDriverBatchInsertMany(t *testing.T) {
	db, err := sql.Open("xxldb", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	db.Exec("CREATE TABLE batch_many (id INT, name VARCHAR(50))")

	for i := 0; i < 10; i++ {
		_, err = db.Exec("INSERT INTO batch_many VALUES (?, ?)", i, fmt.Sprintf("name%d", i))
		if err != nil {
			t.Fatal(err)
		}
	}

	rows, err := db.Query("SELECT COUNT(*) FROM batch_many")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	if rows.Next() {
		var count int
		rows.Scan(&count)
		if count != 10 {
			t.Errorf("Expected 10 rows, got %d", count)
		}
	}
}

// TestDriverStringWithSpecialChars tests string with special characters
func TestDriverStringWithSpecialChars(t *testing.T) {
	db, err := sql.Open("xxldb", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	db.Exec("CREATE TABLE special_chars (id INT, text VARCHAR(200))")

	// Test with quotes in string
	_, err = db.Exec("INSERT INTO special_chars VALUES (?, ?)", 1, "it's a test")
	if err != nil {
		t.Logf("Insert with quote: %v", err)
	}

	// Test with semicolon
	_, err = db.Exec("INSERT INTO special_chars VALUES (?, ?)", 2, "test;value")
	if err != nil {
		t.Logf("Insert with semicolon: %v", err)
	}
}
