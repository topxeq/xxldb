package driver

import (
	"database/sql"
	"testing"
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
