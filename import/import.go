// Package import provides database import functionality for xxldb
package importpkg

import (
	"fmt"
	"strings"

	"github.com/topxeq/xxldb/executor"
	"github.com/topxeq/xxldb/types"
)

// DatabaseType represents the type of source database
type DatabaseType string

const (
	DatabaseTypeMySQL      DatabaseType = "mysql"
	DatabaseTypePostgreSQL DatabaseType = "postgresql"
	DatabaseTypeSQLite     DatabaseType = "sqlite"
	DatabaseTypeOracle     DatabaseType = "oracle"
	DatabaseTypeMSSQL      DatabaseType = "mssql"
)

// ImportConfig holds the configuration for database import
type ImportConfig struct {
	SourceType   DatabaseType
	SourceDSN    string
	SourceDB     string
	SourceTable  string
	TargetTable  string
	BatchSize    int
	Overwrite    bool
	CreateTable  bool
	TransformMap map[string]string
}

// ImportResult holds the result of an import operation
type ImportResult struct {
	TablesImported int
	RowsImported   int64
	Errors         []error
	Warnings       []string
}

// Importer defines the interface for database importers
type Importer interface {
	Connect(dsn string) error
	Disconnect() error
	ListTables(dbName string) ([]string, error)
	GetTableSchema(dbName, tableName string) (*TableSchema, error)
	ReadTable(dbName, tableName string, offset, limit int) ([][]interface{}, error)
	ReadTableCount(dbName, tableName string) (int64, error)
}

// TableSchema represents the schema of a table
type TableSchema struct {
	TableName       string
	Columns         []ColumnInfo
	PrimaryKeys     []string           // Primary key column names
	ForeignKeys     []ForeignKeyInfo   // Foreign key constraints
	UniqueKeys      []UniqueKeyInfo    // Unique constraints
	CheckConstraints []CheckInfo       // Check constraints
	Indexes         []IndexInfo        // Indexes
}

// ColumnInfo represents column information
type ColumnInfo struct {
	Name         string
	SourceType   string
	TargetType   types.DataType
	Length       int
	IsNullable   bool
	IsPrimaryKey bool
	IsUnique     bool
	DefaultValue interface{}
}

// ForeignKeyInfo represents a foreign key constraint
type ForeignKeyInfo struct {
	Name             string   // Constraint name
	Columns          []string // Source columns
	RefTable         string   // Referenced table
	RefColumns       []string // Referenced columns
	OnDelete         string   // ON DELETE action
	OnUpdate         string   // ON UPDATE action
}

// UniqueKeyInfo represents a unique constraint
type UniqueKeyInfo struct {
	Name    string   // Constraint name
	Columns []string // Columns in the unique constraint
}

// CheckInfo represents a check constraint
type CheckInfo struct {
	Name       string // Constraint name
	Definition string // Check condition
}

// IndexInfo represents an index
type IndexInfo struct {
	Name    string   // Index name
	Columns []string // Indexed columns
	Unique  bool     // Whether it's a unique index
}

// ImportManager manages database imports
type ImportManager struct {
	engine    *executor.Engine
	importers map[DatabaseType]Importer
}

// NewImportManager creates a new import manager
func NewImportManager(engine *executor.Engine) *ImportManager {
	return &ImportManager{
		engine:    engine,
		importers: make(map[DatabaseType]Importer),
	}
}

// RegisterImporter registers an importer for a database type
func (m *ImportManager) RegisterImporter(dbType DatabaseType, importer Importer) {
	m.importers[dbType] = importer
}

