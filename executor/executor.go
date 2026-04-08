// Package executor provides SQL execution for xxldb
package executor

import (
	"fmt"
	"strings"
	"sync"

	"github.com/topxeq/xxldb/auth"
	"github.com/topxeq/xxldb/fts"
	"github.com/topxeq/xxldb/function"
	"github.com/topxeq/xxldb/logger"
	"github.com/topxeq/xxldb/parser"
	"github.com/topxeq/xxldb/script"
	"github.com/topxeq/xxldb/storage"
	"github.com/topxeq/xxldb/types"
)

// Version information
const (
	Version   = "2.0.0"
	BuildDate = "2026-04-04"
)

// Engine is the main database engine
type Engine struct {
	mu        sync.RWMutex
	storage   *storage.Storage
	auth      *auth.Auth
	log       *logger.Logger
	scripts   *script.Manager
	fts       *fts.Manager
	config    Config
	currentDB string
}

// Config holds engine configuration
type Config struct {
	Path          string
	InMemory      bool
	LogLevel      string
	Username      string
	Password      string
	AutoCommit    bool
	SyncInterval  int
	BlobThreshold int64 // Size threshold for external blob storage (default: 64KB, 0 = always inline)
	EncryptPassword string // Password for database encryption
}

// DefaultConfig returns default configuration
func DefaultConfig() Config {
	return Config{
		InMemory:   false,
		LogLevel:   "INFO",
		AutoCommit: true,
	}
}

// NewEngine creates a new database engine
func NewEngine(path string, inMemory bool) (*Engine, error) {
	config := DefaultConfig()
	config.Path = path
	config.InMemory = inMemory

	return NewEngineWithConfig(config)
}

// NewEngineWithConfig creates a new database engine with configuration
func NewEngineWithConfig(config Config) (*Engine, error) {
	store, err := storage.NewStorage(config.Path, config.InMemory)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	// Check if database is encrypted
	if !config.InMemory && config.Path != "" {
		isEncrypted := storage.IsDatabaseEncrypted(config.Path)
		if isEncrypted {
			if config.EncryptPassword == "" {
				store.Close()
				return nil, fmt.Errorf("database is encrypted - password required")
			}
			// Get salt from encrypted database
			salt, err := storage.GetEncryptionSalt(config.Path)
			if err != nil {
				store.Close()
				return nil, fmt.Errorf("failed to read encryption salt: %w", err)
			}
			// Set encryption with existing salt
			if err := store.SetEncryptionWithSalt(config.EncryptPassword, salt); err != nil {
				store.Close()
				return nil, fmt.Errorf("failed to set encryption: %w", err)
			}
			// Verify password by attempting to load metadata
			if err := store.LoadAndVerifyMetadata(); err != nil {
				store.Close()
				return nil, fmt.Errorf("decryption failed - wrong password: %w", err)
			}
		} else if config.EncryptPassword != "" {
			// New database or unencrypted database with password - enable encryption
			if err := store.SetEncryption(config.EncryptPassword); err != nil {
				store.Close()
				return nil, fmt.Errorf("failed to enable encryption: %w", err)
			}
		}
	}

	// Configure blob threshold if specified
	if config.BlobThreshold > 0 {
		storeConfig := store.GetConfig()
		storeConfig.BlobThreshold = config.BlobThreshold
		store.SetConfig(storeConfig)
	}

	logLevel := logger.INFO
	if config.LogLevel != "" {
		logLevel = logger.ParseLevel(config.LogLevel)
	}

	engine := &Engine{
		storage: store,
		auth:    auth.NewAuth(),
		log:     logger.NewLogger(logLevel, nil),
		scripts: script.NewManager(),
		fts:     fts.NewManager(),
		config:  config,
	}

	// Initialize system tables
	if err := engine.initSystemTables(); err != nil {
		engine.log.Warn("failed to initialize system tables: %v", err)
	}

	// Load auth config from storage if exists
	if authConfig := store.GetAuthConfig(); authConfig != nil {
		engine.auth.FromMap(authConfig)
	}

	// Set credentials if provided (overrides stored config)
	if config.Username != "" && config.Password != "" {
		engine.auth.SetCredentials(config.Username, config.Password)
	}

	// Restore FTS indexes from storage metadata
	if err := engine.restoreFTSIndexes(); err != nil {
		engine.log.Warn("failed to restore FTS indexes: %v", err)
	}

	return engine, nil
}

// NewEngineWithFS creates a new database engine with custom filesystem
func NewEngineWithFS(path string, inMemory bool, fs storage.FileSystem) (*Engine, error) {
	config := DefaultConfig()
	config.Path = path
	config.InMemory = inMemory

	store, err := storage.NewStorageWithFS(config.Path, config.InMemory, fs)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	// Configure blob threshold if specified
	if config.BlobThreshold > 0 {
		storeConfig := store.GetConfig()
		storeConfig.BlobThreshold = config.BlobThreshold
		store.SetConfig(storeConfig)
	}

	logLevel := logger.INFO
	if config.LogLevel != "" {
		logLevel = logger.ParseLevel(config.LogLevel)
	}

	engine := &Engine{
		storage: store,
		auth:    auth.NewAuth(),
		log:     logger.NewLogger(logLevel, nil),
		scripts: script.NewManager(),
		fts:     fts.NewManager(),
		config:  config,
	}

	// Initialize system tables
	if err := engine.initSystemTables(); err != nil {
		engine.log.Warn("failed to initialize system tables: %v", err)
	}

	// Load auth config from storage if exists
	if authConfig := store.GetAuthConfig(); authConfig != nil {
		engine.auth.FromMap(authConfig)
	}

	// Restore FTS indexes from storage metadata
	if err := engine.restoreFTSIndexes(); err != nil {
		engine.log.Warn("failed to restore FTS indexes: %v", err)
	}

	return engine, nil
}

// initSystemTables initializes system tables
func (e *Engine) initSystemTables() error {
	// Check if xxscript table exists
	tables := e.storage.ListTables()
	for _, t := range tables {
		if strings.ToLower(t) == "xxscript" {
			return nil
		}
	}

	// Create xxscript table for storing script functions
	tableInfo := types.NewTableInfo(0, "xxscript", []types.ColumnDef{
		{Name: "name", Type: types.TypeVarchar, Length: 100, Nullable: false, PrimaryKey: true},
		{Name: "script", Type: types.TypeText, Nullable: false},
		{Name: "description", Type: types.TypeVarchar, Length: 500, Nullable: true},
	})

	return e.storage.CreateTable(tableInfo)
}

