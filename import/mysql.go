package importpkg

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/topxeq/xxldb/types"
)

// MySQLImporter imports data from MySQL
type MySQLImporter struct {
	db        *sql.DB
	dsn       string
	converter *TypeConverter
}

// NewMySQLImporter creates a new MySQL importer
func NewMySQLImporter() *MySQLImporter {
	return &MySQLImporter{
		converter: NewTypeConverter(),
	}
}

// Connect connects to the MySQL database
func (m *MySQLImporter) Connect(dsn string) error {
	// MySQL DSN format: user:password@tcp(host:port)/dbname
	// Also accept: user:password@host:port/dbname (will be converted)
	mysqlDSN := m.convertDSN(dsn)

	db, err := sql.Open("mysql", mysqlDSN)
	if err != nil {
		return fmt.Errorf("failed to open MySQL connection: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping MySQL: %w", err)
	}

	m.db = db
	m.dsn = mysqlDSN
	return nil
}

// convertDSN converts various DSN formats to MySQL driver format
// Accepts: user:pass@host:port/dbname -> user:pass@tcp(host:port)/dbname
// Or already valid: user:pass@tcp(host:port)/dbname
func (m *MySQLImporter) convertDSN(dsn string) string {
	// If already has tcp(), return as is
	if strings.Contains(dsn, "@tcp(") {
		return dsn
	}

	// Parse user:pass@host:port/dbname format
	// Split at @ to get user:pass and host:port/dbname
	atIndex := strings.Index(dsn, "@")
	if atIndex == -1 {
		return dsn // No @ found, return as is
	}

	userPass := dsn[:atIndex+1] // user:pass@
	rest := dsn[atIndex+1:]     // host:port/dbname

	// Find the first / to separate host:port from dbname
	slashIndex := strings.Index(rest, "/")
	if slashIndex == -1 {
		return dsn // No / found, return as is
	}

	hostPort := rest[:slashIndex]
	dbname := rest[slashIndex:] // /dbname

	// Reconstruct with tcp()
	return userPass + "tcp(" + hostPort + ")" + dbname
}

// Disconnect closes the connection
func (m *MySQLImporter) Disconnect() error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}

// ListTables lists all tables in the database
func (m *MySQLImporter) ListTables(dbName string) ([]string, error) {
	query := "SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_TYPE = 'BASE TABLE'"
	if dbName != "" {
		query += " AND TABLE_SCHEMA = ?"
	}

	var rows *sql.Rows
	var err error
	if dbName != "" {
		rows, err = m.db.Query(query, dbName)
	} else {
		rows, err = m.db.Query(query)
	}
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
func (m *MySQLImporter) GetTableSchema(dbName, tableName string) (*TableSchema, error) {
	schema := &TableSchema{
		TableName: tableName,
	}

	// Get columns
	if err := m.getColumns(dbName, tableName, schema); err != nil {
		return nil, err
	}

	// Get primary keys
	if err := m.getPrimaryKeys(dbName, tableName, schema); err != nil {
		return nil, err
	}

	// Get foreign keys
	if err := m.getForeignKeys(dbName, tableName, schema); err != nil {
		return nil, err
	}

	// Get unique constraints
	if err := m.getUniqueKeys(dbName, tableName, schema); err != nil {
		return nil, err
	}

	// Get check constraints (MySQL 8.0+)
	if err := m.getCheckConstraints(dbName, tableName, schema); err != nil {
		// Non-fatal, may not be supported in older MySQL versions
	}

	// Get indexes
	if err := m.getIndexes(dbName, tableName, schema); err != nil {
		return nil, err
	}

	return schema, nil
}

// getColumns retrieves column information
func (m *MySQLImporter) getColumns(dbName, tableName string, schema *TableSchema) error {
	query := `
		SELECT
			COLUMN_NAME,
			COLUMN_TYPE,
			IS_NULLABLE,
			COLUMN_KEY,
			COLUMN_DEFAULT,
			EXTRA
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_NAME = ?`
	args := []interface{}{tableName}

	if dbName != "" {
		query += " AND TABLE_SCHEMA = ?"
		args = append(args, dbName)
	}
	query += " ORDER BY ORDINAL_POSITION"

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return fmt.Errorf("failed to get table schema: %w", err)
	}
	defer rows.Close()

	// Use a map to track processed columns to avoid duplicates
	processedCols := make(map[string]bool)

	for rows.Next() {
		var name, colType, isNullable, columnKey, extra string
		var defaultValue sql.NullString

		if err := rows.Scan(&name, &colType, &isNullable, &columnKey, &defaultValue, &extra); err != nil {
			return err
		}

		// Skip if already processed
		if processedCols[name] {
			continue
		}
		processedCols[name] = true

		targetType, length := m.converter.ConvertMySQLType(colType)

		col := ColumnInfo{
			Name:         name,
			SourceType:   colType,
			TargetType:   targetType,
			Length:       length,
			IsNullable:   isNullable == "YES",
			IsPrimaryKey: columnKey == "PRI",
			IsUnique:     columnKey == "UNI",
		}

		if defaultValue.Valid {
			col.DefaultValue = m.parseDefaultValue(defaultValue.String)
		}

		schema.Columns = append(schema.Columns, col)
	}

	return nil
}

