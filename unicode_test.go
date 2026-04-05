// Package xxldb provides Unicode tests
package xxldb

import (
	"testing"
)

// TestUnicodeIssues identifies all potential Unicode handling issues
func TestUnicodeIssues(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Create test table
	_, err = engine.Execute("CREATE TABLE unicode_test (id SEQ, name VARCHAR(100))")
	if err != nil {
		t.Fatal(err)
	}

	// Insert Unicode data
	_, err = engine.Execute("INSERT INTO unicode_test (name) VALUES ('你好世界')")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("LENGTH", func(t *testing.T) {
		// LENGTH should return character count, not byte count
		// "你好世界" has 4 Chinese characters = 12 bytes in UTF-8
		result, err := engine.Execute("SELECT LENGTH(name), BYTE_LENGTH(name) FROM unicode_test WHERE id = 1")
		if err != nil {
			t.Fatal(err)
		}

		charLen, _ := result.Rows[0].Data[0].ToInt64()
		byteLen, _ := result.Rows[0].Data[1].ToInt64()
		t.Logf("LENGTH('你好世界') = %d characters, BYTE_LENGTH = %d bytes", charLen, byteLen)

		if charLen != 4 {
			t.Errorf("LENGTH should return 4 characters, got %d", charLen)
		}
		if byteLen != 12 {
			t.Errorf("BYTE_LENGTH should return 12 bytes, got %d", byteLen)
		}
	})

	t.Run("SUBSTRING", func(t *testing.T) {
		// SUBSTRING should work with character positions
		// SUBSTRING('你好世界', 1, 2) should return '你好' (first 2 characters)
		result, err := engine.Execute("SELECT SUBSTRING(name, 1, 2) FROM unicode_test WHERE id = 1")
		if err != nil {
			t.Logf("SUBSTRING error: %v", err)
		} else {
			substr := result.Rows[0].Data[0].ToString()
			t.Logf("SUBSTRING('你好世界', 1, 2) = '%s' (expected: '你好')", substr)
			if substr != "你好" {
				t.Errorf("SUBSTRING issue: got '%s', expected '你好'", substr)
			}
		}
	})

	t.Run("LEFT", func(t *testing.T) {
		// LEFT('你好世界', 2) should return '你好'
		result, err := engine.Execute("SELECT LEFT(name, 2) FROM unicode_test WHERE id = 1")
		if err != nil {
			t.Logf("LEFT error: %v", err)
		} else {
			left := result.Rows[0].Data[0].ToString()
			t.Logf("LEFT('你好世界', 2) = '%s' (expected: '你好')", left)
			if left != "你好" {
				t.Errorf("LEFT issue: got '%s', expected '你好'", left)
			}
		}
	})

	t.Run("RIGHT", func(t *testing.T) {
		// RIGHT('你好世界', 2) should return '世界'
		result, err := engine.Execute("SELECT RIGHT(name, 2) FROM unicode_test WHERE id = 1")
		if err != nil {
			t.Logf("RIGHT error: %v", err)
		} else {
			right := result.Rows[0].Data[0].ToString()
			t.Logf("RIGHT('你好世界', 2) = '%s' (expected: '世界')", right)
			if right != "世界" {
				t.Errorf("RIGHT issue: got '%s', expected '世界'", right)
			}
		}
	})

	t.Run("LIKE with Unicode", func(t *testing.T) {
		// Create table for LIKE tests
		_, err := engine.Execute("CREATE TABLE like_unicode (id SEQ, name VARCHAR(100))")
		if err != nil {
			t.Fatal(err)
		}

		_, err = engine.Execute("INSERT INTO like_unicode (name) VALUES ('你好'), ('世界'), ('你好世界')")
		if err != nil {
			t.Fatal(err)
		}

		// Test LIKE with Unicode pattern
		result, err := engine.Execute("SELECT name FROM like_unicode WHERE name LIKE '你好%'")
		if err != nil {
			t.Logf("LIKE error: %v", err)
		} else {
			t.Logf("LIKE '你好%%' found %d rows", len(result.Rows))
			for _, row := range result.Rows {
				t.Logf("  - %s", row.Data[0].ToString())
			}
			if len(result.Rows) != 2 {
				t.Errorf("LIKE '你好%%' should match 2 rows, got %d", len(result.Rows))
			}
		}

		// Test LIKE with _ (single character wildcard)
		result, err = engine.Execute("SELECT name FROM like_unicode WHERE name LIKE '你_'")
		if err != nil {
			t.Logf("LIKE with _ error: %v", err)
		} else {
			t.Logf("LIKE '你_' found %d rows", len(result.Rows))
			for _, row := range result.Rows {
				t.Logf("  - %s", row.Data[0].ToString())
			}
			// '你好' should match '你_' (one char after 你)
			if len(result.Rows) != 1 {
				t.Errorf("LIKE '你_' should match 1 row ('你好'), got %d", len(result.Rows))
			}
		}
	})

	t.Run("UPPER/LOWER with Unicode", func(t *testing.T) {
		// Test UPPER/LOWER with Chinese (should have no effect)
		result, err := engine.Execute("SELECT UPPER('你好'), LOWER('你好')")
		if err != nil {
			t.Logf("UPPER/LOWER error: %v", err)
		} else {
			upper := result.Rows[0].Data[0].ToString()
			lower := result.Rows[0].Data[1].ToString()
			t.Logf("UPPER('你好') = '%s', LOWER('你好') = '%s'", upper, lower)
		}

		// Test with mixed content
		result, err = engine.Execute("SELECT UPPER('Hello你好'), LOWER('Hello你好')")
		if err != nil {
			t.Logf("UPPER/LOWER mixed error: %v", err)
		} else {
			upper := result.Rows[0].Data[0].ToString()
			lower := result.Rows[0].Data[1].ToString()
			t.Logf("UPPER('Hello你好') = '%s', LOWER('Hello你好') = '%s'", upper, lower)
		}
	})

	t.Run("CONCAT with Unicode", func(t *testing.T) {
		result, err := engine.Execute("SELECT CONCAT('你好', '世界')")
		if err != nil {
			t.Logf("CONCAT error: %v", err)
		} else {
			concat := result.Rows[0].Data[0].ToString()
			t.Logf("CONCAT('你好', '世界') = '%s'", concat)
			if concat != "你好世界" {
				t.Errorf("CONCAT issue: got '%s', expected '你好世界'", concat)
			}
		}
	})

	t.Run("INSTR with Unicode", func(t *testing.T) {
		// INSTR('你好世界', '世界') should return 3 (1-indexed, character position)
		result, err := engine.Execute("SELECT INSTR('你好世界', '世界')")
		if err != nil {
			t.Logf("INSTR error: %v", err)
		} else {
			pos, _ := result.Rows[0].Data[0].ToInt64()
			t.Logf("INSTR('你好世界', '世界') = %d (expected character position 3)", pos)
		}
	})

	t.Run("REVERSE with Unicode", func(t *testing.T) {
		result, err := engine.Execute("SELECT REVERSE('你好世界')")
		if err != nil {
			t.Logf("REVERSE error: %v", err)
		} else {
			rev := result.Rows[0].Data[0].ToString()
			t.Logf("REVERSE('你好世界') = '%s' (expected: '界世好你')", rev)
			if rev != "界世好你" {
				t.Errorf("REVERSE issue: got '%s', expected '界世好你'", rev)
			}
		}
	})

	t.Run("REPLACE with Unicode", func(t *testing.T) {
		result, err := engine.Execute("SELECT REPLACE('你好世界', '世界', 'Universe')")
		if err != nil {
			t.Logf("REPLACE error: %v", err)
		} else {
			replaced := result.Rows[0].Data[0].ToString()
			t.Logf("REPLACE('你好世界', '世界', 'Universe') = '%s'", replaced)
			if replaced != "你好Universe" {
				t.Errorf("REPLACE issue: got '%s', expected '你好Universe'", replaced)
			}
		}
	})

	t.Run("LPAD/RPAD with Unicode", func(t *testing.T) {
		// LPAD('你好', 10, '*') - should pad to 10 characters
		result, err := engine.Execute("SELECT LPAD('你好', 10, '*')")
		if err != nil {
			t.Logf("LPAD error: %v", err)
		} else {
			lpad := result.Rows[0].Data[0].ToString()
			t.Logf("LPAD('你好', 10, '*') = '%s' (len=%d)", lpad, len(lpad))
		}

		result, err = engine.Execute("SELECT RPAD('你好', 10, '*')")
		if err != nil {
			t.Logf("RPAD error: %v", err)
		} else {
			rpad := result.Rows[0].Data[0].ToString()
			t.Logf("RPAD('你好', 10, '*') = '%s' (len=%d)", rpad, len(rpad))
		}
	})
}

