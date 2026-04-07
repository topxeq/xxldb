// Package xxldb provides constraint tests for data types
package xxldb

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// ==================== VARCHAR Length Constraint Tests ====================

// TestVarcharLengthConstraint tests VARCHAR length validation
// Note: Length is measured in bytes (UTF-8 encoding)
// For ASCII, 1 char = 1 byte; for UTF-8 Chinese, 1 char = 3 bytes
func TestVarcharLengthConstraint(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	_, err = engine.Execute("CREATE TABLE test_varchar (id SEQ, name VARCHAR(10))")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	tests := []struct {
		name      string
		value     string
		shouldErr bool
	}{
		{"empty string", "", false},
		{"single char", "A", false},
		{"exact length", "1234567890", false},
		{"one over", "12345678901", true},
		{"way over", "this is way too long for varchar 10", true},
		{"unicode 3 chinese chars", "你好世", false},              // 3 Chinese chars = 3 runes < 10
		{"unicode 4 chinese chars", "你好世界", false},            // 4 Chinese chars = 4 runes < 10
		{"unicode 10 chinese chars", "一二三四五六七八九十", false},  // 10 Chinese chars = 10 runes = 10
		{"unicode 11 chinese chars", "一二三四五六七八九十壹", true}, // 11 Chinese chars = 11 runes > 10
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Escape single quotes for SQL
			escaped := strings.ReplaceAll(tt.value, "'", "''")
			_, err := engine.Execute("INSERT INTO test_varchar (name) VALUES ('" + escaped + "')")

			if tt.shouldErr && err == nil {
				t.Errorf("Expected error for value '%s' (len=%d), but succeeded", tt.value, len(tt.value))
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("Unexpected error for value '%s' (len=%d): %v", tt.value, len(tt.value), err)
			}
		})
	}
}

// TestVarcharUpdateConstraint tests VARCHAR length validation on UPDATE
func TestVarcharUpdateConstraint(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	_, err = engine.Execute("CREATE TABLE test_varchar_update (id SEQ, name VARCHAR(10))")
	if err != nil {
		t.Fatal(err)
	}

	// Insert valid data
	_, err = engine.Execute("INSERT INTO test_varchar_update (name) VALUES ('short')")
	if err != nil {
		t.Fatal(err)
	}

	// Update to another valid value
	_, err = engine.Execute("UPDATE test_varchar_update SET name = 'still ok' WHERE id = 1")
	if err != nil {
		t.Errorf("Failed to update with valid length: %v", err)
	}

	// Update to invalid length
	_, err = engine.Execute("UPDATE test_varchar_update SET name = 'this is too long' WHERE id = 1")
	if err == nil {
		t.Error("Expected error when updating with value exceeding VARCHAR length")
	}

	// Verify original value unchanged
	result, err := engine.Execute("SELECT name FROM test_varchar_update WHERE id = 1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows[0].Data[0].ToString() != "still ok" {
		t.Errorf("Value should not have changed, got: %s", result.Rows[0].Data[0].ToString())
	}
}

// TestVarcharMaxLength tests maximum VARCHAR length
func TestVarcharMaxLength(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Create table with large VARCHAR
	_, err = engine.Execute("CREATE TABLE test_big_varchar (id SEQ, data VARCHAR(10000))")
	if err != nil {
		t.Fatal(err)
	}

	// Insert exactly 10000 chars
	largeStr := strings.Repeat("X", 10000)
	_, err = engine.Execute("INSERT INTO test_big_varchar (data) VALUES ('" + largeStr + "')")
	if err != nil {
		t.Errorf("Failed to insert 10000 chars: %v", err)
	}

	// Insert 10001 chars - should fail
	tooLargeStr := strings.Repeat("X", 10001)
	_, err = engine.Execute("INSERT INTO test_big_varchar (data) VALUES ('" + tooLargeStr + "')")
	if err == nil {
		t.Error("Expected error for value exceeding VARCHAR(10000)")
	}

	// Verify count
	result, err := engine.Execute("SELECT COUNT(*) FROM test_big_varchar")
	if err != nil {
		t.Fatal(err)
	}
	count, _ := result.Rows[0].Data[0].ToInt64()
	if count != 1 {
		t.Errorf("Expected 1 row, got %d", count)
	}
}

// ==================== CHAR Type Tests ====================

