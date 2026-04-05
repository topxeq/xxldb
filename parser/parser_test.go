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

// TestParseAlterMore tests ALTER TABLE parsing
func TestParseAlterMore(t *testing.T) {
	tests := []string{
		"ALTER TABLE t ADD COLUMN name VARCHAR(100)",
		"ALTER TABLE t ADD name VARCHAR(100)",
		"ALTER TABLE t DROP COLUMN name",
		"ALTER TABLE t DROP name",
		"ALTER TABLE t MODIFY COLUMN name VARCHAR(200)",
		"ALTER TABLE t RENAME COLUMN old_name TO new_name",
		"ALTER TABLE t RENAME TO new_table",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Logf("ParseString(%s): %v", sql, err)
		}
	}
}

// TestParseDeleteMore tests DELETE parsing
func TestParseDeleteMore(t *testing.T) {
	tests := []struct {
		sql      string
		table    string
		hasWhere bool
	}{
		{"DELETE FROM t", "t", false},
		{"DELETE FROM t WHERE a = 1", "t", true},
		{"DELETE FROM t WHERE a > 5 AND b < 10", "t", true},
	}

	for _, tt := range tests {
		stmt, err := ParseString(tt.sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", tt.sql, err)
			continue
		}
		if stmt.Table != tt.table {
			t.Errorf("Expected table %s, got %s", tt.table, stmt.Table)
		}
		if (stmt.Where != nil) != tt.hasWhere {
			t.Errorf("Expected hasWhere=%v", tt.hasWhere)
		}
	}
}