// restoreFTSIndexes restores FTS indexes from storage metadata
func (e *Engine) restoreFTSIndexes() error {
	ftsIndexes := e.storage.GetFTSIndexes()
	if len(ftsIndexes) == 0 {
		return nil
	}

	for key, info := range ftsIndexes {
		// Check if FTS index already exists in memory (from previous restore)
		if e.fts.HasIndex(info.TableName, info.ColumnName) {
			continue
		}

		// Create the FTS index in memory
		if err := e.fts.CreateIndex(info.TableName, info.ColumnName, nil); err != nil {
			e.log.Warn("failed to create FTS index %s: %v", key, err)
			continue
		}

		// Try to load persisted FTS index from disk
		ftsFilePath := e.storage.GetFTSIndexPath(info.TableName, info.ColumnName)
		if ftsFilePath != "" {
			if indexer := e.fts.GetIndex(info.TableName, info.ColumnName); indexer != nil {
				if invIdx, ok := indexer.(*fts.InvertedIndex); ok {
					if err := invIdx.LoadFromFile(ftsFilePath); err == nil {
						e.log.Info("Loaded FTS index from %s", ftsFilePath)
						continue // Successfully loaded, skip re-indexing
					}
				}
			}
		}

		// FTS index file not found or load failed, re-index existing rows
		e.log.Info("Re-indexing %s.%s...", info.TableName, info.ColumnName)
		if err := e.indexExistingRows(info.TableName, info.ColumnName); err != nil {
			e.log.Warn("failed to re-index %s.%s: %v", info.TableName, info.ColumnName, err)
		}
	}

	return nil
}

// Close closes the engine
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.storage.Close()
}

// GetAuth returns the auth instance for authentication
func (e *Engine) GetAuth() *auth.Auth {
	return e.auth
}

// SetSkipSave enables or disables skip-save mode for bulk imports
func (e *Engine) SetSkipSave(skip bool) {
	e.storage.SetSkipSave(skip)
}

// ForceSave forces a save of the database metadata
func (e *Engine) ForceSave() error {
	return e.storage.ForceSave()
}

// IsEncrypted returns whether the database is encrypted
func (e *Engine) IsEncrypted() bool {
	return e.storage.IsEncrypted()
}

// GetFTSIndexes returns the list of FTS indexes (for debugging)
func (e *Engine) GetFTSIndexes() []string {
	return e.fts.ListIndexes()
}

// SetEncryption enables encryption with the given password
func (e *Engine) SetEncryption(password string) error {
	return e.storage.SetEncryption(password)
}

// ChangeEncryptionPassword changes the encryption password
func (e *Engine) ChangeEncryptionPassword(oldPassword, newPassword string) error {
	return e.storage.ChangePassword(oldPassword, newPassword)
}

// ListTables returns a list of all tables in the database
func (e *Engine) ListTables() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.storage.ListTables()
}

// Execute executes a SQL statement
func (e *Engine) Execute(sql string) (*Result, error) {
	stmt, err := parser.ParseString(sql)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	return e.ExecuteStatement(stmt)
}

// ExecuteStatement executes a parsed statement
func (e *Engine) ExecuteStatement(stmt *parser.Statement) (*Result, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	switch stmt.Type {
	case parser.StmtSelect:
		return e.executeSelect(stmt)
	case parser.StmtInsert:
		return e.executeInsert(stmt)
	case parser.StmtUpdate:
		return e.executeUpdate(stmt)
	case parser.StmtDelete:
		return e.executeDelete(stmt)
	case parser.StmtCreateTable:
		return e.executeCreateTable(stmt)
	case parser.StmtDropTable:
		return e.executeDropTable(stmt)
	case parser.StmtCreateDatabase:
		return e.executeCreateDatabase(stmt)
	case parser.StmtDropDatabase:
		return e.executeDropDatabase(stmt)
	case parser.StmtUse:
		return e.executeUse(stmt)
	case parser.StmtDescribe:
		return e.executeDescribe(stmt)
	case parser.StmtCreateIndex:
		return e.executeCreateIndex(stmt)
	case parser.StmtDropIndex:
		return e.executeDropIndex(stmt)
	case parser.StmtAlterTable:
		return e.executeAlterTable(stmt)
	case parser.StmtSet:
		return e.executeSet(stmt)
	case parser.StmtShow:
		return e.executeShow(stmt)
	case parser.StmtBackup:
		return e.executeBackup(stmt)
	case parser.StmtRestore:
		return e.executeRestore(stmt)
	case parser.StmtBegin:
		return e.executeBegin(stmt)
	case parser.StmtCommit:
		return e.executeCommit(stmt)
	case parser.StmtRollback:
		return e.executeRollback(stmt)
	default:
		return nil, fmt.Errorf("unsupported statement type: %s", stmt.Type)
	}
}

// Result represents a query result
type Result struct {
	Columns           []string
	Rows              []*Row
	RowsAffected      int64
	LastInsertID      int64
	IsExecutionResult bool
	Message           string
}

// Row represents a result row
type Row struct {
	ID   uint64
	Data []types.Value
}

// Format formats the result for display
func (r *Result) Format() string {
	if r.IsExecutionResult {
		if r.Message != "" {
			return r.Message
		}
		return fmt.Sprintf("Rows affected: %d", r.RowsAffected)
	}

	if len(r.Rows) == 0 || len(r.Columns) == 0 {
		return "Empty result set"
	}

	// Calculate column widths
	widths := make([]int, len(r.Columns))
	for i, col := range r.Columns {
		widths[i] = len(col)
	}
	for _, row := range r.Rows {
		for i, val := range row.Data {
			l := len(val.ToString())
			if l > widths[i] {
				widths[i] = l
			}
		}
	}

	var sb strings.Builder

	// Header
	sb.WriteString("+")
	for _, w := range widths {
		sb.WriteString(strings.Repeat("-", w+2))
		sb.WriteString("+")
	}
	sb.WriteString("\n")

	// Column names
	sb.WriteString("|")
	for i, col := range r.Columns {
		sb.WriteString(" ")
		sb.WriteString(col)
		sb.WriteString(strings.Repeat(" ", widths[i]-len(col)))
		sb.WriteString(" |")
	}
	sb.WriteString("\n")

	// Separator
	sb.WriteString("+")
	for _, w := range widths {
		sb.WriteString(strings.Repeat("-", w+2))
		sb.WriteString("+")
	}
	sb.WriteString("\n")

	// Rows
	for _, row := range r.Rows {
		sb.WriteString("|")
		for i, val := range row.Data {
			s := val.ToString()
			sb.WriteString(" ")
			sb.WriteString(s)
			sb.WriteString(strings.Repeat(" ", widths[i]-len(s)))
			sb.WriteString(" |")
		}
		sb.WriteString("\n")
	}

	// Footer
	sb.WriteString("+")
	for _, w := range widths {
		sb.WriteString(strings.Repeat("-", w+2))
		sb.WriteString("+")
	}
	sb.WriteString("\n")

	sb.WriteString(fmt.Sprintf("%d rows\n", len(r.Rows)))

	return sb.String()
}

