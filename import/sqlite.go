package importpkg

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	_ "modernc.org/sqlite"
)

// SQLiteImporter imports data from SQLite
type SQLiteImporter struct {
	db        *sql.DB
	dsn       string
	converter *TypeConverter
}

// NewSQLiteImporter creates a new SQLite importer
func NewSQLiteImporter() *SQLiteImporter {
	return &SQLiteImporter{
		converter: NewTypeConverter(),
	}
}

// Connect connects to the SQLite database
func (s *SQLiteImporter) Connect(dsn string) error {
	// SQLite DSN format: file:/path/to/database.db
	// Or just: /path/to/database.db
	// For in-memory: :memory:

	// Add sqlite driver prefix if not present
	connStr := dsn
	if connStr != ":memory:" && len(connStr) > 0 && connStr[0] != '/' && connStr[0] != '.' && connStr[0] != ':' {
		connStr = "file:" + connStr
	}

	db, err := sql.Open("sqlite", connStr)
	if err != nil {
		return fmt.Errorf("failed to open SQLite connection: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping SQLite: %w", err)
	}

	s.db = db
	s.dsn = dsn
	return nil
}

// Disconnect closes the connection
func (s *SQLiteImporter) Disconnect() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// ListTables lists all tables in the database
func (s *SQLiteImporter) ListTables(dbName string) ([]string, error) {
	// SQLite doesn't have multiple databases, dbName is ignored
	// Use ESCAPE to properly match literal underscore
	query := "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT LIKE '\\_%' ESCAPE '\\'"

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}

	return tables, nil
}

// GetTableSchema gets the schema of a table including constraints
func (s *SQLiteImporter) GetTableSchema(dbName, tableName string) (*TableSchema, error) {
	schema := &TableSchema{
		TableName: tableName,
	}

	// Get columns using PRAGMA
	if err := s.getColumns(tableName, schema); err != nil {
		return nil, err
	}

	// Get CREATE TABLE statement for constraint parsing
	var createSQL string
	query := "SELECT sql FROM sqlite_master WHERE type='table' AND name=?"
	if err := s.db.QueryRow(query, tableName).Scan(&createSQL); err != nil {
		return nil, fmt.Errorf("failed to get table schema: %w", err)
	}

	// Parse constraints from CREATE statement
	s.parseConstraints(createSQL, schema)

	// Get indexes
	if err := s.getIndexes(tableName, schema); err != nil {
		return nil, err
	}

	return schema, nil
}

// getColumns retrieves column information using PRAGMA
func (s *SQLiteImporter) getColumns(tableName string, schema *TableSchema) error {
	pragmaQuery := fmt.Sprintf("PRAGMA table_info(%s)", s.quoteIdentifier(tableName))
	rows, err := s.db.Query(pragmaQuery)
	if err != nil {
		return fmt.Errorf("failed to get table info: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var defaultValue sql.NullString

		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}

		targetType, length := s.converter.ConvertSQLiteType(colType)

		col := ColumnInfo{
			Name:         name,
			SourceType:   colType,
			TargetType:   targetType,
			Length:       length,
			IsNullable:   notNull == 0,
			IsPrimaryKey: pk > 0,
		}

		if defaultValue.Valid {
			col.DefaultValue = s.parseDefaultValue(defaultValue.String)
		}

		schema.Columns = append(schema.Columns, col)

		// Add to primary keys if pk > 0
		if pk > 0 {
			schema.PrimaryKeys = append(schema.PrimaryKeys, name)
		}
	}

	return nil
}

