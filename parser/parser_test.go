package parser

import (
	"strings"
	"testing"
)

func TestParseSelect(t *testing.T) {
	tests := []struct {
		sql      string
		stmtType StatementType
		table    string
	}{
		{"SELECT * FROM users", StmtSelect, "users"},
		{"SELECT id, name FROM products", StmtSelect, "products"},
		{"SELECT DISTINCT name FROM customers", StmtSelect, "customers"},
		{"SELECT * FROM users WHERE age > 18", StmtSelect, "users"},
		{"SELECT * FROM users ORDER BY name", StmtSelect, "users"},
		{"SELECT * FROM users LIMIT 10", StmtSelect, "users"},
		{"SELECT * FROM users LIMIT 10 OFFSET 5", StmtSelect, "users"},
		{"SELECT COUNT(*) FROM orders", StmtSelect, "orders"},
		{"SELECT NOW()", StmtSelect, ""},
	}

	for _, tt := range tests {
		stmt, err := ParseString(tt.sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", tt.sql, err)
			continue
		}
		if stmt.Type != tt.stmtType {
			t.Errorf("ParseString(%s) type = %v, want %v", tt.sql, stmt.Type, tt.stmtType)
		}
		if stmt.Table != tt.table {
			t.Errorf("ParseString(%s) table = %s, want %s", tt.sql, stmt.Table, tt.table)
		}
	}
}

func TestParseSelectColumns(t *testing.T) {
	tests := []struct {
		sql      string
		colCount int
	}{
		{"SELECT * FROM users", 1},
		{"SELECT id, name FROM users", 2},
		{"SELECT id AS user_id, name FROM users", 2},
		{"SELECT COUNT(*) FROM users", 1},
	}

	for _, tt := range tests {
		stmt, err := ParseString(tt.sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", tt.sql, err)
			continue
		}
		if len(stmt.Columns) != tt.colCount {
			t.Errorf("ParseString(%s) column count = %d, want %d", tt.sql, len(stmt.Columns), tt.colCount)
		}
	}
}

func TestParseSelectWhere(t *testing.T) {
	sql := "SELECT * FROM users WHERE age > 18 AND name = 'John'"
	stmt, err := ParseString(sql)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}
	if stmt.Where == nil {
		t.Fatal("WHERE clause should not be nil")
	}
}

func TestParseSelectOrderBy(t *testing.T) {
	sql := "SELECT * FROM users ORDER BY name ASC, age DESC"
	stmt, err := ParseString(sql)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}
	if len(stmt.OrderBy) != 2 {
		t.Fatalf("Expected 2 ORDER BY clauses, got %d", len(stmt.OrderBy))
	}
	if stmt.OrderBy[0].Direction != "ASC" {
		t.Error("First ORDER BY should be ASC")
	}
	if stmt.OrderBy[1].Direction != "DESC" {
		t.Error("Second ORDER BY should be DESC")
	}
}

func TestParseSelectGroupBy(t *testing.T) {
	sql := "SELECT department, COUNT(*) FROM employees GROUP BY department"
	stmt, err := ParseString(sql)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}
	if len(stmt.GroupBy) != 1 {
		t.Errorf("Expected 1 GROUP BY column, got %d", len(stmt.GroupBy))
	}
}

func TestParseInsert(t *testing.T) {
	sql := "INSERT INTO users (id, name, age) VALUES (1, 'John', 25)"
	stmt, err := ParseString(sql)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}
	if stmt.Type != StmtInsert {
		t.Errorf("Expected StmtInsert, got %v", stmt.Type)
	}
	if stmt.Table != "users" {
		t.Errorf("Expected table 'users', got '%s'", stmt.Table)
	}
}

func TestParseUpdate(t *testing.T) {
	sql := "UPDATE users SET name = 'Jane', age = 30 WHERE id = 1"
	stmt, err := ParseString(sql)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}
	if stmt.Type != StmtUpdate {
		t.Errorf("Expected StmtUpdate, got %v", stmt.Type)
	}
	if stmt.Table != "users" {
		t.Errorf("Expected table 'users', got '%s'", stmt.Table)
	}
}

func TestParseDelete(t *testing.T) {
	sql := "DELETE FROM users WHERE id = 1"
	stmt, err := ParseString(sql)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}
	if stmt.Type != StmtDelete {
		t.Errorf("Expected StmtDelete, got %v", stmt.Type)
	}
	if stmt.Table != "users" {
		t.Errorf("Expected table 'users', got '%s'", stmt.Table)
	}
}

func TestParseCreateTable(t *testing.T) {
	sql := "CREATE TABLE users (id SEQ PRIMARY KEY, name VARCHAR(100), age INT)"
	stmt, err := ParseString(sql)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}
	if stmt.Type != StmtCreateTable {
		t.Errorf("Expected StmtCreateTable, got %v", stmt.Type)
	}
	if stmt.Table != "users" {
		t.Errorf("Expected table 'users', got '%s'", stmt.Table)
	}
}

func TestParseDropTable(t *testing.T) {
	sql := "DROP TABLE users"
	stmt, err := ParseString(sql)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}
	if stmt.Type != StmtDropTable {
		t.Errorf("Expected StmtDropTable, got %v", stmt.Type)
	}
	if stmt.Table != "users" {
		t.Errorf("Expected table 'users', got '%s'", stmt.Table)
	}
}