// executeSelect executes a SELECT statement
func (e *Engine) executeSelect(stmt *parser.Statement) (*Result, error) {
	// Handle SELECT without FROM (e.g., SELECT NOW())
	if stmt.Table == "" {
		return e.executeSelectWithoutTable(stmt)
	}

	// Handle information_schema virtual tables
	if strings.EqualFold(stmt.From.Database, "information_schema") || strings.EqualFold(stmt.Table, "information_schema") || strings.HasPrefix(strings.ToLower(stmt.Table), "information_schema.") {
		return e.executeInformationSchemaQuery(stmt)
	}

	// Get table info
	tableInfo, err := e.storage.GetTableInfo(stmt.Table)
	if err != nil {
		return nil, err
	}

	// Get all rows with their IDs (needed for FTS)
	rowsWithIDs, err := e.storage.GetRowsWithIDs(stmt.Table)
	if err != nil {
		return nil, err
	}

	// Build column index map
	colIndex := make(map[string]int)
	for i, col := range tableInfo.Columns {
		colIndex[strings.ToLower(col.Name)] = i
	}

	// Convert to result rows with IDs
	resultRows := make([]*Row, len(rowsWithIDs))
	for i, rowWithID := range rowsWithIDs {
		resultRows[i] = &Row{ID: rowWithID.ID, Data: rowWithID.Row}
	}

	// Apply WHERE filter
	if stmt.Where != nil {
		resultRows = e.filterRows(resultRows, stmt.Where, colIndex)
	}

	// Handle GROUP BY with aggregates
	if len(stmt.GroupBy) > 0 && e.hasAggregateFunctions(stmt.Columns) {
		resultRows = e.groupWithAggregates(resultRows, stmt.GroupBy, stmt.Columns, colIndex)
	} else if len(stmt.GroupBy) > 0 {
		resultRows = e.groupRows(resultRows, stmt.GroupBy, colIndex, stmt.Columns)
	} else if e.hasAggregateFunctions(stmt.Columns) {
		// Aggregate without GROUP BY - compute single result
		resultRows = e.computeAggregates(resultRows, stmt.Columns, colIndex)
	}

	// Handle ORDER BY
	if len(stmt.OrderBy) > 0 {
		resultRows = e.sortRows(resultRows, stmt.OrderBy, colIndex)
	}

	// Handle LIMIT
	if stmt.Limit != nil {
		limit := int(e.evalInt(stmt.Limit))
		if limit < len(resultRows) {
			resultRows = resultRows[:limit]
		}
	}

	// Handle OFFSET
	if stmt.Offset != nil {
		offset := int(e.evalInt(stmt.Offset))
		if offset < len(resultRows) {
			resultRows = resultRows[offset:]
		} else {
			resultRows = []*Row{}
		}
	}

	// Build result columns
	var columns []string
	if len(stmt.Columns) == 0 {
		columns = tableInfo.ColumnNames()
	} else {
		columns = make([]string, len(stmt.Columns))
		for i, col := range stmt.Columns {
			if col.All {
				if col.TableName != "" {
					// table.*
					columns = tableInfo.ColumnNames()
				} else {
					columns = tableInfo.ColumnNames()
				}
			} else if col.Alias != "" {
				columns[i] = col.Alias
			} else if col.Expr != nil {
				columns[i] = col.Expr.String()
			}
		}
	}

	// Project columns (skip if aggregates were computed - they're already projected)
	hasAggregates := e.hasAggregateFunctions(stmt.Columns)
	if !hasAggregates && len(stmt.Columns) > 0 && !stmt.Columns[0].All {
		projectedRows := make([]*Row, len(resultRows))
		for i, row := range resultRows {
			data := make([]types.Value, len(stmt.Columns))
			for j, col := range stmt.Columns {
				if col.Expr != nil {
					data[j] = e.evalExpr(col.Expr, row.Data, colIndex, row.ID)
				}
			}
			projectedRows[i] = &Row{ID: row.ID, Data: data}
		}
		resultRows = projectedRows
	}

	// Handle DISTINCT
	if stmt.Distinct {
		resultRows = e.distinctRows(resultRows)
	}

	// Handle UNION
	for _, unionStmt := range stmt.Union {
		unionResult, err := e.executeSelect(unionStmt)
		if err != nil {
			return nil, err
		}
		resultRows = append(resultRows, unionResult.Rows...)
	}

	return &Result{
		Columns: columns,
		Rows:    resultRows,
	}, nil
}

// executeSelectWithoutTable handles SELECT without FROM clause (e.g., SELECT NOW())
func (e *Engine) executeSelectWithoutTable(stmt *parser.Statement) (*Result, error) {
	// Build result columns
	columns := make([]string, len(stmt.Columns))
	data := make([]types.Value, len(stmt.Columns))

	for i, col := range stmt.Columns {
		if col.Alias != "" {
			columns[i] = col.Alias
		} else if col.Expr != nil {
			columns[i] = col.Expr.String()
		}

		// Evaluate expression
		if col.Expr != nil {
			data[i] = e.evalExpr(col.Expr, nil, nil, 0)
		}
	}

	return &Result{
		Columns: columns,
		Rows:    []*Row{{Data: data}},
	}, nil
}

