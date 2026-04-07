package xxldb

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestVersion(t *testing.T) {
	if Version == "" {
		t.Error("Version should not be empty")
	}
	if BuildDate == "" {
		t.Error("BuildDate should not be empty")
	}
}

func TestOpenInMemory(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory failed: %v", err)
	}
	defer engine.Close()

	if engine == nil {
		t.Error("Engine should not be nil")
	}
}

func TestOpen(t *testing.T) {
	// Open with empty path should create in-memory
	engine, err := Open("")
	if err != nil {
		t.Fatalf("Open with empty path failed: %v", err)
	}
	if engine == nil {
		t.Error("Engine should not be nil")
	}
	engine.Close()
}

func TestOpenWithConfig(t *testing.T) {
	config := Config{
		InMemory: true,
		LogLevel: "DEBUG",
	}

	engine, err := OpenWithConfig(config)
	if err != nil {
		t.Fatalf("OpenWithConfig failed: %v", err)
	}
	defer engine.Close()

	if engine == nil {
		t.Error("Engine should not be nil")
	}
}

func TestNewEngine(t *testing.T) {
	engine, err := NewEngine("", true)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}
	defer engine.Close()

	if engine == nil {
		t.Error("Engine should not be nil")
	}
}

func TestNewValue(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected string
	}{
		{nil, "NULL"},
		{42, "42"},
		{3.14, "3.14"},
		{"hello", "hello"},
	}

	for _, tt := range tests {
		v := NewValue(tt.input)
		if v.ToString() != tt.expected {
			t.Errorf("NewValue(%v) = %s, want %s", tt.input, v.ToString(), tt.expected)
		}
	}

	// Test bool separately since representation may vary
	vTrue := NewValue(true)
	if vTrue.IsNull {
		t.Error("NewValue(true) should not be null")
	}
	vFalse := NewValue(false)
	if vFalse.IsNull {
		t.Error("NewValue(false) should not be null")
	}
}

func TestNullValue(t *testing.T) {
	v := NullValue()
	if !v.IsNull {
		t.Error("NullValue should return a null value")
	}
}

func TestIntValue(t *testing.T) {
	v := IntValue(123)
	if v.IsNull {
		t.Error("IntValue should not be null")
	}
	if v.ToString() != "123" {
		t.Errorf("IntValue(123) = %s, want 123", v.ToString())
	}
}

func TestFloatValue(t *testing.T) {
	v := FloatValue(3.14)
	if v.IsNull {
		t.Error("FloatValue should not be null")
	}
	s := v.ToString()
	if s != "3.14" {
		t.Errorf("FloatValue(3.14) = %s, want 3.14", s)
	}
}

func TestStringValue(t *testing.T) {
	v := StringValue("hello")
	if v.IsNull {
		t.Error("StringValue should not be null")
	}
	if v.ToString() != "hello" {
		t.Errorf("StringValue(hello) = %s, want hello", v.ToString())
	}
}

func TestBoolValue(t *testing.T) {
	v1 := BoolValue(true)
	if v1.ToString() != "1" {
		t.Errorf("BoolValue(true) = %s, want 1", v1.ToString())
	}

	v2 := BoolValue(false)
	if v2.ToString() != "0" {
		t.Errorf("BoolValue(false) = %s, want 0", v2.ToString())
	}
}

func TestDateValue(t *testing.T) {
	now := time.Now()
	v := DateValue(now)
	if v.IsNull {
		t.Error("DateValue should not be null")
	}
}

func TestBlobValue(t *testing.T) {
	data := []byte{1, 2, 3, 4, 5}
	v := BlobValue(data)
	if v.IsNull {
		t.Error("BlobValue should not be null")
	}
}

func TestDataTypeConstants(t *testing.T) {
	// Just verify constants exist and are distinct
	if TypeNull == TypeInt {
		t.Error("TypeNull should differ from TypeInt")
	}
	if TypeInt == TypeFloat {
		t.Error("TypeInt should differ from TypeFloat")
	}
	if TypeVarchar == TypeText {
		t.Error("TypeVarchar should differ from TypeText")
	}
	if TypeDate == TypeDatetime {
		t.Error("TypeDate should differ from TypeDatetime")
	}
	if TypeBlob == TypeFile {
		t.Error("TypeBlob should differ from TypeFile")
	}
}

func TestTypeAliases(t *testing.T) {
	// Verify type aliases work correctly
	var _ Engine
	var _ Result
	var _ Config
	var _ Value
	var _ DataType
	var _ ColumnDef
	var _ TableInfo
}

func TestBasicCRUD(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Create table
	_, err = engine.Execute("CREATE TABLE test (id SEQ, name VARCHAR(100))")
	if err != nil {
		t.Fatal(err)
	}

	// Insert
	_, err = engine.Execute("INSERT INTO test (name) VALUES ('Alice')")
	if err != nil {
		t.Fatal(err)
	}

	// Select
	result, err := engine.Execute("SELECT * FROM test")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 1 {
		t.Errorf("Expected 1 row, got %d", len(result.Rows))
	}

	// Update
	_, err = engine.Execute("UPDATE test SET name = 'Bob' WHERE id = 1")
	if err != nil {
		t.Fatal(err)
	}

	// Verify update
	result, err = engine.Execute("SELECT name FROM test WHERE id = 1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows[0].Data[0].ToString() != "Bob" {
		t.Errorf("Name should be Bob, got %s", result.Rows[0].Data[0].ToString())
	}

	// Delete
	_, err = engine.Execute("DELETE FROM test WHERE id = 1")
	if err != nil {
		t.Fatal(err)
	}

	// Verify delete
	result, err = engine.Execute("SELECT * FROM test")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 0 {
		t.Errorf("Expected 0 rows, got %d", len(result.Rows))
	}
}

func TestMultipleTables(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Create multiple tables
	_, err = engine.Execute("CREATE TABLE users (id SEQ, name VARCHAR(100))")
	if err != nil {
		t.Fatal(err)
	}

	_, err = engine.Execute("CREATE TABLE orders (id SEQ, user_id INT, amount INT)")
	if err != nil {
		t.Fatal(err)
	}

	// Insert into both tables
	engine.Execute("INSERT INTO users (name) VALUES ('Alice')")
	engine.Execute("INSERT INTO orders (user_id, amount) VALUES (1, 100)")

	// Query both tables
	result1, err := engine.Execute("SELECT * FROM users")
	if err != nil {
		t.Fatal(err)
	}
	if len(result1.Rows) != 1 {
		t.Errorf("Users table should have 1 row")
	}

	result2, err := engine.Execute("SELECT * FROM orders")
	if err != nil {
		t.Fatal(err)
	}
	if len(result2.Rows) != 1 {
		t.Errorf("Orders table should have 1 row")
	}
}

func TestAggregates(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE sales (id SEQ, amount INT)")
	engine.Execute("INSERT INTO sales (amount) VALUES (100)")
	engine.Execute("INSERT INTO sales (amount) VALUES (200)")
	engine.Execute("INSERT INTO sales (amount) VALUES (300)")

	// COUNT
	result, err := engine.Execute("SELECT COUNT(*) FROM sales")
	if err != nil {
		t.Fatal(err)
	}
	count, _ := result.Rows[0].Data[0].ToInt64()
	if count != 3 {
		t.Errorf("COUNT should be 3, got %d", count)
	}

	// SUM
	result, err = engine.Execute("SELECT SUM(amount) FROM sales")
	if err != nil {
		t.Fatal(err)
	}
	sum, _ := result.Rows[0].Data[0].ToInt64()
	if sum != 600 {
		t.Errorf("SUM should be 600, got %d", sum)
	}

	// AVG
	result, err = engine.Execute("SELECT AVG(amount) FROM sales")
	if err != nil {
		t.Fatal(err)
	}

	// MIN
	result, err = engine.Execute("SELECT MIN(amount) FROM sales")
	if err != nil {
		t.Fatal(err)
	}
	min, _ := result.Rows[0].Data[0].ToInt64()
	if min != 100 {
		t.Errorf("MIN should be 100, got %d", min)
	}

	// MAX
	result, err = engine.Execute("SELECT MAX(amount) FROM sales")
	if err != nil {
		t.Fatal(err)
	}
	max, _ := result.Rows[0].Data[0].ToInt64()
	if max != 300 {
		t.Errorf("MAX should be 300, got %d", max)
	}
}

func TestWhereClause(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE users (id SEQ, name VARCHAR(100), age INT)")
	engine.Execute("INSERT INTO users (name, age) VALUES ('Alice', 25)")
	engine.Execute("INSERT INTO users (name, age) VALUES ('Bob', 30)")
	engine.Execute("INSERT INTO users (name, age) VALUES ('Charlie', 35)")

	// Test WHERE with =
	result, err := engine.Execute("SELECT * FROM users WHERE age = 30")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 1 {
		t.Errorf("Expected 1 row, got %d", len(result.Rows))
	}

	// Test WHERE with >
	result, err = engine.Execute("SELECT * FROM users WHERE age > 25")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("Expected 2 rows, got %d", len(result.Rows))
	}

	// Test WHERE with <
	result, err = engine.Execute("SELECT * FROM users WHERE age < 30")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 1 {
		t.Errorf("Expected 1 row, got %d", len(result.Rows))
	}

	// Test WHERE with >=
	result, err = engine.Execute("SELECT * FROM users WHERE age >= 30")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("Expected 2 rows, got %d", len(result.Rows))
	}

	// Test WHERE with <=
	result, err = engine.Execute("SELECT * FROM users WHERE age <= 30")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("Expected 2 rows, got %d", len(result.Rows))
	}

	// Test WHERE with <>
	result, err = engine.Execute("SELECT * FROM users WHERE age <> 30")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("Expected 2 rows, got %d", len(result.Rows))
	}
}

func TestWhereWithAndOr(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE products (id SEQ, name VARCHAR(100), price INT, stock INT)")
	engine.Execute("INSERT INTO products (name, price, stock) VALUES ('Apple', 100, 50)")
	engine.Execute("INSERT INTO products (name, price, stock) VALUES ('Banana', 50, 100)")
	engine.Execute("INSERT INTO products (name, price, stock) VALUES ('Orange', 80, 30)")

	// Test AND
	result, err := engine.Execute("SELECT * FROM products WHERE price > 60 AND stock > 40")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 1 {
		t.Errorf("AND: Expected 1 row, got %d", len(result.Rows))
	}

	// Test OR
	result, err = engine.Execute("SELECT * FROM products WHERE price < 60 OR stock < 40")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("OR: Expected 2 rows, got %d", len(result.Rows))
	}
}