// ImportTable imports a single table
func (m *ImportManager) ImportTable(config *ImportConfig) (*ImportResult, error) {
	result := &ImportResult{}

	importer, exists := m.importers[config.SourceType]
	if !exists {
		return nil, fmt.Errorf("unsupported database type: %s", config.SourceType)
	}

	// Connect to source
	if err := importer.Connect(config.SourceDSN); err != nil {
		return nil, fmt.Errorf("failed to connect to source: %w", err)
	}
	defer importer.Disconnect()

	// Get source table name
	sourceTable := config.SourceTable
	if sourceTable == "" {
		return nil, fmt.Errorf("source table name is required")
	}

	// Get target table name
	targetTable := config.TargetTable
	if targetTable == "" {
		targetTable = sourceTable
	}

	// Get source table schema
	schema, err := importer.GetTableSchema(config.SourceDB, sourceTable)
	if err != nil {
		return nil, fmt.Errorf("failed to get table schema: %w", err)
	}

	// Create target table if needed
	if config.CreateTable {
		if err := m.createTableFromSchema(targetTable, schema, config.Overwrite); err != nil {
			return nil, fmt.Errorf("failed to create target table: %w", err)
		}
	}

	// Get total rows
	totalRows, err := importer.ReadTableCount(config.SourceDB, sourceTable)
	if err != nil {
		return nil, fmt.Errorf("failed to get row count: %w", err)
	}

	// Enable skip-save mode for bulk import
	m.engine.SetSkipSave(true)
	defer func() {
		// Disable skip-save and force save
		m.engine.SetSkipSave(false)
		m.engine.ForceSave()
	}()

	// Import data in batches
	batchSize := config.BatchSize
	if batchSize <= 0 {
		batchSize = 1000
	}

	var importedRows int64
	for offset := 0; int64(offset) < totalRows; offset += batchSize {
		limit := batchSize
		if int64(offset+limit) > totalRows {
			limit = int(totalRows - int64(offset))
		}

		rows, err := importer.ReadTable(config.SourceDB, sourceTable, offset, limit)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("batch %d: %w", offset/batchSize, err))
			continue
		}

		if err := m.insertRows(targetTable, schema, rows); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("insert batch %d: %w", offset/batchSize, err))
			continue
		}

		importedRows += int64(len(rows))
	}

	result.TablesImported = 1
	result.RowsImported = importedRows

	return result, nil
}

// ImportAll imports all tables from a database
func (m *ImportManager) ImportAll(config *ImportConfig) (*ImportResult, error) {
	result := &ImportResult{}

	importer, exists := m.importers[config.SourceType]
	if !exists {
		return nil, fmt.Errorf("unsupported database type: %s", config.SourceType)
	}

	// Connect to source
	if err := importer.Connect(config.SourceDSN); err != nil {
		return nil, fmt.Errorf("failed to connect to source: %w", err)
	}
	defer importer.Disconnect()

	// List all tables
	tables, err := importer.ListTables(config.SourceDB)
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}

	// Import each table
	for _, tableName := range tables {
		tableConfig := &ImportConfig{
			SourceType:   config.SourceType,
			SourceDSN:    config.SourceDSN,
			SourceDB:     config.SourceDB,
			SourceTable:  tableName,
			TargetTable:  tableName,
			BatchSize:    config.BatchSize,
			Overwrite:    config.Overwrite,
			CreateTable:  true,
			TransformMap: config.TransformMap,
		}

		tableResult, err := m.ImportTable(tableConfig)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("table %s: %w", tableName, err))
			continue
		}

		result.TablesImported += tableResult.TablesImported
		result.RowsImported += tableResult.RowsImported
		result.Warnings = append(result.Warnings, tableResult.Warnings...)
	}

	return result, nil
}