// executeInformationSchemaQuery handles queries to information_schema
func (e *Engine) executeInformationSchemaQuery(stmt *parser.Statement) (*Result, error) {
	// Determine which information_schema table is being queried
	tableName := strings.ToLower(stmt.Table)
	if strings.HasPrefix(tableName, "information_schema.") {
		tableName = strings.TrimPrefix(tableName, "information_schema.")
	}

	// Also check From clause for table name (handles alias case)
	if stmt.From != nil && strings.HasPrefix(strings.ToLower(stmt.From.Table), "information_schema.") {
		tableName = strings.TrimPrefix(strings.ToLower(stmt.From.Table), "information_schema.")
	}

	switch tableName {
	case "tables", "information_schema":
		// Return all tables
		tables := e.storage.ListTables()
		var rows []*Row
		for _, t := range tables {
			rows = append(rows, &Row{Data: []types.Value{
				types.NewStringValue("def"),              // TABLE_CATALOG
				types.NewStringValue("xxldb"),            // TABLE_SCHEMA
				types.NewStringValue(t),                  // TABLE_NAME
				types.NewStringValue("BASE TABLE"),       // TABLE_TYPE
				types.NewStringValue("InnoDB"),           // ENGINE
				types.NewStringValue("10"),               // VERSION
				types.NewStringValue("Compact"),          // ROW_FORMAT
				types.NewStringValue("0"),                // TABLE_ROWS
				types.NewStringValue("0"),                // AVG_ROW_LENGTH
				types.NewStringValue("0"),                // DATA_LENGTH
				types.NewStringValue("0"),                // MAX_DATA_LENGTH
				types.NewStringValue("0"),                // INDEX_LENGTH
				types.NewStringValue("0"),                // DATA_FREE
				types.NewStringValue("0"),                // AUTO_INCREMENT
				types.NewStringValue(""),                 // CREATE_TIME
				types.NewStringValue(""),                 // UPDATE_TIME
				types.NewStringValue(""),                 // CHECK_TIME
				types.NewStringValue("utf8mb4_general_ci"), // TABLE_COLLATION
				types.NewStringValue(""),                 // CHECKSUM
				types.NewStringValue(""),                 // CREATE_OPTIONS
				types.NewStringValue(""),                 // TABLE_COMMENT
			}})
		}
		// Apply WHERE filter
		if stmt.Where != nil {
			colIndex := map[string]int{
				"table_catalog": 0, "table_schema": 1, "table_name": 2, "table_type": 3,
				"engine": 4, "version": 5, "row_format": 6, "table_rows": 7,
				"avg_row_length": 8, "data_length": 9, "max_data_length": 10,
				"index_length": 11, "data_free": 12, "auto_increment": 13,
				"create_time": 14, "update_time": 15, "check_time": 16,
				"table_collation": 17, "checksum": 18, "create_options": 19, "table_comment": 20,
			}
			rows = e.filterRows(rows, stmt.Where, colIndex)
		}
		return &Result{
			Columns: []string{"TABLE_CATALOG", "TABLE_SCHEMA", "TABLE_NAME", "TABLE_TYPE", "ENGINE", "VERSION", "ROW_FORMAT", "TABLE_ROWS", "AVG_ROW_LENGTH", "DATA_LENGTH", "MAX_DATA_LENGTH", "INDEX_LENGTH", "DATA_FREE", "AUTO_INCREMENT", "CREATE_TIME", "UPDATE_TIME", "CHECK_TIME", "TABLE_COLLATION", "CHECKSUM", "CREATE_OPTIONS", "TABLE_COMMENT"},
			Rows:    rows,
		}, nil

	case "columns":
		// Return all columns for all tables
		var rows []*Row
		tables := e.storage.ListTables()
		ordinalPos := 1
		for _, t := range tables {
			tableInfo, err := e.storage.GetTableInfo(t)
			if err != nil {
				continue
			}
			for _, col := range tableInfo.Columns {
				// Infer type if UNKNOWN
				dataType := col.Type.String()
				if col.Type == types.TypeUnknown || col.Type == types.TypeNull {
					if col.PrimaryKey {
						dataType = "INT"
					} else if col.Length > 0 {
						if col.Length > 255 {
							dataType = "TEXT"
						} else {
							dataType = "VARCHAR"
						}
					} else {
						dataType = "TEXT"
					}
				}
				// Determine COLUMN_KEY
				columnKey := ""
				if col.PrimaryKey {
					columnKey = "PRI"
				}
				rows = append(rows, &Row{Data: []types.Value{
					types.NewStringValue("def"),       // TABLE_CATALOG
					types.NewStringValue("xxldb"),     // TABLE_SCHEMA
					types.NewStringValue(t),           // TABLE_NAME
					types.NewStringValue(col.Name),    // COLUMN_NAME
					types.NewStringValue(fmt.Sprintf("%d", ordinalPos)), // ORDINAL_POSITION
					types.NewStringValue(""),          // COLUMN_DEFAULT
					types.NewStringValue("YES"),       // IS_NULLABLE
					types.NewStringValue(dataType),    // DATA_TYPE
					types.NewStringValue(fmt.Sprintf("%d", col.Length)), // CHARACTER_MAXIMUM_LENGTH
					types.NewStringValue("0"),         // NUMERIC_PRECISION
					types.NewStringValue("0"),         // NUMERIC_SCALE
					types.NewStringValue(""),          // DATETIME_PRECISION
					types.NewStringValue("utf8mb4"),   // CHARACTER_SET_NAME
					types.NewStringValue("utf8mb4_general_ci"), // COLLATION_NAME
					types.NewStringValue(fmt.Sprintf("%s(%d)", dataType, col.Length)), // COLUMN_TYPE
					types.NewStringValue(columnKey),   // COLUMN_KEY
					types.NewStringValue(""),          // EXTRA
					types.NewStringValue("select,insert,update,references"), // PRIVILEGES
					types.NewStringValue(""),          // COLUMN_COMMENT
				}})
				ordinalPos++
			}
		}
		// Apply WHERE filter
		if stmt.Where != nil {
			colIndex := map[string]int{
				"table_catalog": 0, "table_schema": 1, "table_name": 2, "column_name": 3,
				"ordinal_position": 4, "column_default": 5, "is_nullable": 6, "data_type": 7,
				"character_maximum_length": 8, "numeric_precision": 9, "numeric_scale": 10,
				"datetime_precision": 11, "character_set_name": 12, "collation_name": 13,
				"column_type": 14, "column_key": 15, "extra": 16, "privileges": 17, "column_comment": 18,
			}
			rows = e.filterRows(rows, stmt.Where, colIndex)
		}
		return &Result{
			Columns: []string{"TABLE_CATALOG", "TABLE_SCHEMA", "TABLE_NAME", "COLUMN_NAME", "ORDINAL_POSITION", "COLUMN_DEFAULT", "IS_NULLABLE", "DATA_TYPE", "CHARACTER_MAXIMUM_LENGTH", "NUMERIC_PRECISION", "NUMERIC_SCALE", "DATETIME_PRECISION", "CHARACTER_SET_NAME", "COLLATION_NAME", "COLUMN_TYPE", "COLUMN_KEY", "EXTRA", "PRIVILEGES", "COLUMN_COMMENT"},
			Rows:    rows,
		}, nil

	case "schemata":
		// Return database/schema info
		return &Result{
			Columns: []string{"CATALOG_NAME", "SCHEMA_NAME", "DEFAULT_CHARACTER_SET_NAME", "DEFAULT_COLLATION_NAME"},
			Rows: []*Row{
				{Data: []types.Value{
					types.NewStringValue("def"),
					types.NewStringValue("xxldb"),
					types.NewStringValue("utf8mb4"),
					types.NewStringValue("utf8mb4_general_ci"),
				}},
			},
		}, nil

	case "processlist":
		// Return current process
		return &Result{
			Columns: []string{"ID", "USER", "HOST", "DB", "COMMAND", "TIME", "STATE", "INFO"},
			Rows: []*Row{
				{Data: []types.Value{
					types.NewStringValue("1"),
					types.NewStringValue("admin"),
					types.NewStringValue("localhost"),
					types.NewStringValue("xxldb"),
					types.NewStringValue("Query"),
					types.NewStringValue("0"),
					types.NewStringValue(""),
					types.NewStringValue("SELECT"),
				}},
			},
		}, nil

	case "key_column_usage":
		// Return primary key information
		var rows []*Row
		tables := e.storage.ListTables()
		for _, t := range tables {
			tableInfo, err := e.storage.GetTableInfo(t)
			if err != nil {
				continue
			}
			for _, col := range tableInfo.Columns {
				if col.PrimaryKey {
					rows = append(rows, &Row{Data: []types.Value{
						types.NewStringValue("def"),       // CONSTRAINT_CATALOG
						types.NewStringValue("xxldb"),     // CONSTRAINT_SCHEMA
						types.NewStringValue("PRIMARY"),   // CONSTRAINT_NAME
						types.NewStringValue("def"),       // TABLE_CATALOG
						types.NewStringValue("xxldb"),     // TABLE_SCHEMA
						types.NewStringValue(t),           // TABLE_NAME
						types.NewStringValue(col.Name),    // COLUMN_NAME
						types.NewStringValue("1"),         // ORDINAL_POSITION
						types.NewStringValue("1"),         // POSITION_IN_UNIQUE_CONSTRAINT
						types.NewStringValue(""),          // REFERENCED_TABLE_SCHEMA
						types.NewStringValue(""),          // REFERENCED_TABLE_NAME
						types.NewStringValue(""),          // REFERENCED_COLUMN_NAME
					}})
				}
			}
		}
		// Apply WHERE filter
		if stmt.Where != nil {
			colIndex := map[string]int{
				"constraint_catalog": 0, "constraint_schema": 1, "constraint_name": 2,
				"table_catalog": 3, "table_schema": 4, "table_name": 5, "column_name": 6,
				"ordinal_position": 7, "position_in_unique_constraint": 8,
				"referenced_table_schema": 9, "referenced_table_name": 10, "referenced_column_name": 11,
			}
			rows = e.filterRows(rows, stmt.Where, colIndex)
		}
		return &Result{
			Columns: []string{"CONSTRAINT_CATALOG", "CONSTRAINT_SCHEMA", "CONSTRAINT_NAME", "TABLE_CATALOG", "TABLE_SCHEMA", "TABLE_NAME", "COLUMN_NAME", "ORDINAL_POSITION", "POSITION_IN_UNIQUE_CONSTRAINT", "REFERENCED_TABLE_SCHEMA", "REFERENCED_TABLE_NAME", "REFERENCED_COLUMN_NAME"},
			Rows:    rows,
		}, nil

	case "table_constraints":
		// Return constraint information
		var rows []*Row
		tables := e.storage.ListTables()
		for _, t := range tables {
			tableInfo, err := e.storage.GetTableInfo(t)
			if err != nil {
				continue
			}
			hasPK := false
			for _, col := range tableInfo.Columns {
				if col.PrimaryKey {
					hasPK = true
					break
				}
			}
			if hasPK {
				rows = append(rows, &Row{Data: []types.Value{
					types.NewStringValue("def"),       // CONSTRAINT_CATALOG
					types.NewStringValue("xxldb"),     // CONSTRAINT_SCHEMA
					types.NewStringValue("PRIMARY"),   // CONSTRAINT_NAME
					types.NewStringValue("def"),       // TABLE_CATALOG
					types.NewStringValue("xxldb"),     // TABLE_SCHEMA
					types.NewStringValue(t),           // TABLE_NAME
					types.NewStringValue("PRIMARY KEY"), // CONSTRAINT_TYPE
					types.NewStringValue("YES"),       // IS_DEFERRABLE
					types.NewStringValue("NO"),        // INITIALLY_DEFERRED
					types.NewStringValue("YES"),       // ENFORCED
				}})
			}
		}
		// Apply WHERE filter
		if stmt.Where != nil {
			colIndex := map[string]int{
				"constraint_catalog": 0, "constraint_schema": 1, "constraint_name": 2,
				"table_catalog": 3, "table_schema": 4, "table_name": 5, "constraint_type": 6,
				"is_deferrable": 7, "initially_deferred": 8, "enforced": 9,
			}
			rows = e.filterRows(rows, stmt.Where, colIndex)
		}
		return &Result{
			Columns: []string{"CONSTRAINT_CATALOG", "CONSTRAINT_SCHEMA", "CONSTRAINT_NAME", "TABLE_CATALOG", "TABLE_SCHEMA", "TABLE_NAME", "CONSTRAINT_TYPE", "IS_DEFERRABLE", "INITIALLY_DEFERRED", "ENFORCED"},
			Rows:    rows,
		}, nil

	case "statistics":
		// Return index statistics
		var rows []*Row
		tables := e.storage.ListTables()
		for _, t := range tables {
			tableInfo, err := e.storage.GetTableInfo(t)
			if err != nil {
				continue
			}
			seq := 1
			for _, col := range tableInfo.Columns {
				if col.PrimaryKey {
					rows = append(rows, &Row{Data: []types.Value{
						types.NewStringValue("xxldb"),    // TABLE_CATALOG
						types.NewStringValue("xxldb"),    // TABLE_SCHEMA
						types.NewStringValue(t),          // TABLE_NAME
						types.NewStringValue("0"),        // NON_UNIQUE
						types.NewStringValue("xxldb"),    // INDEX_SCHEMA
						types.NewStringValue("PRIMARY"),  // INDEX_NAME
						types.NewStringValue(fmt.Sprintf("%d", seq)), // SEQ_IN_INDEX
						types.NewStringValue(col.Name),   // COLUMN_NAME
						types.NewStringValue("A"),        // COLLATION
						types.NewStringValue("0"),        // CARDINALITY
						types.NewStringValue(""),         // SUB_PART
						types.NewStringValue(""),         // PACKED
						types.NewStringValue(""),         // NULLABLE
						types.NewStringValue("BTREE"),    // INDEX_TYPE
						types.NewStringValue(""),         // COMMENT
						types.NewStringValue(""),         // INDEX_COMMENT
					}})
					seq++
				}
			}
		}
		// Apply WHERE filter
		if stmt.Where != nil {
			colIndex := map[string]int{
				"table_catalog": 0, "table_schema": 1, "table_name": 2, "non_unique": 3,
				"index_schema": 4, "index_name": 5, "seq_in_index": 6, "column_name": 7,
				"collation": 8, "cardinality": 9, "sub_part": 10, "packed": 11,
				"nullable": 12, "index_type": 13, "comment": 14, "index_comment": 15,
			}
			rows = e.filterRows(rows, stmt.Where, colIndex)
		}
		return &Result{
			Columns: []string{"TABLE_CATALOG", "TABLE_SCHEMA", "TABLE_NAME", "NON_UNIQUE", "INDEX_SCHEMA", "INDEX_NAME", "SEQ_IN_INDEX", "COLUMN_NAME", "COLLATION", "CARDINALITY", "SUB_PART", "PACKED", "NULLABLE", "INDEX_TYPE", "COMMENT", "INDEX_COMMENT"},
			Rows:    rows,
		}, nil

	default:
		// Return empty result for unknown information_schema tables
		return &Result{
			Columns: []string{},
			Rows:    []*Row{},
		}, nil
	}
}