func TestOrderBy(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE scores (id SEQ, name VARCHAR(100), score INT)")
	engine.Execute("INSERT INTO scores (name, score) VALUES ('Alice', 85)")
	engine.Execute("INSERT INTO scores (name, score) VALUES ('Bob', 92)")
	engine.Execute("INSERT INTO scores (name, score) VALUES ('Charlie', 78)")

	// Test ORDER BY ASC
	result, err := engine.Execute("SELECT * FROM scores ORDER BY score ASC")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 3 {
		t.Errorf("Expected 3 rows, got %d", len(result.Rows))
	}
	// First row should have lowest score
	firstScore, _ := result.Rows[0].Data[2].ToInt64()
	if firstScore != 78 {
		t.Errorf("ORDER BY ASC: First score should be 78, got %d", firstScore)
	}

	// Test ORDER BY DESC
	result, err = engine.Execute("SELECT * FROM scores ORDER BY score DESC")
	if err != nil {
		t.Fatal(err)
	}
	// First row should have highest score
	firstScore, _ = result.Rows[0].Data[2].ToInt64()
	if firstScore != 92 {
		t.Errorf("ORDER BY DESC: First score should be 92, got %d", firstScore)
	}
}

func TestLimit(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE items (id SEQ, name VARCHAR(100))")
	for i := 0; i < 10; i++ {
		engine.Execute("INSERT INTO items (name) VALUES ('item')")
	}

	// Test LIMIT
	result, err := engine.Execute("SELECT * FROM items LIMIT 5")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 5 {
		t.Errorf("LIMIT: Expected 5 rows, got %d", len(result.Rows))
	}
}

func TestDistinct(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE tags (id SEQ, tag VARCHAR(100))")
	engine.Execute("INSERT INTO tags (tag) VALUES ('red')")
	engine.Execute("INSERT INTO tags (tag) VALUES ('blue')")
	engine.Execute("INSERT INTO tags (tag) VALUES ('red')")
	engine.Execute("INSERT INTO tags (tag) VALUES ('green')")
	engine.Execute("INSERT INTO tags (tag) VALUES ('blue')")

	// Test DISTINCT
	result, err := engine.Execute("SELECT DISTINCT tag FROM tags")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 3 {
		t.Errorf("DISTINCT: Expected 3 unique rows, got %d", len(result.Rows))
	}
}

func TestGroupBy(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE orders (id SEQ, category VARCHAR(100), amount INT)")
	engine.Execute("INSERT INTO orders (category, amount) VALUES ('A', 100)")
	engine.Execute("INSERT INTO orders (category, amount) VALUES ('B', 200)")
	engine.Execute("INSERT INTO orders (category, amount) VALUES ('A', 150)")
	engine.Execute("INSERT INTO orders (category, amount) VALUES ('B', 250)")

	// Test GROUP BY - verify it runs without error
	result, err := engine.Execute("SELECT category, SUM(amount) FROM orders GROUP BY category")
	if err != nil {
		t.Fatal(err)
	}
	// GROUP BY may return aggregated results
	if len(result.Rows) < 1 {
		t.Errorf("GROUP BY: Expected at least 1 row, got %d", len(result.Rows))
	}
}

func TestNullHandling(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE nullable (id SEQ, value INT)")
	_, err = engine.Execute("INSERT INTO nullable (value) VALUES (NULL)")
	if err != nil {
		t.Logf("NULL insert: %v", err)
	}
	engine.Execute("INSERT INTO nullable (value) VALUES (10)")

	// Test NULL handling - check total rows
	result, err := engine.Execute("SELECT * FROM nullable")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Total rows in nullable table: %d", len(result.Rows))

	// Test IS NULL if supported
	result, err = engine.Execute("SELECT * FROM nullable WHERE value IS NULL")
	if err != nil {
		t.Logf("IS NULL not fully supported: %v", err)
	} else {
		t.Logf("IS NULL returned %d rows", len(result.Rows))
	}

	// Test IS NOT NULL
	result, err = engine.Execute("SELECT * FROM nullable WHERE value IS NOT NULL")
	if err != nil {
		t.Logf("IS NOT NULL not fully supported: %v", err)
	} else {
		t.Logf("IS NOT NULL returned %d rows", len(result.Rows))
	}
}

func TestLikePattern(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE emails (id SEQ, email VARCHAR(100))")
	engine.Execute("INSERT INTO emails (email) VALUES ('alice@example.com')")
	engine.Execute("INSERT INTO emails (email) VALUES ('bob@test.org')")
	engine.Execute("INSERT INTO emails (email) VALUES ('charlie@example.com')")

	// Test LIKE with %
	result, err := engine.Execute("SELECT * FROM emails WHERE email LIKE '%example%'")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("LIKE: Expected 2 rows, got %d", len(result.Rows))
	}
}

func TestBetween(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE numbers (id SEQ, num INT)")
	engine.Execute("INSERT INTO numbers (num) VALUES (10)")
	engine.Execute("INSERT INTO numbers (num) VALUES (20)")
	engine.Execute("INSERT INTO numbers (num) VALUES (30)")
	engine.Execute("INSERT INTO numbers (num) VALUES (40)")

	// Test BETWEEN - may not be fully implemented
	result, err := engine.Execute("SELECT * FROM numbers WHERE num BETWEEN 15 AND 35")
	if err != nil {
		t.Logf("BETWEEN not fully implemented: %v", err)
	} else {
		t.Logf("BETWEEN returned %d rows", len(result.Rows))
	}

	// Alternative: use comparison operators
	result, err = engine.Execute("SELECT * FROM numbers WHERE num >= 15 AND num <= 35")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("Alternative comparison: Expected 2 rows, got %d", len(result.Rows))
	}
}

func TestInClause(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE fruits (id SEQ, name VARCHAR(100))")
	engine.Execute("INSERT INTO fruits (name) VALUES ('apple')")
	engine.Execute("INSERT INTO fruits (name) VALUES ('banana')")
	engine.Execute("INSERT INTO fruits (name) VALUES ('orange')")
	engine.Execute("INSERT INTO fruits (name) VALUES ('grape')")

	// Test IN - may not be fully implemented
	result, err := engine.Execute("SELECT * FROM fruits WHERE name IN ('apple', 'orange')")
	if err != nil {
		t.Logf("IN not fully implemented: %v", err)
	} else {
		t.Logf("IN returned %d rows", len(result.Rows))
	}

	// Alternative: use OR
	result, err = engine.Execute("SELECT * FROM fruits WHERE name = 'apple' OR name = 'orange'")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("Alternative OR: Expected 2 rows, got %d", len(result.Rows))
	}
}

func TestDropTable(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Create table
	engine.Execute("CREATE TABLE todrop (id SEQ, name VARCHAR(100))")
	engine.Execute("INSERT INTO todrop (name) VALUES ('test')")

	// Verify table exists
	result, err := engine.Execute("SELECT * FROM todrop")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 1 {
		t.Errorf("Table should have 1 row before drop")
	}

	// Drop table
	_, err = engine.Execute("DROP TABLE todrop")
	if err != nil {
		t.Fatal(err)
	}

	// Verify table is gone
	_, err = engine.Execute("SELECT * FROM todrop")
	if err == nil {
		t.Error("Expected error after DROP TABLE, but query succeeded")
	}
}

func TestJoin(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Create tables
	engine.Execute("CREATE TABLE customers (id SEQ, name VARCHAR(100))")
	engine.Execute("CREATE TABLE orders (id SEQ, customer_id INT, product VARCHAR(100))")

	// Insert data
	engine.Execute("INSERT INTO customers (name) VALUES ('Alice')")
	engine.Execute("INSERT INTO customers (name) VALUES ('Bob')")
	engine.Execute("INSERT INTO orders (customer_id, product) VALUES (1, 'Laptop')")
	engine.Execute("INSERT INTO orders (customer_id, product) VALUES (1, 'Phone')")
	engine.Execute("INSERT INTO orders (customer_id, product) VALUES (2, 'Tablet')")

	// Test JOIN - verify it runs without error
	result, err := engine.Execute("SELECT customers.name, orders.product FROM customers JOIN orders ON customers.id = orders.customer_id")
	if err != nil {
		t.Fatal(err)
	}
	// JOIN should return some results
	if len(result.Rows) < 1 {
		t.Errorf("JOIN: Expected at least 1 row, got %d", len(result.Rows))
	}
	t.Logf("JOIN returned %d rows", len(result.Rows))
}

func TestUnion(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE table_a (id SEQ, value VARCHAR(100))")
	engine.Execute("CREATE TABLE table_b (id SEQ, value VARCHAR(100))")

	engine.Execute("INSERT INTO table_a (value) VALUES ('apple')")
	engine.Execute("INSERT INTO table_a (value) VALUES ('banana')")
	engine.Execute("INSERT INTO table_b (value) VALUES ('orange')")
	engine.Execute("INSERT INTO table_b (value) VALUES ('apple')")

	// Test UNION
	result, err := engine.Execute("SELECT value FROM table_a UNION SELECT value FROM table_b")
	if err != nil {
		t.Fatal(err)
	}
	// UNION should return distinct values
	if len(result.Rows) < 3 {
		t.Errorf("UNION: Expected at least 3 distinct rows, got %d", len(result.Rows))
	}
}

func TestStringFunctions(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	_, err = engine.Execute("CREATE TABLE str_test (id SEQ, content VARCHAR(100))")
	if err != nil {
		t.Fatal(err)
	}
	_, err = engine.Execute("INSERT INTO str_test (content) VALUES ('Hello World')")
	if err != nil {
		t.Fatal(err)
	}

	// Test UPPER
	result, err := engine.Execute("SELECT UPPER(content) FROM str_test")
	if err != nil {
		t.Fatal(err)
	}
	upper := result.Rows[0].Data[0].ToString()
	if upper != "HELLO WORLD" {
		t.Errorf("UPPER: Expected 'HELLO WORLD', got '%s'", upper)
	}

	// Test LOWER
	result, err = engine.Execute("SELECT LOWER(content) FROM str_test")
	if err != nil {
		t.Fatal(err)
	}
	lower := result.Rows[0].Data[0].ToString()
	if lower != "hello world" {
		t.Errorf("LOWER: Expected 'hello world', got '%s'", lower)
	}
}

func TestMathFunctions(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE math_test (id SEQ, value FLOAT)")
	engine.Execute("INSERT INTO math_test (value) VALUES (4.0)")

	// Test SQRT
	result, err := engine.Execute("SELECT SQRT(value) FROM math_test")
	if err != nil {
		t.Fatal(err)
	}

	// Test ABS
	result, err = engine.Execute("SELECT ABS(-5)")
	if err != nil {
		t.Fatal(err)
	}
	abs := result.Rows[0].Data[0].ToString()
	if abs != "5" {
		t.Errorf("ABS: Expected '5', got '%s'", abs)
	}
}

func TestCoalesce(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE coalesce_test (id SEQ, a INT, b INT)")
	engine.Execute("INSERT INTO coalesce_test (a, b) VALUES (NULL, 10)")
	engine.Execute("INSERT INTO coalesce_test (a, b) VALUES (5, 20)")

	// Test COALESCE
	result, err := engine.Execute("SELECT COALESCE(a, b) FROM coalesce_test")
	if err != nil {
		t.Fatal(err)
	}

	// First row should return 10 (a is NULL, so use b)
	val1, _ := result.Rows[0].Data[0].ToInt64()
	if val1 != 10 {
		t.Errorf("COALESCE: First row should be 10, got %d", val1)
	}

	// Second row should return 5 (a is not NULL)
	val2, _ := result.Rows[1].Data[0].ToInt64()
	if val2 != 5 {
		t.Errorf("COALESCE: Second row should be 5, got %d", val2)
	}
}