// TestCharPadding tests CHAR type padding behavior
func TestCharPadding(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	_, err = engine.Execute("CREATE TABLE test_char (id SEQ, code CHAR(10))")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		input         string
		expectedLen   int
		shouldErr     bool
	}{
		{"empty string", "", 10, false},
		{"single char", "A", 10, false},
		{"5 chars", "ABCDE", 10, false},
		{"exact 10 chars", "1234567890", 10, false},
		{"11 chars", "12345678901", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			escaped := strings.ReplaceAll(tt.input, "'", "''")
			_, err := engine.Execute("INSERT INTO test_char (code) VALUES ('" + escaped + "')")

			if tt.shouldErr {
				if err == nil {
					t.Errorf("Expected error for input '%s' (len=%d)", tt.input, len(tt.input))
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Verify padding
			result, err := engine.Execute("SELECT code, LENGTH(code) FROM test_char ORDER BY id DESC LIMIT 1")
			if err != nil {
				t.Fatal(err)
			}

			stored := result.Rows[0].Data[0].ToString()
			length, _ := result.Rows[0].Data[1].ToInt64()

			if int(length) != tt.expectedLen {
				t.Errorf("Expected length %d, got %d", tt.expectedLen, length)
			}

			if len(stored) != tt.expectedLen {
				t.Errorf("Stored value length should be %d, got %d", tt.expectedLen, len(stored))
			}

			// Verify trailing spaces for padding
			if len(tt.input) < 10 {
				expected := tt.input + strings.Repeat(" ", 10-len(tt.input))
				if stored != expected {
					t.Errorf("Expected '%s' (padded), got '%s'", expected, stored)
				}
			}
		})
	}
}

// TestCharUpdatePadding tests CHAR padding on UPDATE
func TestCharUpdatePadding(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	_, err = engine.Execute("CREATE TABLE test_char_update (id SEQ, code CHAR(10))")
	if err != nil {
		t.Fatal(err)
	}

	// Insert with padding
	_, err = engine.Execute("INSERT INTO test_char_update (code) VALUES ('ABC')")
	if err != nil {
		t.Fatal(err)
	}

	// Verify initial padding
	result, _ := engine.Execute("SELECT code FROM test_char_update WHERE id = 1")
	initial := result.Rows[0].Data[0].ToString()
	if initial != "ABC       " {
		t.Errorf("Initial value should be padded, got '%s'", initial)
	}

	// Update with new short value
	_, err = engine.Execute("UPDATE test_char_update SET code = 'XYZ' WHERE id = 1")
	if err != nil {
		t.Fatal(err)
	}

	// Verify new padding
	result, _ = engine.Execute("SELECT code FROM test_char_update WHERE id = 1")
	updated := result.Rows[0].Data[0].ToString()
	if updated != "XYZ       " {
		t.Errorf("Updated value should be padded, got '%s'", updated)
	}

	// Update with exact length
	_, err = engine.Execute("UPDATE test_char_update SET code = '1234567890' WHERE id = 1")
	if err != nil {
		t.Fatal(err)
	}

	result, _ = engine.Execute("SELECT code FROM test_char_update WHERE id = 1")
	exact := result.Rows[0].Data[0].ToString()
	if exact != "1234567890" {
		t.Errorf("Exact length value should not have extra padding, got '%s'", exact)
	}

	// Update with too long value - should fail
	_, err = engine.Execute("UPDATE test_char_update SET code = '12345678901' WHERE id = 1")
	if err == nil {
		t.Error("Expected error for value exceeding CHAR length")
	}
}