// filterRows filters rows by WHERE condition
func (e *Engine) filterRows(rows []*Row, where *parser.Expression, colIndex map[string]int) []*Row {
	var result []*Row
	for _, row := range rows {
		if e.evalBool(where, row.Data, colIndex, row.ID) {
			result = append(result, row)
		}
	}
	return result
}

// groupRows groups rows by GROUP BY columns
func (e *Engine) groupRows(rows []*Row, groupBy []string, colIndex map[string]int, columns []parser.SelectColumn) []*Row {
	if len(groupBy) == 0 {
		return rows
	}

	groups := make(map[string][]*Row)
	for _, row := range rows {
		key := e.buildGroupKey(row, groupBy, colIndex)
		groups[key] = append(groups[key], row)
	}

	var result []*Row
	for _, groupRows := range groups {
		if len(groupRows) > 0 {
			result = append(result, groupRows[0])
		}
	}

	return result
}

// groupWithAggregates groups rows by GROUP BY columns and computes aggregates per group
func (e *Engine) groupWithAggregates(rows []*Row, groupBy []string, columns []parser.SelectColumn, colIndex map[string]int) []*Row {
	if len(groupBy) == 0 {
		return rows
	}

	// Group rows by GROUP BY columns
	groups := make(map[string][]*Row)
	for _, row := range rows {
		key := e.buildGroupKey(row, groupBy, colIndex)
		groups[key] = append(groups[key], row)
	}

	// For each group, compute aggregates and build result row
	var result []*Row
	for _, groupRows := range groups {
		if len(groupRows) == 0 {
			continue
		}

		// Build result row for this group
		data := make([]types.Value, len(columns))
		for i, col := range columns {
			if col.Expr == nil {
				continue
			}

			if col.Expr.Type == parser.ExprFunction && function.IsAggregate(col.Expr.FuncName) {
				// Compute aggregate for this group
				funcName := strings.ToUpper(col.Expr.FuncName)
				var values []types.Value

				// Collect values from all rows in this group
				for _, row := range groupRows {
					if len(col.Expr.Args) > 0 && col.Expr.Args[0].Type != parser.ExprStar {
						val := e.evalExpr(col.Expr.Args[0], row.Data, colIndex, row.ID)
						values = append(values, val)
					} else {
						// COUNT(*) case
						values = append(values, types.NewIntValue(1))
					}
				}

				// Call aggregate function
				aggResult, err := function.Call(funcName, values)
				if err != nil {
					data[i] = types.NewNullValue()
				} else {
					data[i] = aggResult
				}
			} else if col.Expr.Type == parser.ExprColumn {
				// Non-aggregate column - use value from first row in group
				idx := colIndex[strings.ToLower(col.Expr.Column)]
				if idx < len(groupRows[0].Data) {
					data[i] = groupRows[0].Data[idx]
				}
			} else {
				// Other expression - evaluate with first row
				data[i] = e.evalExpr(col.Expr, groupRows[0].Data, colIndex, groupRows[0].ID)
			}
		}

		result = append(result, &Row{Data: data})
	}

	return result
}