// getPrimaryKeys retrieves primary key constraint
func (m *MySQLImporter) getPrimaryKeys(dbName, tableName string, schema *TableSchema) error {
	query := `
		SELECT COLUMN_NAME, CONSTRAINT_NAME
		FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE
		WHERE TABLE_NAME = ? AND CONSTRAINT_NAME = 'PRIMARY'`
	args := []interface{}{tableName}

	if dbName != "" {
		query += " AND TABLE_SCHEMA = ?"
		args = append(args, dbName)
	}
	query += " ORDER BY ORDINAL_POSITION"

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	processedCols := make(map[string]bool)

	for rows.Next() {
		var columnName, constraintName string
		if err := rows.Scan(&columnName, &constraintName); err != nil {
			return err
		}
		if !processedCols[columnName] {
			schema.PrimaryKeys = append(schema.PrimaryKeys, columnName)
			processedCols[columnName] = true
		}
	}

	return nil
}

// getForeignKeys retrieves foreign key constraints
func (m *MySQLImporter) getForeignKeys(dbName, tableName string, schema *TableSchema) error {
	query := `
		SELECT
			KCU.CONSTRAINT_NAME,
			KCU.COLUMN_NAME,
			KCU.REFERENCED_TABLE_NAME,
			KCU.REFERENCED_COLUMN_NAME,
			RC.DELETE_RULE,
			RC.UPDATE_RULE
		FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE KCU
		JOIN INFORMATION_SCHEMA.REFERENTIAL_CONSTRAINTS RC
			ON KCU.CONSTRAINT_NAME = RC.CONSTRAINT_NAME
			AND KCU.TABLE_SCHEMA = RC.CONSTRAINT_SCHEMA
		WHERE KCU.TABLE_NAME = ? AND KCU.REFERENCED_TABLE_NAME IS NOT NULL`
	args := []interface{}{tableName}

	if dbName != "" {
		query += " AND KCU.TABLE_SCHEMA = ?"
		args = append(args, dbName)
	}
	query += " ORDER BY KCU.CONSTRAINT_NAME, KCU.ORDINAL_POSITION"

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	fkMap := make(map[string]*ForeignKeyInfo)
	for rows.Next() {
		var constraintName, columnName, refTable, refColumn, deleteRule, updateRule string
		if err := rows.Scan(&constraintName, &columnName, &refTable, &refColumn, &deleteRule, &updateRule); err != nil {
			return err
		}

		if fk, exists := fkMap[constraintName]; exists {
			fk.Columns = append(fk.Columns, columnName)
			fk.RefColumns = append(fk.RefColumns, refColumn)
		} else {
			fkMap[constraintName] = &ForeignKeyInfo{
				Name:       constraintName,
				Columns:    []string{columnName},
				RefTable:   refTable,
				RefColumns: []string{refColumn},
				OnDelete:   deleteRule,
				OnUpdate:   updateRule,
			}
		}
	}

	for _, fk := range fkMap {
		schema.ForeignKeys = append(schema.ForeignKeys, *fk)
	}

	return nil
}

// getUniqueKeys retrieves unique constraints
func (m *MySQLImporter) getUniqueKeys(dbName, tableName string, schema *TableSchema) error {
	query := `
		SELECT
			TC.CONSTRAINT_NAME,
			KCU.COLUMN_NAME
		FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS TC
		JOIN INFORMATION_SCHEMA.KEY_COLUMN_USAGE KCU
			ON TC.CONSTRAINT_NAME = KCU.CONSTRAINT_NAME
			AND TC.TABLE_SCHEMA = KCU.TABLE_SCHEMA
		WHERE TC.TABLE_NAME = ?
			AND TC.CONSTRAINT_TYPE = 'UNIQUE'`
	args := []interface{}{tableName}

	if dbName != "" {
		query += " AND TC.TABLE_SCHEMA = ?"
		args = append(args, dbName)
	}
	query += " ORDER BY TC.CONSTRAINT_NAME, KCU.ORDINAL_POSITION"

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	ukMap := make(map[string]*UniqueKeyInfo)
	colProcessed := make(map[string]map[string]bool) // constraint -> column -> processed

	for rows.Next() {
		var constraintName, columnName string
		if err := rows.Scan(&constraintName, &columnName); err != nil {
			return err
		}

		// Initialize processed tracking for this constraint
		if colProcessed[constraintName] == nil {
			colProcessed[constraintName] = make(map[string]bool)
		}

		// Skip duplicates
		if colProcessed[constraintName][columnName] {
			continue
		}
		colProcessed[constraintName][columnName] = true

		if uk, exists := ukMap[constraintName]; exists {
			uk.Columns = append(uk.Columns, columnName)
		} else {
			ukMap[constraintName] = &UniqueKeyInfo{
				Name:    constraintName,
				Columns: []string{columnName},
			}
		}
	}

	for _, uk := range ukMap {
		schema.UniqueKeys = append(schema.UniqueKeys, *uk)
	}

	return nil
}