func TestParseExpression(t *testing.T) {
	tests := []struct {
		sql      string
		exprType ExprType
	}{
		{"SELECT * FROM t WHERE a = 1", ExprBinaryOp},
		{"SELECT * FROM t WHERE a > 1", ExprBinaryOp},
		{"SELECT * FROM t WHERE a AND b", ExprBinaryOp},
		{"SELECT * FROM t WHERE a OR b", ExprBinaryOp},
		{"SELECT * FROM t WHERE NOT a", ExprUnaryOp},
		{"SELECT * FROM t WHERE a LIKE '%test%'", ExprBinaryOp},
		{"SELECT * FROM t WHERE a IN (1, 2, 3)", ExprIn},
		{"SELECT * FROM t WHERE a BETWEEN 1 AND 10", ExprBetween},
		{"SELECT COUNT(*) FROM t", ExprFunction},
	}

	for _, tt := range tests {
		stmt, err := ParseString(tt.sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", tt.sql, err)
			continue
		}
		if stmt.Where != nil && stmt.Where.Type != tt.exprType {
			t.Errorf("ParseString(%s) where type = %v, want %v", tt.sql, stmt.Where.Type, tt.exprType)
		}
	}
}

func TestParseFunction(t *testing.T) {
	sql := "SELECT COUNT(*), SUM(amount), AVG(price) FROM orders"
	stmt, err := ParseString(sql)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	if len(stmt.Columns) != 3 {
		t.Errorf("Expected 3 columns, got %d", len(stmt.Columns))
	}
}

func TestParseJoin(t *testing.T) {
	sql := "SELECT * FROM users INNER JOIN orders ON users.id = orders.user_id"
	stmt, err := ParseString(sql)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	if len(stmt.Joins) != 1 {
		t.Errorf("Expected 1 JOIN, got %d", len(stmt.Joins))
	}
	if stmt.Joins[0].Type != "INNER" {
		t.Errorf("Expected INNER JOIN, got %s", stmt.Joins[0].Type)
	}
	if stmt.Joins[0].Table != "orders" {
		t.Errorf("Expected orders table, got %s", stmt.Joins[0].Table)
	}
}

func TestParseCase(t *testing.T) {
	sql := "SELECT CASE WHEN age < 18 THEN 'minor' WHEN age < 65 THEN 'adult' ELSE 'senior' END FROM users"
	stmt, err := ParseString(sql)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	if len(stmt.Columns) != 1 {
		t.Errorf("Expected 1 column, got %d", len(stmt.Columns))
	}
	if stmt.Columns[0].Expr == nil || stmt.Columns[0].Expr.Type != ExprCase {
		t.Error("Expected CASE expression")
	}
}

func TestParseSubquery(t *testing.T) {
	sql := "SELECT * FROM (SELECT id FROM users) AS sub"
	stmt, err := ParseString(sql)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	if stmt.From == nil || stmt.From.Subquery == nil {
		t.Error("Expected subquery in FROM clause")
	}
}

func TestParseUnion(t *testing.T) {
	sql := "SELECT id FROM users UNION SELECT id FROM admins"
	stmt, err := ParseString(sql)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	if len(stmt.Union) != 1 {
		t.Errorf("Expected 1 UNION, got %d", len(stmt.Union))
	}
}

func TestParseInvalid(t *testing.T) {
	invalidSQLs := []string{
		"INVALID SQL",
	}

	for _, sql := range invalidSQLs {
		_, err := ParseString(sql)
		if err == nil {
			t.Errorf("ParseString(%s) should return error", sql)
		}
	}
}

func TestTokenize(t *testing.T) {
	input := "SELECT * FROM users"
	tokens := Tokenize(input)

	if len(tokens) == 0 {
		t.Error("Tokenize should return tokens")
	}

	// Verify specific tokens exist
	hasKeyword := false
	hasStar := false
	for _, tok := range tokens {
		if tok.Type == TokKeyword {
			hasKeyword = true
		}
		if tok.Type == TokStar {
			hasStar = true
		}
	}

	if !hasKeyword {
		t.Error("Expected keyword token")
	}
	if !hasStar {
		t.Error("Expected star token")
	}
}

func TestParserCaseInsensitive(t *testing.T) {
	sqls := []string{
		"select * from users",
		"SELECT * FROM users",
		"Select * From Users",
	}

	for _, sql := range sqls {
		stmt, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
			continue
		}
		if stmt.Type != StmtSelect {
			t.Errorf("ParseString(%s) type = %v, want StmtSelect", sql, stmt.Type)
		}
	}
}

func TestParseStringLiterals(t *testing.T) {
	sql := "SELECT * FROM users WHERE name = 'John'"
	stmt, err := ParseString(sql)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	if stmt.Where == nil {
		t.Fatal("WHERE clause should not be nil")
	}
}

func TestParseQualifiedColumns(t *testing.T) {
	sql := "SELECT u.id, u.name FROM users u"
	stmt, err := ParseString(sql)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	if len(stmt.Columns) != 2 {
		t.Errorf("Expected 2 columns, got %d", len(stmt.Columns))
	}
}

func TestParseShow(t *testing.T) {
	tests := []string{
		"SHOW TABLES",
		"SHOW COLUMNS FROM users",
	}

	for _, sql := range tests {
		stmt, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
			continue
		}
		if stmt.Type != StmtShow {
			t.Errorf("ParseString(%s) type = %v, want StmtShow", sql, stmt.Type)
		}
	}
}

func TestExpressionString(t *testing.T) {
	// Test that expressions can be converted back to strings
	expr := NewBinaryExpr(
		NewColumnExpr("a"),
		"=",
		NewLiteralExpr(1),
	)

	s := expr.String()
	if !strings.Contains(s, "a") || !strings.Contains(s, "=") {
		t.Errorf("Expression.String() = %s, expected to contain 'a' and '='", s)
	}
}