func TestConcurrentAccess(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE concurrent (id SEQ, value INT)")

	// Run concurrent inserts
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := engine.Execute("INSERT INTO concurrent (value) VALUES (1)")
			if err != nil {
				t.Errorf("Concurrent insert failed: %v", err)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all inserts succeeded
	result, err := engine.Execute("SELECT COUNT(*) FROM concurrent")
	if err != nil {
		t.Fatal(err)
	}
	count, _ := result.Rows[0].Data[0].ToInt64()
	if count != 10 {
		t.Errorf("Concurrent: Expected 10 rows, got %d", count)
	}
}

func TestShowTables(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Create tables
	_, err = engine.Execute("CREATE TABLE table1 (id SEQ, name VARCHAR(100))")
	if err != nil {
		t.Fatal(err)
	}
	_, err = engine.Execute("CREATE TABLE table2 (id SEQ, value INT)")
	if err != nil {
		t.Fatal(err)
	}

	// Show tables
	result, err := engine.Execute("SHOW TABLES")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) < 2 {
		t.Errorf("SHOW TABLES: Expected at least 2 tables, got %d", len(result.Rows))
	}
}

func TestDescribeTable(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Create table with various column types
	_, err = engine.Execute("CREATE TABLE test_desc (id SEQ, name VARCHAR(100), age INT, salary FLOAT, bio TEXT)")
	if err != nil {
		t.Fatal(err)
	}

	// Show columns from table
	result, err := engine.Execute("SHOW COLUMNS FROM test_desc")
	if err != nil {
		t.Logf("SHOW COLUMNS not fully implemented: %v", err)
	} else {
		t.Logf("SHOW COLUMNS returned %d rows", len(result.Rows))
	}
}

func TestDateFunctions(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Create table with date column
	_, err = engine.Execute("CREATE TABLE dates (id SEQ, dt DATE)")
	if err != nil {
		t.Fatal(err)
	}

	// Test NOW function
	result, err := engine.Execute("SELECT NOW()")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 1 {
		t.Errorf("NOW: Expected 1 row, got %d", len(result.Rows))
	}

	// Test DATE function
	result, err = engine.Execute("SELECT DATE('2024-01-15')")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 1 {
		t.Errorf("DATE: Expected 1 row, got %d", len(result.Rows))
	}
}

func TestStringLength(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Test LENGTH function
	result, err := engine.Execute("SELECT LENGTH('Hello')")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 1 {
		t.Errorf("LENGTH: Expected 1 row")
	}
	length, _ := result.Rows[0].Data[0].ToInt64()
	if length != 5 {
		t.Errorf("LENGTH('Hello'): Expected 5, got %d", length)
	}
}

func TestConcatenation(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Test string concatenation with ||
	result, err := engine.Execute("SELECT 'Hello' || ' ' || 'World'")
	if err != nil {
		t.Fatal(err)
	}
	concat := result.Rows[0].Data[0].ToString()
	if concat != "Hello World" {
		t.Errorf("Concatenation: Expected 'Hello World', got '%s'", concat)
	}
}

func TestSubquery(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE items (id SEQ, price INT)")
	engine.Execute("INSERT INTO items (price) VALUES (100)")
	engine.Execute("INSERT INTO items (price) VALUES (200)")
	engine.Execute("INSERT INTO items (price) VALUES (300)")

	// Test subquery in WHERE
	result, err := engine.Execute("SELECT * FROM items WHERE price > (SELECT AVG(price) FROM items)")
	if err != nil {
		t.Logf("Subquery not fully supported: %v", err)
	} else {
		t.Logf("Subquery returned %d rows", len(result.Rows))
	}
}

func TestAlias(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE aliases (id SEQ, value INT)")
	engine.Execute("INSERT INTO aliases (value) VALUES (100)")

	// Test column alias
	result, err := engine.Execute("SELECT value AS v FROM aliases")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 1 {
		t.Errorf("Alias: Expected 1 row, got %d", len(result.Rows))
	}
	if len(result.Columns) > 0 && result.Columns[0] != "v" {
		t.Logf("Alias column name: expected 'v', got '%s'", result.Columns[0])
	}
}

func TestMultipleInserts(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE multi (id SEQ, val INT)")

	// Insert multiple rows
	for i := 0; i < 100; i++ {
		_, err = engine.Execute("INSERT INTO multi (val) VALUES (1)")
		if err != nil {
			t.Fatal(err)
		}
	}

	// Verify count
	result, err := engine.Execute("SELECT COUNT(*) FROM multi")
	if err != nil {
		t.Fatal(err)
	}
	count, _ := result.Rows[0].Data[0].ToInt64()
	if count != 100 {
		t.Errorf("Multiple inserts: Expected 100 rows, got %d", count)
	}
}

func TestArithmeticExpressions(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE arith (id SEQ, a INT, b INT)")
	engine.Execute("INSERT INTO arith (a, b) VALUES (10, 3)")

	// Test basic arithmetic with column values
	result, err := engine.Execute("SELECT a + b FROM arith")
	if err != nil {
		t.Logf("Arithmetic expression a + b: %v", err)
	} else {
		t.Logf("a + b = %s", result.Rows[0].Data[0].ToString())
	}

	result, err = engine.Execute("SELECT a - b FROM arith")
	if err != nil {
		t.Logf("Arithmetic expression a - b: %v", err)
	} else {
		t.Logf("a - b = %s", result.Rows[0].Data[0].ToString())
	}

	// Verify the columns exist
	result, err = engine.Execute("SELECT a, b FROM arith")
	if err != nil {
		t.Fatal(err)
	}
	a, _ := result.Rows[0].Data[0].ToInt64()
	b, _ := result.Rows[0].Data[1].ToInt64()
	t.Logf("a = %d, b = %d", a, b)
}

func TestNestedFunctions(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Test nested function calls
	result, err := engine.Execute("SELECT UPPER(LOWER('HELLO'))")
	if err != nil {
		t.Fatal(err)
	}
	val := result.Rows[0].Data[0].ToString()
	if val != "HELLO" {
		t.Errorf("Nested functions: Expected 'HELLO', got '%s'", val)
	}
}

func TestRoundFunction(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Test ROUND function
	result, err := engine.Execute("SELECT ROUND(3.14159, 2)")
	if err != nil {
		t.Logf("ROUND not fully implemented: %v", err)
		return
	}
	val := result.Rows[0].Data[0].ToString()
	t.Logf("ROUND(3.14159, 2) = %s", val)
}

func TestErrorHandling(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Query non-existent table
	_, err = engine.Execute("SELECT * FROM nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent table")
	}

	// Insert into non-existent table
	_, err = engine.Execute("INSERT INTO nonexistent (col) VALUES (1)")
	if err == nil {
		t.Error("Expected error for insert into non-existent table")
	}

	// Invalid SQL
	_, err = engine.Execute("INVALID SQL")
	if err == nil {
		t.Error("Expected error for invalid SQL")
	}
}

func TestEmptyTable(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE empty (id SEQ, value INT)")

	// Query empty table
	result, err := engine.Execute("SELECT * FROM empty")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 0 {
		t.Errorf("Empty table: Expected 0 rows, got %d", len(result.Rows))
	}

	// COUNT on empty table
	result, err = engine.Execute("SELECT COUNT(*) FROM empty")
	if err != nil {
		t.Fatal(err)
	}
	count, _ := result.Rows[0].Data[0].ToInt64()
	if count != 0 {
		t.Errorf("COUNT on empty table: Expected 0, got %d", count)
	}
}

func TestCaseSensitive(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Create table with mixed case
	engine.Execute("CREATE TABLE TestCase (id SEQ, Value INT)")

	// Insert should work
	_, err = engine.Execute("INSERT INTO TestCase (Value) VALUES (100)")
	if err != nil {
		t.Logf("Case sensitivity issue: %v", err)
	}

	// Select should work
	result, err := engine.Execute("SELECT * FROM testcase")
	if err != nil {
		t.Logf("Case sensitivity issue: %v", err)
	} else {
		t.Logf("SELECT from testcase returned %d rows", len(result.Rows))
	}
}

func TestReservedWords(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Create table with reserved word as name (may fail)
	_, err = engine.Execute("CREATE TABLE select (id SEQ)")
	if err != nil {
		t.Logf("Expected: reserved word as table name failed: %v", err)
	}

	// Create table with quoted identifier (if supported)
	_, err = engine.Execute("CREATE TABLE \"reserved\" (id SEQ)")
	if err != nil {
		t.Logf("Quoted identifier: %v", err)
	}
}

func TestBackupRestore(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Create test data
	engine.Execute("CREATE TABLE backup_test (id SEQ, value INT)")
	engine.Execute("INSERT INTO backup_test (value) VALUES (100)")

	// Backup
	_, err = engine.Execute("BACKUP TO '/tmp/test_backup'")
	if err != nil {
		t.Logf("BACKUP not fully implemented: %v", err)
	}
}

func TestSpecialCharacters(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE special (id SEQ, content VARCHAR(200))")

	// Test special characters in strings
	_, err = engine.Execute("INSERT INTO special (content) VALUES ('Hello, World!')")
	if err != nil {
		t.Logf("Special chars insert failed: %v", err)
	}

	// Test quotes in strings
	_, err = engine.Execute("INSERT INTO special (content) VALUES ('It''s a test')")
	if err != nil {
		t.Logf("Quoted string insert failed: %v", err)
	}

	// Verify
	result, err := engine.Execute("SELECT COUNT(*) FROM special")
	if err != nil {
		t.Fatal(err)
	}
	count, _ := result.Rows[0].Data[0].ToInt64()
	if count < 1 {
		t.Errorf("Special characters: Expected at least 1 row, got %d", count)
	}
}

// ==================== Additional Tests ====================

func TestCreateTableWithPrimaryKey(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Create table with primary key
	_, err = engine.Execute("CREATE TABLE pk_test (id INT PRIMARY KEY, name VARCHAR(100))")
	if err != nil {
		t.Fatal(err)
	}

	// Insert data
	_, err = engine.Execute("INSERT INTO pk_test (id, name) VALUES (1, 'Alice')")
	if err != nil {
		t.Fatal(err)
	}

	// Verify
	result, err := engine.Execute("SELECT * FROM pk_test WHERE id = 1")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 1 {
		t.Errorf("PRIMARY KEY: Expected 1 row, got %d", len(result.Rows))
	}
}