// parseConstraints parses constraints from CREATE TABLE statement
func (s *SQLiteImporter) parseConstraints(createSQL string, schema *TableSchema) {
	upperSQL := strings.ToUpper(createSQL)

	// Find FOREIGN KEY constraints
	fkRegex := regexp.MustCompile(`(?i)FOREIGN\s+KEY\s*\(([^)]+)\)\s+REFERENCES\s+(\w+)\s*\(([^)]+)\)(?:\s+ON\s+DELETE\s+(CASCADE|SET\s+NULL|SET\s+DEFAULT|RESTRICT|NO\s+ACTION))?(?:\s+ON\s+UPDATE\s+(CASCADE|SET\s+NULL|SET\s+DEFAULT|RESTRICT|NO\s+ACTION))?`)
	fkMatches := fkRegex.FindAllStringSubmatch(createSQL, -1)
	for i, match := range fkMatches {
		if len(match) >= 4 {
			columns := s.parseColumnList(match[1])
			refColumns := s.parseColumnList(match[3])
			refTable := strings.Trim(match[2], "\"`[]")

			fk := ForeignKeyInfo{
				Name:       fmt.Sprintf("fk_%s_%d", schema.TableName, i),
				Columns:    columns,
				RefTable:   refTable,
				RefColumns: refColumns,
			}
			if len(match) > 4 && match[4] != "" {
				fk.OnDelete = strings.ToUpper(match[4])
			}
			if len(match) > 5 && match[5] != "" {
				fk.OnUpdate = strings.ToUpper(match[5])
			}
			schema.ForeignKeys = append(schema.ForeignKeys, fk)
		}
	}

	// Find UNIQUE constraints (table-level)
	uniqueRegex := regexp.MustCompile(`(?i)UNIQUE\s*\(([^)]+)\)`)
	uniqueMatches := uniqueRegex.FindAllStringSubmatch(createSQL, -1)
	for i, match := range uniqueMatches {
		if len(match) >= 2 {
			columns := s.parseColumnList(match[1])

			// Check if it's a column-level unique (already handled)
			isColumnLevel := false
			for _, col := range schema.Columns {
				if len(columns) == 1 && col.Name == columns[0] && col.IsUnique {
					isColumnLevel = true
					break
				}
			}

			if !isColumnLevel {
				schema.UniqueKeys = append(schema.UniqueKeys, UniqueKeyInfo{
					Name:    fmt.Sprintf("uk_%s_%d", schema.TableName, i),
					Columns: columns,
				})
			}
		}
	}

	// Find CHECK constraints
	checkRegex := regexp.MustCompile(`(?i)CHECK\s*\(([^)]+(?:\([^)]*\)[^)]*)*)\)`)
	checkMatches := checkRegex.FindAllStringSubmatch(upperSQL, -1)
	for i, match := range checkMatches {
		if len(match) >= 2 {
			// Extract from original SQL to preserve case
			origMatches := checkRegex.FindAllStringSubmatch(createSQL, -1)
			if i < len(origMatches) && len(origMatches[i]) >= 2 {
				schema.CheckConstraints = append(schema.CheckConstraints, CheckInfo{
					Name:       fmt.Sprintf("ck_%s_%d", schema.TableName, i),
					Definition: origMatches[i][1],
				})
			}
		}
	}

	// Check for column-level UNIQUE keyword in CREATE statement
	for i, col := range schema.Columns {
		colPattern := regexp.MustCompile(fmt.Sprintf(`(?i)\b%s\b[^,)]*?\bUNIQUE\b`, regexp.QuoteMeta(col.Name)))
		if colPattern.MatchString(createSQL) {
			schema.Columns[i].IsUnique = true
		}
	}
}

// parseColumnList parses a comma-separated column list
func (s *SQLiteImporter) parseColumnList(list string) []string {
	list = strings.TrimSpace(list)
	var columns []string
	for _, col := range strings.Split(list, ",") {
		col = strings.TrimSpace(col)
		col = strings.Trim(col, "\"`[]")
		if col != "" {
			columns = append(columns, col)
		}
	}
	return columns
}

// getIndexes retrieves index information
func (s *SQLiteImporter) getIndexes(tableName string, schema *TableSchema) error {
	query := "SELECT name, sql FROM sqlite_master WHERE type='index' AND tbl_name=? AND sql IS NOT NULL"
	rows, err := s.db.Query(query, tableName)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var name, sql string
		if err := rows.Scan(&name, &sql); err != nil {
			return err
		}

		// Parse index columns from SQL
		upperSQL := strings.ToUpper(sql)
		isUnique := strings.Contains(upperSQL, " UNIQUE ")

		// Extract columns: CREATE ... INDEX ... ON table (col1, col2)
		colRegex := regexp.MustCompile(`\(([^)]+)\)`)
		match := colRegex.FindStringSubmatch(sql)
		if len(match) >= 2 {
			columns := s.parseColumnList(match[1])

			// Skip if this is a unique constraint index
			isConstraintIndex := false
			for _, uk := range schema.UniqueKeys {
				if strings.Contains(name, uk.Name) || len(uk.Columns) == len(columns) {
					// Check if same columns
					sameCols := true
					for i, c := range columns {
						if i >= len(uk.Columns) || uk.Columns[i] != c {
							sameCols = false
							break
						}
					}
					if sameCols {
						isConstraintIndex = true
						break
					}
				}
			}

			if !isConstraintIndex && len(columns) > 0 {
				schema.Indexes = append(schema.Indexes, IndexInfo{
					Name:    name,
					Columns: columns,
					Unique:  isUnique,
				})
			}
		}
	}

	return nil
}

// parseDefaultValue parses SQLite default value
func (s *SQLiteImporter) parseDefaultValue(val string) interface{} {
	if val == "" || val == "NULL" {
		return nil
	}
	// Remove quotes if present
	if len(val) >= 2 && (val[0] == '\'' || val[0] == '"') && val[0] == val[len(val)-1] {
		return val[1 : len(val)-1]
	}
	return val
}

// ReadTable reads rows from a table
func (s *SQLiteImporter) ReadTable(dbName, tableName string, offset, limit int) ([][]interface{}, error) {
	query := fmt.Sprintf("SELECT * FROM %s LIMIT %d OFFSET %d", s.quoteIdentifier(tableName), limit, offset)

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to read table: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results [][]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		// Convert []byte to string for text columns
		for i, v := range values {
			if b, ok := v.([]byte); ok {
				values[i] = string(b)
			}
		}

		results = append(results, values)
	}

	return results, nil
}

// ReadTableCount gets the total number of rows in a table
func (s *SQLiteImporter) ReadTableCount(dbName, tableName string) (int64, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", s.quoteIdentifier(tableName))

	var count int64
	if err := s.db.QueryRow(query).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count rows: %w", err)
	}

	return count, nil
}

// quoteIdentifier quotes a SQLite identifier
func (s *SQLiteImporter) quoteIdentifier(name string) string {
	return fmt.Sprintf("\"%s\"", name)
}
