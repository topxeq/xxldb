package importpkg

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/denisenkom/go-mssqldb"
)

// MSSQLImporter imports data from MS SQL Server
type MSSQLImporter struct {
	db        *sql.DB
	dsn       string
	converter *TypeConverter
}

// NewMSSQLImporter creates a new MSSQL importer
func NewMSSQLImporter() *MSSQLImporter {
	return &MSSQLImporter{
		converter: NewTypeConverter(),
	}
}

// Connect connects to the MS SQL Server database
func (m *MSSQLImporter) Connect(dsn string) error {
	// MSSQL DSN format: sqlserver://user:password@host:port/database?params
	// Or: server=host;user id=user;password=pass;database=db

	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		return fmt.Errorf("failed to open MSSQL connection: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping MSSQL: %w", err)
	}

	m.db = db
	m.dsn = dsn
	return nil
}

// Disconnect closes the connection
func (m *MSSQLImporter) Disconnect() error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}

// ListTables lists all tables in the database
func (m *MSSQLImporter) ListTables(dbName string) ([]string, error) {
	query := `
		SELECT TABLE_NAME
		FROM INFORMATION_SCHEMA.TABLES
		WHERE TABLE_TYPE = 'BASE TABLE'`
	args := []interface{}{}

	if dbName != "" {
		query += " AND TABLE_CATALOG = ?"
		args = append(args, dbName)
	}
	query += " ORDER BY TABLE_NAME"

	rows, err := m.db.Query(query, args...)
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
func (m *MSSQLImporter) GetTableSchema(dbName, tableName string) (*TableSchema, error) {
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

	// Get check constraints
	if err := m.getCheckConstraints(dbName, tableName, schema); err != nil {
		return nil, err
	}

	// Get indexes
	if err := m.getIndexes(dbName, tableName, schema); err != nil {
		return nil, err
	}

	return schema, nil
}

// getColumns retrieves column information
func (m *MSSQLImporter) getColumns(dbName, tableName string, schema *TableSchema) error {
	query := `
		SELECT
			COLUMN_NAME,
			DATA_TYPE,
			CHARACTER_MAXIMUM_LENGTH,
			IS_NULLABLE,
			COLUMN_DEFAULT
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_NAME = ?`
	args := []interface{}{tableName}

	if dbName != "" {
		query += " AND TABLE_CATALOG = ?"
		args = append(args, dbName)
	}
	query += " ORDER BY ORDINAL_POSITION"

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return fmt.Errorf("failed to get table schema: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name, colType, isNullable string
		var maxLen sql.NullInt64
		var defaultValue sql.NullString

		if err := rows.Scan(&name, &colType, &maxLen, &isNullable, &defaultValue); err != nil {
			return err
		}

		// Combine type with length if available
		fullType := colType
		if maxLen.Valid && maxLen.Int64 > 0 && maxLen.Int64 != -1 {
			if strings.ToLower(colType) == "nvarchar" || strings.ToLower(colType) == "nchar" {
				fullType = fmt.Sprintf("%s(%d)", colType, maxLen.Int64/2) // NVARCHAR stores Unicode
			} else {
				fullType = fmt.Sprintf("%s(%d)", colType, maxLen.Int64)
			}
		}

		targetType, length := m.converter.ConvertMSSQLType(fullType)

		col := ColumnInfo{
			Name:       name,
			SourceType: fullType,
			TargetType: targetType,
			Length:     length,
			IsNullable: isNullable == "YES",
		}

		if defaultValue.Valid {
			col.DefaultValue = m.parseDefaultValue(defaultValue.String)
		}

		schema.Columns = append(schema.Columns, col)
	}

	return nil
}

// getPrimaryKeys retrieves primary key constraint
func (m *MSSQLImporter) getPrimaryKeys(dbName, tableName string, schema *TableSchema) error {
	query := `
		SELECT KCU.COLUMN_NAME, TC.CONSTRAINT_NAME
		FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS TC
		JOIN INFORMATION_SCHEMA.KEY_COLUMN_USAGE KCU
			ON TC.CONSTRAINT_NAME = KCU.CONSTRAINT_NAME
		WHERE TC.TABLE_NAME = ?
		AND TC.CONSTRAINT_TYPE = 'PRIMARY KEY'`
	args := []interface{}{tableName}

	if dbName != "" {
		query += " AND TC.TABLE_CATALOG = ?"
		args = append(args, dbName)
	}
	query += " ORDER BY KCU.ORDINAL_POSITION"

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var columnName, constraintName string
		if err := rows.Scan(&columnName, &constraintName); err != nil {
			return err
		}
		schema.PrimaryKeys = append(schema.PrimaryKeys, columnName)

		// Mark column as primary key
		for i, col := range schema.Columns {
			if strings.EqualFold(col.Name, columnName) {
				schema.Columns[i].IsPrimaryKey = true
			}
		}
	}

	return nil
}