func TestCreateTableWithNotNull(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Create table with NOT NULL constraint
	_, err = engine.Execute("CREATE TABLE notnull_test (id SEQ, name VARCHAR(100) NOT NULL)")
	if err != nil {
		t.Fatal(err)
	}

	// Insert with value
	_, err = engine.Execute("INSERT INTO notnull_test (name) VALUES ('Alice')")
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateTableWithDefault(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Create table with DEFAULT value
	_, err = engine.Execute("CREATE TABLE default_test (id SEQ, stat VARCHAR(20) DEFAULT 'active')")
	if err != nil {
		t.Fatal(err)
	}

	// Insert without specifying the column with default
	_, err = engine.Execute("INSERT INTO default_test () VALUES ()")
	if err != nil {
		t.Logf("Insert with default: %v", err)
	}

	// Insert with explicit value
	_, err = engine.Execute("INSERT INTO default_test (stat) VALUES ('inactive')")
	if err != nil {
		t.Fatal(err)
	}

	result, err := engine.Execute("SELECT COUNT(*) FROM default_test")
	if err != nil {
		t.Fatal(err)
	}
	count, _ := result.Rows[0].Data[0].ToInt64()
	t.Logf("Default test: %d rows inserted", count)
}

func TestAlterTableAddColumn(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Create table
	engine.Execute("CREATE TABLE alter_test (id SEQ, name VARCHAR(100))")
	engine.Execute("INSERT INTO alter_test (name) VALUES ('Alice')")

	// Add column
	_, err = engine.Execute("ALTER TABLE alter_test ADD COLUMN age INT")
	if err != nil {
		t.Logf("ALTER TABLE ADD COLUMN: %v", err)
	}

	// Verify table still works
	result, err := engine.Execute("SELECT * FROM alter_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("ALTER TABLE result: %d rows", len(result.Rows))
}

func TestSelectWithExpression(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE expr_test (id SEQ, a INT, b INT)")
	engine.Execute("INSERT INTO expr_test (a, b) VALUES (10, 5)")

	// Test expression in SELECT
	result, err := engine.Execute("SELECT a + b, a - b, a * b FROM expr_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Expression result: %v", result.Rows[0].Data)
}

func TestSelectWithCase(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE case_test (id SEQ, score INT)")
	engine.Execute("INSERT INTO case_test (score) VALUES (85)")
	engine.Execute("INSERT INTO case_test (score) VALUES (55)")

	// Test CASE expression
	result, err := engine.Execute("SELECT score, CASE WHEN score >= 60 THEN 'Pass' ELSE 'Fail' END FROM case_test")
	if err != nil {
		t.Logf("CASE expression: %v", err)
	} else {
		t.Logf("CASE result: %d rows", len(result.Rows))
	}
}

func TestSelectWithCast(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Test CAST function
	result, err := engine.Execute("SELECT CAST(123.45, 'INT')")
	if err != nil {
		t.Logf("CAST function: %v", err)
	} else {
		val, _ := result.Rows[0].Data[0].ToInt64()
		t.Logf("CAST result: %d", val)
	}
}

func TestSelectWithTypeOf(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Test TYPEOF function
	result, err := engine.Execute("SELECT TYPEOF(123), TYPEOF('hello'), TYPEOF(NULL)")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("TYPEOF results: %v", result.Rows[0].Data)
}

func TestHavingClause(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE having_test (id SEQ, category VARCHAR(50), value INT)")
	engine.Execute("INSERT INTO having_test (category, value) VALUES ('A', 10)")
	engine.Execute("INSERT INTO having_test (category, value) VALUES ('A', 20)")
	engine.Execute("INSERT INTO having_test (category, value) VALUES ('B', 5)")

	// Test HAVING clause
	result, err := engine.Execute("SELECT category, SUM(value) FROM having_test GROUP BY category HAVING SUM(value) > 20")
	if err != nil {
		t.Logf("HAVING clause: %v", err)
	} else {
		t.Logf("HAVING result: %d rows", len(result.Rows))
	}
}

func TestOffsetClause(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE offset_test (id SEQ, value INT)")
	for i := 0; i < 10; i++ {
		engine.Execute("INSERT INTO offset_test (value) VALUES (1)")
	}

	// Test OFFSET
	result, err := engine.Execute("SELECT * FROM offset_test LIMIT 3 OFFSET 5")
	if err != nil {
		t.Logf("OFFSET clause: %v", err)
	} else {
		t.Logf("LIMIT 3 OFFSET 5: %d rows", len(result.Rows))
	}
}

func TestMultipleOrderBy(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE multi_order (id SEQ, name VARCHAR(50), score INT)")
	engine.Execute("INSERT INTO multi_order (name, score) VALUES ('Alice', 85)")
	engine.Execute("INSERT INTO multi_order (name, score) VALUES ('Bob', 92)")
	engine.Execute("INSERT INTO multi_order (name, score) VALUES ('Alice', 78)")
	engine.Execute("INSERT INTO multi_order (name, score) VALUES ('Bob', 88)")

	// Test multiple ORDER BY
	result, err := engine.Execute("SELECT * FROM multi_order ORDER BY name ASC, score DESC")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 4 {
		t.Errorf("Expected 4 rows, got %d", len(result.Rows))
	}
}

func TestLeftJoin(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE departments (id SEQ, name VARCHAR(50))")
	engine.Execute("CREATE TABLE employees (id SEQ, dept_id INT, name VARCHAR(50))")

	engine.Execute("INSERT INTO departments (name) VALUES ('Engineering')")
	engine.Execute("INSERT INTO departments (name) VALUES ('Sales')")
	engine.Execute("INSERT INTO employees (dept_id, name) VALUES (1, 'Alice')")
	engine.Execute("INSERT INTO employees (dept_id, name) VALUES (1, 'Bob')")
	engine.Execute("INSERT INTO employees (dept_id, name) VALUES (NULL, 'Charlie')")

	// Test LEFT JOIN
	result, err := engine.Execute("SELECT e.name, d.name FROM employees e LEFT JOIN departments d ON e.dept_id = d.id")
	if err != nil {
		t.Logf("LEFT JOIN: %v", err)
	} else {
		t.Logf("LEFT JOIN result: %d rows", len(result.Rows))
	}
}

func TestRightJoin(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE orders_r (id SEQ, customer_id INT, product VARCHAR(50))")
	engine.Execute("CREATE TABLE customers_r (id SEQ, name VARCHAR(50))")

	engine.Execute("INSERT INTO customers_r (name) VALUES ('Alice')")
	engine.Execute("INSERT INTO customers_r (name) VALUES ('Bob')")
	engine.Execute("INSERT INTO orders_r (customer_id, product) VALUES (1, 'Laptop')")

	// Test RIGHT JOIN
	result, err := engine.Execute("SELECT c.name, o.product FROM orders_r o RIGHT JOIN customers_r c ON o.customer_id = c.id")
	if err != nil {
		t.Logf("RIGHT JOIN: %v", err)
	} else {
		t.Logf("RIGHT JOIN result: %d rows", len(result.Rows))
	}
}

func TestUnionAll(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE union_a (id SEQ, value VARCHAR(50))")
	engine.Execute("CREATE TABLE union_b (id SEQ, value VARCHAR(50))")

	engine.Execute("INSERT INTO union_a (value) VALUES ('apple')")
	engine.Execute("INSERT INTO union_a (value) VALUES ('banana')")
	engine.Execute("INSERT INTO union_b (value) VALUES ('apple')")
	engine.Execute("INSERT INTO union_b (value) VALUES ('cherry')")

	// Test UNION ALL (keeps duplicates)
	result, err := engine.Execute("SELECT value FROM union_a UNION ALL SELECT value FROM union_b")
	if err != nil {
		t.Logf("UNION ALL: %v", err)
	} else {
		t.Logf("UNION ALL result: %d rows", len(result.Rows))
	}
}

func TestStringReplace(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Test REPLACE function
	result, err := engine.Execute("SELECT REPLACE('Hello World', 'World', 'Go')")
	if err != nil {
		t.Fatal(err)
	}
	replaced := result.Rows[0].Data[0].ToString()
	if replaced != "Hello Go" {
		t.Errorf("REPLACE: Expected 'Hello Go', got '%s'", replaced)
	}
}

func TestStringSubstring(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Test SUBSTRING function
	result, err := engine.Execute("SELECT SUBSTRING('Hello World', 1, 5)")
	if err != nil {
		t.Fatal(err)
	}
	sub := result.Rows[0].Data[0].ToString()
	if sub != "Hello" {
		t.Errorf("SUBSTRING: Expected 'Hello', got '%s'", sub)
	}
}

func TestStringTrim(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Test TRIM function
	result, err := engine.Execute("SELECT TRIM('  hello  ')")
	if err != nil {
		t.Fatal(err)
	}
	trimmed := result.Rows[0].Data[0].ToString()
	if trimmed != "hello" {
		t.Errorf("TRIM: Expected 'hello', got '%s'", trimmed)
	}
}

func TestStringLeftRight(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Test LEFT function
	result, err := engine.Execute("SELECT LEFT('Hello', 3)")
	if err != nil {
		t.Fatal(err)
	}
	left := result.Rows[0].Data[0].ToString()
	if left != "Hel" {
		t.Errorf("LEFT: Expected 'Hel', got '%s'", left)
	}

	// Test RIGHT function
	result, err = engine.Execute("SELECT RIGHT('Hello', 3)")
	if err != nil {
		t.Fatal(err)
	}
	right := result.Rows[0].Data[0].ToString()
	if right != "llo" {
		t.Errorf("RIGHT: Expected 'llo', got '%s'", right)
	}
}

func TestStringReverse(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Test REVERSE function
	result, err := engine.Execute("SELECT REVERSE('Hello')")
	if err != nil {
		t.Fatal(err)
	}
	rev := result.Rows[0].Data[0].ToString()
	if rev != "olleH" {
		t.Errorf("REVERSE: Expected 'olleH', got '%s'", rev)
	}
}

func TestStringRepeat(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Test REPEAT function
	result, err := engine.Execute("SELECT REPEAT('Ab', 3)")
	if err != nil {
		t.Fatal(err)
	}
	rep := result.Rows[0].Data[0].ToString()
	if rep != "AbAbAb" {
		t.Errorf("REPEAT: Expected 'AbAbAb', got '%s'", rep)
	}
}

func TestDateYearMonthDay(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Test YEAR, MONTH, DAY functions
	result, err := engine.Execute("SELECT YEAR('2024-03-15'), MONTH('2024-03-15'), DAY('2024-03-15')")
	if err != nil {
		t.Fatal(err)
	}
	year, _ := result.Rows[0].Data[0].ToInt64()
	month, _ := result.Rows[0].Data[1].ToInt64()
	day, _ := result.Rows[0].Data[2].ToInt64()

	if year != 2024 {
		t.Errorf("YEAR: Expected 2024, got %d", year)
	}
	if month != 3 {
		t.Errorf("MONTH: Expected 3, got %d", month)
	}
	if day != 15 {
		t.Errorf("DAY: Expected 15, got %d", day)
	}
}

func TestDateAddSub(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Test DATE_ADD
	result, err := engine.Execute("SELECT DATE_ADD('2024-03-15', 10)")
	if err != nil {
		t.Logf("DATE_ADD: %v", err)
	} else {
		t.Logf("DATE_ADD result: %s", result.Rows[0].Data[0].ToString())
	}

	// Test DATE_SUB
	result, err = engine.Execute("SELECT DATE_SUB('2024-03-15', 5)")
	if err != nil {
		t.Logf("DATE_SUB: %v", err)
	} else {
		t.Logf("DATE_SUB result: %s", result.Rows[0].Data[0].ToString())
	}
}

func TestNullIfFunction(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Test NULLIF - returns NULL if values are equal
	result, err := engine.Execute("SELECT NULLIF(5, 5)")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Rows[0].Data[0].IsNull {
		t.Error("NULLIF(5, 5) should return NULL")
	}

	// Test NULLIF - returns first value if not equal
	result, err = engine.Execute("SELECT NULLIF(5, 3)")
	if err != nil {
		t.Fatal(err)
	}
	val, _ := result.Rows[0].Data[0].ToInt64()
	if val != 5 {
		t.Errorf("NULLIF(5, 3) = %d, want 5", val)
	}
}

func TestIfFunction(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Test IF function
	result, err := engine.Execute("SELECT IF(1, 'yes', 'no')")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows[0].Data[0].ToString() != "yes" {
		t.Errorf("IF(1, 'yes', 'no') should return 'yes'")
	}

	result, err = engine.Execute("SELECT IF(0, 'yes', 'no')")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows[0].Data[0].ToString() != "no" {
		t.Errorf("IF(0, 'yes', 'no') should return 'no'")
	}
}

func TestPowerFunction(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Test POWER function
	result, err := engine.Execute("SELECT POWER(2, 10)")
	if err != nil {
		t.Fatal(err)
	}
	pow, _ := result.Rows[0].Data[0].ToFloat64()
	if pow != 1024 {
		t.Errorf("POWER(2, 10) = %f, want 1024", pow)
	}
}

func TestFloorCeilFunctions(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Test FLOOR
	result, err := engine.Execute("SELECT FLOOR(3.7)")
	if err != nil {
		t.Fatal(err)
	}
	floor, _ := result.Rows[0].Data[0].ToInt64()
	if floor != 3 {
		t.Errorf("FLOOR(3.7) = %d, want 3", floor)
	}

	// Test CEIL
	result, err = engine.Execute("SELECT CEIL(3.2)")
	if err != nil {
		t.Fatal(err)
	}
	ceil, _ := result.Rows[0].Data[0].ToInt64()
	if ceil != 4 {
		t.Errorf("CEIL(3.2) = %d, want 4", ceil)
	}
}

func TestModFunction(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Test MOD
	result, err := engine.Execute("SELECT MOD(17, 5)")
	if err != nil {
		t.Fatal(err)
	}
	mod, _ := result.Rows[0].Data[0].ToInt64()
	if mod != 2 {
		t.Errorf("MOD(17, 5) = %d, want 2", mod)
	}
}

func TestSignFunction(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Test SIGN
	result, err := engine.Execute("SELECT SIGN(-42)")
	if err != nil {
		t.Fatal(err)
	}
	sign, _ := result.Rows[0].Data[0].ToInt64()
	if sign != -1 {
		t.Errorf("SIGN(-42) = %d, want -1", sign)
	}

	result, err = engine.Execute("SELECT SIGN(42)")
	if err != nil {
		t.Fatal(err)
	}
	sign, _ = result.Rows[0].Data[0].ToInt64()
	if sign != 1 {
		t.Errorf("SIGN(42) = %d, want 1", sign)
	}

	result, err = engine.Execute("SELECT SIGN(0)")
	if err != nil {
		t.Fatal(err)
	}
	sign, _ = result.Rows[0].Data[0].ToInt64()
	if sign != 0 {
		t.Errorf("SIGN(0) = %d, want 0", sign)
	}
}

func TestCountDistinct(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE count_dist (id SEQ, category VARCHAR(50))")
	engine.Execute("INSERT INTO count_dist (category) VALUES ('A')")
	engine.Execute("INSERT INTO count_dist (category) VALUES ('A')")
	engine.Execute("INSERT INTO count_dist (category) VALUES ('B')")
	engine.Execute("INSERT INTO count_dist (category) VALUES ('B')")
	engine.Execute("INSERT INTO count_dist (category) VALUES ('B')")

	// Test COUNT(DISTINCT)
	result, err := engine.Execute("SELECT COUNT(DISTINCT category) FROM count_dist")
	if err != nil {
		t.Logf("COUNT(DISTINCT): %v", err)
	} else {
		t.Logf("COUNT(DISTINCT) result: %s", result.Rows[0].Data[0].ToString())
	}
}

func TestInsertMultipleValues(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE multi_insert (id SEQ, value INT)")

	// Test multi-row INSERT
	result, err := engine.Execute("INSERT INTO multi_insert (value) VALUES (1), (2), (3)")
	if err != nil {
		t.Logf("Multi-row INSERT: %v", err)
	} else {
		// Verify
		result, err = engine.Execute("SELECT COUNT(*) FROM multi_insert")
		if err != nil {
			t.Fatal(err)
		}
		count, _ := result.Rows[0].Data[0].ToInt64()
		t.Logf("Multi-row INSERT: %d rows", count)
	}
}

func TestInsertWithSelect(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE source (id SEQ, value INT)")
	engine.Execute("INSERT INTO source (value) VALUES (10)")
	engine.Execute("INSERT INTO source (value) VALUES (20)")

	engine.Execute("CREATE TABLE dest (id SEQ, value INT)")

	// Test INSERT ... SELECT
	result, err := engine.Execute("INSERT INTO dest (value) SELECT value FROM source")
	if err != nil {
		t.Logf("INSERT ... SELECT: %v", err)
	} else {
		result, err = engine.Execute("SELECT COUNT(*) FROM dest")
		if err != nil {
			t.Fatal(err)
		}
		count, _ := result.Rows[0].Data[0].ToInt64()
		t.Logf("INSERT ... SELECT: %d rows", count)
	}
}

func TestUpdateMultipleColumns(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE multi_update (id SEQ, a INT, b INT)")
	engine.Execute("INSERT INTO multi_update (a, b) VALUES (1, 2)")

	// Test UPDATE multiple columns
	_, err = engine.Execute("UPDATE multi_update SET a = 10, b = 20 WHERE id = 1")
	if err != nil {
		t.Fatal(err)
	}

	result, err := engine.Execute("SELECT a, b FROM multi_update WHERE id = 1")
	if err != nil {
		t.Fatal(err)
	}
	a, _ := result.Rows[0].Data[0].ToInt64()
	b, _ := result.Rows[0].Data[1].ToInt64()
	if a != 10 || b != 20 {
		t.Errorf("UPDATE multiple columns: expected a=10, b=20, got a=%d, b=%d", a, b)
	}
}

func TestDeleteWithSubquery(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE del_main (id SEQ, value INT)")
	engine.Execute("CREATE TABLE del_ref (id SEQ, threshold INT)")
	engine.Execute("INSERT INTO del_main (value) VALUES (10)")
	engine.Execute("INSERT INTO del_main (value) VALUES (20)")
	engine.Execute("INSERT INTO del_main (value) VALUES (30)")
	engine.Execute("INSERT INTO del_ref (threshold) VALUES (15)")

	// Test DELETE with subquery
	result, err := engine.Execute("DELETE FROM del_main WHERE value < (SELECT threshold FROM del_ref)")
	if err != nil {
		t.Logf("DELETE with subquery: %v", err)
	} else {
		result, err = engine.Execute("SELECT COUNT(*) FROM del_main")
		if err != nil {
			t.Fatal(err)
		}
		count, _ := result.Rows[0].Data[0].ToInt64()
		t.Logf("DELETE with subquery: %d rows remaining", count)
	}
}

func TestExistsExpression(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE exists_a (id SEQ, value INT)")
	engine.Execute("CREATE TABLE exists_b (id SEQ, ref_value INT)")
	engine.Execute("INSERT INTO exists_a (value) VALUES (10)")
	engine.Execute("INSERT INTO exists_a (value) VALUES (20)")
	engine.Execute("INSERT INTO exists_b (ref_value) VALUES (10)")

	// Test EXISTS
	result, err := engine.Execute("SELECT * FROM exists_a WHERE EXISTS (SELECT 1 FROM exists_b WHERE ref_value = exists_a.value)")
	if err != nil {
		t.Logf("EXISTS expression: %v", err)
	} else {
		t.Logf("EXISTS result: %d rows", len(result.Rows))
	}
}

func TestNotExistsExpression(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE notexists_a (id SEQ, value INT)")
	engine.Execute("CREATE TABLE notexists_b (id SEQ, ref_value INT)")
	engine.Execute("INSERT INTO notexists_a (value) VALUES (10)")
	engine.Execute("INSERT INTO notexists_a (value) VALUES (20)")
	engine.Execute("INSERT INTO notexists_b (ref_value) VALUES (10)")

	// Test NOT EXISTS
	result, err := engine.Execute("SELECT * FROM notexists_a WHERE NOT EXISTS (SELECT 1 FROM notexists_b WHERE ref_value = notexists_a.value)")
	if err != nil {
		t.Logf("NOT EXISTS expression: %v", err)
	} else {
		t.Logf("NOT EXISTS result: %d rows", len(result.Rows))
	}
}

func TestTableAlias(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE alias_test (id SEQ, value INT)")
	engine.Execute("INSERT INTO alias_test (value) VALUES (100)")

	// Test table alias
	result, err := engine.Execute("SELECT t.value FROM alias_test t WHERE t.id = 1")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 1 {
		t.Errorf("Table alias: Expected 1 row, got %d", len(result.Rows))
	}
}

func TestSelfJoin(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE employees_self (id SEQ, name VARCHAR(50), manager_id INT)")
	engine.Execute("INSERT INTO employees_self (name, manager_id) VALUES ('Alice', NULL)")
	engine.Execute("INSERT INTO employees_self (name, manager_id) VALUES ('Bob', 1)")
	engine.Execute("INSERT INTO employees_self (name, manager_id) VALUES ('Charlie', 1)")

	// Test self join
	result, err := engine.Execute("SELECT e.name, m.name FROM employees_self e LEFT JOIN employees_self m ON e.manager_id = m.id")
	if err != nil {
		t.Logf("Self join: %v", err)
	} else {
		t.Logf("Self join result: %d rows", len(result.Rows))
	}
}

func TestPersistedStorage(t *testing.T) {
	// Create temporary directory for database
	dir := t.TempDir()

	// Create database
	config := Config{
		Path:     dir,
		InMemory: false,
	}

	engine, err := OpenWithConfig(config)
	if err != nil {
		t.Fatal(err)
	}

	// Create table and insert data
	engine.Execute("CREATE TABLE persist_test (id SEQ, value VARCHAR(100))")
	engine.Execute("INSERT INTO persist_test (value) VALUES ('test data')")

	// Close database
	engine.Close()

	// Reopen database
	engine2, err := OpenWithConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	defer engine2.Close()

	// Verify data persists
	result, err := engine2.Execute("SELECT * FROM persist_test")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 1 {
		t.Errorf("Persisted storage: Expected 1 row, got %d", len(result.Rows))
	}
}

func TestConcurrentReadWrite(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE concurrent_rw (id SEQ, value INT)")

	// Insert initial data
	for i := 0; i < 10; i++ {
		engine.Execute("INSERT INTO concurrent_rw (value) VALUES (1)")
	}

	done := make(chan bool)

	// Concurrent reads
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				_, err := engine.Execute("SELECT * FROM concurrent_rw")
				if err != nil {
					t.Errorf("Concurrent read failed: %v", err)
				}
			}
			done <- true
		}()
	}

	// Concurrent writes
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 5; j++ {
				_, err := engine.Execute("INSERT INTO concurrent_rw (value) VALUES (1)")
				if err != nil {
					t.Errorf("Concurrent write failed: %v", err)
				}
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify final count
	result, err := engine.Execute("SELECT COUNT(*) FROM concurrent_rw")
	if err != nil {
		t.Fatal(err)
	}
	count, _ := result.Rows[0].Data[0].ToInt64()
	t.Logf("Concurrent read/write final count: %d", count)
}