// buildGroupKey builds a key for grouping
func (e *Engine) buildGroupKey(row *Row, groupBy []string, colIndex map[string]int) string {
	var key string
	for _, col := range groupBy {
		idx := colIndex[strings.ToLower(col)]
		if idx < len(row.Data) {
			key += row.Data[idx].ToString() + "|"
		}
	}
	return key
}

// sortRows sorts rows by ORDER BY
func (e *Engine) sortRows(rows []*Row, orderBy []parser.OrderByClause, colIndex map[string]int) []*Row {
	if len(orderBy) == 0 || len(rows) == 0 {
		return rows
	}

	// Simple insertion sort
	sorted := make([]*Row, len(rows))
	copy(sorted, rows)

	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0; j-- {
			if e.shouldSwap(sorted[j-1], sorted[j], orderBy, colIndex) {
				sorted[j-1], sorted[j] = sorted[j], sorted[j-1]
			} else {
				break
			}
		}
	}

	return sorted
}

// shouldSwap determines if two rows should be swapped
func (e *Engine) shouldSwap(row1, row2 *Row, orderBy []parser.OrderByClause, colIndex map[string]int) bool {
	for _, ob := range orderBy {
		var val1, val2 types.Value
		if ob.Expr.Type == parser.ExprColumn {
			idx := colIndex[strings.ToLower(ob.Expr.Column)]
			if idx < len(row1.Data) {
				val1 = row1.Data[idx]
			}
			if idx < len(row2.Data) {
				val2 = row2.Data[idx]
			}
		} else {
			val1 = e.evalExpr(ob.Expr, row1.Data, colIndex, row1.ID)
			val2 = e.evalExpr(ob.Expr, row2.Data, colIndex, row2.ID)
		}

		cmp := val1.Compare(val2)
		if cmp != 0 {
			if ob.Direction == "DESC" {
				return cmp < 0
			}
			return cmp > 0
		}
	}
	return false
}

// distinctRows removes duplicate rows
func (e *Engine) distinctRows(rows []*Row) []*Row {
	seen := make(map[string]bool)
	var result []*Row

	for _, row := range rows {
		key := ""
		for _, val := range row.Data {
			key += val.ToString() + "|"
		}
		if !seen[key] {
			seen[key] = true
			result = append(result, row)
		}
	}

	return result
}

// hasAggregateFunctions checks if any column contains an aggregate function
func (e *Engine) hasAggregateFunctions(columns []parser.SelectColumn) bool {
	for _, col := range columns {
		if col.Expr != nil && col.Expr.Type == parser.ExprFunction {
			if function.IsAggregate(col.Expr.FuncName) {
				return true
			}
		}
	}
	return false
}

// computeAggregates computes aggregate functions and returns a single row
func (e *Engine) computeAggregates(rows []*Row, columns []parser.SelectColumn, colIndex map[string]int) []*Row {
	if len(rows) == 0 {
		// Return single row with nulls/zeros for aggregates
		data := make([]types.Value, len(columns))
		for i, col := range columns {
			if col.Expr != nil && col.Expr.Type == parser.ExprFunction {
				if function.IsAggregate(col.Expr.FuncName) {
					if strings.ToUpper(col.Expr.FuncName) == "COUNT" {
						data[i] = types.NewIntValue(0)
					} else {
						data[i] = types.NewNullValue()
					}
				}
			}
		}
		return []*Row{{Data: data}}
	}

	// Collect values for aggregate columns and compute
	data := make([]types.Value, len(columns))
	for i, col := range columns {
		if col.Expr != nil && col.Expr.Type == parser.ExprFunction {
					funcName := strings.ToUpper(col.Expr.FuncName)
					if function.IsAggregate(funcName) {
						// Collect all values for this column
						var values []types.Value
						for _, row := range rows {
							// Check if it's COUNT(*) case (single arg with ExprStar type)
							if len(col.Expr.Args) > 0 && col.Expr.Args[0].Type != parser.ExprStar {
								// Get the column/value from args
								val := e.evalExpr(col.Expr.Args[0], row.Data, colIndex, row.ID)
								values = append(values, val)
							} else {
								// COUNT(*) case or no args - count each row as 1
								values = append(values, types.NewIntValue(1))
							}
						}
				// Call the aggregate function
				result, err := function.Call(funcName, values)
				if err != nil {
					data[i] = types.NewNullValue()
				} else {
					data[i] = result
				}
			} else {
				// Non-aggregate function, evaluate with first row
				data[i] = e.evalExpr(col.Expr, rows[0].Data, colIndex, rows[0].ID)
			}
		} else if col.Expr != nil {
			// Non-function expression
			data[i] = e.evalExpr(col.Expr, rows[0].Data, colIndex, rows[0].ID)
		}
	}

	return []*Row{{Data: data}}
}