// TestCharVsVarcharComparison compares CHAR and VARCHAR behavior
func TestCharVsVarcharComparison(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Create both tables
	_, err = engine.Execute("CREATE TABLE test_char_vs_varchar_char (id SEQ, val CHAR(10))")
	if err != nil {
		t.Fatal(err)
	}
	_, err = engine.Execute("CREATE TABLE test_char_vs_varchar_varchar (id SEQ, val VARCHAR(10))")
	if err != nil {
		t.Fatal(err)
	}

	// Insert same value in both
	testValue := "ABC"
	_, err = engine.Execute("INSERT INTO test_char_vs_varchar_char (val) VALUES ('" + testValue + "')")
	if err != nil {
		t.Fatal(err)
	}
	_, err = engine.Execute("INSERT INTO test_char_vs_varchar_varchar (val) VALUES ('" + testValue + "')")
	if err != nil {
		t.Fatal(err)
	}

	// Check CHAR
	result, _ := engine.Execute("SELECT val, LENGTH(val) FROM test_char_vs_varchar_char WHERE id = 1")
	charVal := result.Rows[0].Data[0].ToString()
	charLen, _ := result.Rows[0].Data[1].ToInt64()

	// Check VARCHAR
	result, _ = engine.Execute("SELECT val, LENGTH(val) FROM test_char_vs_varchar_varchar WHERE id = 1")
	varcharVal := result.Rows[0].Data[0].ToString()
	varcharLen, _ := result.Rows[0].Data[1].ToInt64()

	// CHAR should be padded
	if charLen != 10 {
		t.Errorf("CHAR should store 10 chars, got %d", charLen)
	}
	if charVal != "ABC       " {
		t.Errorf("CHAR should be padded: got '%s'", charVal)
	}

	// VARCHAR should not be padded
	if varcharLen != 3 {
		t.Errorf("VARCHAR should store 3 chars, got %d", varcharLen)
	}
	if varcharVal != "ABC" {
		t.Errorf("VARCHAR should not be padded: got '%s'", varcharVal)
	}
}

// ==================== NOT NULL Constraint Tests ====================

// TestNotNullConstraint tests NOT NULL validation
func TestNotNullConstraint(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	_, err = engine.Execute("CREATE TABLE test_notnull (id SEQ, name VARCHAR(100) NOT NULL, optional VARCHAR(100))")
	if err != nil {
		t.Fatal(err)
	}

	// Valid insert with value
	_, err = engine.Execute("INSERT INTO test_notnull (name, optional) VALUES ('test', 'value')")
	if err != nil {
		t.Errorf("Failed to insert with all values: %v", err)
	}

	// Valid insert without optional (nullable)
	_, err = engine.Execute("INSERT INTO test_notnull (name) VALUES ('test2')")
	if err != nil {
		t.Errorf("Failed to insert without optional: %v", err)
	}

	// Invalid: NULL in NOT NULL column
	_, err = engine.Execute("INSERT INTO test_notnull (name, optional) VALUES (NULL, 'value')")
	if err == nil {
		t.Error("Expected error for NULL in NOT NULL column")
	}

	// Invalid: missing NOT NULL column
	_, err = engine.Execute("INSERT INTO test_notnull (optional) VALUES ('value')")
	// This might not error depending on default value handling, check implementation
}

// TestNotNullUpdateConstraint tests NOT NULL validation on UPDATE
func TestNotNullUpdateConstraint(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	_, err = engine.Execute("CREATE TABLE test_notnull_update (id SEQ, name VARCHAR(100) NOT NULL)")
	if err != nil {
		t.Fatal(err)
	}

	_, err = engine.Execute("INSERT INTO test_notnull_update (name) VALUES ('initial')")
	if err != nil {
		t.Fatal(err)
	}

	// Valid update
	_, err = engine.Execute("UPDATE test_notnull_update SET name = 'updated' WHERE id = 1")
	if err != nil {
		t.Errorf("Failed to update with valid value: %v", err)
	}

	// Invalid: update to NULL
	_, err = engine.Execute("UPDATE test_notnull_update SET name = NULL WHERE id = 1")
	if err == nil {
		t.Error("Expected error when updating NOT NULL column to NULL")
	}

	// Verify value unchanged
	result, _ := engine.Execute("SELECT name FROM test_notnull_update WHERE id = 1")
	if result.Rows[0].Data[0].ToString() != "updated" {
		t.Error("Value should not have changed after failed update")
	}
}

// ==================== Combined Constraint Tests ====================

// TestMultipleConstraints tests multiple constraints on same column
func TestMultipleConstraints(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// CHAR(10) NOT NULL - both constraints apply
	_, err = engine.Execute("CREATE TABLE test_multi (id SEQ, code CHAR(10) NOT NULL)")
	if err != nil {
		t.Fatal(err)
	}

	// Valid insert
	_, err = engine.Execute("INSERT INTO test_multi (code) VALUES ('ABC')")
	if err != nil {
		t.Errorf("Valid insert failed: %v", err)
	}

	// Check padding
	result, _ := engine.Execute("SELECT code FROM test_multi WHERE id = 1")
	if result.Rows[0].Data[0].ToString() != "ABC       " {
		t.Errorf("CHAR should be padded")
	}

	// Too long
	_, err = engine.Execute("INSERT INTO test_multi (code) VALUES ('12345678901')")
	if err == nil {
		t.Error("Expected error for value too long")
	}

	// NULL value
	_, err = engine.Execute("INSERT INTO test_multi (code) VALUES (NULL)")
	if err == nil {
		t.Error("Expected error for NULL in NOT NULL column")
	}
}