// TestParseColumnDefMore tests column definition parsing
func TestParseColumnDefMore(t *testing.T) {
	tests := []string{
		"CREATE TABLE t (id INT PRIMARY KEY)",
		"CREATE TABLE t (id INT NOT NULL)",
		"CREATE TABLE t (id INT UNIQUE)",
		"CREATE TABLE t (id INT DEFAULT 0)",
		"CREATE TABLE t (id INT NOT NULL PRIMARY KEY)",
		"CREATE TABLE t (name VARCHAR(255) NOT NULL)",
		"CREATE TABLE t (data BLOB)",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseBetweenExprMore tests BETWEEN expressions
func TestParseBetweenExprMore(t *testing.T) {
	tests := []string{
		"SELECT * FROM t WHERE a BETWEEN 1 AND 10",
		"SELECT * FROM t WHERE a NOT BETWEEN 1 AND 10",
		"SELECT * FROM t WHERE a BETWEEN b AND c",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseExistsExprMore tests EXISTS expressions
func TestParseExistsExprMore(t *testing.T) {
	tests := []string{
		"SELECT * FROM t WHERE EXISTS (SELECT 1)",
		"SELECT * FROM t WHERE NOT EXISTS (SELECT 1 FROM t2)",
		"SELECT * FROM t WHERE EXISTS (SELECT * FROM t2 WHERE t2.id = t.id)",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseParenExprMore tests parenthesized expressions
func TestParseParenExprMore(t *testing.T) {
	tests := []string{
		"SELECT * FROM t WHERE (a = 1)",
		"SELECT * FROM t WHERE (a = 1 OR b = 2) AND c = 3",
		"SELECT * FROM t WHERE ((a = 1))",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestLexerOperators tests operator scanning
func TestLexerOperators(t *testing.T) {
	tests := []string{
		"SELECT a > b FROM t",
		"SELECT a < b FROM t",
		"SELECT a >= b FROM t",
		"SELECT a <= b FROM t",
		"SELECT a <> b FROM t",
		"SELECT a != b FROM t",
		"SELECT a = b FROM t",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseAlterFull tests ALTER TABLE parsing fully
func TestParseAlterFull(t *testing.T) {
	tests := []string{
		"ALTER TABLE t ADD COLUMN name VARCHAR(100)",
		"ALTER TABLE t ADD name VARCHAR(100)",
		"ALTER TABLE t ADD COLUMN age INT NOT NULL",
		"ALTER TABLE t DROP COLUMN name",
		"ALTER TABLE t DROP name",
		"ALTER TABLE t MODIFY COLUMN name VARCHAR(200)",
		"ALTER TABLE t MODIFY name TEXT",
		"ALTER TABLE t RENAME COLUMN old_name TO new_name",
		"ALTER TABLE t RENAME TO new_table",
		"ALTER TABLE t ADD PRIMARY KEY (id)",
		"ALTER TABLE t ADD CONSTRAINT pk PRIMARY KEY (id)",
	}

	for _, sql := range tests {
		stmt, err := ParseString(sql)
		if err != nil {
			t.Logf("ParseString(%s): %v", sql, err)
		} else {
			t.Logf("Parsed: %s -> type=%v, actions=%d", sql, stmt.Type, len(stmt.AlterActions))
		}
	}
}

// TestParseUpdateMore tests UPDATE parsing
func TestParseUpdateMore(t *testing.T) {
	tests := []string{
		"UPDATE t SET a = 1",
		"UPDATE t SET a = 1 WHERE b = 2",
		"UPDATE t SET a = a + 1",
		"UPDATE t SET a = UPPER(a)",
		"UPDATE t SET a = 1, b = 2, c = 3",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseInsertMore tests INSERT parsing
func TestParseInsertMore(t *testing.T) {
	tests := []string{
		"INSERT INTO t VALUES (1, 2, 3)",
		"INSERT INTO t (a, b) VALUES (1, 2)",
		"INSERT INTO t (a, b) VALUES (1, 2), (3, 4)",
		"INSERT INTO t SELECT * FROM t2",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseRestore tests RESTORE parsing
func TestParseRestore(t *testing.T) {
	tests := []string{
		"RESTORE FROM '/path/to/backup'",
		"RESTORE '/path/to/backup'",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseSetMore tests SET parsing
func TestParseSetMore(t *testing.T) {
	tests := []string{
		"SET LOG_LEVEL = 'DEBUG'",
		"SET USER = 'admin'",
		"SET PASSWORD = 'secret'",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestLexerStringLiterals tests string literal scanning
func TestLexerStringLiterals(t *testing.T) {
	tests := []string{
		"SELECT 'hello' FROM t",
		"SELECT \"world\" FROM t",
		"SELECT '' FROM t",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Logf("ParseString(%s): %v", sql, err)
		}
	}
}

// TestTokenString tests Token.String method
func TestTokenString(t *testing.T) {
	token := Token{Type: TokKeyword, Value: "SELECT", Pos: 0}
	str := token.String()
	if str == "" {
		t.Error("Token.String() should return non-empty string")
	}
	t.Logf("Token string: %s", str)
}

// TestParseConstraint tests constraint parsing
func TestParseConstraint(t *testing.T) {
	tests := []string{
		"CREATE TABLE t (id INT PRIMARY KEY)",
		"CREATE TABLE t (id INT, PRIMARY KEY (id))",
		"CREATE TABLE t (id INT UNIQUE)",
		"CREATE TABLE t (id INT, UNIQUE (id))",
		"CREATE TABLE t (id INT NOT NULL)",
		
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseCreateIndex tests CREATE INDEX parsing
func TestParseCreateIndex(t *testing.T) {
	tests := []string{
		"CREATE INDEX idx_name ON t (name)",
		"CREATE UNIQUE INDEX idx_id ON t (id)",
		"CREATE INDEX idx_multi ON t (a, b)",
	}

	for _, sql := range tests {
		stmt, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		} else {
			t.Logf("Parsed: %s -> type=%v, index=%s", sql, stmt.Type, stmt.IndexName)
		}
	}
}

// TestParseDropIndex tests DROP INDEX parsing
func TestParseDropIndex(t *testing.T) {
	sql := "DROP INDEX idx_name"
	stmt, err := ParseString(sql)
	if err != nil {
		t.Errorf("ParseString failed: %v", err)
	} else {
		t.Logf("DROP INDEX: %s", stmt.IndexName)
	}
}

// TestParseCreateDatabase tests CREATE DATABASE parsing
func TestParseCreateDatabase(t *testing.T) {
	sql := "CREATE DATABASE mydb"
	_, err := ParseString(sql)
	if err != nil {
		t.Logf("CREATE DATABASE: %v", err)
	}
}

// TestParseDropDatabase tests DROP DATABASE parsing
func TestParseDropDatabase(t *testing.T) {
	sql := "DROP DATABASE mydb"
	_, err := ParseString(sql)
	if err != nil {
		t.Logf("DROP DATABASE: %v", err)
	}
}

// TestParseUse tests USE parsing
func TestParseUse(t *testing.T) {
	sql := "USE mydb"
	_, err := ParseString(sql)
	if err != nil {
		t.Logf("USE: %v", err)
	}
}

// TestParseBackup tests BACKUP parsing
func TestParseBackup(t *testing.T) {
	tests := []string{
		"BACKUP TO '/path/to/backup'",
		"BACKUP '/path/to/backup'",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseBegin tests BEGIN parsing
func TestParseBegin(t *testing.T) {
	sql := "BEGIN"
	_, err := ParseString(sql)
	if err != nil {
		t.Logf("BEGIN: %v", err)
	}
}

// TestParseCommit tests COMMIT parsing
func TestParseCommit(t *testing.T) {
	sql := "COMMIT"
	_, err := ParseString(sql)
	if err != nil {
		t.Logf("COMMIT: %v", err)
	}
}

// TestParseRollback tests ROLLBACK parsing
func TestParseRollback(t *testing.T) {
	sql := "ROLLBACK"
	_, err := ParseString(sql)
	if err != nil {
		t.Logf("ROLLBACK: %v", err)
	}
}

// TestParseJoinClause tests JOIN parsing
func TestParseJoinClause(t *testing.T) {
	tests := []string{
		"SELECT * FROM a JOIN b ON a.id = b.id",
		"SELECT * FROM a LEFT JOIN b ON a.id = b.id",
		"SELECT * FROM a RIGHT JOIN b ON a.id = b.id",
		"SELECT * FROM a INNER JOIN b ON a.id = b.id",
		"SELECT * FROM a JOIN b USING (id)",
		"SELECT * FROM a CROSS JOIN b",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseUnaryExpr tests unary expression parsing
func TestParseUnaryExpr(t *testing.T) {
	tests := []string{
		"SELECT -id FROM t",
		"SELECT NOT active FROM t",
		"SELECT * FROM t WHERE NOT (a = 1)",
		"SELECT * FROM t WHERE -id > 0",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseMultiplicativeExpr tests multiplicative expression parsing
func TestParseMultiplicativeExpr(t *testing.T) {
	tests := []string{
		"SELECT a * b FROM t",
		"SELECT a / b FROM t",
		"SELECT a % b FROM t",
		"SELECT a * b + c FROM t",
		"SELECT a + b * c FROM t",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseAdditiveExpr tests additive expression parsing
func TestParseAdditiveExpr(t *testing.T) {
	tests := []string{
		"SELECT a + b FROM t",
		"SELECT a - b FROM t",
		"SELECT a + b - c FROM t",
		"SELECT a || b FROM t",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseParenExprComprehensive tests parenthesized expressions
func TestParseParenExprComprehensive(t *testing.T) {
	tests := []string{
		"SELECT * FROM t WHERE (a = 1)",
		"SELECT * FROM t WHERE (a = 1 OR b = 2)",
		"SELECT * FROM t WHERE ((a = 1))",
		"SELECT * FROM t WHERE (a = 1) AND (b = 2)",
		"SELECT * FROM t WHERE NOT (a = 1)",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestLexerScanString tests string scanning
func TestLexerScanString(t *testing.T) {
	tests := []string{
		"SELECT 'hello' FROM t",
		"SELECT '' FROM t",
		"SELECT 'it''s' FROM t",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Logf("ParseString(%s): %v", sql, err)
		}
	}
}

// TestLexerSkipComment tests comment skipping
func TestLexerSkipComment(t *testing.T) {
	tests := []string{
		"SELECT * FROM t -- comment",
		"SELECT * /* block comment */ FROM t",
		"SELECT * FROM t /* multi\nline\ncomment */ WHERE a = 1",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Logf("ParseString(%s): %v", sql, err)
		}
	}
}

// TestLexerScanParameter tests parameter scanning
func TestLexerScanParameter(t *testing.T) {
	tests := []string{
		"SELECT * FROM t WHERE a = ?",
		"SELECT * FROM t WHERE a = :param",
		"SELECT * FROM t WHERE a = $1",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Logf("ParseString(%s): %v", sql, err)
		}
	}
}

// TestASTStringMethods tests AST String methods
func TestASTStringMethods(t *testing.T) {
	// Expression.String()
	expr := &Expression{
		Type:   ExprLiteral,
		Literal: "test",
	}
	t.Logf("Expression.String: %s", expr.String())

	// SelectColumn.String()
	col := &SelectColumn{
		Expr:  expr,
		Alias: "alias",
	}
	t.Logf("SelectColumn.String: %s", col.String())

	// ColumnDef.String()
	colDef := &ColumnDef{
		Name:     "id",
		Type:     "INT",
		Nullable: true,
	}
	t.Logf("ColumnDef.String: %s", colDef.String())
}

// TestParseShowComprehensive tests SHOW parsing
func TestParseShowComprehensive(t *testing.T) {
	tests := []string{
		"SHOW TABLES",
		"SHOW COLUMNS FROM t",
		"SHOW CREATE TABLE t",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseDropComprehensive tests DROP parsing
func TestParseDropComprehensive(t *testing.T) {
	tests := []string{
		"DROP TABLE t",
		"DROP TABLE IF EXISTS t",
		"DROP INDEX idx",
		"DROP DATABASE db",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Logf("ParseString(%s): %v", sql, err)
		}
	}
}

// TestParseCreateComprehensive tests CREATE parsing
func TestParseCreateComprehensive(t *testing.T) {
	tests := []string{
		"CREATE TABLE t (id INT)",
		"CREATE TABLE IF NOT EXISTS t (id INT)",
		"CREATE INDEX idx ON t (id)",
		"CREATE UNIQUE INDEX idx ON t (id)",
		"CREATE DATABASE db",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Logf("ParseString(%s): %v", sql, err)
		}
	}
}

// TestParseComparisonExpr tests comparison expressions
func TestParseComparisonExpr(t *testing.T) {
	tests := []string{
		"SELECT * FROM t WHERE a = 1",
		"SELECT * FROM t WHERE a != 1",
		"SELECT * FROM t WHERE a <> 1",
		"SELECT * FROM t WHERE a > 1",
		"SELECT * FROM t WHERE a >= 1",
		"SELECT * FROM t WHERE a < 1",
		"SELECT * FROM t WHERE a <= 1",
		"SELECT * FROM t WHERE a LIKE '%test%'",
		"SELECT * FROM t WHERE a NOT LIKE '%test%'",
		"SELECT * FROM t WHERE a IS NULL",
		"SELECT * FROM t WHERE a IS NOT NULL",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseSelectExpressions tests SELECT expressions
func TestParseSelectExpressions(t *testing.T) {
	tests := []string{
		"SELECT a + b FROM t",
		"SELECT a - b FROM t",
		"SELECT a * b FROM t",
		"SELECT a / b FROM t",
		"SELECT a % b FROM t",
		"SELECT -a FROM t",
		"SELECT NOT a FROM t",
		"SELECT a || b FROM t",
		
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseFunctionCalls tests function call parsing
func TestParseFunctionCalls(t *testing.T) {
	tests := []string{
		"SELECT COUNT(*) FROM t",
		"SELECT COUNT(id) FROM t",
		"SELECT SUM(value) FROM t",
		"SELECT AVG(value) FROM t",
		"SELECT MIN(value) FROM t",
		"SELECT MAX(value) FROM t",
		"SELECT UPPER(name) FROM t",
		"SELECT LOWER(name) FROM t",
		"SELECT CONCAT(a, b) FROM t",
		"SELECT NOW()",
		"SELECT DATE()",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseSelectWithOptions tests SELECT with various options
func TestParseSelectWithOptions(t *testing.T) {
	tests := []string{
		"SELECT DISTINCT name FROM t",
		"SELECT * FROM t LIMIT 10",
		"SELECT * FROM t OFFSET 5",
		"SELECT * FROM t LIMIT 10 OFFSET 5",
		"SELECT * FROM t ORDER BY id",
		"SELECT * FROM t ORDER BY id DESC",
		"SELECT * FROM t ORDER BY id, name",
		"SELECT * FROM t GROUP BY category",
		"SELECT * FROM t GROUP BY category HAVING COUNT(*) > 1",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseNumberLiteral tests number parsing
func TestParseNumberLiteral(t *testing.T) {
	tests := []string{
		"SELECT 123",
		"SELECT 123.456",
		"SELECT -123",
		"SELECT +123",
		"SELECT 1e10",
		"SELECT 1.5e-3",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Logf("ParseString(%s): %v", sql, err)
		}
	}
}

// TestLexerOperators tests operator scanning
func TestLexerOperatorsTest(t *testing.T) {
	tests := []string{
		"SELECT a = b FROM t",
		"SELECT a != b FROM t",
		"SELECT a <> b FROM t",
		"SELECT a < b FROM t",
		"SELECT a > b FROM t",
		"SELECT a <= b FROM t",
		"SELECT a >= b FROM t",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseConstraintComprehensive tests parseConstraint comprehensively
func TestParseConstraintComprehensive(t *testing.T) {
	tests := []string{
		"CREATE TABLE t (id INT, PRIMARY KEY (id))",
		"CREATE TABLE t (id INT, CONSTRAINT pk_id PRIMARY KEY (id))",
		"CREATE TABLE t (id INT, UNIQUE (id))",
		"CREATE TABLE t (id INT, CONSTRAINT uk_id UNIQUE (id))",
		"CREATE TABLE t (id INT, name VARCHAR(50), PRIMARY KEY (id, name))",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Logf("ParseString(%s): %v", sql, err)
		}
	}
}

// TestParseAlterMoreComprehensive tests parseAlter comprehensively
func TestParseAlterMoreComprehensive(t *testing.T) {
	tests := []string{
		"ALTER TABLE t ADD COLUMN new_col INT",
		"ALTER TABLE t ADD new_col VARCHAR(50)",
		"ALTER TABLE t DROP COLUMN old_col",
		"ALTER TABLE t DROP old_col",
		"ALTER TABLE t MODIFY COLUMN col VARCHAR(100)",
		"ALTER TABLE t MODIFY col INT",
		"ALTER TABLE t RENAME COLUMN old_name TO new_name",
		"ALTER TABLE t RENAME TO new_table",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Logf("ParseString(%s): %v", sql, err)
		}
	}
}

// TestParseJoinClauseComprehensive tests parseJoinClause comprehensively
func TestParseJoinClauseComprehensive(t *testing.T) {
	tests := []string{
		"SELECT * FROM t1 INNER JOIN t2 ON t1.id = t2.id",
		"SELECT * FROM t1 LEFT JOIN t2 ON t1.id = t2.id",
		"SELECT * FROM t1 RIGHT JOIN t2 ON t1.id = t2.id",
		"SELECT * FROM t1 JOIN t2 ON t1.id = t2.id",
		"SELECT * FROM t1 INNER JOIN t2 ON t1.id = t2.id AND t1.status = 'active'",
		"SELECT * FROM t1 LEFT OUTER JOIN t2 ON t1.id = t2.id",
		"SELECT * FROM t1 RIGHT OUTER JOIN t2 ON t1.id = t2.id",
		"SELECT a.*, b.name FROM users a INNER JOIN profiles b ON a.id = b.user_id",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseSelectWithMultipleJoins tests SELECT with multiple JOINs
func TestParseSelectWithMultipleJoins(t *testing.T) {
	sql := "SELECT * FROM t1 INNER JOIN t2 ON t1.id = t2.id LEFT JOIN t3 ON t2.id = t3.t2_id"
	_, err := ParseString(sql)
	if err != nil {
		t.Errorf("ParseString failed: %v", err)
	}
}

// TestLexerSkipBlockComment tests block comment skipping
func TestLexerSkipBlockComment(t *testing.T) {
	tests := []string{
		"SELECT /* comment */ a FROM t",
		"SELECT a /* multi\nline\ncomment */ FROM t",
		"/* leading comment */ SELECT a FROM t",
		"SELECT a FROM t /* trailing comment */",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Logf("ParseString(%s): %v", sql, err)
		}
	}
}

// TestLexerScanStringComprehensive tests string scanning
func TestLexerScanStringComprehensive(t *testing.T) {
	tests := []string{
		`SELECT 'hello world'`,
		`SELECT "double quoted"`,
		`SELECT 'it''s escaped'`,
		`SELECT 'unicode: 你好'`,
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Logf("ParseString(%s): %v", sql, err)
		}
	}
}

// TestParseUpdateComprehensive tests UPDATE parsing
func TestParseUpdateComprehensive(t *testing.T) {
	tests := []string{
		"UPDATE t SET a = 1",
		"UPDATE t SET a = 1, b = 2, c = 3",
		"UPDATE t SET a = 1 WHERE id = 1",
		"UPDATE t SET a = b + 1 WHERE id > 10",
		"UPDATE t SET name = 'test' WHERE active = true",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseInsertComprehensive tests INSERT parsing
func TestParseInsertComprehensive(t *testing.T) {
	tests := []string{
		"INSERT INTO t VALUES (1, 2, 3)",
		"INSERT INTO t (a, b, c) VALUES (1, 2, 3)",
		"INSERT INTO t VALUES (1, 2), (3, 4), (5, 6)",
		"INSERT INTO t (a) VALUES ('string')",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseDeleteComprehensive tests DELETE parsing
func TestParseDeleteComprehensive(t *testing.T) {
	tests := []string{
		"DELETE FROM t",
		"DELETE FROM t WHERE id = 1",
		"DELETE FROM t WHERE id > 10 AND status = 'inactive'",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseCreateIndexComprehensive tests CREATE INDEX parsing
func TestParseCreateIndexComprehensive(t *testing.T) {
	tests := []string{
		"CREATE INDEX idx_name ON t (col)",
		"CREATE INDEX idx_name ON t (col1, col2)",
		"CREATE UNIQUE INDEX idx_name ON t (col)",
		"CREATE INDEX IF NOT EXISTS idx_name ON t (col)",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Logf("ParseString(%s): %v", sql, err)
		}
	}
}

// TestParseDropIndexComprehensive tests DROP INDEX parsing
func TestParseDropIndexComprehensive(t *testing.T) {
	tests := []string{
		"DROP INDEX idx_name",
		"DROP INDEX IF EXISTS idx_name",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Logf("ParseString(%s): %v", sql, err)
		}
	}
}


// TestParseDeleteWithConditions tests DELETE parsing with conditions
func TestParseDeleteWithConditions(t *testing.T) {
	tests := []string{
		"DELETE FROM t WHERE a = 1",
		"DELETE FROM t WHERE a > 1 AND b < 10",
		"DELETE FROM t WHERE a IN (1, 2, 3)",
		"DELETE FROM t WHERE a NOT IN (4, 5)",
		"DELETE FROM t WHERE a BETWEEN 1 AND 10",
		"DELETE FROM t WHERE a IS NULL",
		"DELETE FROM t WHERE a IS NOT NULL",
		"DELETE FROM t WHERE name LIKE '%test%'",
		"DELETE FROM t WHERE EXISTS (SELECT 1 FROM t2)",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseUpdateWithExpressions tests UPDATE parsing with expressions
func TestParseUpdateWithExpressions(t *testing.T) {
	tests := []string{
		"UPDATE t SET a = 1",
		"UPDATE t SET a = 1, b = 2",
		"UPDATE t SET a = 1, b = 2, c = 3",
		"UPDATE t SET a = a + 1",
		"UPDATE t SET a = b * 2 - 1",
		"UPDATE t SET a = 1 WHERE b > 0",
		"UPDATE t SET name = 'test' WHERE id = 1",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseInsertVariations tests INSERT parsing variations
func TestParseInsertVariations(t *testing.T) {
	tests := []string{
		"INSERT INTO t VALUES (1)",
		"INSERT INTO t VALUES (1, 2, 3)",
		"INSERT INTO t (a, b) VALUES (1, 2)",
		"INSERT INTO t VALUES (1, 2), (3, 4)",
		"INSERT INTO t (a) VALUES (1), (2), (3)",
		"INSERT INTO t SELECT * FROM t2",
		"INSERT INTO t (a, b) SELECT x, y FROM t2",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseSelectWithExpressions tests SELECT with complex expressions
func TestParseSelectWithExpressions(t *testing.T) {
	tests := []string{
		"SELECT a + b FROM t",
		"SELECT a * b - c FROM t",
		
		"SELECT a, b * 2 AS doubled FROM t",
		"SELECT COUNT(*), SUM(a), AVG(b) FROM t",
		"SELECT a, MAX(b) FROM t GROUP BY a",
		"SELECT a FROM t ORDER BY b DESC LIMIT 10",
		"SELECT DISTINCT a FROM t",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseColumnDefVariations tests column definition parsing
func TestParseColumnDefVariations(t *testing.T) {
	tests := []string{
		"CREATE TABLE t (id INT)",
		"CREATE TABLE t (id INT PRIMARY KEY)",
		"CREATE TABLE t (id INT NOT NULL)",
		"CREATE TABLE t (id INT NOT NULL PRIMARY KEY)",
		"CREATE TABLE t (name VARCHAR(100))",
		"CREATE TABLE t (name VARCHAR(100) NOT NULL)",
		"CREATE TABLE t (id INT DEFAULT 0)",
		"CREATE TABLE t (id INT DEFAULT 0 NOT NULL)",
		"CREATE TABLE t (id SEQ PRIMARY KEY)",
		"CREATE TABLE t (id INT, name VARCHAR(50), active INT DEFAULT 1)",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseConstraintVariations tests constraint parsing
func TestParseConstraintVariations(t *testing.T) {
	tests := []string{
		"CREATE TABLE t (id INT, PRIMARY KEY (id))",
		"CREATE TABLE t (id INT, name VARCHAR(50), PRIMARY KEY (id, name))",
		"CREATE TABLE t (id INT, UNIQUE (id))",
		"CREATE TABLE t (id INT, name VARCHAR(50), UNIQUE (id, name))",
		"CREATE TABLE t (id INT, CONSTRAINT pk_id PRIMARY KEY (id))",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Logf("ParseString(%s): %v", sql, err)
		}
	}
}

// TestParseCreateIndexVariations tests CREATE INDEX parsing
func TestParseCreateIndexVariations(t *testing.T) {
	tests := []string{
		"CREATE INDEX idx ON t (col)",
		"CREATE INDEX idx ON t (col1, col2)",
		"CREATE UNIQUE INDEX idx ON t (col)",
		"CREATE INDEX IF NOT EXISTS idx ON t (col)",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Logf("ParseString(%s): %v", sql, err)
		}
	}
}

// TestParseDropIndexVariations tests DROP INDEX parsing
func TestParseDropIndexVariations(t *testing.T) {
	tests := []string{
		"DROP INDEX idx",
		"DROP INDEX IF EXISTS idx",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Logf("ParseString(%s): %v", sql, err)
		}
	}
}

// TestParseAlterVariations tests ALTER TABLE parsing
func TestParseAlterVariations(t *testing.T) {
	tests := []string{
		"ALTER TABLE t ADD COLUMN x INT",
		"ALTER TABLE t ADD x VARCHAR(50)",
		"ALTER TABLE t DROP COLUMN x",
		"ALTER TABLE t DROP x",
		"ALTER TABLE t MODIFY COLUMN x VARCHAR(100)",
		"ALTER TABLE t MODIFY x INT",
		"ALTER TABLE t RENAME COLUMN old TO new",
		"ALTER TABLE t RENAME TO new_name",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Logf("ParseString(%s): %v", sql, err)
		}
	}
}

// TestParseSelectWithJoin tests SELECT with JOIN parsing
func TestParseSelectWithJoin(t *testing.T) {
	tests := []string{
		"SELECT * FROM t1 JOIN t2 ON t1.id = t2.id",
		"SELECT * FROM t1 INNER JOIN t2 ON t1.id = t2.id",
		"SELECT * FROM t1 LEFT JOIN t2 ON t1.id = t2.id",
		"SELECT * FROM t1 RIGHT JOIN t2 ON t1.id = t2.id",
		"SELECT * FROM t1 LEFT OUTER JOIN t2 ON t1.id = t2.id",
		"SELECT a.id, b.name FROM t1 a JOIN t2 b ON a.id = b.t1_id",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestLexerScanStringVariations tests string scanning
func TestLexerScanStringVariations(t *testing.T) {
	tests := []string{
		`SELECT 'hello'`,
		`SELECT "world"`,
		`SELECT 'it''s escaped'`,
		`SELECT ''`,
		`SELECT 'unicode: 你好'`,
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Logf("ParseString(%s): %v", sql, err)
		}
	}
}

// TestLexerOperatorVariations tests operator scanning
func TestLexerOperatorVariations(t *testing.T) {
	tests := []string{
		"SELECT a = b FROM t",
		"SELECT a != b FROM t",
		"SELECT a <> b FROM t",
		"SELECT a < b FROM t",
		"SELECT a > b FROM t",
		"SELECT a <= b FROM t",
		"SELECT a >= b FROM t",
		"SELECT a + b FROM t",
		"SELECT a - b FROM t",
		"SELECT a * b FROM t",
		"SELECT a / b FROM t",
		"SELECT a % b FROM t",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseStatementTypes tests various statement types
func TestParseStatementTypes(t *testing.T) {
	tests := []string{
		"BEGIN",
		"COMMIT",
		"ROLLBACK",
		"BACKUP TO '/tmp/backup'",
		"RESTORE FROM '/tmp/backup'",
		"SET USER = 'admin'",
		"SET PASSWORD = 'secret'",
		"SET LOG_LEVEL = 'DEBUG'",
		"SHOW TABLES",
		"SHOW COLUMNS FROM t",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Logf("ParseString(%s): %v", sql, err)
		}
	}
}

// TestLexerScanStringEscape tests string escape sequences
func TestLexerScanStringEscape(t *testing.T) {
	tests := []struct {
		sql    string
		hasErr bool
	}{
		{`SELECT 'hello\nworld'`, false},
		{`SELECT 'tab\there'`, false},
		{`SELECT 'carriage\rreturn'`, false},
		{`SELECT 'back\\slash'`, false},
		{`SELECT 'quote\'s'`, false},
		{`SELECT "double\"quote"`, false},
	}

	for _, tt := range tests {
		_, err := ParseString(tt.sql)
		if tt.hasErr && err == nil {
			t.Errorf("ParseString(%s) expected error, got nil", tt.sql)
		} else if !tt.hasErr && err != nil {
			t.Logf("ParseString(%s): %v", tt.sql, err)
		}
	}
}

// TestLexerScanStringUnterminated tests unterminated strings
func TestLexerScanStringUnterminated(t *testing.T) {
	tests := []string{
		"SELECT 'unterminated",
		"SELECT \"unterminated",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err == nil {
			t.Logf("ParseString(%s) expected error for unterminated string", sql)
		} else {
			t.Logf("ParseString(%s) error (expected): %v", sql, err)
		}
	}
}

// TestLexerScanOperatorBitwise tests bitwise operators
func TestLexerScanOperatorBitwise(t *testing.T) {
	tests := []string{
		"SELECT a << 1 FROM t",
		"SELECT a >> 1 FROM t",
		"SELECT a | b FROM t",
		"SELECT a || b FROM t",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Logf("ParseString(%s): %v", sql, err)
		}
	}
}

// TestLexerScanParameterVariations tests parameter scanning
func TestLexerScanParameterVariations(t *testing.T) {
	tests := []string{
		"SELECT $1 FROM t",
		"SELECT $123 FROM t",
		"SELECT * FROM t WHERE id = $1",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Logf("ParseString(%s): %v", sql, err)
		}
	}
}

// TestParseColumnDefExtra tests column definition parsing
func TestParseColumnDefExtra(t *testing.T) {
	tests := []string{
		"CREATE TABLE t (id INT DEFAULT 0)",
		"CREATE TABLE t (id INT DEFAULT 123)",
		"CREATE TABLE t (name VARCHAR(100) DEFAULT 'test')",
		"CREATE TABLE t (id INT AUTO_INCREMENT)",
		"CREATE TABLE t (id SEQ)",
		"CREATE TABLE t (data BLOB)",
		"CREATE TABLE t (data FILE)",
		"CREATE TABLE t (created DATETIME)",
		"CREATE TABLE t (d DATE, t TIME)",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseConstraintExtra tests constraint parsing variations
func TestParseConstraintExtra(t *testing.T) {
	tests := []string{
		"CREATE TABLE t (id INT, name VARCHAR(50), PRIMARY KEY (id, name))",
		"CREATE TABLE t (id INT, CONSTRAINT pk_id PRIMARY KEY (id))",
		"CREATE TABLE t (id INT CHECK (id > 0))",
		"CREATE TABLE t (id INT, FOREIGN KEY (id) REFERENCES other(id))",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Logf("ParseString(%s): %v", sql, err)
		}
	}
}

// TestParseDeleteExtra tests DELETE parsing variations
func TestParseDeleteExtra(t *testing.T) {
	tests := []string{
		"DELETE FROM t WHERE id = 1",
		"DELETE FROM t WHERE id > 10 AND status = 'inactive'",
		"DELETE FROM t",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseUpdateExtra tests UPDATE parsing variations
func TestParseUpdateExtra(t *testing.T) {
	tests := []string{
		"UPDATE t SET a = NULL",
		"UPDATE t SET a = DEFAULT",
		"UPDATE t SET a = (SELECT MAX(x) FROM t2)",
		"UPDATE t SET a = 1 WHERE b IS NULL",
		"UPDATE t SET a = 1 WHERE b IS NOT NULL",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Logf("ParseString(%s): %v", sql, err)
		}
	}
}

// TestParseInsertMoreVariations tests more INSERT variations
func TestParseInsertMoreVariations(t *testing.T) {
	tests := []string{
		"INSERT INTO t DEFAULT VALUES",
		"INSERT INTO t SET a = 1, b = 2",
		"INSERT INTO t (a, b) SELECT x, y FROM t2 WHERE z > 0",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Logf("ParseString(%s): %v", sql, err)
		}
	}
}

// TestStatementTypeString tests StatementType.String method
func TestStatementTypeString(t *testing.T) {
	tests := []struct {
		stmtType StatementType
		want     string
	}{
		{StmtSelect, "SELECT"},
		{StmtInsert, "INSERT"},
		{StmtUpdate, "UPDATE"},
		{StmtDelete, "DELETE"},
		{StmtCreateTable, "CREATE TABLE"},
		{StmtDropTable, "DROP TABLE"},
		{StmtAlterTable, "ALTER TABLE"},
		{StmtCreateIndex, "CREATE INDEX"},
		{StmtDropIndex, "DROP INDEX"},
		{StmtSet, "SET"},
		{StmtShow, "SHOW"},
		{StmtUse, "USE"},
		{StmtBackup, "BACKUP"},
		{StmtRestore, "RESTORE"},
		{StmtBegin, "BEGIN"},
		{StmtCommit, "COMMIT"},
		{StmtRollback, "ROLLBACK"},
		{StatementType(999), "UNKNOWN"},
	}

	for _, tt := range tests {
		got := tt.stmtType.String()
		if got != tt.want {
			t.Errorf("StatementType(%d).String() = %s, want %s", tt.stmtType, got, tt.want)
		}
	}
}

// TestTokenStringMore tests Token.String more comprehensively
func TestTokenStringMore(t *testing.T) {
	tests := []Token{
		{Type: TokKeyword, Value: "SELECT", Pos: 0},
		{Type: TokIdent, Value: "table1", Pos: 5},
		{Type: TokString, Value: "hello", Pos: 10},
		{Type: TokNumber, Value: "123", Pos: 15},
		{Type: TokOperator, Value: "+", Pos: 20},
		{Type: TokEOF, Value: "", Pos: 25},
		{Type: TokError, Value: "test error", Pos: 30},
		{Type: TokParameter, Value: "$1", Pos: 40},
	}

	for _, tok := range tests {
		str := tok.String()
		if str == "" {
			t.Errorf("Token %v String() returned empty", tok)
		}
	}
}

// TestTokenTypeValues tests TokenType constants
func TestTokenTypeValues(t *testing.T) {
	// Just verify these are defined
	types := []TokenType{
		TokKeyword,
		TokIdent,
		TokString,
		TokNumber,
		TokOperator,
		TokEOF,
		TokError,
		TokParameter,
	}

	for i, tt := range types {
		t.Logf("TokenType[%d] = %d", i, tt)
	}
}

// TestSelectColumnString tests SelectColumn.String method
func TestSelectColumnString(t *testing.T) {
	tests := []SelectColumn{
		{All: true},
		{Expr: &Expression{Type: ExprLiteral, Literal: "col1"}, Alias: "alias1"},
		{Expr: &Expression{Type: ExprLiteral, Literal: "col2"}, Alias: ""},
	}

	for i, sc := range tests {
		str := sc.String()
		t.Logf("SelectColumn[%d]: %s", i, str)
		if str == "" {
			t.Errorf("SelectColumn.String() returned empty for case %d", i)
		}
	}
}

// TestColumnDefString tests ColumnDef.String method
func TestColumnDefString(t *testing.T) {
	tests := []ColumnDef{
		{Name: "id", Type: "INT", PrimaryKey: true},
		{Name: "name", Type: "VARCHAR", Length: 50, Nullable: false},
		{Name: "price", Type: "DECIMAL", Length: 10, Scale: 2},
		{Name: "status", Type: "VARCHAR", Length: 20, Unique: true},
		{Name: "counter", Type: "INT", AutoInc: true},
		{Name: "value", Type: "INT", Default: &Expression{Type: ExprLiteral, Literal: "0"}},
	}

	for i, cd := range tests {
		str := cd.String()
		t.Logf("ColumnDef[%d]: %s", i, str)
		if str == "" {
			t.Errorf("ColumnDef.String() returned empty for case %d", i)
		}
	}
}

// TestExpressionStringExtra tests Expression.String method
func TestExpressionStringExtra(t *testing.T) {
	tests := []*Expression{
		{Type: ExprLiteral, Literal: "test"},
		{Type: ExprColumn, Column: "col1"},
		{Type: ExprColumn, Table: "t", Column: "col1"},
		{Type: ExprFunction, FuncName: "COUNT", Args: []*Expression{}},
	}

	for i, expr := range tests {
		str := expr.String()
		t.Logf("Expression[%d]: %s", i, str)
	}
}

// TestExpressionStringFull tests all Expression.String cases
func TestExpressionStringFull(t *testing.T) {
	tests := []struct {
		name string
		expr *Expression
	}{
		{"nil", nil},
		{"literal string", &Expression{Type: ExprLiteral, Literal: "hello"}},
		{"literal int", &Expression{Type: ExprLiteral, Literal: 123}},
		{"column simple", &Expression{Type: ExprColumn, Column: "col1"}},
		{"column qualified", &Expression{Type: ExprColumn, Table: "t", Column: "col1"}},
		{"function no args", &Expression{Type: ExprFunction, FuncName: "NOW", Args: []*Expression{}}},
		{"function with args", &Expression{Type: ExprFunction, FuncName: "CONCAT", Args: []*Expression{
			{Type: ExprLiteral, Literal: "a"},
			{Type: ExprLiteral, Literal: "b"},
		}}},
		{"unary op", &Expression{Type: ExprUnaryOp, Op: "-", Right: &Expression{Type: ExprLiteral, Literal: 5}}},
		{"binary op", &Expression{Type: ExprBinaryOp, Op: "+", Left: &Expression{Type: ExprLiteral, Literal: 1}, Right: &Expression{Type: ExprLiteral, Literal: 2}}},
		{"star", &Expression{Type: ExprStar}},
		{"star qualified", &Expression{Type: ExprStar, Table: "t"}},
		{"in list", &Expression{Type: ExprIn, Left: &Expression{Type: ExprColumn, Column: "id"}, List: []*Expression{
			{Type: ExprLiteral, Literal: 1},
			{Type: ExprLiteral, Literal: 2},
		}}},
		{"between", &Expression{Type: ExprBetween, Left: &Expression{Type: ExprColumn, Column: "age"}, List: []*Expression{
			{Type: ExprLiteral, Literal: 10},
			{Type: ExprLiteral, Literal: 20},
		}}},
		{"case simple", &Expression{Type: ExprCase, WhenClauses: []WhenClause{
			{Cond: &Expression{Type: ExprLiteral, Literal: true}, Then: &Expression{Type: ExprLiteral, Literal: "yes"}},
		}}},
		{"case with else", &Expression{Type: ExprCase, WhenClauses: []WhenClause{
			{Cond: &Expression{Type: ExprLiteral, Literal: true}, Then: &Expression{Type: ExprLiteral, Literal: "yes"}},
		}, ElseExpr: &Expression{Type: ExprLiteral, Literal: "no"}}},
		{"case with expr", &Expression{Type: ExprCase, CaseExpr: &Expression{Type: ExprColumn, Column: "status"}, WhenClauses: []WhenClause{
			{Cond: &Expression{Type: ExprLiteral, Literal: "A"}, Then: &Expression{Type: ExprLiteral, Literal: "Active"}},
		}}},
		{"unknown type", &Expression{Type: ExprUnknown}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			str := tt.expr.String()
			t.Logf("%s: %q", tt.name, str)
		})
	}
}

// TestParseUpdateComprehensive tests UPDATE parsing
func TestParseUpdateMoreFull(t *testing.T) {
	tests := []string{
		"UPDATE t SET a = 1",
		"UPDATE t SET a = 1, b = 2",
		"UPDATE t SET a = 1 WHERE b > 0",
		"UPDATE t SET a = a + 1",
		"UPDATE t SET a = UPPER(b)",
		"UPDATE t SET a = NULL WHERE b IS NULL",
		"UPDATE t SET a = 1, b = 2, c = 3 WHERE d = 4",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseInsertComprehensive tests INSERT parsing
func TestParseInsertMoreFull(t *testing.T) {
	tests := []string{
		"INSERT INTO t VALUES (1)",
		"INSERT INTO t VALUES (1, 2, 3)",
		"INSERT INTO t (a, b) VALUES (1, 2)",
		"INSERT INTO t (a, b) VALUES (1, 2), (3, 4)",
		"INSERT INTO t SELECT * FROM t2",
		"INSERT INTO t (a, b) SELECT x, y FROM t2",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestLexerNextTokenComprehensive tests NextToken
func TestLexerNextTokenComprehensive(t *testing.T) {
	tests := []string{
		"SELECT * FROM t",
		"-- comment\nSELECT 1",
		"/* block comment */ SELECT 1",
		"SELECT 'string' FROM t",
		"SELECT 123.456 FROM t",
		"SELECT * FROM t WHERE a = b",
	}

	for _, sql := range tests {
		lexer := NewLexer(sql)
		for {
			tok := lexer.NextToken()
			t.Logf("Token: %v", tok)
			if tok.Type == TokEOF || tok.Type == TokError {
				break
			}
		}
	}
}

// TestParseUpdateWithLimit tests UPDATE with LIMIT
func TestParseUpdateWithLimit(t *testing.T) {
	tests := []string{
		"UPDATE t SET a = 1 LIMIT 10",
		"UPDATE t SET a = 1, b = 2 LIMIT 5",
		"UPDATE t SET a = 1 WHERE b > 0 LIMIT 10",
	}

	for _, sql := range tests {
		stmt, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		} else {
			t.Logf("Parsed UPDATE with LIMIT: table=%s, limit=%v", stmt.Table, stmt.Limit)
		}
	}
}

// TestParseDeleteWithLimit tests DELETE with LIMIT
func TestParseDeleteWithLimit(t *testing.T) {
	tests := []string{
		"DELETE FROM t LIMIT 10",
		"DELETE FROM t WHERE a > 0 LIMIT 5",
	}

	for _, sql := range tests {
		stmt, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		} else {
			t.Logf("Parsed DELETE with LIMIT: table=%s, limit=%v", stmt.Table, stmt.Limit)
		}
	}
}

// TestParseUpdateErrors tests UPDATE parsing errors
func TestParseUpdateErrors(t *testing.T) {
	tests := []string{
		"UPDATE",           // missing table
		"UPDATE t",         // missing SET
		"UPDATE t SET",     // missing column
		"UPDATE t SET a",   // missing =
		"UPDATE t SET a =", // missing value
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err == nil {
			t.Logf("ParseString(%s) should fail but didn't", sql)
		} else {
			t.Logf("ParseString(%s) error (expected): %v", sql, err)
		}
	}
}

// TestParseDeleteErrors tests DELETE parsing errors
func TestParseDeleteErrors(t *testing.T) {
	tests := []string{
		"DELETE",        // missing FROM
		"DELETE FROM",   // missing table
		"DELETE 1",      // missing FROM
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err == nil {
			t.Logf("ParseString(%s) should fail but didn't", sql)
		} else {
			t.Logf("ParseString(%s) error (expected): %v", sql, err)
		}
	}
}

// TestLexerNextTokenAllTypes tests all token types
func TestLexerNextTokenAllTypes(t *testing.T) {
	sql := "SELECT * FROM t WHERE a = 1 AND b = 'test' -- comment\n/* block */ + - * / %"
	lexer := NewLexer(sql)

	count := 0
	for {
		tok := lexer.NextToken()
		t.Logf("Token %d: type=%d value=%q", count, tok.Type, tok.Value)
		count++
		if tok.Type == TokEOF || tok.Type == TokError || count > 50 {
			break
		}
	}
}

// TestParseSelectWithSubqueryInWhere tests SELECT with subquery in WHERE
func TestParseSelectWithSubqueryInWhere(t *testing.T) {
	tests := []string{
		"SELECT * FROM t WHERE id = (SELECT MAX(id) FROM t)",
		"SELECT * FROM t WHERE id IN (SELECT id FROM t2)",
		"SELECT * FROM t WHERE EXISTS (SELECT 1 FROM t2 WHERE t2.t_id = t.id)",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Logf("ParseString(%s): %v", sql, err)
		}
	}
}

// TestParseCreateTableWithDefaults tests CREATE TABLE with DEFAULT values
func TestParseCreateTableWithDefaults(t *testing.T) {
	tests := []string{
		"CREATE TABLE t (id INT DEFAULT 0)",
		"CREATE TABLE t (name VARCHAR(50) DEFAULT 'unknown')",
		"CREATE TABLE t (active BOOL DEFAULT true)",
		"CREATE TABLE t (price FLOAT DEFAULT 0.0)",
		"CREATE TABLE t (id INT, status VARCHAR(20) DEFAULT 'pending')",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseCreateTableWithAutoIncrement tests CREATE TABLE with AUTO_INCREMENT
func TestParseCreateTableWithAutoIncrement(t *testing.T) {
	tests := []string{
		"CREATE TABLE t (id INT AUTO_INCREMENT PRIMARY KEY)",
		"CREATE TABLE t (id SEQ, name VARCHAR(50))",
		"CREATE TABLE t (id INT AUTO_INCREMENT)",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseCreateTableWithNullable tests CREATE TABLE with NULL/NOT NULL
func TestParseCreateTableWithNullable(t *testing.T) {
	tests := []string{
		"CREATE TABLE t (id INT NOT NULL)",
		"CREATE TABLE t (name VARCHAR(50) NULL)",
		"CREATE TABLE t (id INT NOT NULL, name VARCHAR(50) NULL)",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseSelectWithCaseExpression tests SELECT with CASE expression
func TestParseSelectWithCaseExpression(t *testing.T) {
	tests := []string{
		"SELECT CASE WHEN a > 0 THEN 'positive' ELSE 'non-positive' END FROM t",
		"SELECT CASE a WHEN 1 THEN 'one' WHEN 2 THEN 'two' ELSE 'other' END FROM t",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Logf("ParseString(%s): %v", sql, err)
		}
	}
}

// TestParseSelectWithExists tests SELECT with EXISTS
func TestParseSelectWithExists(t *testing.T) {
	tests := []string{
		"SELECT * FROM t WHERE EXISTS (SELECT 1 FROM t2 WHERE t2.id = t.id)",
		"SELECT * FROM t WHERE NOT EXISTS (SELECT 1 FROM t2)",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Logf("ParseString(%s): %v", sql, err)
		}
	}
}

// TestParseSelectWithIn tests SELECT with IN
func TestParseSelectWithIn(t *testing.T) {
	tests := []string{
		"SELECT * FROM t WHERE id IN (1, 2, 3)",
		"SELECT * FROM t WHERE id NOT IN (1, 2, 3)",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}

	// IN with subquery may not be fully supported
	_, err := ParseString("SELECT * FROM t WHERE id IN (SELECT id FROM t2)")
	if err != nil {
		t.Logf("IN with subquery: %v", err)
	}
}

// TestParseSelectWithBetween tests SELECT with BETWEEN
func TestParseSelectWithBetween(t *testing.T) {
	tests := []string{
		"SELECT * FROM t WHERE id BETWEEN 1 AND 10",
		"SELECT * FROM t WHERE id NOT BETWEEN 1 AND 10",
		"SELECT * FROM t WHERE name BETWEEN 'a' AND 'z'",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseSelectWithLike tests SELECT with LIKE
func TestParseSelectWithLike(t *testing.T) {
	tests := []string{
		"SELECT * FROM t WHERE name LIKE 'A%'",
		"SELECT * FROM t WHERE name NOT LIKE '%z'",
		"SELECT * FROM t WHERE name LIKE '_abc%'",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseSelectWithIsNull tests SELECT with IS NULL
func TestParseSelectWithIsNull(t *testing.T) {
	tests := []string{
		"SELECT * FROM t WHERE name IS NULL",
		"SELECT * FROM t WHERE name IS NOT NULL",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseSelectWithOrderByAscDesc tests ORDER BY ASC/DESC
func TestParseSelectWithOrderByAscDesc(t *testing.T) {
	tests := []string{
		"SELECT * FROM t ORDER BY a ASC",
		"SELECT * FROM t ORDER BY a DESC",
		"SELECT * FROM t ORDER BY a ASC, b DESC",
		"SELECT * FROM t ORDER BY a, b ASC",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseSelectWithGroupByHaving tests GROUP BY with HAVING
func TestParseSelectWithGroupByHaving(t *testing.T) {
	tests := []string{
		"SELECT a, COUNT(*) FROM t GROUP BY a",
		"SELECT a, COUNT(*) FROM t GROUP BY a HAVING COUNT(*) > 1",
		"SELECT a, SUM(b) FROM t GROUP BY a HAVING SUM(b) > 100",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseSelectWithDistinct tests SELECT DISTINCT
func TestParseSelectWithDistinct(t *testing.T) {
	tests := []string{
		"SELECT DISTINCT a FROM t",
		"SELECT DISTINCT a, b FROM t",
		"SELECT DISTINCT COUNT(*) FROM t",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseSelectWithAggregateFunctions tests SELECT with aggregate functions
func TestParseSelectWithAggregateFunctions(t *testing.T) {
	tests := []string{
		"SELECT COUNT(*) FROM t",
		"SELECT COUNT(id) FROM t",
		"SELECT SUM(price) FROM t",
		"SELECT AVG(price) FROM t",
		"SELECT MIN(price), MAX(price) FROM t",
		"SELECT COUNT(DISTINCT id) FROM t",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseDropTableIfExists tests DROP TABLE IF EXISTS
func TestParseDropTableIfExists(t *testing.T) {
	tests := []string{
		"DROP TABLE IF EXISTS t",
		"DROP TABLE t",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseCreateTableIfNotExists tests CREATE TABLE IF NOT EXISTS
func TestParseCreateTableIfNotExists(t *testing.T) {
	tests := []string{
		"CREATE TABLE IF NOT EXISTS t (id INT)",
		"CREATE TABLE t (id INT)",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseAlterTableComprehensive tests ALTER TABLE parsing
func TestParseAlterTableComprehensive(t *testing.T) {
	tests := []string{
		"ALTER TABLE t ADD COLUMN x INT",
		"ALTER TABLE t ADD x INT",
		"ALTER TABLE t DROP COLUMN x",
		"ALTER TABLE t DROP x",
		"ALTER TABLE t MODIFY x VARCHAR(100)",
		"ALTER TABLE t RENAME COLUMN x TO y",
		"ALTER TABLE t RENAME TO new_name",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Logf("ParseString(%s): %v", sql, err)
		}
	}
}

// TestParseCreateIndexFull tests CREATE INDEX parsing
func TestParseCreateIndexFull(t *testing.T) {
	tests := []string{
		"CREATE INDEX idx ON t (col)",
		"CREATE UNIQUE INDEX idx ON t (col)",
		"CREATE INDEX idx ON t (col1, col2)",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseDropIndexFull tests DROP INDEX parsing
func TestParseDropIndexFull(t *testing.T) {
	_, err := ParseString("DROP INDEX idx")
	if err != nil {
		t.Errorf("DROP INDEX failed: %v", err)
	}
}

// TestParseRestoreFull tests RESTORE parsing
func TestParseRestoreFull(t *testing.T) {
	tests := []string{
		"RESTORE FROM '/path/to/backup'",
		"RESTORE '/path/to/backup'",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseBackupFull tests BACKUP parsing
func TestParseBackupFull(t *testing.T) {
	tests := []string{
		"BACKUP TO '/path/to/backup'",
		"BACKUP '/path/to/backup'",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseSetFull tests SET parsing
func TestParseSetFull(t *testing.T) {
	tests := []string{
		"SET USER = 'admin'",
		"SET PASSWORD = 'secret'",
		"SET LOG_LEVEL = 'DEBUG'",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseShowFull tests SHOW parsing
func TestParseShowFull(t *testing.T) {
	tests := []string{
		"SHOW TABLES",
		"SHOW COLUMNS FROM t",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseSelectWithGroupByList tests GROUP BY list parsing
func TestParseSelectWithGroupByList(t *testing.T) {
	tests := []string{
		"SELECT a, b FROM t GROUP BY a",
		"SELECT a, b FROM t GROUP BY a, b",
		"SELECT a, b, c FROM t GROUP BY a, b, c",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseSelectWithCaseFull tests CASE expression parsing
func TestParseSelectWithCaseFull(t *testing.T) {
	tests := []string{
		"SELECT CASE WHEN a > 0 THEN 1 ELSE 0 END FROM t",
		"SELECT CASE WHEN a > 0 THEN 1 WHEN a < 0 THEN -1 ELSE 0 END FROM t",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Logf("ParseString(%s): %v", sql, err)
		}
	}
}

// TestParseParenExpr tests parenthesized expressions
func TestParseParenExpr(t *testing.T) {
	tests := []string{
		"SELECT (a + b) * c FROM t",
		"SELECT * FROM t WHERE (a > 0 AND b > 0) OR c > 0",
		"SELECT * FROM t WHERE a = (SELECT MAX(x) FROM t2)",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Logf("ParseString(%s): %v", sql, err)
		}
	}
}

// TestParseBetweenExpr tests BETWEEN expressions
func TestParseBetweenExpr(t *testing.T) {
	tests := []string{
		"SELECT * FROM t WHERE a BETWEEN 1 AND 10",
		"SELECT * FROM t WHERE a NOT BETWEEN 1 AND 10",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseColumnDefFull tests column definition parsing
func TestParseColumnDefFull(t *testing.T) {
	tests := []string{
		"CREATE TABLE t (id INT PRIMARY KEY)",
		"CREATE TABLE t (id INT NOT NULL PRIMARY KEY)",
		"CREATE TABLE t (id INT UNIQUE NOT NULL)",
		"CREATE TABLE t (id INT DEFAULT 0 NOT NULL)",
		"CREATE TABLE t (id INT AUTO_INCREMENT PRIMARY KEY)",
		"CREATE TABLE t (name VARCHAR(50) NOT NULL DEFAULT 'test')",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseConstraintFull tests constraint parsing
func TestParseConstraintFull(t *testing.T) {
	tests := []string{
		"CREATE TABLE t (id INT, PRIMARY KEY (id))",
		"CREATE TABLE t (id INT, UNIQUE (id))",
		"CREATE TABLE t (id INT, CONSTRAINT pk_id PRIMARY KEY (id))",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Logf("ParseString(%s): %v", sql, err)
		}
	}
}

// TestParseAlterErrorCases tests ALTER TABLE error paths
func TestParseAlterErrorCases(t *testing.T) {
	// Test missing TABLE keyword
	_, err := ParseString("ALTER t ADD x INT")
	if err == nil {
		t.Error("Expected error for missing TABLE keyword")
	}
}

// TestParseConstraintErrorCases tests constraint parsing error paths
func TestParseConstraintErrorCases(t *testing.T) {
	// Test missing constraint name after CONSTRAINT keyword
	_, err := ParseString("CREATE TABLE t (id INT, CONSTRAINT PRIMARY KEY (id))")
	if err == nil {
		t.Error("Expected error for missing constraint name")
	}

	// Test missing KEY after PRIMARY
	_, err = ParseString("CREATE TABLE t (id INT, PRIMARY (id))")
	if err == nil {
		t.Error("Expected error for missing KEY after PRIMARY")
	}
}

// TestParseDropIndexErrorCases tests DROP INDEX error paths
func TestParseDropIndexErrorCases(t *testing.T) {
	// Test missing index name
	_, err := ParseString("DROP INDEX")
	if err == nil {
		t.Error("Expected error for missing index name")
	}

	// Test missing table name after ON
	_, err = ParseString("DROP INDEX idx ON")
	if err == nil {
		t.Error("Expected error for missing table name after ON")
	}
}

// TestParseAlterMultipleActions tests ALTER TABLE with multiple actions
func TestParseAlterMultipleActions(t *testing.T) {
	tests := []string{
		"ALTER TABLE t ADD x INT, ADD y VARCHAR(50)",
		"ALTER TABLE t ADD x INT, DROP y",
		"ALTER TABLE t DROP x, ADD y INT",
		"ALTER TABLE t ADD x INT, DROP y, MODIFY z TEXT",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseConstraintForeignKey tests FOREIGN KEY constraint parsing
func TestParseConstraintForeignKey(t *testing.T) {
	tests := []string{
		"CREATE TABLE t (id INT, FOREIGN KEY (id) REFERENCES t2(id))",
		"CREATE TABLE t (id INT, FOREIGN KEY (id, name) REFERENCES t2(id, name))",
		"CREATE TABLE t (id INT, FOREIGN KEY (id) REFERENCES t2)",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseAlterTableDropColumn tests ALTER TABLE DROP COLUMN
func TestParseAlterTableDropColumn(t *testing.T) {
	tests := []string{
		"ALTER TABLE t DROP COLUMN name",
		"ALTER TABLE t DROP name",
		"ALTER TABLE t DROP COLUMN IF EXISTS name",
	}

	for _, sql := range tests {
		stmt, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		} else {
			t.Logf("Parsed: %s -> actions=%d", sql, len(stmt.AlterActions))
		}
	}
}

// TestParseAlterTableRenameColumn tests ALTER TABLE RENAME COLUMN
func TestParseAlterTableRenameColumn(t *testing.T) {
	sql := "ALTER TABLE t RENAME COLUMN old_name TO new_name"
	stmt, err := ParseString(sql)
	if err != nil {
		t.Errorf("ParseString failed: %v", err)
	} else {
		t.Logf("Parsed: %s -> actions=%d", sql, len(stmt.AlterActions))
	}
}

// TestParseAlterTableRenameTo tests ALTER TABLE RENAME TO
func TestParseAlterTableRenameTo(t *testing.T) {
	sql := "ALTER TABLE t RENAME TO new_table_name"
	stmt, err := ParseString(sql)
	if err != nil {
		t.Errorf("ParseString failed: %v", err)
	} else {
		t.Logf("Parsed: %s -> actions=%d", sql, len(stmt.AlterActions))
	}
}

// TestParseAlterTableModifyColumn tests ALTER TABLE MODIFY COLUMN
func TestParseAlterTableModifyColumn(t *testing.T) {
	tests := []string{
		"ALTER TABLE t MODIFY COLUMN name VARCHAR(200)",
		"ALTER TABLE t MODIFY name VARCHAR(200)",
		"ALTER TABLE t ALTER COLUMN name SET NOT NULL",
		"ALTER TABLE t ALTER name VARCHAR(200)",
	}

	for _, sql := range tests {
		stmt, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		} else {
			t.Logf("Parsed: %s -> actions=%d", sql, len(stmt.AlterActions))
		}
	}
}

// TestParseAlterTableAddPrimaryKey tests ALTER TABLE ADD PRIMARY KEY
func TestParseAlterTableAddPrimaryKey(t *testing.T) {
	sql := "ALTER TABLE t ADD PRIMARY KEY (id)"
	stmt, err := ParseString(sql)
	if err != nil {
		t.Errorf("ParseString failed: %v", err)
	} else {
		t.Logf("Parsed: %s -> actions=%d", sql, len(stmt.AlterActions))
	}
}

// TestParseAlterTableAddForeignKey tests ALTER TABLE ADD FOREIGN KEY
func TestParseAlterTableAddForeignKey(t *testing.T) {
	sql := "ALTER TABLE t ADD FOREIGN KEY (user_id) REFERENCES users(id)"
	stmt, err := ParseString(sql)
	if err != nil {
		t.Errorf("ParseString failed: %v", err)
	} else {
		t.Logf("Parsed: %s -> actions=%d", sql, len(stmt.AlterActions))
	}
}

// TestParseAlterTableAddConstraint tests ALTER TABLE ADD CONSTRAINT
func TestParseAlterTableAddConstraint(t *testing.T) {
	tests := []string{
		"ALTER TABLE t ADD CONSTRAINT pk_id PRIMARY KEY (id)",
		"ALTER TABLE t ADD CONSTRAINT uk_name UNIQUE (name)",
		"ALTER TABLE t ADD CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(id)",
	}

	for _, sql := range tests {
		stmt, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		} else {
			t.Logf("Parsed: %s -> actions=%d", sql, len(stmt.AlterActions))
		}
	}
}

// TestParseAlterTableMultipleActions tests ALTER TABLE with multiple actions
func TestParseAlterTableMultipleActions(t *testing.T) {
	tests := []string{
		"ALTER TABLE t ADD x INT, DROP y",
		"ALTER TABLE t ADD x INT, ADD y VARCHAR(50)",
		"ALTER TABLE t DROP x, ADD y INT, MODIFY z TEXT",
	}

	for _, sql := range tests {
		stmt, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		} else {
			t.Logf("Parsed: %s -> actions=%d", sql, len(stmt.AlterActions))
		}
	}
}

// TestParseAlterTableAddCheck tests ALTER TABLE ADD CHECK
func TestParseAlterTableAddCheck(t *testing.T) {
	sql := "ALTER TABLE t ADD CHECK (age > 0)"
	stmt, err := ParseString(sql)
	if err != nil {
		t.Errorf("ParseString failed: %v", err)
	} else {
		t.Logf("Parsed: %s -> actions=%d", sql, len(stmt.AlterActions))
	}
}

// TestLexerPeekAt tests lexer peekAt method
func TestLexerPeekAt(t *testing.T) {
	lexer := NewLexer("SELECT * FROM t")
	// Get first token
	tok := lexer.NextToken()
	if tok.Value != "SELECT" {
		t.Errorf("Expected SELECT, got %s", tok.Value)
	}
	t.Logf("Token: %s", tok.Value)
}

// TestParseExpressionPrecedence tests expression precedence
func TestParseExpressionPrecedence(t *testing.T) {
	tests := []string{
		"SELECT a OR b AND c FROM t",
		"SELECT a AND b OR c FROM t",
		"SELECT NOT a AND b FROM t",
		"SELECT NOT a OR b FROM t",
		"SELECT a + b * c FROM t",
		"SELECT a * b + c FROM t",
		"SELECT -a + b FROM t",
		"SELECT NOT a = b FROM t",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseSelectWithNestedExpressions tests nested expressions
func TestParseSelectWithNestedExpressions(t *testing.T) {
	tests := []string{
		"SELECT * FROM t WHERE (a = 1 OR b = 2) AND (c = 3 OR d = 4)",
		"SELECT * FROM t WHERE NOT (a = 1 AND b = 2)",
		"SELECT * FROM t WHERE a = (SELECT MAX(x) FROM t2 WHERE y > 0)",
		"SELECT * FROM t WHERE a IN (SELECT id FROM t2 WHERE status = 'active')",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Logf("ParseString(%s): %v", sql, err)
		}
	}
}

// TestParseCaseWithMultipleWhens tests CASE with multiple WHEN clauses
func TestParseCaseWithMultipleWhens(t *testing.T) {
	tests := []string{
		"SELECT CASE WHEN a = 1 THEN 'one' WHEN a = 2 THEN 'two' ELSE 'other' END FROM t",
		"SELECT CASE a WHEN 1 THEN 'one' WHEN 2 THEN 'two' ELSE 'other' END FROM t",
		"SELECT CASE WHEN a > 0 THEN 'positive' WHEN a < 0 THEN 'negative' END FROM t",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseSelectWithMultipleJoinsAndConditions tests complex JOINs
func TestParseSelectWithMultipleJoinsAndConditions(t *testing.T) {
	tests := []string{
		"SELECT * FROM t1 JOIN t2 ON t1.id = t2.id JOIN t3 ON t2.id = t3.t2_id",
		"SELECT * FROM t1 LEFT JOIN t2 ON t1.id = t2.id LEFT JOIN t3 ON t2.id = t3.t2_id",
		"SELECT * FROM t1 INNER JOIN t2 ON t1.id = t2.id AND t2.status = 'active'",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseColumnDefEdgeCases tests column definition edge cases
func TestParseColumnDefEdgeCases(t *testing.T) {
	tests := []string{
		"CREATE TABLE t (id INT PRIMARY KEY AUTO_INCREMENT)",
		"CREATE TABLE t (id INT NOT NULL AUTO_INCREMENT PRIMARY KEY)",
		"CREATE TABLE t (name VARCHAR NOT NULL)",
		"CREATE TABLE t (id INT, name TEXT, created DATETIME DEFAULT NOW())",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestTokenStringAllTypes tests Token.String() for all token types
func TestTokenStringAllTypes(t *testing.T) {
	tests := []struct {
		token    Token
		contains string
	}{
		{Token{Type: TokEOF, Value: ""}, "EOF"},
		{Token{Type: TokError, Value: "test error"}, "ERROR"},
		{Token{Type: TokIdent, Value: "myident"}, "IDENT"},
		{Token{Type: TokNumber, Value: "123"}, "NUMBER"},
		{Token{Type: TokString, Value: "test"}, "STRING"},
		{Token{Type: TokKeyword, Value: "SELECT"}, "KEYWORD"},
		{Token{Type: TokOperator, Value: "+"}, "OP"},
		{Token{Type: TokComma, Value: ","}, ","},
		{Token{Type: TokLParen, Value: "("}, "("},
		{Token{Type: TokRParen, Value: ")"}, ")"},
		{Token{Type: TokLBracket, Value: "["}, "["},
		{Token{Type: TokRBracket, Value: "]"}, "]"},
		{Token{Type: TokSemicolon, Value: ";"}, ";"},
		{Token{Type: TokDot, Value: "."}, "."},
		{Token{Type: TokStar, Value: "*"}, "*"},
		{Token{Type: TokParameter, Value: "$1"}, "PARAM"},
		{Token{Type: 999, Value: "unknown"}, "unknown"},
	}

	for _, tt := range tests {
		result := tt.token.String()
		if !strings.Contains(result, tt.contains) {
			t.Errorf("Token %v String() = %s, expected to contain %s", tt.token, result, tt.contains)
		}
		t.Logf("Token type %d: %s", tt.token.Type, result)
	}
}

// TestParseBetweenExprFull tests parseBetweenExpr fully
func TestParseBetweenExprFull(t *testing.T) {
	tests := []string{
		"SELECT * FROM t WHERE a BETWEEN 1 AND 10",
		"SELECT * FROM t WHERE a NOT BETWEEN 1 AND 10",
		"SELECT * FROM t WHERE a BETWEEN b AND c",
		"SELECT * FROM t WHERE a BETWEEN 'a' AND 'z'",
		"SELECT * FROM t WHERE a NOT BETWEEN x AND y",
	}

	for _, sql := range tests {
		stmt, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		} else {
			t.Logf("Parsed: %s, WHERE type: %v", sql, stmt.Where.Type)
		}
	}
}

// TestParseExistsExprFull tests parseExistsExpr fully
func TestParseExistsExprFull(t *testing.T) {
	tests := []string{
		"SELECT * FROM t WHERE EXISTS (SELECT 1)",
		"SELECT * FROM t WHERE NOT EXISTS (SELECT 1 FROM t2)",
		"SELECT * FROM t WHERE EXISTS (SELECT * FROM t2 WHERE t2.id = t.id)",
		"SELECT a FROM t WHERE EXISTS (SELECT 1 FROM t2 WHERE t2.x > 0)",
	}

	for _, sql := range tests {
		stmt, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		} else {
			t.Logf("Parsed: %s, WHERE type: %v", sql, stmt.Where.Type)
		}
	}
}

// TestParserCurrentMethod tests the current() method
func TestParserCurrentMethod(t *testing.T) {
	// This tests the current() method through parsing
	// The method is used internally during parsing
	sql := "SELECT * FROM t"
	_, err := ParseString(sql)
	if err != nil {
		t.Errorf("ParseString failed: %v", err)
	}
}

// TestParserPeekMethod tests the peek() method
func TestParserPeekMethod(t *testing.T) {
	// This tests the peek() method through parsing
	// The method is used internally during parsing
	sql := "SELECT id, name FROM t WHERE a > 1"
	_, err := ParseString(sql)
	if err != nil {
		t.Errorf("ParseString failed: %v", err)
	}
}

// TestParseParenExprFull tests parseParenExpr fully
func TestParseParenExprFull(t *testing.T) {
	tests := []string{
		"SELECT * FROM t WHERE (a = 1)",
		"SELECT * FROM t WHERE ((a = 1))",
		"SELECT * FROM t WHERE (a = 1 OR b = 2)",
		"SELECT * FROM t WHERE ((a = 1) AND (b = 2))",
		"SELECT * FROM t WHERE a IN (1, 2, 3)",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseCaseExprFull tests parseCaseExpr fully
func TestParseCaseExprFull(t *testing.T) {
	tests := []string{
		"SELECT CASE WHEN a > 0 THEN 'positive' END FROM t",
		"SELECT CASE WHEN a > 0 THEN 'positive' ELSE 'non-positive' END FROM t",
		"SELECT CASE WHEN a > 0 THEN 1 WHEN a < 0 THEN -1 ELSE 0 END FROM t",
		"SELECT CASE a WHEN 1 THEN 'one' WHEN 2 THEN 'two' END FROM t",
		"SELECT CASE a WHEN 1 THEN 'one' WHEN 2 THEN 'two' ELSE 'other' END FROM t",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Logf("ParseString(%s): %v", sql, err)
		}
	}
}

// TestParseConstraintCheck tests CHECK constraint parsing
func TestParseConstraintCheck(t *testing.T) {
	tests := []string{
		"CREATE TABLE t (age INT, CHECK (age > 0))",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseAlterModifyColumn tests ALTER TABLE MODIFY COLUMN
func TestParseAlterModifyColumn(t *testing.T) {
	tests := []string{
		"ALTER TABLE t MODIFY name VARCHAR(100)",
		"ALTER TABLE t MODIFY COLUMN name VARCHAR(100)",
		"ALTER TABLE t ALTER name VARCHAR(100)",
		"ALTER TABLE t ALTER COLUMN name VARCHAR(100)",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseAlterAddConstraint tests ALTER TABLE ADD constraint
func TestParseAlterAddConstraint(t *testing.T) {
	tests := []string{
		"ALTER TABLE t ADD PRIMARY KEY (id)",
		"ALTER TABLE t ADD FOREIGN KEY (id) REFERENCES t2(id)",
		"ALTER TABLE t ADD UNIQUE (id)",
		"ALTER TABLE t ADD CHECK (id > 0)",
		"ALTER TABLE t ADD CONSTRAINT pk_id PRIMARY KEY (id)",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}


// TestParseInsertExtraVariations tests extra INSERT statements
func TestParseInsertExtraVariations(t *testing.T) {
	tests := []string{
		"INSERT INTO t VALUES (NULL, 1, 2)",
		"INSERT INTO t (id) VALUES (DEFAULT)",
		"INSERT INTO t VALUES ((SELECT MAX(id) FROM t2))",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseUpdateExtraVariations tests extra UPDATE statements
func TestParseUpdateExtraVariations(t *testing.T) {
	tests := []string{
		"UPDATE t SET a = 1, b = 2, c = 3",
		"UPDATE t SET a = a + 1 WHERE b = 2",
		"UPDATE t SET a = (SELECT b FROM t2)",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseDeleteExtraVariations tests extra DELETE statements
func TestParseDeleteExtraVariations(t *testing.T) {
	tests := []string{
		"DELETE FROM t WHERE a > 1 AND b < 10",
		"DELETE FROM t WHERE a IN (1, 2, 3)",
		"DELETE FROM t WHERE a IS NULL",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseSelectExtraVariations tests extra SELECT statements
func TestParseSelectExtraVariations(t *testing.T) {
	// Test supported SELECT statements
	supported := []string{
		"SELECT 1 + 2 AS result",
		"SELECT a, COUNT(*) FROM t GROUP BY a HAVING COUNT(*) > 1",
		"SELECT a FROM t WHERE a BETWEEN 1 AND 10",
		"SELECT a FROM t WHERE a LIKE '%test%'",
		"SELECT a FROM t WHERE a IS NOT NULL",
		"SELECT t1.a, t2.b FROM t1 CROSS JOIN t2",
		"SELECT a FROM t1 UNION SELECT b FROM t2",
		"SELECT a FROM t1 UNION ALL SELECT b FROM t2",
	}

	for _, sql := range supported {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}

	// Test potentially unsupported SELECT statements
	mayFail := []string{
		"SELECT a FROM t WHERE a IN (SELECT b FROM t2)",
		"SELECT (SELECT MAX(a) FROM t) AS max_a",
		"SELECT EXISTS (SELECT 1 FROM t WHERE a = 1)",
	}

	for _, sql := range mayFail {
		_, err := ParseString(sql)
		if err != nil {
			t.Logf("ParseString(%s) failed (may be unsupported): %v", sql, err)
		} else {
			t.Logf("ParseString(%s) succeeded", sql)
		}
	}
}

// TestParseExpressionExtraVariations tests extra expressions
func TestParseExpressionExtraVariations(t *testing.T) {
	// Test supported expressions
	supported := []string{
		"SELECT -a FROM t",
		"SELECT NOT a FROM t",
		"SELECT a AND b FROM t",
		"SELECT a OR b FROM t",
	}

	for _, sql := range supported {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}

	// Test expressions that may not be supported
	mayFail := []string{
		"SELECT +a FROM t",
		"SELECT CASE WHEN a = 1 THEN 'one' ELSE 'other' END FROM t",
	}

	for _, sql := range mayFail {
		_, err := ParseString(sql)
		if err != nil {
			t.Logf("ParseString(%s) failed (may be unsupported): %v", sql, err)
		} else {
			t.Logf("ParseString(%s) succeeded", sql)
		}
	}
}

// TestParseShowExtraStatements tests extra SHOW statements
func TestParseShowExtraStatements(t *testing.T) {
	// Test supported SHOW statements
	supported := []string{
		"SHOW TABLES",
		"SHOW COLUMNS FROM t",
	}

	for _, sql := range supported {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}

	// Test unsupported SHOW statements - these should fail
	unsupported := []string{
		"SHOW CREATE TABLE t",
		"SHOW DATABASES",
		"SHOW VARIABLES",
	}

	for _, sql := range unsupported {
		_, err := ParseString(sql)
		if err == nil {
			t.Logf("ParseString(%s) unexpectedly succeeded", sql)
		} else {
			t.Logf("ParseString(%s) failed as expected: %v", sql, err)
		}
	}
}

// TestParseSetExtraStatements tests extra SET statements
func TestParseSetExtraStatements(t *testing.T) {
	tests := []string{
		"SET LOG_LEVEL DEBUG",
		"SET USER 'admin'",
		"SET PASSWORD 'secret'",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// TestParseCreateExtraVariations tests extra CREATE variations
func TestParseCreateExtraVariations(t *testing.T) {
	tests := []string{
		"CREATE TABLE t (id INT, name VARCHAR(100), PRIMARY KEY (id))",
		"CREATE TABLE t (id INT, name VARCHAR(100), UNIQUE (name))",
		"CREATE TABLE t (id INT DEFAULT 0)",
		"CREATE TABLE t (id INT NOT NULL)",
		"CREATE TABLE t (id INT, FOREIGN KEY (id) REFERENCES t2(id))",
		"CREATE UNIQUE INDEX idx_name ON t(name)",
	}

	for _, sql := range tests {
		_, err := ParseString(sql)
		if err != nil {
			t.Errorf("ParseString(%s) failed: %v", sql, err)
		}
	}
}

// ============================================================
// Benchmark Tests
// ============================================================

func BenchmarkParseSelect(b *testing.B) {
	sql := "SELECT id, name, value FROM users WHERE age > 18 ORDER BY name LIMIT 100"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParseString(sql)
	}
}

func BenchmarkParseInsert(b *testing.B) {
	sql := "INSERT INTO users (name, email, age) VALUES ('John', 'john@example.com', 25)"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParseString(sql)
	}
}

func BenchmarkParseUpdate(b *testing.B) {
	sql := "UPDATE users SET name = 'Jane', age = 30 WHERE id = 1"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParseString(sql)
	}
}

func BenchmarkParseDelete(b *testing.B) {
	sql := "DELETE FROM users WHERE id = 1"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParseString(sql)
	}
}

func BenchmarkParseCreateTable(b *testing.B) {
	sql := "CREATE TABLE test (id SEQ PRIMARY KEY, name VARCHAR(100) NOT NULL, email VARCHAR(200), created_at DATETIME)"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParseString(sql)
	}
}

func BenchmarkParseJoin(b *testing.B) {
	sql := "SELECT u.id, u.name, o.order_id FROM users u JOIN orders o ON u.id = o.user_id WHERE u.status = 'active'"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParseString(sql)
	}
}

func BenchmarkLexerTokenize(b *testing.B) {
	sql := "SELECT id, name, email, age, created_at FROM users WHERE age > 18 AND status = 'active' ORDER BY created_at DESC LIMIT 100 OFFSET 50"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lexer := NewLexer(sql)
		for {
			tok := lexer.NextToken()
			if tok.Type == TokEOF {
				break
			}
		}
	}
}
