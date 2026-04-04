package executor

import (
	"testing"
)

func TestBasicCRUD(t *testing.T) {
	// Create in-memory engine
	engine, err := NewEngine("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Create table
	_, err = engine.Execute(`
		CREATE TABLE users (
			id SEQ,
			name VARCHAR(100),
			email VARCHAR(100),
			age INT
		)
	`)
	if err != nil {
		t.Fatal(err)
	}

	// Insert rows
	_, err = engine.Execute(`INSERT INTO users (name, email, age) VALUES ('Alice', 'alice@example.com', 30)`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = engine.Execute(`INSERT INTO users (name, email, age) VALUES ('Bob', 'bob@example.com', 25)`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = engine.Execute(`INSERT INTO users (name, email, age) VALUES ('Charlie', 'charlie@example.com', 35)`)
	if err != nil {
		t.Fatal(err)
	}

	// Select all
	result, err := engine.Execute(`SELECT * FROM users`)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 3 {
		t.Errorf("Expected 3 rows, got %d", len(result.Rows))
	}

	// Select with WHERE
	result, err = engine.Execute(`SELECT * FROM users WHERE age > 28`)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("Expected 2 rows with age > 28, got %d", len(result.Rows))
	}

	// Update
	result, err = engine.Execute(`UPDATE users SET age = 31 WHERE name = 'Alice'`)
	if err != nil {
		t.Fatal(err)
	}
	if result.RowsAffected != 1 {
		t.Errorf("Expected 1 row affected, got %d", result.RowsAffected)
	}

	// Verify update
	result, err = engine.Execute(`SELECT age FROM users WHERE name = 'Alice'`)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 1 {
		t.Fatal("Expected 1 row")
	}
	age, _ := result.Rows[0].Data[0].ToInt64()
	if age != 31 {
		t.Errorf("Expected age 31, got %d", age)
	}

	// Delete
	result, err = engine.Execute(`DELETE FROM users WHERE name = 'Bob'`)
	if err != nil {
		t.Fatal(err)
	}
	if result.RowsAffected != 1 {
		t.Errorf("Expected 1 row deleted, got %d", result.RowsAffected)
	}

	// Verify delete
	result, err = engine.Execute(`SELECT * FROM users`)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("Expected 2 rows after delete, got %d", len(result.Rows))
	}

	// Drop table
	_, err = engine.Execute(`DROP TABLE users`)
	if err != nil {
		t.Fatal(err)
	}

	// Verify table dropped
	result, err = engine.Execute(`SHOW TABLES`)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 1 { // Only xxscript system table
		t.Errorf("Expected 1 system table, got %d rows", len(result.Rows))
	}
}

func TestFunctions(t *testing.T) {
	engine, err := NewEngine("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Create table
	_, err = engine.Execute(`
		CREATE TABLE products (
			id SEQ,
			name VARCHAR(100),
			price FLOAT
		)
	`)
	if err != nil {
		t.Fatal(err)
	}

	// Insert
	_, err = engine.Execute(`INSERT INTO products (name, price) VALUES ('Apple', 1.5)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = engine.Execute(`INSERT INTO products (name, price) VALUES ('Banana', 0.75)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = engine.Execute(`INSERT INTO products (name, price) VALUES ('Orange', 2.0)`)
	if err != nil {
		t.Fatal(err)
	}

	// Test string functions
	result, err := engine.Execute(`SELECT CONCAT(name, ' - $', price) AS item FROM products WHERE name = 'Apple'`)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 1 {
		t.Errorf("Expected 1 row, got %d", len(result.Rows))
	}

	// Test aggregate functions
	result, err = engine.Execute(`SELECT COUNT(*) AS count FROM products`)
	if err != nil {
		t.Fatal(err)
	}
	count, _ := result.Rows[0].Data[0].ToInt64()
	if count != 3 {
		t.Errorf("Expected count 3, got %d", count)
	}

	// Test SUM
	result, err = engine.Execute(`SELECT SUM(price) AS total FROM products`)
	if err != nil {
		t.Fatal(err)
	}
	// SUM should work
	if len(result.Rows) != 1 {
		t.Errorf("Expected 1 row for SUM, got %d", len(result.Rows))
	}
}

func TestOrderBy(t *testing.T) {
	engine, err := NewEngine("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	_, err = engine.Execute(`
		CREATE TABLE scores (
			id SEQ,
			name VARCHAR(50),
			score INT
		)
	`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = engine.Execute(`INSERT INTO scores (name, score) VALUES ('Alice', 85)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = engine.Execute(`INSERT INTO scores (name, score) VALUES ('Bob', 92)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = engine.Execute(`INSERT INTO scores (name, score) VALUES ('Charlie', 78)`)
	if err != nil {
		t.Fatal(err)
	}

	// Test ORDER BY ASC
	result, err := engine.Execute(`SELECT name, score FROM scores ORDER BY score ASC`)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 3 {
		t.Fatalf("Expected 3 rows, got %d", len(result.Rows))
	}
	name := result.Rows[0].Data[0].ToString()
	if name != "Charlie" {
		t.Errorf("Expected Charlie first (ASC), got %s", name)
	}

	// Test ORDER BY DESC
	result, err = engine.Execute(`SELECT name, score FROM scores ORDER BY score DESC`)
	if err != nil {
		t.Fatal(err)
	}
	name = result.Rows[0].Data[0].ToString()
	if name != "Bob" {
		t.Errorf("Expected Bob first (DESC), got %s", name)
	}
}