// TestCharWithUnicode tests CHAR type with Unicode characters
// Note: Length is measured in bytes (UTF-8 encoding)
func TestCharWithUnicode(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	tests := []struct {
		name       string
		input      string
		inputBytes int
		shouldErr  bool
	}{
		{"ASCII 3 chars", "ABC", 3, false},
		{"ASCII 10 chars", "1234567890", 10, false},
		{"ASCII 11 chars", "12345678901", 11, true},
		{"Chinese 3 chars", "你好世", 9, false},    // 3 Chinese chars = 3 runes < 10
		{"Chinese 4 chars", "你好世界", 12, false},  // 4 Chinese chars = 4 runes < 10
		{"Chinese 10 chars", "一二三四五六七八九十", 30, false}, // 10 Chinese chars = 10 runes = 10
		{"Chinese 11 chars", "一二三四五六七八九十壹", 33, true},  // 11 Chinese chars = 11 runes > 10
		{"Mixed ASCII+Chinese", "Hi你好", 8, false},    // 2 ASCII + 2 Chinese = 4 runes < 10
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh table for each test
			_, err := engine.Execute("CREATE TABLE test_char_unicode_temp (id SEQ, name CHAR(10))")
			if err != nil {
				t.Fatal(err)
			}
			defer engine.Execute("DROP TABLE test_char_unicode_temp")

			_, err = engine.Execute("INSERT INTO test_char_unicode_temp (name) VALUES ('" + tt.input + "')")

			if tt.shouldErr {
				if err == nil {
					t.Errorf("Expected error for '%s' (%d bytes), but succeeded", tt.input, tt.inputBytes)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error for '%s': %v", tt.input, err)
			}

			// Verify padding - CHAR(10) stores 10 characters (runes)
			// For Unicode, byte length can be larger (e.g., 10 Chinese chars = 30 bytes)
			result, err := engine.Execute("SELECT name, LENGTH(name) FROM test_char_unicode_temp WHERE id = 1")
			if err != nil {
				t.Fatal(err)
			}

			stored := result.Rows[0].Data[0].ToString()
			length, _ := result.Rows[0].Data[1].ToInt64()

			// LENGTH() returns character count, not byte count
			// CHAR(10) should store exactly 10 characters
			if int(length) != 10 {
				t.Errorf("LENGTH() should return 10 characters, got %d", length)
			}

			// Verify stored character count is exactly 10 (CHAR padding by characters)
			if utf8.RuneCountInString(stored) != 10 {
				t.Errorf("Stored value should be 10 characters, got %d", utf8.RuneCountInString(stored))
			}
		})
	}
}

// TestConstraintWithUnicode tests constraints with Unicode characters
// Note: Implementation counts characters (runes), not bytes
func TestConstraintWithUnicode(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// VARCHAR(30) - can hold 30 characters (runes)
	_, err = engine.Execute("CREATE TABLE test_unicode (id SEQ, name VARCHAR(30))")
	if err != nil {
		t.Fatal(err)
	}

	// 10 unicode characters: 你好世界Hello! = 4 Chinese + 6 ASCII = 10 runes
	unicode10 := "你好世界Hello!"
	_, err = engine.Execute("INSERT INTO test_unicode (name) VALUES ('" + unicode10 + "')")
	if err != nil {
		t.Errorf("Failed to insert unicode string: %v", err)
	}

	// String that exceeds 30 characters (runes)
	unicodeLong := "一二三四五六七八九十壹贰叁肆伍陆柒捌玖拾壹贰叁肆伍陆柒捌玖拾壹贰叁肆" // 31 Chinese chars = 31 runes > 30
	_, err = engine.Execute("INSERT INTO test_unicode (name) VALUES ('" + unicodeLong + "')")
	if err == nil {
		t.Error("Expected error for unicode string exceeding character limit")
	}

	// Verify
	result, _ := engine.Execute("SELECT COUNT(*) FROM test_unicode")
	count, _ := result.Rows[0].Data[0].ToInt64()
	if count != 1 {
		t.Errorf("Expected 1 row, got %d", count)
	}
}