// TestUnicodeSorting tests ORDER BY with Unicode strings
func TestUnicodeSorting(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	_, err = engine.Execute("CREATE TABLE sort_unicode (id SEQ, name VARCHAR(100))")
	if err != nil {
		t.Fatal(err)
	}

	// Insert Unicode strings
	_, err = engine.Execute("INSERT INTO sort_unicode (name) VALUES ('苹果'), ('香蕉'), ('橘子'), ('草莓')")
	if err != nil {
		t.Fatal(err)
	}

	// Test ORDER BY ASC
	result, err := engine.Execute("SELECT name FROM sort_unicode ORDER BY name ASC")
	if err != nil {
		t.Fatal(err)
	}

	t.Log("ORDER BY ASC:")
	for _, row := range result.Rows {
		t.Logf("  %s", row.Data[0].ToString())
	}

	// Test ORDER BY DESC
	result, err = engine.Execute("SELECT name FROM sort_unicode ORDER BY name DESC")
	if err != nil {
		t.Fatal(err)
	}

	t.Log("ORDER BY DESC:")
	for _, row := range result.Rows {
		t.Logf("  %s", row.Data[0].ToString())
	}
}

// TestUnicodeGroupBy tests GROUP BY with Unicode strings
func TestUnicodeGroupBy(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	_, err = engine.Execute("CREATE TABLE group_unicode (id SEQ, category VARCHAR(100), value INT)")
	if err != nil {
		t.Fatal(err)
	}

	// Insert data with Unicode categories
	_, err = engine.Execute("INSERT INTO group_unicode (category, value) VALUES ('水果', 10), ('水果', 20), ('蔬菜', 15), ('蔬菜', 25)")
	if err != nil {
		t.Fatal(err)
	}

	result, err := engine.Execute("SELECT category, SUM(value) FROM group_unicode GROUP BY category")
	if err != nil {
		t.Fatal(err)
	}

	t.Log("GROUP BY result:")
	for _, row := range result.Rows {
		cat := row.Data[0].ToString()
		sum, _ := row.Data[1].ToInt64()
		t.Logf("  %s: %d", cat, sum)
	}

	if len(result.Rows) != 2 {
		t.Errorf("Expected 2 groups, got %d", len(result.Rows))
	}

	// Verify sums
	expectedSums := map[string]int64{"水果": 30, "蔬菜": 40}
	for _, row := range result.Rows {
		cat := row.Data[0].ToString()
		sum, _ := row.Data[1].ToInt64()
		if expected, ok := expectedSums[cat]; ok {
			if sum != expected {
				t.Errorf("SUM for %s: got %d, expected %d", cat, sum, expected)
			}
		}
	}
}