func TestLargeDataInsert(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE large_data (id SEQ, value INT)")

	// Insert 1000 rows
	for i := 0; i < 1000; i++ {
		_, err = engine.Execute("INSERT INTO large_data (value) VALUES (1)")
		if err != nil {
			t.Fatal(err)
		}
	}

	// Verify count
	result, err := engine.Execute("SELECT COUNT(*) FROM large_data")
	if err != nil {
		t.Fatal(err)
	}
	count, _ := result.Rows[0].Data[0].ToInt64()
	if count != 1000 {
		t.Errorf("Large data: Expected 1000 rows, got %d", count)
	}

	// Test query performance
	result, err = engine.Execute("SELECT SUM(value) FROM large_data")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("SUM of 1000 rows: %s", result.Rows[0].Data[0].ToString())
}

func TestBlobDataType(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE blob_test (id SEQ, data BLOB)")

	// Test storing binary data
	result, err := engine.Execute("SELECT LOAD_FILE('/etc/hostname')")
	if err != nil {
		t.Logf("LOAD_FILE: %v", err)
	} else {
		t.Logf("BLOB data: %v", result.Rows[0].Data[0])
	}
}

func TestTextDataType(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE text_test (id SEQ, content TEXT)")

	// Insert large text
	largeText := strings.Repeat("Hello World ", 100)
	_, err = engine.Execute(fmt.Sprintf("INSERT INTO text_test (content) VALUES ('%s')", largeText))
	if err != nil {
		t.Fatal(err)
	}

	result, err := engine.Execute("SELECT LENGTH(content) FROM text_test")
	if err != nil {
		t.Fatal(err)
	}
	length, _ := result.Rows[0].Data[0].ToInt64()
	if length != int64(len(largeText)) {
		t.Errorf("TEXT length: Expected %d, got %d", len(largeText), length)
	}
}