// evalExpr evaluates an expression
func (e *Engine) evalExpr(expr *parser.Expression, row []types.Value, colIndex map[string]int, rowID uint64) types.Value {
	if expr == nil {
		return types.NewNullValue()
	}

	switch expr.Type {
	case parser.ExprLiteral:
		return types.NewValue(expr.Literal)

	case parser.ExprColumn:
		// Check if it's a system variable (@@xxx)
		if strings.HasPrefix(expr.Column, "@@") {
			return e.evalSystemVariable(expr.Column)
		}
		idx := colIndex[strings.ToLower(expr.Column)]
		if idx < len(row) {
			return row[idx]
		}
		return types.NewNullValue()

	case parser.ExprFunction:
		return e.evalFunction(expr.FuncName, expr.Args, row, colIndex, rowID)

	case parser.ExprBinaryOp:
		return e.evalBinaryOp(expr, row, colIndex, rowID)

	case parser.ExprUnaryOp:
		return e.evalUnaryOp(expr, row, colIndex, rowID)

	case parser.ExprCase:
		return e.evalCase(expr, row, colIndex, rowID)

	case parser.ExprIn:
		return e.evalIn(expr, row, colIndex, rowID)

	case parser.ExprBetween:
		return e.evalBetween(expr, row, colIndex, rowID)

	case parser.ExprMatch:
		return e.evalMatchAgainst(expr, row, colIndex, rowID)

	default:
		return types.NewNullValue()
	}
}

// evalBool evaluates an expression as boolean
func (e *Engine) evalBool(expr *parser.Expression, row []types.Value, colIndex map[string]int, rowID uint64) bool {
	val := e.evalExpr(expr, row, colIndex, rowID)
	return val.ToBool()
}

// evalInt evaluates an expression as integer
func (e *Engine) evalInt(expr *parser.Expression) int64 {
	val := e.evalExpr(expr, nil, nil, 0)
	n, _ := val.ToInt64()
	return n
}

// evalFunction evaluates a function call
func (e *Engine) evalFunction(name string, args []*parser.Expression, row []types.Value, colIndex map[string]int, rowID uint64) types.Value {
	// Evaluate arguments
	values := make([]types.Value, len(args))
	for i, arg := range args {
		values[i] = e.evalExpr(arg, row, colIndex, rowID)
	}

	// Check if it's a script function
	if script.IsScriptFunc(name) {
		return e.evalScriptFunc(name, values)
	}

	// Try built-in function
	result, err := function.Call(name, values)
	if err != nil {
		e.log.Debug("function error: %v", err)
		return types.NewNullValue()
	}

	return result
}

// evalSystemVariable evaluates a MySQL system variable
func (e *Engine) evalSystemVariable(name string) types.Value {
	// Normalize variable name
	name = strings.ToLower(name)
	name = strings.TrimPrefix(name, "@@")
	name = strings.TrimPrefix(name, "session.")
	name = strings.TrimPrefix(name, "global.")

	// Return default values for common MySQL variables
	switch name {
	case "auto_increment_increment":
		return types.NewIntValue(1)
	case "character_set_client", "character_set_connection", "character_set_results", "character_set_server":
		return types.NewStringValue("utf8mb4")
	case "collation_server":
		return types.NewStringValue("utf8mb4_general_ci")
	case "init_connect":
		return types.NewStringValue("")
	case "interactive_timeout":
		return types.NewIntValue(28800)
	case "license":
		return types.NewStringValue("GPL")
	case "lower_case_table_names":
		return types.NewIntValue(0)
	case "max_allowed_packet":
		return types.NewIntValue(4194304)
	case "net_buffer_length":
		return types.NewIntValue(16384)
	case "net_write_timeout":
		return types.NewIntValue(60)
	case "query_cache_size":
		return types.NewIntValue(0)
	case "query_cache_type":
		return types.NewStringValue("OFF")
	case "sql_mode":
		return types.NewStringValue("")
	case "system_time_zone":
		return types.NewStringValue("UTC")
	case "time_zone":
		return types.NewStringValue("SYSTEM")
	case "tx_isolation", "transaction_isolation":
		return types.NewStringValue("REPEATABLE-READ")
	case "wait_timeout":
		return types.NewIntValue(28800)
	case "version":
		return types.NewStringValue("5.7.42")
	case "version_comment":
		return types.NewStringValue("XxLdb")
	default:
		// Return empty string for unknown variables
		return types.NewStringValue("")
	}
}

// evalScriptFunc evaluates a script function
func (e *Engine) evalScriptFunc(name string, args []types.Value) types.Value {
	// First, try to get the script from the manager (cache)
	if _, ok := e.scripts.Get(name); !ok {
		// Script not in cache, try to load from xxscript table
		rows, err := e.storage.GetRows("xxscript")
		if err == nil {
			for _, row := range rows {
				if len(row) >= 2 {
					scriptName := row[0].ToString()
					if strings.ToLower(scriptName) == strings.ToLower(name) {
						scriptExpr := row[1].ToString()
						e.scripts.Register(name, scriptExpr)
						break
					}
				}
			}
		}
	}

	result, err := e.scripts.Execute(name, args)
	if err != nil {
		e.log.Debug("script function error: %v", err)
		return types.NewNullValue()
	}
	return result
}

// evalBinaryOp evaluates a binary operation
func (e *Engine) evalBinaryOp(expr *parser.Expression, row []types.Value, colIndex map[string]int, rowID uint64) types.Value {
	left := e.evalExpr(expr.Left, row, colIndex, rowID)
	right := e.evalExpr(expr.Right, row, colIndex, rowID)

	switch expr.Op {
	case "=":
		return types.NewBoolValue(left.Equals(right))
	case "<>", "!=":
		return types.NewBoolValue(!left.Equals(right))
	case "<":
		return types.NewBoolValue(left.Compare(right) < 0)
	case ">":
		return types.NewBoolValue(left.Compare(right) > 0)
	case "<=":
		return types.NewBoolValue(left.Compare(right) <= 0)
	case ">=":
		return types.NewBoolValue(left.Compare(right) >= 0)
	case "AND":
		return types.NewBoolValue(left.ToBool() && right.ToBool())
	case "OR":
		return types.NewBoolValue(left.ToBool() || right.ToBool())
	case "LIKE":
		return e.evalLike(left, right)
	case "+":
		return e.evalArithmetic(left, right, "+")
	case "-":
		return e.evalArithmetic(left, right, "-")
	case "*":
		return e.evalArithmetic(left, right, "*")
	case "/":
		return e.evalArithmetic(left, right, "/")
	case "||":
		return types.NewStringValue(left.ToString() + right.ToString())
	default:
		return types.NewNullValue()
	}
}