// createTableFromSchema creates a table from schema information
func (m *ImportManager) createTableFromSchema(tableName string, schema *TableSchema, overwrite bool) error {
	if overwrite {
		m.engine.Execute(fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName))
	}

	var columnDefs []string
	for _, col := range schema.Columns {
		def := m.buildColumnDef(col, schema)
		columnDefs = append(columnDefs, def)
	}

	// Add table-level constraints
	// Primary key constraint (composite)
	if len(schema.PrimaryKeys) > 1 {
		pkDef := fmt.Sprintf("PRIMARY KEY (%s)", strings.Join(schema.PrimaryKeys, ", "))
		columnDefs = append(columnDefs, pkDef)
	}

	// Foreign key constraints
	for _, fk := range schema.ForeignKeys {
		fkDef := fmt.Sprintf("CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (%s)",
			fk.Name,
			strings.Join(fk.Columns, ", "),
			fk.RefTable,
			strings.Join(fk.RefColumns, ", "),
		)
		if fk.OnDelete != "" {
			fkDef += fmt.Sprintf(" ON DELETE %s", fk.OnDelete)
		}
		if fk.OnUpdate != "" {
			fkDef += fmt.Sprintf(" ON UPDATE %s", fk.OnUpdate)
		}
		columnDefs = append(columnDefs, fkDef)
	}

	// Unique constraints
	for _, uk := range schema.UniqueKeys {
		ukDef := fmt.Sprintf("CONSTRAINT %s UNIQUE (%s)", uk.Name, strings.Join(uk.Columns, ", "))
		columnDefs = append(columnDefs, ukDef)
	}

	// Check constraints
	for _, ck := range schema.CheckConstraints {
		ckDef := fmt.Sprintf("CONSTRAINT %s CHECK (%s)", ck.Name, ck.Definition)
		columnDefs = append(columnDefs, ckDef)
	}

	createSQL := fmt.Sprintf("CREATE TABLE %s (%s)", tableName, strings.Join(columnDefs, ", "))
	_, err := m.engine.Execute(createSQL)
	if err != nil {
		return err
	}

	// Create indexes after table creation
	for _, idx := range schema.Indexes {
		uniqueStr := ""
		if idx.Unique {
			uniqueStr = "UNIQUE "
		}
		idxSQL := fmt.Sprintf("CREATE %sINDEX %s ON %s (%s)",
			uniqueStr, idx.Name, tableName, strings.Join(idx.Columns, ", "))
		if _, err := m.engine.Execute(idxSQL); err != nil {
			// Log warning but don't fail
			m.engine.Execute(fmt.Sprintf("-- Warning: Failed to create index %s: %v", idx.Name, err))
		}
	}

	return nil
}

// buildColumnDef builds a column definition string
func (m *ImportManager) buildColumnDef(col ColumnInfo, schema *TableSchema) string {
	var parts []string

	parts = append(parts, col.Name)
	parts = append(parts, col.TargetType.String())

	if col.Length > 0 {
		parts[len(parts)-1] = fmt.Sprintf("%s(%d)", col.TargetType.String(), col.Length)
	}

	// Only add PRIMARY KEY inline for single-column primary key
	if col.IsPrimaryKey && len(schema.PrimaryKeys) <= 1 {
		parts = append(parts, "PRIMARY KEY")
	}

	// Add UNIQUE constraint inline if it's a single-column unique and not part of a multi-column unique
	if col.IsUnique && !m.isPartOfMultiColumnUnique(col.Name, schema) {
		parts = append(parts, "UNIQUE")
	}

	if !col.IsNullable {
		parts = append(parts, "NOT NULL")
	}

	if col.DefaultValue != nil {
		parts = append(parts, handleDefaultValue(col.DefaultValue))
	}

	return strings.Join(parts, " ")
}

// isPartOfMultiColumnUnique checks if a column is part of a multi-column unique constraint
func (m *ImportManager) isPartOfMultiColumnUnique(colName string, schema *TableSchema) bool {
	for _, uk := range schema.UniqueKeys {
		if len(uk.Columns) > 1 {
			for _, c := range uk.Columns {
				if c == colName {
					return true
				}
			}
		}
	}
	return false
}

// insertRows inserts rows into a table using batch INSERT for better performance
func (m *ImportManager) insertRows(tableName string, schema *TableSchema, rows [][]interface{}) error {
	if len(rows) == 0 {
		return nil
	}

	// Build column names
	var colNames []string
	for _, col := range schema.Columns {
		colNames = append(colNames, col.Name)
	}

	// Build batch VALUES clause
	var allValues []string
	for _, row := range rows {
		var valueStrs []string
		for i, val := range row {
			converted := m.convertValue(val, schema.Columns[i].TargetType)
			if converted == nil {
				valueStrs = append(valueStrs, "NULL")
			} else {
				switch v := converted.(type) {
				case int, int32, int64, float32, float64:
					valueStrs = append(valueStrs, fmt.Sprintf("%v", v))
				case string:
					escaped := strings.ReplaceAll(v, "'", "''")
					valueStrs = append(valueStrs, fmt.Sprintf("'%s'", escaped))
				default:
					escaped := strings.ReplaceAll(fmt.Sprintf("%v", v), "'", "''")
					valueStrs = append(valueStrs, fmt.Sprintf("'%s'", escaped))
				}
			}
		}
		allValues = append(allValues, fmt.Sprintf("(%s)", strings.Join(valueStrs, ", ")))
	}

	// Execute batch INSERT (all rows in one statement)
	insertSQL := fmt.Sprintf("INSERT INTO %s (%s) VALUES %s",
		tableName,
		strings.Join(colNames, ", "),
		strings.Join(allValues, ", "))

	if _, err := m.engine.Execute(insertSQL); err != nil {
		return err
	}

	return nil
}