// TestUnicodeComparison tests string comparison with Unicode
func TestUnicodeComparison(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	_, err = engine.Execute("CREATE TABLE compare_unicode (id SEQ, name VARCHAR(100))")
	if err != nil {
		t.Fatal(err)
	}

	_, err = engine.Execute("INSERT INTO compare_unicode (name) VALUES ('苹果'), ('香蕉'), ('橘子')")
	if err != nil {
		t.Fatal(err)
	}

	// Test comparison operators
	result, err := engine.Execute("SELECT name FROM compare_unicode WHERE name > '橘'")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("name > '橘': %d rows", len(result.Rows))
	for _, row := range result.Rows {
		t.Logf("  %s", row.Data[0].ToString())
	}

	// Test BETWEEN with Unicode
	result, err = engine.Execute("SELECT name FROM compare_unicode WHERE name BETWEEN '橘子' AND '香蕉'")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("BETWEEN '橘子' AND '香蕉': %d rows", len(result.Rows))
}

// TestUnicodeDistinct tests DISTINCT with Unicode
func TestUnicodeDistinct(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	_, err = engine.Execute("CREATE TABLE distinct_unicode (id SEQ, name VARCHAR(100))")
	if err != nil {
		t.Fatal(err)
	}

	_, err = engine.Execute("INSERT INTO distinct_unicode (name) VALUES ('你好'), ('世界'), ('你好'), ('世界'), ('你好世界')")
	if err != nil {
		t.Fatal(err)
	}

	result, err := engine.Execute("SELECT DISTINCT name FROM distinct_unicode")
	if err != nil {
		t.Fatal(err)
	}

	t.Log("DISTINCT result:")
	for _, row := range result.Rows {
		t.Logf("  %s", row.Data[0].ToString())
	}

	if len(result.Rows) != 3 {
		t.Errorf("Expected 3 distinct values, got %d", len(result.Rows))
	}
}