// evalUnaryOp evaluates a unary operation
func (e *Engine) evalUnaryOp(expr *parser.Expression, row []types.Value, colIndex map[string]int, rowID uint64) types.Value {
	right := e.evalExpr(expr.Right, row, colIndex, rowID)

	switch expr.Op {
	case "NOT":
		return types.NewBoolValue(!right.ToBool())
	case "-":
		if n, err := right.ToInt64(); err == nil {
			return types.NewIntValue(-n)
		}
		if f, err := right.ToFloat64(); err == nil {
			return types.NewFloatValue(-f)
		}
	case "+":
		return right
	}

	return types.NewNullValue()
}

// evalCase evaluates a CASE expression
func (e *Engine) evalCase(expr *parser.Expression, row []types.Value, colIndex map[string]int, rowID uint64) types.Value {
	for _, when := range expr.WhenClauses {
		if e.evalBool(when.Cond, row, colIndex, rowID) {
			return e.evalExpr(when.Then, row, colIndex, rowID)
		}
	}

	if expr.ElseExpr != nil {
		return e.evalExpr(expr.ElseExpr, row, colIndex, rowID)
	}

	return types.NewNullValue()
}

// evalLike evaluates a LIKE expression
func (e *Engine) evalLike(left, right types.Value) types.Value {
	pattern := right.ToString()
	str := left.ToString()

	// Convert SQL LIKE pattern to regex
	regex := "^"
	for _, c := range pattern {
		switch c {
		case '%':
			regex += ".*"
		case '_':
			regex += "."
		default:
			regex += string(c)
		}
	}
	regex += "$"

	// Simple implementation without regex
	return types.NewBoolValue(e.matchLike(str, pattern))
}

// matchLike matches a string against a LIKE pattern
// Supports Unicode characters properly - _ matches exactly one character (not one byte)
func (e *Engine) matchLike(str, pattern string) bool {
	// Convert to runes for proper Unicode handling
	strRunes := []rune(str)
	patternRunes := []rune(pattern)

	// Use dynamic programming approach for LIKE matching
	return matchLikeRunes(strRunes, patternRunes, 0, 0)
}

// matchLikeRunes performs LIKE matching on rune slices
func matchLikeRunes(str, pattern []rune, si, pi int) bool {
	for si < len(str) {
		if pi < len(pattern) && (pattern[pi] == '_' || pattern[pi] == str[si]) {
			si++
			pi++
		} else if pi < len(pattern) && pattern[pi] == '%' {
			// Try matching zero or more characters
			for i := si; i <= len(str); i++ {
				if matchLikeRunes(str, pattern, i, pi+1) {
					return true
				}
			}
			return false
		} else {
			return false
		}
	}

	// Skip remaining % in pattern
	for pi < len(pattern) && pattern[pi] == '%' {
		pi++
	}

	return pi == len(pattern)
}

// evalArithmetic evaluates an arithmetic operation
func (e *Engine) evalArithmetic(left, right types.Value, op string) types.Value {
	lf, lerr := left.ToFloat64()
	rf, rerr := right.ToFloat64()

	if lerr != nil || rerr != nil {
		return types.NewNullValue()
	}

	var result float64
	switch op {
	case "+":
		result = lf + rf
	case "-":
		result = lf - rf
	case "*":
		result = lf * rf
	case "/":
		if rf == 0 {
			return types.NewNullValue()
		}
		result = lf / rf
	default:
		return types.NewNullValue()
	}

	// Return int if both operands were ints
	if left.Type == types.TypeInt && right.Type == types.TypeInt {
		return types.NewIntValue(int64(result))
	}
	return types.NewFloatValue(result)
}

// evalIn evaluates an IN expression
func (e *Engine) evalIn(expr *parser.Expression, row []types.Value, colIndex map[string]int, rowID uint64) types.Value {
	// Evaluate the left operand
	left := e.evalExpr(expr.Left, row, colIndex, rowID)

	// Check against each value in the list
	for _, item := range expr.List {
		right := e.evalExpr(item, row, colIndex, rowID)
		if left.Compare(right) == 0 {
			return types.NewBoolValue(true)
		}
	}

	return types.NewBoolValue(false)
}

// evalBetween evaluates a BETWEEN expression
func (e *Engine) evalBetween(expr *parser.Expression, row []types.Value, colIndex map[string]int, rowID uint64) types.Value {
	// Evaluate the value
	val := e.evalExpr(expr.Left, row, colIndex, rowID)

	// Evaluate the bounds
	lower := e.evalExpr(expr.List[0], row, colIndex, rowID)
	upper := e.evalExpr(expr.List[1], row, colIndex, rowID)

	// Check if val is between lower and upper (inclusive)
	if val.Compare(lower) >= 0 && val.Compare(upper) <= 0 {
		return types.NewBoolValue(true)
	}

	return types.NewBoolValue(false)
}

// evalMatchAgainst evaluates a MATCH...AGAINST expression for full-text search
func (e *Engine) evalMatchAgainst(expr *parser.Expression, row []types.Value, colIndex map[string]int, rowID uint64) types.Value {
	// Get the column value
	colName := strings.ToLower(expr.MatchColumn)
	idx, exists := colIndex[colName]
	if !exists || idx >= len(row) {
		return types.NewBoolValue(false)
	}

	content := row[idx].ToString()
	query := expr.MatchQuery
	level := expr.MatchLevel

	// Find the table name from context (stored in expression or inferred)
	tableName := expr.Table
	if tableName == "" {
		// Try to find FTS index by column name alone
		for _, key := range e.fts.ListIndexes() {
			parts := strings.SplitN(key, ".", 2)
			if len(parts) == 2 && strings.EqualFold(parts[1], colName) {
				tableName = parts[0]
				break
			}
		}
	}

	// Check if FTS index exists
	if e.fts.HasIndex(tableName, colName) {
		var results []fts.SearchResult
		var err error
		// Use SearchWithLevel if level is specified
		if level > 0 {
			results, err = e.fts.SearchWithLevel(tableName, colName, query, level, 1000, 0)
		} else {
			results, err = e.fts.Search(tableName, colName, query, 1000, 0)
		}
		if err != nil {
			e.log.Debug("FTS search error: %v", err)
		} else {
			// Build a set of matching row IDs for faster lookup
			matchingIDs := make(map[uint64]bool)
			for _, result := range results {
				if result.Score > 0 {
					matchingIDs[result.RowID] = true
				}
			}
			return types.NewBoolValue(matchingIDs[rowID])
		}
	}

	// No FTS index or search failed, fall back to simple string search
	return types.NewBoolValue(strings.Contains(strings.ToLower(content), strings.ToLower(query)))
}