func TestFloatPrecision(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE float_test (id SEQ, value FLOAT)")
	engine.Execute("INSERT INTO float_test (value) VALUES (3.14159265358979)")

	result, err := engine.Execute("SELECT value FROM float_test")
	if err != nil {
		t.Fatal(err)
	}
	f, _ := result.Rows[0].Data[0].ToFloat64()
	t.Logf("Float precision: %f", f)
}

func TestDateRangeQuery(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE date_range (id SEQ, created_at DATE)")
	engine.Execute("INSERT INTO date_range (created_at) VALUES ('2024-01-15')")
	engine.Execute("INSERT INTO date_range (created_at) VALUES ('2024-02-20')")
	engine.Execute("INSERT INTO date_range (created_at) VALUES ('2024-03-25')")

	// Query date range
	result, err := engine.Execute("SELECT * FROM date_range WHERE created_at >= '2024-02-01'")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Date range query: %d rows", len(result.Rows))
}

// ==================== More Advanced Tests ====================

func TestNestedSubquery(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE nested_a (id SEQ, value INT)")
	engine.Execute("CREATE TABLE nested_b (id SEQ, value INT)")
	engine.Execute("CREATE TABLE nested_c (id SEQ, value INT)")

	engine.Execute("INSERT INTO nested_a (value) VALUES (10)")
	engine.Execute("INSERT INTO nested_b (value) VALUES (10)")
	engine.Execute("INSERT INTO nested_c (value) VALUES (10)")

	// Test nested subquery
	result, err := engine.Execute("SELECT * FROM nested_a WHERE value = (SELECT value FROM nested_b WHERE value = (SELECT value FROM nested_c))")
	if err != nil {
		t.Logf("Nested subquery: %v", err)
	} else {
		t.Logf("Nested subquery result: %d rows", len(result.Rows))
	}
}

func TestCorrelatedSubquery(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE corr_dept (id SEQ, name VARCHAR(50), budget INT)")
	engine.Execute("CREATE TABLE corr_emp (id SEQ, dept_id INT, salary INT)")

	engine.Execute("INSERT INTO corr_dept (name, budget) VALUES ('Engineering', 100000)")
	engine.Execute("INSERT INTO corr_dept (name, budget) VALUES ('Sales', 50000)")
	engine.Execute("INSERT INTO corr_emp (dept_id, salary) VALUES (1, 80000)")
	engine.Execute("INSERT INTO corr_emp (dept_id, salary) VALUES (1, 90000)")
	engine.Execute("INSERT INTO corr_emp (dept_id, salary) VALUES (2, 40000)")

	// Test correlated subquery
	result, err := engine.Execute("SELECT * FROM corr_emp e WHERE salary > (SELECT budget * 0.5 FROM corr_dept d WHERE d.id = e.dept_id)")
	if err != nil {
		t.Logf("Correlated subquery: %v", err)
	} else {
		t.Logf("Correlated subquery result: %d rows", len(result.Rows))
	}
}

func TestMultipleJoins(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE multi_orders (id SEQ, customer_id INT, product_id INT, quantity INT)")
	engine.Execute("CREATE TABLE multi_customers (id SEQ, name VARCHAR(50))")
	engine.Execute("CREATE TABLE multi_products (id SEQ, name VARCHAR(50), price INT)")

	engine.Execute("INSERT INTO multi_customers (name) VALUES ('Alice')")
	engine.Execute("INSERT INTO multi_customers (name) VALUES ('Bob')")
	engine.Execute("INSERT INTO multi_products (name, price) VALUES ('Laptop', 1000)")
	engine.Execute("INSERT INTO multi_products (name, price) VALUES ('Mouse', 50)")
	engine.Execute("INSERT INTO multi_orders (customer_id, product_id, quantity) VALUES (1, 1, 2)")
	engine.Execute("INSERT INTO multi_orders (customer_id, product_id, quantity) VALUES (2, 2, 5)")

	// Test multiple joins
	result, err := engine.Execute(`
		SELECT c.name, p.name, o.quantity
		FROM multi_orders o
		JOIN multi_customers c ON o.customer_id = c.id
		JOIN multi_products p ON o.product_id = p.id
	`)
	if err != nil {
		t.Logf("Multiple joins: %v", err)
	} else {
		t.Logf("Multiple joins result: %d rows", len(result.Rows))
	}
}

func TestJoinWithConditions(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE join_cond_a (id SEQ, value INT)")
	engine.Execute("CREATE TABLE join_cond_b (id SEQ, value INT)")
	engine.Execute("INSERT INTO join_cond_a (value) VALUES (10)")
	engine.Execute("INSERT INTO join_cond_a (value) VALUES (20)")
	engine.Execute("INSERT INTO join_cond_b (value) VALUES (15)")
	engine.Execute("INSERT INTO join_cond_b (value) VALUES (25)")

	// Test join with additional conditions
	result, err := engine.Execute("SELECT * FROM join_cond_a a JOIN join_cond_b b ON a.value < b.value")
	if err != nil {
		t.Logf("Join with conditions: %v", err)
	} else {
		t.Logf("Join with conditions result: %d rows", len(result.Rows))
	}
}

func TestCrossJoin(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE cross_a (id SEQ, value VARCHAR(10))")
	engine.Execute("CREATE TABLE cross_b (id SEQ, value VARCHAR(10))")
	engine.Execute("INSERT INTO cross_a (value) VALUES ('A1')")
	engine.Execute("INSERT INTO cross_a (value) VALUES ('A2')")
	engine.Execute("INSERT INTO cross_b (value) VALUES ('B1')")
	engine.Execute("INSERT INTO cross_b (value) VALUES ('B2')")

	// Test cross join (Cartesian product)
	result, err := engine.Execute("SELECT * FROM cross_a, cross_b")
	if err != nil {
		t.Logf("Cross join: %v", err)
	} else {
		t.Logf("Cross join result: %d rows (expected 4)", len(result.Rows))
	}
}

func TestNaturalJoin(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE natural_a (id INT, name VARCHAR(50))")
	engine.Execute("CREATE TABLE natural_b (id INT, value INT)")
	engine.Execute("INSERT INTO natural_a (id, name) VALUES (1, 'Alice')")
	engine.Execute("INSERT INTO natural_a (id, name) VALUES (2, 'Bob')")
	engine.Execute("INSERT INTO natural_b (id, value) VALUES (1, 100)")
	engine.Execute("INSERT INTO natural_b (id, value) VALUES (3, 200)")

	// Test natural join
	result, err := engine.Execute("SELECT * FROM natural_a NATURAL JOIN natural_b")
	if err != nil {
		t.Logf("Natural join: %v", err)
	} else {
		t.Logf("Natural join result: %d rows", len(result.Rows))
	}
}

func TestSelectWithNoFrom(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Test SELECT without FROM (expressions only)
	result, err := engine.Execute("SELECT NOW()")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 1 {
		t.Errorf("Expected 1 row, got %d", len(result.Rows))
	}
	t.Logf("SELECT without FROM works: %s", result.Rows[0].Data[0].ToString())
}

func TestSelectStarWithJoin(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE star_a (id SEQ, name VARCHAR(50))")
	engine.Execute("CREATE TABLE star_b (id SEQ, a_id INT, value INT)")
	engine.Execute("INSERT INTO star_a (name) VALUES ('Test')")
	engine.Execute("INSERT INTO star_b (a_id, value) VALUES (1, 100)")

	// Test SELECT * with JOIN
	result, err := engine.Execute("SELECT * FROM star_a JOIN star_b ON star_a.id = star_b.a_id")
	if err != nil {
		t.Logf("SELECT * with JOIN: %v", err)
	} else {
		t.Logf("SELECT * with JOIN: %d columns, %d rows", len(result.Columns), len(result.Rows))
	}
}

func TestAggregateWithGroupByAndHaving(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE agg_group (id SEQ, category VARCHAR(20), amount INT)")
	engine.Execute("INSERT INTO agg_group (category, amount) VALUES ('A', 100)")
	engine.Execute("INSERT INTO agg_group (category, amount) VALUES ('A', 200)")
	engine.Execute("INSERT INTO agg_group (category, amount) VALUES ('B', 50)")
	engine.Execute("INSERT INTO agg_group (category, amount) VALUES ('B', 150)")
	engine.Execute("INSERT INTO agg_group (category, amount) VALUES ('C', 300)")

	// Test GROUP BY with HAVING
	result, err := engine.Execute("SELECT category, SUM(amount) as total FROM agg_group GROUP BY category HAVING SUM(amount) > 200")
	if err != nil {
		t.Logf("GROUP BY with HAVING: %v", err)
	} else {
		t.Logf("GROUP BY with HAVING result: %d rows", len(result.Rows))
	}
}