// TestUnicodeInClause tests IN clause with Unicode
func TestUnicodeInClause(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	_, err = engine.Execute("CREATE TABLE in_unicode (id SEQ, name VARCHAR(100))")
	if err != nil {
		t.Fatal(err)
	}

	_, err = engine.Execute("INSERT INTO in_unicode (name) VALUES ('苹果'), ('香蕉'), ('橘子'), ('草莓')")
	if err != nil {
		t.Fatal(err)
	}

	result, err := engine.Execute("SELECT name FROM in_unicode WHERE name IN ('苹果', '草莓')")
	if err != nil {
		t.Fatal(err)
	}

	t.Log("IN ('苹果', '草莓'):")
	for _, row := range result.Rows {
		t.Logf("  %s", row.Data[0].ToString())
	}

	if len(result.Rows) != 2 {
		t.Errorf("Expected 2 rows, got %d", len(result.Rows))
	}
}

// TestUnicodeEmoji tests handling of emoji characters
func TestUnicodeEmoji(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	_, err = engine.Execute("CREATE TABLE emoji_test (id SEQ, name VARCHAR(100))")
	if err != nil {
		t.Fatal(err)
	}

	// Emoji characters (4 bytes each in UTF-8)
	_, err = engine.Execute("INSERT INTO emoji_test (name) VALUES ('😀🎉🌟'), ('Hello😀World'), ('👍🏻👎🏻')")
	if err != nil {
		t.Fatal(err)
	}

	// Test LENGTH with emoji
	result, err := engine.Execute("SELECT name, LENGTH(name) FROM emoji_test")
	if err != nil {
		t.Fatal(err)
	}

	t.Log("Emoji test:")
	for _, row := range result.Rows {
		name := row.Data[0].ToString()
		length, _ := row.Data[1].ToInt64()
		t.Logf("  '%s' -> LENGTH=%d", name, length)
	}

	// Test SUBSTRING with emoji
	result, err = engine.Execute("SELECT SUBSTRING(name, 1, 2) FROM emoji_test WHERE id = 1")
	if err != nil {
		t.Fatal(err)
	}

	substr := result.Rows[0].Data[0].ToString()
	t.Logf("SUBSTRING('😀🎉🌟', 1, 2) = '%s'", substr)
	if substr != "😀🎉" {
		t.Errorf("SUBSTRING issue with emoji: got '%s', expected '😀🎉'", substr)
	}
}
