package importpkg

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/lib/pq"
)

// PostgreSQLImporter imports data from PostgreSQL
type PostgreSQLImporter struct {
	db        *sql.DB
	dsn       string
	converter *TypeConverter
}

// NewPostgreSQLImporter creates a new PostgreSQL importer
func NewPostgreSQLImporter() *PostgreSQLImporter {
	return &PostgreSQLImporter{
		converter: NewTypeConverter(),
	}
}

// Connect connects to the PostgreSQL database
func (p *PostgreSQLImporter) Connect(dsn string) error {
	// PostgreSQL DSN format: postgres://user:password@host:port/dbname?sslmode=disable
	// Or: user=postgres password=secret host=localhost port=5432 dbname=test sslmode=disable
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("failed to open PostgreSQL connection: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	p.db = db
	p.dsn = dsn
	return nil
}

// Disconnect closes the connection
func (p *PostgreSQLImporter) Disconnect() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

// ListTables lists all tables in the database
func (p *PostgreSQLImporter) ListTables(dbName string) ([]string, error) {
	query := `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public'
		AND table_type = 'BASE TABLE'`

	rows, err := p.db.Query(query)
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
func (p *PostgreSQLImporter) GetTableSchema(dbName, tableName string) (*TableSchema, error) {
	schema := &TableSchema{
		TableName: tableName,
	}

	// Get columns
	if err := p.getColumns(dbName, tableName, schema); err != nil {
		return nil, err
	}

	// Get primary keys
	if err := p.getPrimaryKeys(tableName, schema); err != nil {
		return nil, err
	}

	// Get foreign keys
	if err := p.getForeignKeys(tableName, schema); err != nil {
		return nil, err
	}

	// Get unique constraints
	if err := p.getUniqueKeys(tableName, schema); err != nil {
		return nil, err
	}

	// Get check constraints
	if err := p.getCheckConstraints(tableName, schema); err != nil {
		return nil, err
	}

	// Get indexes
	if err := p.getIndexes(tableName, schema); err != nil {
		return nil, err
	}

	return schema, nil
}

// getColumns retrieves column information
func (p *PostgreSQLImporter) getColumns(dbName, tableName string, schema *TableSchema) error {
	query := `
		SELECT
			column_name,
			data_type,
			CAST(character_maximum_length AS TEXT),
			is_nullable,
			column_default,
			udt_name
		FROM information_schema.columns c
		WHERE table_name = $1
		AND table_schema = 'public'
		ORDER BY ordinal_position`

	rows, err := p.db.Query(query, tableName)
	if err != nil {
		return fmt.Errorf("failed to get table schema: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name, colType, isNullable string
		var maxLen, defaultValue sql.NullString
		var udtName string

		if err := rows.Scan(&name, &colType, &maxLen, &isNullable, &defaultValue, &udtName); err != nil {
			return err
		}

		// Handle USER-DEFINED types
		if colType == "USER-DEFINED" {
			colType = udtName
		}

		// Combine type with length if available
		fullType := colType
		if maxLen.Valid && maxLen.String != "" {
			fullType = fmt.Sprintf("%s(%s)", colType, maxLen.String)
		}

		targetType, length := p.converter.ConvertPostgreSQLType(fullType)

		col := ColumnInfo{
			Name:       name,
			SourceType: fullType,
			TargetType: targetType,
			Length:     length,
			IsNullable: isNullable == "YES",
		}

		if defaultValue.Valid {
			col.DefaultValue = p.parseDefaultValue(defaultValue.String)
		}

		schema.Columns = append(schema.Columns, col)
	}

	return nil
}

// getPrimaryKeys retrieves primary key constraint
func (p *PostgreSQLImporter) getPrimaryKeys(tableName string, schema *TableSchema) error {
	query := `
		SELECT kcu.column_name, tc.constraint_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
		WHERE tc.table_name = $1
		AND tc.constraint_type = 'PRIMARY KEY'
		AND tc.table_schema = 'public'
		ORDER BY kcu.ordinal_position`

	rows, err := p.db.Query(query, tableName)
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
			if col.Name == columnName {
				schema.Columns[i].IsPrimaryKey = true
			}
		}
	}

	return nil
}

// getForeignKeys retrieves foreign key constraints
func (p *PostgreSQLImporter) getForeignKeys(tableName string, schema *TableSchema) error {
	query := `
		SELECT
			tc.constraint_name,
			kcu.column_name,
			ccu.table_name AS foreign_table_name,
			ccu.column_name AS foreign_column_name,
			rc.delete_rule,
			rc.update_rule
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
		JOIN information_schema.constraint_column_usage ccu
			ON ccu.constraint_name = tc.constraint_name
		JOIN information_schema.referential_constraints rc
			ON tc.constraint_name = rc.constraint_name
		WHERE tc.table_name = $1
		AND tc.constraint_type = 'FOREIGN KEY'
		AND tc.table_schema = 'public'
		ORDER BY tc.constraint_name, kcu.ordinal_position`

	rows, err := p.db.Query(query, tableName)
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
func (p *PostgreSQLImporter) getUniqueKeys(tableName string, schema *TableSchema) error {
	query := `
		SELECT tc.constraint_name, kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
		WHERE tc.table_name = $1
		AND tc.constraint_type = 'UNIQUE'
		AND tc.table_schema = 'public'
		ORDER BY tc.constraint_name, kcu.ordinal_position`

	rows, err := p.db.Query(query, tableName)
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
				if col.Name == uk.Columns[0] {
					schema.Columns[i].IsUnique = true
				}
			}
		}
	}

	return nil
}