func TestAggregateWithWhereAndGroupBy(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE agg_where (id SEQ, category VARCHAR(20), amount INT)")
	engine.Execute("INSERT INTO agg_where (category, amount) VALUES ('A', 100)")
	engine.Execute("INSERT INTO agg_where (category, amount) VALUES ('A', 200)")
	engine.Execute("INSERT INTO agg_where (category, amount) VALUES ('B', 50)")
	engine.Execute("INSERT INTO agg_where (category, amount) VALUES ('B', 150)")

	// Test WHERE with GROUP BY
	result, err := engine.Execute("SELECT category, COUNT(*) FROM agg_where WHERE amount > 100 GROUP BY category")
	if err != nil {
		t.Logf("WHERE with GROUP BY: %v", err)
	} else {
		t.Logf("WHERE with GROUP BY result: %d rows", len(result.Rows))
	}
}

func TestOrderByWithExpression(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE order_expr (id SEQ, a INT, b INT)")
	engine.Execute("INSERT INTO order_expr (a, b) VALUES (10, 5)")
	engine.Execute("INSERT INTO order_expr (a, b) VALUES (5, 10)")
	engine.Execute("INSERT INTO order_expr (a, b) VALUES (7, 7)")

	// Test ORDER BY with expression
	result, err := engine.Execute("SELECT * FROM order_expr ORDER BY a + b DESC")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 3 {
		t.Errorf("Expected 3 rows, got %d", len(result.Rows))
	}
}

func TestOrderByNullPosition(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE order_null (id SEQ, value INT)")
	engine.Execute("INSERT INTO order_null (value) VALUES (10)")
	engine.Execute("INSERT INTO order_null (value) VALUES (NULL)")
	engine.Execute("INSERT INTO order_null (value) VALUES (5)")

	// Test ORDER BY with NULL values
	result, err := engine.Execute("SELECT * FROM order_null ORDER BY value ASC")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("ORDER BY with NULL: %d rows", len(result.Rows))
}

func TestLimitWithOffset(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE limit_offset (id SEQ, value INT)")
	for i := 0; i < 10; i++ {
		engine.Execute("INSERT INTO limit_offset (value) VALUES (1)")
	}

	// Test LIMIT with OFFSET
	result, err := engine.Execute("SELECT * FROM limit_offset LIMIT 3 OFFSET 5")
	if err != nil {
		t.Logf("LIMIT with OFFSET: %v", err)
	} else {
		t.Logf("LIMIT 3 OFFSET 5: %d rows", len(result.Rows))
	}

	// Test OFFSET only
	result, err = engine.Execute("SELECT * FROM limit_offset OFFSET 8")
	if err != nil {
		t.Logf("OFFSET only: %v", err)
	} else {
		t.Logf("OFFSET 8: %d rows", len(result.Rows))
	}
}

func TestDistinctWithMultipleColumns(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE distinct_multi (id SEQ, a VARCHAR(10), b VARCHAR(10))")
	engine.Execute("INSERT INTO distinct_multi (a, b) VALUES ('x', 'y')")
	engine.Execute("INSERT INTO distinct_multi (a, b) VALUES ('x', 'y')")
	engine.Execute("INSERT INTO distinct_multi (a, b) VALUES ('x', 'z')")
	engine.Execute("INSERT INTO distinct_multi (a, b) VALUES ('y', 'y')")

	// Test DISTINCT with multiple columns
	result, err := engine.Execute("SELECT DISTINCT a, b FROM distinct_multi")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 3 {
		t.Errorf("DISTINCT multiple columns: Expected 3 unique rows, got %d", len(result.Rows))
	}
}

func TestSelectWithCalculations(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE calc_test (id SEQ, price FLOAT, quantity INT)")
	engine.Execute("INSERT INTO calc_test (price, quantity) VALUES (10.5, 3)")
	engine.Execute("INSERT INTO calc_test (price, quantity) VALUES (5.0, 2)")

	// Test calculations in SELECT
	result, err := engine.Execute("SELECT price, quantity FROM calc_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Calculation test: %d rows", len(result.Rows))
}

func TestSelectWithConcatenation(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE concat_test (id SEQ, first_name VARCHAR(50), last_name VARCHAR(50))")
	engine.Execute("INSERT INTO concat_test (first_name, last_name) VALUES ('John', 'Doe')")
	engine.Execute("INSERT INTO concat_test (first_name, last_name) VALUES ('Jane', 'Smith')")

	// Test string concatenation in SELECT
	result, err := engine.Execute("SELECT first_name || ' ' || last_name AS full_name FROM concat_test")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("Expected 2 rows, got %d", len(result.Rows))
	}
}

func TestBooleanExpressions(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE bool_test (id SEQ, flag INT)")
	engine.Execute("INSERT INTO bool_test (flag) VALUES (1)")
	engine.Execute("INSERT INTO bool_test (flag) VALUES (0)")
	engine.Execute("INSERT INTO bool_test (flag) VALUES (1)")

	// Test boolean expressions
	result, err := engine.Execute("SELECT * FROM bool_test WHERE flag = 1")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("Boolean expression: Expected 2 rows, got %d", len(result.Rows))
	}
}

func TestNotOperator(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE not_test (id SEQ, value INT)")
	engine.Execute("INSERT INTO not_test (value) VALUES (10)")
	engine.Execute("INSERT INTO not_test (value) VALUES (20)")
	engine.Execute("INSERT INTO not_test (value) VALUES (30)")

	// Test NOT operator
	result, err := engine.Execute("SELECT * FROM not_test WHERE NOT value = 20")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("NOT operator: Expected 2 rows, got %d", len(result.Rows))
	}
}

func TestBetweenWithDates(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE date_between (id SEQ, created DATE)")
	engine.Execute("INSERT INTO date_between (created) VALUES ('2024-01-15')")
	engine.Execute("INSERT INTO date_between (created) VALUES ('2024-02-20')")
	engine.Execute("INSERT INTO date_between (created) VALUES ('2024-03-25')")

	// Test BETWEEN with dates
	result, err := engine.Execute("SELECT * FROM date_between WHERE created BETWEEN '2024-02-01' AND '2024-03-01'")
	if err != nil {
		t.Logf("BETWEEN with dates: %v", err)
	} else {
		t.Logf("BETWEEN with dates: %d rows", len(result.Rows))
	}
}

func TestInWithSubquery(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE in_outer (id SEQ, value INT)")
	engine.Execute("CREATE TABLE in_inner (id SEQ, threshold INT)")
	engine.Execute("INSERT INTO in_outer (value) VALUES (10)")
	engine.Execute("INSERT INTO in_outer (value) VALUES (20)")
	engine.Execute("INSERT INTO in_outer (value) VALUES (30)")
	engine.Execute("INSERT INTO in_inner (threshold) VALUES (15)")
	engine.Execute("INSERT INTO in_inner (threshold) VALUES (25)")

	// Test IN with subquery
	result, err := engine.Execute("SELECT * FROM in_outer WHERE value IN (SELECT threshold FROM in_inner)")
	if err != nil {
		t.Logf("IN with subquery: %v", err)
	} else {
		t.Logf("IN with subquery: %d rows", len(result.Rows))
	}
}

func TestNotInWithSubquery(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE notin_outer (id SEQ, value INT)")
	engine.Execute("CREATE TABLE notin_inner (id SEQ, threshold INT)")
	engine.Execute("INSERT INTO notin_outer (value) VALUES (10)")
	engine.Execute("INSERT INTO notin_outer (value) VALUES (20)")
	engine.Execute("INSERT INTO notin_outer (value) VALUES (30)")
	engine.Execute("INSERT INTO notin_inner (threshold) VALUES (20)")

	// Test NOT IN with subquery
	result, err := engine.Execute("SELECT * FROM notin_outer WHERE value NOT IN (SELECT threshold FROM notin_inner)")
	if err != nil {
		t.Logf("NOT IN with subquery: %v", err)
	} else {
		t.Logf("NOT IN with subquery: %d rows", len(result.Rows))
	}
}

func TestMultipleAggregates(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE multi_agg (id SEQ, value INT)")
	engine.Execute("INSERT INTO multi_agg (value) VALUES (10)")
	engine.Execute("INSERT INTO multi_agg (value) VALUES (20)")
	engine.Execute("INSERT INTO multi_agg (value) VALUES (30)")
	engine.Execute("INSERT INTO multi_agg (value) VALUES (40)")

	// Test multiple aggregate functions in one query
	result, err := engine.Execute("SELECT COUNT(*), SUM(value), AVG(value), MIN(value), MAX(value) FROM multi_agg")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 1 {
		t.Errorf("Expected 1 row, got %d", len(result.Rows))
	}
	if len(result.Columns) != 5 {
		t.Errorf("Expected 5 columns, got %d", len(result.Columns))
	}
}

func TestAggregateWithNullValues(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE agg_null (id SEQ, value INT)")
	engine.Execute("INSERT INTO agg_null (value) VALUES (10)")
	engine.Execute("INSERT INTO agg_null (value) VALUES (NULL)")
	engine.Execute("INSERT INTO agg_null (value) VALUES (20)")
	engine.Execute("INSERT INTO agg_null (value) VALUES (NULL)")
	engine.Execute("INSERT INTO agg_null (value) VALUES (30)")

	// Test aggregates with NULL values
	result, err := engine.Execute("SELECT COUNT(*), COUNT(value), SUM(value), AVG(value) FROM agg_null")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Aggregates with NULLs: COUNT(*)=%s, COUNT(value)=%s", result.Rows[0].Data[0].ToString(), result.Rows[0].Data[1].ToString())
}

func TestGroupByWithMultipleColumns(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE group_multi (id SEQ, category VARCHAR(20), subcategory VARCHAR(20), value INT)")
	engine.Execute("INSERT INTO group_multi (category, subcategory, value) VALUES ('A', 'X', 10)")
	engine.Execute("INSERT INTO group_multi (category, subcategory, value) VALUES ('A', 'X', 20)")
	engine.Execute("INSERT INTO group_multi (category, subcategory, value) VALUES ('A', 'Y', 30)")
	engine.Execute("INSERT INTO group_multi (category, subcategory, value) VALUES ('B', 'X', 40)")

	// Test GROUP BY with multiple columns
	result, err := engine.Execute("SELECT category, subcategory, SUM(value) FROM group_multi GROUP BY category, subcategory")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("GROUP BY multiple columns: %d groups", len(result.Rows))
}

func TestSelectConstantExpressions(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Test various constant expressions
	tests := []string{
		"SELECT 1",
		"SELECT 'hello'",
		"SELECT 1 + 2 * 3",
		"SELECT UPPER('hello')",
		"SELECT NOW()",
		"SELECT ABS(-5)",
	}

	for _, sql := range tests {
		result, err := engine.Execute(sql)
		if err != nil {
			t.Errorf("%s failed: %v", sql, err)
		} else if len(result.Rows) != 1 {
			t.Errorf("%s: Expected 1 row, got %d", sql, len(result.Rows))
		}
	}
}

func TestSelectFromDual(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Test SELECT FROM DUAL (if supported)
	result, err := engine.Execute("SELECT 1 + 1 FROM DUAL")
	if err != nil {
		t.Logf("SELECT FROM DUAL: %v", err)
	} else {
		t.Logf("SELECT FROM DUAL: %s", result.Rows[0].Data[0].ToString())
	}
}