// getForeignKeys retrieves foreign key constraints
func (m *MSSQLImporter) getForeignKeys(dbName, tableName string, schema *TableSchema) error {
	query := `
		SELECT
			TC.CONSTRAINT_NAME,
			KCU.COLUMN_NAME,
			CCU.TABLE_NAME AS REF_TABLE,
			CCU.COLUMN_NAME AS REF_COLUMN,
			RC.DELETE_RULE,
			RC.UPDATE_RULE
		FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS TC
		JOIN INFORMATION_SCHEMA.KEY_COLUMN_USAGE KCU
			ON TC.CONSTRAINT_NAME = KCU.CONSTRAINT_NAME
		JOIN INFORMATION_SCHEMA.CONSTRAINT_COLUMN_USAGE CCU
			ON TC.CONSTRAINT_NAME = CCU.CONSTRAINT_NAME
		JOIN INFORMATION_SCHEMA.REFERENTIAL_CONSTRAINTS RC
			ON TC.CONSTRAINT_NAME = RC.CONSTRAINT_NAME
		WHERE TC.TABLE_NAME = ?
		AND TC.CONSTRAINT_TYPE = 'FOREIGN KEY'`
	args := []interface{}{tableName}

	if dbName != "" {
		query += " AND TC.TABLE_CATALOG = ?"
		args = append(args, dbName)
	}
	query += " ORDER BY TC.CONSTRAINT_NAME, KCU.ORDINAL_POSITION"

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
func (m *MSSQLImporter) getUniqueKeys(dbName, tableName string, schema *TableSchema) error {
	query := `
		SELECT TC.CONSTRAINT_NAME, KCU.COLUMN_NAME
		FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS TC
		JOIN INFORMATION_SCHEMA.KEY_COLUMN_USAGE KCU
			ON TC.CONSTRAINT_NAME = KCU.CONSTRAINT_NAME
		WHERE TC.TABLE_NAME = ?
		AND TC.CONSTRAINT_TYPE = 'UNIQUE'`
	args := []interface{}{tableName}

	if dbName != "" {
		query += " AND TC.TABLE_CATALOG = ?"
		args = append(args, dbName)
	}
	query += " ORDER BY TC.CONSTRAINT_NAME, KCU.ORDINAL_POSITION"

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	ukMap := make(map[string]*UniqueKeyInfo)
	for rows.Next() {
		var constraintName, columnName string
		if err := rows.Scan(&constraintName, &columnName); err != nil {
			return err
		}

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

		// Mark single-column unique
		if len(uk.Columns) == 1 {
			for i, col := range schema.Columns {
				if strings.EqualFold(col.Name, uk.Columns[0]) {
					schema.Columns[i].IsUnique = true
				}
			}
		}
	}

	return nil
}

// getCheckConstraints retrieves check constraints
func (m *MSSQLImporter) getCheckConstraints(dbName, tableName string, schema *TableSchema) error {
	query := `
		SELECT CC.CONSTRAINT_NAME, CC.CHECK_CLAUSE
		FROM INFORMATION_SCHEMA.CHECK_CONSTRAINTS CC
		JOIN INFORMATION_SCHEMA.CONSTRAINT_TABLE_USAGE CTU
			ON CC.CONSTRAINT_NAME = CTU.CONSTRAINT_NAME
		WHERE CTU.TABLE_NAME = ?`
	args := []interface{}{tableName}

	if dbName != "" {
		query += " AND CTU.TABLE_CATALOG = ?"
		args = append(args, dbName)
	}

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		var clause sql.NullString
		if err := rows.Scan(&name, &clause); err != nil {
			return err
		}

		if clause.Valid {
			schema.CheckConstraints = append(schema.CheckConstraints, CheckInfo{
				Name:       name,
				Definition: clause.String,
			})
		}
	}

	return nil
}