// getCheckConstraints retrieves check constraints
func (p *PostgreSQLImporter) getCheckConstraints(tableName string, schema *TableSchema) error {
	query := `
		SELECT pgc.conname AS constraint_name,
			   pg_get_constraintdef(pgc.oid) AS definition
		FROM pg_constraint pgc
		JOIN pg_class cls ON pgc.conrelid = cls.oid
		JOIN pg_namespace nsp ON cls.relnamespace = nsp.oid
		WHERE cls.relname = $1
		AND pgc.contype = 'c'
		AND nsp.nspname = 'public'`

	rows, err := p.db.Query(query, tableName)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var name, definition string
		if err := rows.Scan(&name, &definition); err != nil {
			return err
		}

		// Extract check clause from definition
		// Definition format: CHECK (condition)
		def := definition
		if strings.HasPrefix(def, "CHECK ") {
			def = strings.TrimPrefix(def, "CHECK ")
		}

		schema.CheckConstraints = append(schema.CheckConstraints, CheckInfo{
			Name:       name,
			Definition: def,
		})
	}

	return nil
}

// getIndexes retrieves index information
func (p *PostgreSQLImporter) getIndexes(tableName string, schema *TableSchema) error {
	query := `
		SELECT
			i.relname AS index_name,
			a.attname AS column_name,
			ix.indisunique AS is_unique,
			ix.indisprimary AS is_primary
		FROM pg_class t
		JOIN pg_index ix ON t.oid = ix.indrelid
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY(ix.indkey)
		WHERE t.relname = $1
		AND t.relnamespace = (SELECT oid FROM pg_namespace WHERE nspname = 'public')
		ORDER BY i.relname, a.attnum`

	rows, err := p.db.Query(query, tableName)
	if err != nil {
		return err
	}
	defer rows.Close()

	idxMap := make(map[string]*IndexInfo)
	for rows.Next() {
		var indexName, columnName string
		var isUnique, isPrimary bool
		if err := rows.Scan(&indexName, &columnName, &isUnique, &isPrimary); err != nil {
			return err
		}

		// Skip primary key indexes
		if isPrimary {
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
			if uk.Name == idx.Name || strings.HasSuffix(idx.Name, "_"+uk.Name+"_idx") {
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

// parseDefaultValue parses PostgreSQL default value
func (p *PostgreSQLImporter) parseDefaultValue(val string) interface{} {
	if val == "" {
		return nil
	}
	// Remove type casts like ::character varying
	if idx := strings.Index(val, "::"); idx > 0 {
		val = val[:idx]
	}
	// Remove quotes if present
	if len(val) >= 2 && (val[0] == '\'' || val[0] == '"') && val[0] == val[len(val)-1] {
		return val[1 : len(val)-1]
	}
	return val
}

// ReadTable reads rows from a table
func (p *PostgreSQLImporter) ReadTable(dbName, tableName string, offset, limit int) ([][]interface{}, error) {
	query := fmt.Sprintf("SELECT * FROM %s LIMIT %d OFFSET %d", p.quoteIdentifier(tableName), limit, offset)

	rows, err := p.db.Query(query)
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
func (p *PostgreSQLImporter) ReadTableCount(dbName, tableName string) (int64, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", p.quoteIdentifier(tableName))

	var count int64
	if err := p.db.QueryRow(query).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count rows: %w", err)
	}

	return count, nil
}

// quoteIdentifier quotes a PostgreSQL identifier
func (p *PostgreSQLImporter) quoteIdentifier(name string) string {
	return fmt.Sprintf("\"%s\"", name)
}