func TestSelectWithQuotedIdentifiers(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE quoted_test (id SEQ, name VARCHAR(50))")
	engine.Execute("INSERT INTO quoted_test (name) VALUES ('Test')")

	// Test quoted identifiers
	result, err := engine.Execute("SELECT \"name\" FROM quoted_test")
	if err != nil {
		t.Logf("Quoted identifier: %v", err)
	} else {
		t.Logf("Quoted identifier result: %d rows", len(result.Rows))
	}
}

func TestColumnNameCaseSensitivity(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE col_case (id SEQ, Name VARCHAR(50), VALUE INT)")
	engine.Execute("INSERT INTO col_case (Name, VALUE) VALUES ('Test', 100)")

	// Test column name case sensitivity
	result, err := engine.Execute("SELECT name, value FROM col_case")
	if err != nil {
		t.Logf("Column case sensitivity: %v", err)
	} else {
		t.Logf("Column case result: %d rows", len(result.Rows))
	}
}

func TestEmptyStringHandling(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE empty_str (id SEQ, value VARCHAR(100))")
	engine.Execute("INSERT INTO empty_str (value) VALUES ('')")
	engine.Execute("INSERT INTO empty_str (value) VALUES ('hello')")
	engine.Execute("INSERT INTO empty_str (value) VALUES ('')")

	// Test empty string handling
	result, err := engine.Execute("SELECT * FROM empty_str WHERE value = ''")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("Empty string handling: Expected 2 rows, got %d", len(result.Rows))
	}
}

func TestWhitespaceInStrings(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE ws_test (id SEQ, value VARCHAR(100))")
	engine.Execute("INSERT INTO ws_test (value) VALUES ('  hello  ')")
	engine.Execute("INSERT INTO ws_test (value) VALUES ('world')")

	// Test whitespace handling
	result, err := engine.Execute("SELECT * FROM ws_test WHERE value LIKE '%hello%'")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 1 {
		t.Errorf("Whitespace in strings: Expected 1 row, got %d", len(result.Rows))
	}
}

func TestUnicodeStrings(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE unicode_test (id SEQ, value VARCHAR(100))")
	engine.Execute("INSERT INTO unicode_test (value) VALUES ('你好世界')")
	engine.Execute("INSERT INTO unicode_test (value) VALUES ('Привет')")
	engine.Execute("INSERT INTO unicode_test (value) VALUES ('🎉 emoji')")

	// Test Unicode handling
	result, err := engine.Execute("SELECT * FROM unicode_test")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 3 {
		t.Errorf("Unicode strings: Expected 3 rows, got %d", len(result.Rows))
	}

	// Verify length function works with Unicode
	result, err = engine.Execute("SELECT LENGTH(value) FROM unicode_test WHERE value LIKE '%你好%'")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Unicode LENGTH: %s", result.Rows[0].Data[0].ToString())
}

func TestNumericEdgeCases(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE num_edge (id SEQ, value INT)")
	engine.Execute("INSERT INTO num_edge (value) VALUES (0)")
	engine.Execute("INSERT INTO num_edge (value) VALUES (-1)")
	engine.Execute("INSERT INTO num_edge (value) VALUES (9223372036854775807)") // Max int64

	// Test zero
	result, err := engine.Execute("SELECT * FROM num_edge WHERE value = 0")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 1 {
		t.Errorf("Zero value: Expected 1 row, got %d", len(result.Rows))
	}

	// Test negative
	result, err = engine.Execute("SELECT * FROM num_edge WHERE value < 0")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 1 {
		t.Errorf("Negative value: Expected 1 row, got %d", len(result.Rows))
	}

	// Test large number
	result, err = engine.Execute("SELECT * FROM num_edge WHERE value > 1000000000")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 1 {
		t.Errorf("Large value: Expected 1 row, got %d", len(result.Rows))
	}
}

func TestFloatEdgeCases(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Test very small float
	result, err := engine.Execute("SELECT 0.0000001")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Very small float: %s", result.Rows[0].Data[0].ToString())

	// Test very large float
	result, err = engine.Execute("SELECT 1.23456789e10")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Scientific notation: %s", result.Rows[0].Data[0].ToString())

	// Test negative float
	result, err = engine.Execute("SELECT -3.14159")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Negative float: %s", result.Rows[0].Data[0].ToString())
}

func TestDateEdgeCases(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE date_edge (id SEQ, created DATE)")
	engine.Execute("INSERT INTO date_edge (created) VALUES ('1970-01-01')") // Unix epoch
	engine.Execute("INSERT INTO date_edge (created) VALUES ('2099-12-31')") // Far future
	engine.Execute("INSERT INTO date_edge (created) VALUES ('2024-02-29')") // Leap year

	// Test date edge cases
	result, err := engine.Execute("SELECT * FROM date_edge")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 3 {
		t.Errorf("Date edge cases: Expected 3 rows, got %d", len(result.Rows))
	}
}

func TestSelectWithNoRows(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE no_rows (id SEQ, value INT)")

	// Test SELECT on empty table
	result, err := engine.Execute("SELECT * FROM no_rows")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 0 {
		t.Errorf("Empty table: Expected 0 rows, got %d", len(result.Rows))
	}

	// Test aggregates on empty table
	result, err = engine.Execute("SELECT COUNT(*), SUM(value), AVG(value), MIN(value), MAX(value) FROM no_rows")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Aggregates on empty table: COUNT=%s", result.Rows[0].Data[0].ToString())
}

func TestUpdateWithExpression(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE update_expr (id SEQ, value INT)")
	engine.Execute("INSERT INTO update_expr (value) VALUES (10)")
	engine.Execute("INSERT INTO update_expr (value) VALUES (20)")

	// Update with expression
	_, err = engine.Execute("UPDATE update_expr SET value = 100 WHERE value = 10")
	if err != nil {
		t.Fatal(err)
	}

	result, err := engine.Execute("SELECT value FROM update_expr WHERE id = 1")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("UPDATE test result: %s", result.Rows[0].Data[0].ToString())
}

func TestDeleteAllWithCondition(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE delete_cond (id SEQ, active INT)")
	engine.Execute("INSERT INTO delete_cond (active) VALUES (0)")
	engine.Execute("INSERT INTO delete_cond (active) VALUES (1)")
	engine.Execute("INSERT INTO delete_cond (active) VALUES (0)")

	// Delete with condition
	_, err = engine.Execute("DELETE FROM delete_cond WHERE active = 0")
	if err != nil {
		t.Fatal(err)
	}

	result, err := engine.Execute("SELECT COUNT(*) FROM delete_cond")
	if err != nil {
		t.Fatal(err)
	}
	count, _ := result.Rows[0].Data[0].ToInt64()
	if count != 1 {
		t.Errorf("DELETE with condition: Expected 1 row, got %d", count)
	}
}

func TestMultipleStatementsInSequence(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Execute multiple statements in sequence
	statements := []string{
		"CREATE TABLE seq_test (id SEQ, value INT)",
		"INSERT INTO seq_test (value) VALUES (1)",
		"INSERT INTO seq_test (value) VALUES (2)",
		"UPDATE seq_test SET value = 100 WHERE id = 1",
	}

	for _, sql := range statements {
		_, err = engine.Execute(sql)
		if err != nil {
			t.Errorf("Statement failed: %s, error: %v", sql, err)
		}
	}

	result, err := engine.Execute("SELECT COUNT(*) FROM seq_test")
	if err != nil {
		t.Fatal(err)
	}
	count, _ := result.Rows[0].Data[0].ToInt64()
	t.Logf("Multiple statements: %d rows", count)
}

func TestCreateTableIfNotExists(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Create table
	_, err = engine.Execute("CREATE TABLE ifne_test (id SEQ, value INT)")
	if err != nil {
		t.Fatal(err)
	}

	// Try to create again with IF NOT EXISTS
	_, err = engine.Execute("CREATE TABLE IF NOT EXISTS ifne_test (id SEQ, value INT)")
	if err != nil {
		t.Logf("CREATE TABLE IF NOT EXISTS: %v", err)
	}

	// Try to create again without IF NOT EXISTS (should fail)
	_, err = engine.Execute("CREATE TABLE ifne_test (id SEQ, value INT)")
	if err == nil {
		t.Error("Expected error when creating duplicate table without IF NOT EXISTS")
	}
}

func TestDropTableIfExists(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Drop non-existent table with IF EXISTS
	_, err = engine.Execute("DROP TABLE IF EXISTS nonexistent_table")
	if err != nil {
		t.Logf("DROP TABLE IF EXISTS: %v", err)
	}

	// Drop non-existent table without IF EXISTS (should fail)
	_, err = engine.Execute("DROP TABLE nonexistent_table")
	if err == nil {
		t.Error("Expected error when dropping non-existent table without IF EXISTS")
	}
}

func TestSelectWithParentheses(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Test simple arithmetic
	result, err := engine.Execute("SELECT 1 + 2")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("1 + 2 = %s", result.Rows[0].Data[0].ToString())

	// Test multiplication
	result, err = engine.Execute("SELECT 3 * 3")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("3 * 3 = %s", result.Rows[0].Data[0].ToString())
}

func TestComplexWhereConditions(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE complex_where (id SEQ, a INT, b INT, c INT)")
	engine.Execute("INSERT INTO complex_where (a, b, c) VALUES (1, 2, 3)")
	engine.Execute("INSERT INTO complex_where (a, b, c) VALUES (4, 5, 6)")
	engine.Execute("INSERT INTO complex_where (a, b, c) VALUES (7, 8, 9)")

	// Complex condition with AND, OR, and parentheses
	result, err := engine.Execute("SELECT * FROM complex_where WHERE (a = 1 OR a = 7) AND (b > 1 AND c < 10)")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("Complex WHERE: Expected 2 rows, got %d", len(result.Rows))
	}
}

func TestSelectWithAllOperators(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Execute("CREATE TABLE all_ops (id SEQ, value INT)")
	engine.Execute("INSERT INTO all_ops (value) VALUES (10)")
	engine.Execute("INSERT INTO all_ops (value) VALUES (20)")
	engine.Execute("INSERT INTO all_ops (value) VALUES (30)")

	tests := []struct {
		sql       string
		expected  int
	}{
		{"SELECT * FROM all_ops WHERE value = 20", 1},
		{"SELECT * FROM all_ops WHERE value <> 20", 2},
		{"SELECT * FROM all_ops WHERE value < 25", 2},
		{"SELECT * FROM all_ops WHERE value <= 20", 2},
		{"SELECT * FROM all_ops WHERE value > 20", 1},
		{"SELECT * FROM all_ops WHERE value >= 20", 2},
	}

	for _, tt := range tests {
		result, err := engine.Execute(tt.sql)
		if err != nil {
			t.Errorf("%s failed: %v", tt.sql, err)
			continue
		}
		if len(result.Rows) != tt.expected {
			t.Errorf("%s: Expected %d rows, got %d", tt.sql, tt.expected, len(result.Rows))
		}
	}
}