// getCheckConstraints retrieves check constraints (MySQL 8.0+)
func (m *MySQLImporter) getCheckConstraints(dbName, tableName string, schema *TableSchema) error {
	query := `
		SELECT CONSTRAINT_NAME, CHECK_CLAUSE
		FROM INFORMATION_SCHEMA.CHECK_CONSTRAINTS
		WHERE TABLE_NAME = ?`
	args := []interface{}{tableName}

	if dbName != "" {
		query += " AND CONSTRAINT_SCHEMA = ?"
		args = append(args, dbName)
	}

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var name, clause string
		if err := rows.Scan(&name, &clause); err != nil {
			return err
		}
		schema.CheckConstraints = append(schema.CheckConstraints, CheckInfo{
			Name:       name,
			Definition: clause,
		})
	}

	return nil
}

// getIndexes retrieves index information
func (m *MySQLImporter) getIndexes(dbName, tableName string, schema *TableSchema) error {
	query := `
		SELECT
			INDEX_NAME,
			COLUMN_NAME,
			NON_UNIQUE
		FROM INFORMATION_SCHEMA.STATISTICS
		WHERE TABLE_NAME = ? AND INDEX_NAME != 'PRIMARY'`
	args := []interface{}{tableName}

	if dbName != "" {
		query += " AND TABLE_SCHEMA = ?"
		args = append(args, dbName)
	}
	query += " ORDER BY INDEX_NAME, SEQ_IN_INDEX"

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	idxMap := make(map[string]*IndexInfo)
	for rows.Next() {
		var indexName, columnName string
		var nonUnique int
		if err := rows.Scan(&indexName, &columnName, &nonUnique); err != nil {
			return err
		}

		if idx, exists := idxMap[indexName]; exists {
			idx.Columns = append(idx.Columns, columnName)
		} else {
			idxMap[indexName] = &IndexInfo{
				Name:    indexName,
				Columns: []string{columnName},
				Unique:  nonUnique == 0,
			}
		}
	}

	// Filter out unique constraints (they are already in UniqueKeys)
	for _, idx := range idxMap {
		isUniqueConstraint := false
		for _, uk := range schema.UniqueKeys {
			if uk.Name == idx.Name {
				isUniqueConstraint = true
				break
			}
		}
		if !isUniqueConstraint {
			schema.Indexes = append(schema.Indexes, *idx)
		}
	}

	return nil
}

// parseDefaultValue parses MySQL default value
func (m *MySQLImporter) parseDefaultValue(val string) interface{} {
	if val == "NULL" || val == "" {
		return nil
	}
	// Remove quotes if present
	if len(val) >= 2 && (val[0] == '\'' || val[0] == '"') && val[0] == val[len(val)-1] {
		return val[1 : len(val)-1]
	}
	return val
}

// ReadTable reads rows from a table
func (m *MySQLImporter) ReadTable(dbName, tableName string, offset, limit int) ([][]interface{}, error) {
	query := fmt.Sprintf("SELECT * FROM %s", m.quoteIdentifier(tableName))
	if dbName != "" {
		query = fmt.Sprintf("SELECT * FROM %s.%s", m.quoteIdentifier(dbName), m.quoteIdentifier(tableName))
	}
	query += fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)

	rows, err := m.db.Query(query)
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
func (m *MySQLImporter) ReadTableCount(dbName, tableName string) (int64, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", m.quoteIdentifier(tableName))
	if dbName != "" {
		query = fmt.Sprintf("SELECT COUNT(*) FROM %s.%s", m.quoteIdentifier(dbName), m.quoteIdentifier(tableName))
	}

	var count int64
	if err := m.db.QueryRow(query).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count rows: %w", err)
	}

	return count, nil
}

// quoteIdentifier quotes a MySQL identifier
func (m *MySQLImporter) quoteIdentifier(name string) string {
	return fmt.Sprintf("`%s`", name)
}

// ConvertType converts MySQL type to XxLdb type (public helper)
func (m *MySQLImporter) ConvertType(mysqlType string) (types.DataType, int) {
	return m.converter.ConvertMySQLType(mysqlType)
}