// getIndexes retrieves index information
func (m *MSSQLImporter) getIndexes(dbName, tableName string, schema *TableSchema) error {
	query := `
		SELECT
			i.name AS index_name,
			c.name AS column_name,
			i.is_unique,
			i.is_primary_key
		FROM sys.indexes i
		JOIN sys.index_columns ic ON i.object_id = ic.object_id AND i.index_id = ic.index_id
		JOIN sys.columns c ON ic.object_id = c.object_id AND ic.column_id = c.column_id
		JOIN sys.tables t ON i.object_id = t.object_id
		WHERE t.name = ?
		ORDER BY i.name, ic.key_ordinal`

	args := []interface{}{tableName}

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	idxMap := make(map[string]*IndexInfo)
	for rows.Next() {
		var indexName, columnName string
		var isUnique, isPrimaryKey bool
		if err := rows.Scan(&indexName, &columnName, &isUnique, &isPrimaryKey); err != nil {
			return err
		}

		// Skip primary key indexes
		if isPrimaryKey {
			continue
		}

		if idx, exists := idxMap[indexName]; exists {
			idx.Columns = append(idx.Columns, columnName)
		} else {
			idxMap[indexName] = &IndexInfo{
				Name:    indexName,
				Columns: []string{columnName},
				Unique:  isUnique,
			}
		}
	}

	// Filter out unique constraint indexes
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

// parseDefaultValue parses MSSQL default value
func (m *MSSQLImporter) parseDefaultValue(val string) interface{} {
	if val == "" {
		return nil
	}
	// Remove parentheses and (( )) wrapping
	val = strings.Trim(val, "()")
	// Remove N prefix for Unicode strings
	if len(val) > 1 && val[0] == 'N' && (val[1] == '\'' || val[1] == '"') {
		val = val[1:]
	}
	// Remove quotes if present
	if len(val) >= 2 && (val[0] == '\'' || val[0] == '"') && val[0] == val[len(val)-1] {
		return val[1 : len(val)-1]
	}
	return val
}

// ReadTable reads rows from a table
func (m *MSSQLImporter) ReadTable(dbName, tableName string, offset, limit int) ([][]interface{}, error) {
	// SQL Server 2012+ uses OFFSET FETCH
	query := fmt.Sprintf("SELECT * FROM %s ORDER BY (SELECT NULL) OFFSET %d ROWS FETCH NEXT %d ROWS ONLY",
		m.quoteIdentifier(tableName), offset, limit)

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
func (m *MSSQLImporter) ReadTableCount(dbName, tableName string) (int64, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", m.quoteIdentifier(tableName))

	var count int64
	if err := m.db.QueryRow(query).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count rows: %w", err)
	}

	return count, nil
}

// quoteIdentifier quotes a MSSQL identifier
func (m *MSSQLImporter) quoteIdentifier(name string) string {
	return fmt.Sprintf("[%s]", name)
}