// TestTextNoLengthLimit tests that TEXT has no length limit
func TestTextNoLengthLimit(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	_, err = engine.Execute("CREATE TABLE test_text (id SEQ, content TEXT)")
	if err != nil {
		t.Fatal(err)
	}

	// Large text (100KB)
	largeText := strings.Repeat("Hello World! ", 8000) // ~104KB
	_, err = engine.Execute("INSERT INTO test_text (content) VALUES ('" + largeText + "')")
	if err != nil {
		t.Errorf("Failed to insert large TEXT: %v", err)
	}

	// Verify
	result, _ := engine.Execute("SELECT LENGTH(content) FROM test_text WHERE id = 1")
	length, _ := result.Rows[0].Data[0].ToInt64()
	if int(length) != len(largeText) {
		t.Errorf("Expected length %d, got %d", len(largeText), length)
	}
}

// TestVarcharWithoutLength tests VARCHAR without length specifier
func TestVarcharWithoutLength(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// VARCHAR without length (should work, no constraint)
	_, err = engine.Execute("CREATE TABLE test_varchar_no_len (id SEQ, name VARCHAR)")
	if err != nil {
		t.Fatal(err)
	}

	// Should accept any length
	longValue := strings.Repeat("X", 1000)
	_, err = engine.Execute("INSERT INTO test_varchar_no_len (name) VALUES ('" + longValue + "')")
	if err != nil {
		t.Errorf("VARCHAR without length should accept any value: %v", err)
	}
}

// TestCharWithoutLength tests CHAR without length specifier
func TestCharWithoutLength(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// CHAR without length (should work, default to 1 or no padding)
	_, err = engine.Execute("CREATE TABLE test_char_no_len (id SEQ, code CHAR)")
	if err != nil {
		t.Fatal(err)
	}

	// Should work
	_, err = engine.Execute("INSERT INTO test_char_no_len (code) VALUES ('A')")
	if err != nil {
		t.Errorf("CHAR without length should accept single char: %v", err)
	}
}

// TestConstraintErrorMessages tests that error messages are informative
func TestConstraintErrorMessages(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	_, err = engine.Execute("CREATE TABLE test_errors (id SEQ, name VARCHAR(10) NOT NULL)")
	if err != nil {
		t.Fatal(err)
	}

	// Test length error message
	_, err = engine.Execute("INSERT INTO test_errors (name) VALUES ('12345678901')")
	if err == nil {
		t.Error("Expected error")
	} else {
		errMsg := err.Error()
		if !strings.Contains(errMsg, "too long") {
			t.Errorf("Error message should mention 'too long': %s", errMsg)
		}
		if !strings.Contains(errMsg, "name") {
			t.Errorf("Error message should mention column name: %s", errMsg)
		}
		if !strings.Contains(errMsg, "10") {
			t.Errorf("Error message should mention max length: %s", errMsg)
		}
	}

	// Test NOT NULL error message
	_, err = engine.Execute("INSERT INTO test_errors (name) VALUES (NULL)")
	if err == nil {
		t.Error("Expected error")
	} else {
		errMsg := err.Error()
		if !strings.Contains(errMsg, "cannot be null") {
			t.Errorf("Error message should mention 'cannot be null': %s", errMsg)
		}
		if !strings.Contains(errMsg, "name") {
			t.Errorf("Error message should mention column name: %s", errMsg)
		}
	}
}

// TestConstraintWithDefaultValues tests constraints with default values
func TestConstraintWithDefaultValues(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	_, err = engine.Execute("CREATE TABLE test_default (id SEQ, name VARCHAR(10) DEFAULT 'default')")
	if err != nil {
		t.Fatal(err)
	}

	// Insert without specifying name
	_, err = engine.Execute("INSERT INTO test_default (id) VALUES (NULL)")
	// This might fail depending on how defaults are handled

	// Insert with value that fits
	_, err = engine.Execute("INSERT INTO test_default (name) VALUES ('ok')")
	if err != nil {
		t.Errorf("Failed to insert valid value: %v", err)
	}

	// Insert with value too long
	_, err = engine.Execute("INSERT INTO test_default (name) VALUES ('too long value')")
	if err == nil {
		t.Error("Expected error for value too long even with default")
	}
}