// convertValue converts a value to the target type
func (m *ImportManager) convertValue(val interface{}, targetType types.DataType) interface{} {
	if val == nil {
		return nil
	}

	switch targetType {
	case types.TypeInt, types.TypeSeq:
		switch v := val.(type) {
		case int:
			return int64(v)
		case int32:
			return int64(v)
		case int64:
			return v
		case float64:
			return int64(v)
		case float32:
			return int64(v)
		}
	case types.TypeFloat:
		switch v := val.(type) {
		case float32:
			return float64(v)
		case float64:
			return v
		case int:
			return float64(v)
		case int64:
			return float64(v)
		}
	case types.TypeVarchar, types.TypeText, types.TypeChar:
		return fmt.Sprintf("%v", val)
	}

	return val
}

// ParseDSN parses a DSN string and returns database type, connection info, and database name
func ParseDSN(dsn string) (DatabaseType, string, string, error) {
	// DSN format: <type>://<connection_string>
	// Examples:
	//   mysql://user:pass@host:3306/dbname
	//   postgresql://user:pass@host:5432/dbname
	//   sqlite:///path/to/database.db
	//   oracle://user:pass@host:1521/sid
	//   mssql://user:pass@host:1433/dbname

	parts := strings.SplitN(dsn, "://", 2)
	if len(parts) != 2 {
		return "", "", "", fmt.Errorf("invalid DSN format, expected: <type>://<connection_string>")
	}

	dbType := DatabaseType(strings.ToLower(parts[0]))
	connStr := parts[1]

	// Extract database name from connection string
	var dbName string
	switch dbType {
	case DatabaseTypeMySQL, DatabaseTypePostgreSQL, DatabaseTypeMSSQL:
		// Format: user:pass@host:port/dbname or user:pass@tcp(host:port)/dbname
		if slashIdx := strings.LastIndex(connStr, "/"); slashIdx != -1 {
			dbName = connStr[slashIdx+1:]
			// Remove query parameters if any
			if qIdx := strings.Index(dbName, "?"); qIdx != -1 {
				dbName = dbName[:qIdx]
			}
		}
	case DatabaseTypeSQLite:
		// For SQLite, the database name is the file name without extension
		if slashIdx := strings.LastIndex(connStr, "/"); slashIdx != -1 {
			dbName = connStr[slashIdx+1:]
			if dotIdx := strings.LastIndex(dbName, "."); dotIdx != -1 {
				dbName = dbName[:dotIdx]
			}
		}
	case DatabaseTypeOracle:
		// Format: user:pass@host:port/sid
		if slashIdx := strings.LastIndex(connStr, "/"); slashIdx != -1 {
			dbName = connStr[slashIdx+1:]
		}
	}

	switch dbType {
	case DatabaseTypeMySQL, DatabaseTypePostgreSQL, DatabaseTypeSQLite, DatabaseTypeOracle, DatabaseTypeMSSQL:
		return dbType, connStr, dbName, nil
	default:
		return "", "", "", fmt.Errorf("unsupported database type: %s", dbType)
	}
}

// handleDefaultValue converts special default values for XxLdb compatibility
func handleDefaultValue(val interface{}) string {
	if val == nil {
		return "DEFAULT NULL"
	}
	
	defaultStr := fmt.Sprintf("%v", val)
	
	// Convert CURRENT_TIMESTAMP to NOW()
	if strings.ToUpper(defaultStr) == "CURRENT_TIMESTAMP" {
		return "DEFAULT NOW()"
	}
	
	return fmt.Sprintf("DEFAULT %v", val)
}
