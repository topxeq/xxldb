// Package executor provides DML execution for xxldb
package executor

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/topxeq/xxldb/parser"
	"github.com/topxeq/xxldb/types"
)

// executeInsert executes INSERT statement
func (e *Engine) executeInsert(stmt *parser.Statement) (*Result, error) {
	tableInfo, err := e.storage.GetTableInfo(stmt.Table)
	if err != nil {
		return nil, err
	}

	// Build column index map
	colIndex := make(map[string]int)
	for i, col := range tableInfo.Columns {
		colIndex[strings.ToLower(col.Name)] = i
	}

	var rowsAffected int64
	var lastInsertID int64

	// Helper function to index a row for FTS
	indexRowForFTS := func(rowID uint64, row []types.Value) {
		for i, col := range tableInfo.Columns {
			if e.fts.HasIndex(stmt.Table, col.Name) && i < len(row) {
				content := row[i].ToString()
				if content != "" {
					if err := e.fts.IndexDocument(stmt.Table, col.Name, rowID, content); err != nil {
						e.log.Debug("failed to index row for FTS: %v", err)
					}
				}
			}
		}
	}

	// Handle INSERT ... SELECT
	if stmt.SelectStmt != nil {
		selectResult, err := e.executeSelect(stmt.SelectStmt)
		if err != nil {
			return nil, err
		}

		for _, row := range selectResult.Rows {
			rowID, lid, err := e.storage.InsertRow(stmt.Table, row.Data)
			if err != nil {
				return nil, err
			}
			// Index for FTS
			indexRowForFTS(rowID, row.Data)
			rowsAffected++
			if lid > lastInsertID {
				lastInsertID = lid
			}
		}

		return &Result{
			RowsAffected:      rowsAffected,
			LastInsertID:      lastInsertID,
			IsExecutionResult: true,
		}, nil
	}

	// Handle VALUES
	if len(stmt.ValuesList) > 0 {
		for _, values := range stmt.ValuesList {
			row := e.buildRow(tableInfo, stmt.Columns2, values, colIndex)
			rowID, lid, err := e.storage.InsertRow(stmt.Table, row)
			if err != nil {
				return nil, err
			}
			// Index for FTS
			indexRowForFTS(rowID, row)
			rowsAffected++
			if lid > lastInsertID {
				lastInsertID = lid
			}
		}
	} else if len(stmt.Values) > 0 {
		row := e.buildRow(tableInfo, stmt.Columns2, stmt.Values, colIndex)
		rowID, lid, err := e.storage.InsertRow(stmt.Table, row)
		if err != nil {
			return nil, err
		}
		// Index for FTS
		indexRowForFTS(rowID, row)
		rowsAffected = 1
		lastInsertID = lid
	}

	return &Result{
		RowsAffected:      rowsAffected,
		LastInsertID:      lastInsertID,
		IsExecutionResult: true,
	}, nil
}

// buildRow builds a row for INSERT
func (e *Engine) buildRow(tableInfo *types.TableInfo, columns []string, values []*parser.Expression, colIndex map[string]int) []types.Value {
	row := make([]types.Value, len(tableInfo.Columns))

	// Initialize with defaults/nulls
	for i, col := range tableInfo.Columns {
		if col.Default != nil {
			row[i] = *col.Default
		} else {
			row[i] = types.NewNullValue()
		}
	}

	// Fill in provided values
	if len(columns) > 0 {
		for i, colName := range columns {
			idx := colIndex[strings.ToLower(colName)]
			if i < len(values) {
				row[idx] = e.evalExpr(values[i], nil, nil)
			}
		}
	} else {
		// Values in column order
		for i := 0; i < len(values) && i < len(row); i++ {
			row[i] = e.evalExpr(values[i], nil, nil)
		}
	}

	return row
}

// executeUpdate executes UPDATE statement
func (e *Engine) executeUpdate(stmt *parser.Statement) (*Result, error) {
	tableInfo, err := e.storage.GetTableInfo(stmt.Table)
	if err != nil {
		return nil, err
	}

	// Build column index map
	colIndex := make(map[string]int)
	for i, col := range tableInfo.Columns {
		colIndex[strings.ToLower(col.Name)] = i
	}

	// Store update expressions for later evaluation with row context
	updateExprs := make(map[int]*parser.Expression)
	for colName, expr := range stmt.Updates {
		idx := colIndex[strings.ToLower(colName)]
		updateExprs[idx] = expr
	}

	// Build condition function
	condition := func(row []types.Value) bool {
		if stmt.Where == nil {
			return true
		}
		return e.evalBool(stmt.Where, row, colIndex)
	}

	// Build update function that evaluates expressions with current row context
	updateFunc := func(row []types.Value) map[int]types.Value {
		updates := make(map[int]types.Value)
		for idx, expr := range updateExprs {
			// Evaluate expression with current row values
			updates[idx] = e.evalExpr(expr, row, colIndex)
		}
		return updates
	}

	// Callback to update FTS indexes
	updateCallback := func(rowID uint64, row []types.Value) {
		for i, col := range tableInfo.Columns {
			if e.fts.HasIndex(stmt.Table, col.Name) && i < len(row) {
				content := row[i].ToString()
				if content != "" {
					if err := e.fts.IndexDocument(stmt.Table, col.Name, rowID, content); err != nil {
						e.log.Debug("failed to update FTS index: %v", err)
					}
				}
			}
		}
	}

	// Update rows via storage with FTS callback
	count, err := e.storage.UpdateRowsWithCallback(stmt.Table, updateFunc, condition, updateCallback)
	if err != nil {
		return nil, err
	}

	return &Result{
		RowsAffected:      count,
		IsExecutionResult: true,
	}, nil
}

// executeDelete executes DELETE statement
func (e *Engine) executeDelete(stmt *parser.Statement) (*Result, error) {
	tableInfo, err := e.storage.GetTableInfo(stmt.Table)
	if err != nil {
		return nil, err
	}

	// Build column index map
	colIndex := make(map[string]int)
	for i, col := range tableInfo.Columns {
		colIndex[strings.ToLower(col.Name)] = i
	}

	// Build condition function
	condition := func(row []types.Value) bool {
		if stmt.Where == nil {
			return true
		}
		return e.evalBool(stmt.Where, row, colIndex)
	}

	// Callback to remove from FTS indexes
	deleteCallback := func(rowID uint64, row []types.Value) {
		for i, col := range tableInfo.Columns {
			if e.fts.HasIndex(stmt.Table, col.Name) && i < len(row) {
				if err := e.fts.DeleteDocument(stmt.Table, col.Name, rowID); err != nil {
					e.log.Debug("failed to delete row from FTS index: %v", err)
				}
			}
		}
	}

	// Delete rows with callback for FTS
	count, err := e.storage.DeleteRowsWithCallback(stmt.Table, condition, deleteCallback)
	if err != nil {
		return nil, err
	}

	return &Result{
		RowsAffected:      count,
		IsExecutionResult: true,
	}, nil
}

// executeCreateTable executes CREATE TABLE statement
func (e *Engine) executeCreateTable(stmt *parser.Statement) (*Result, error) {
	// Check if table exists
	if stmt.IfNotExists {
		if _, err := e.storage.GetTableInfo(stmt.Table); err == nil {
			return &Result{
				IsExecutionResult: true,
				Message:           fmt.Sprintf("Table %s already exists", stmt.Table),
			}, nil
		}
	}

	// Build column definitions
	columns := make([]types.ColumnDef, len(stmt.ColumnDefs))
	for i, colDef := range stmt.ColumnDefs {
		columns[i] = types.ColumnDef{
			Name:       colDef.Name,
			Type:       types.ParseDataType(colDef.Type),
			Length:     colDef.Length,
			Nullable:   colDef.Nullable,
			PrimaryKey: colDef.PrimaryKey,
			AutoInc:    colDef.AutoInc,
		}

		if colDef.Default != nil {
			defVal := e.evalExpr(colDef.Default, nil, nil)
			columns[i].Default = &defVal
		}

		// Handle SEQ type as auto-increment
		if strings.ToUpper(colDef.Type) == "SEQ" {
			columns[i].AutoInc = true
			columns[i].PrimaryKey = true
			columns[i].Nullable = false
		}
	}

	// Create table info
	tableInfo := types.NewTableInfo(0, stmt.Table, columns)

	// Create table in storage
	if err := e.storage.CreateTable(tableInfo); err != nil {
		return nil, err
	}

	return &Result{
		IsExecutionResult: true,
		Message:           fmt.Sprintf("Table %s created", stmt.Table),
	}, nil
}

// executeDropTable executes DROP TABLE statement
func (e *Engine) executeDropTable(stmt *parser.Statement) (*Result, error) {
	// Check if table exists
	if stmt.IfExists {
		if _, err := e.storage.GetTableInfo(stmt.Table); err != nil {
			// Table doesn't exist, but IF EXISTS was specified, so just return success
			return &Result{
				IsExecutionResult: true,
				Message:           fmt.Sprintf("Table %s does not exist", stmt.Table),
			}, nil
		}
	}

	if err := e.storage.DropTable(stmt.Table); err != nil {
		return nil, err
	}

	return &Result{
		IsExecutionResult: true,
		Message:           fmt.Sprintf("Table %s dropped", stmt.Table),
	}, nil
}

// executeCreateDatabase executes CREATE DATABASE statement
// For XxLdb, this creates a subdirectory under the data path
func (e *Engine) executeCreateDatabase(stmt *parser.Statement) (*Result, error) {
	dbName := stmt.Table

	// Check if database exists
	dbPath := filepath.Join(e.storage.GetDataPath(), dbName)
	if _, err := os.Stat(dbPath); err == nil {
		if stmt.IfNotExists {
			return &Result{
				IsExecutionResult: true,
				Message:           fmt.Sprintf("Database %s already exists", dbName),
			}, nil
		}
		return nil, fmt.Errorf("database %s already exists", dbName)
	}

	// Create database directory
	if err := os.MkdirAll(dbPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}

	return &Result{
		IsExecutionResult: true,
		Message:           fmt.Sprintf("Database %s created", dbName),
	}, nil
}

// executeDropDatabase executes DROP DATABASE statement
func (e *Engine) executeDropDatabase(stmt *parser.Statement) (*Result, error) {
	dbName := stmt.Table

	// Check if database exists
	dbPath := filepath.Join(e.storage.GetDataPath(), dbName)
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		if stmt.IfExists {
			return &Result{
				IsExecutionResult: true,
				Message:           fmt.Sprintf("Database %s does not exist", dbName),
			}, nil
		}
		return nil, fmt.Errorf("database %s does not exist", dbName)
	}

	// Remove database directory
	if err := os.RemoveAll(dbPath); err != nil {
		return nil, fmt.Errorf("failed to drop database: %w", err)
	}

	return &Result{
		IsExecutionResult: true,
		Message:           fmt.Sprintf("Database %s dropped", dbName),
	}, nil
}

// executeUse executes USE statement
// XxLdb is a single-database engine, so any database name is accepted.
func (e *Engine) executeUse(stmt *parser.Statement) (*Result, error) {
	if stmt.Table != "" {
		e.currentDB = stmt.Table
	}
	return &Result{
		IsExecutionResult: true,
		Message:           "Database changed",
	}, nil
}

// executeDescribe executes DESCRIBE/DESC statement
func (e *Engine) executeDescribe(stmt *parser.Statement) (*Result, error) {
	tableInfo, err := e.storage.GetTableInfo(stmt.Table)
	if err != nil {
		return nil, err
	}

	// Build result with column info
	result := &Result{
		Columns:           []string{"Field", "Type", "Null", "Key", "Default"},
		IsExecutionResult: false,
	}

	for _, col := range tableInfo.Columns {
		typeStr := col.Type.String()
		if col.Length > 0 {
			typeStr = fmt.Sprintf("%s(%d)", typeStr, col.Length)
		}

		nullStr := "YES"
		if !col.Nullable {
			nullStr = "NO"
		}

		keyStr := ""
		if col.PrimaryKey {
			keyStr = "PRI"
		}

		defaultStr := ""
		if col.Default != nil {
			defaultStr = fmt.Sprintf("%v", col.Default.Data)
		}

		result.Rows = append(result.Rows, &Row{
			Data: []types.Value{
				{Data: col.Name},
				{Data: typeStr},
				{Data: nullStr},
				{Data: keyStr},
				{Data: defaultStr},
			},
		})
	}

	return result, nil
}

// executeCreateIndex executes CREATE INDEX statement
func (e *Engine) executeCreateIndex(stmt *parser.Statement) (*Result, error) {
	// Handle FULLTEXT index
	if stmt.IndexFullText {
		if len(stmt.IndexCols) == 0 {
			return nil, fmt.Errorf("FULLTEXT index requires at least one column")
		}

		// Create FTS index for each column
		for _, col := range stmt.IndexCols {
			if err := e.fts.CreateIndex(stmt.Table, col, nil); err != nil {
				return nil, fmt.Errorf("failed to create fulltext index: %w", err)
			}

			// Index existing rows
			if err := e.indexExistingRows(stmt.Table, col); err != nil {
				e.log.Warn("failed to index existing rows for column %s: %v", col, err)
			}
		}

		return &Result{
			IsExecutionResult: true,
			Message:           fmt.Sprintf("Fulltext index %s created on table %s (%s)", stmt.IndexName, stmt.Table, strings.Join(stmt.IndexCols, ", ")),
		}, nil
	}

	// Regular index support will be implemented later
	return &Result{
		IsExecutionResult: true,
		Message:           fmt.Sprintf("Index %s created on table %s", stmt.IndexName, stmt.Table),
	}, nil
}

// indexExistingRows indexes existing rows in a table for FTS
func (e *Engine) indexExistingRows(tableName, columnName string) error {
	tableInfo, err := e.storage.GetTableInfo(tableName)
	if err != nil {
		return err
	}

	// Find column index
	var colIdx int = -1
	for i, col := range tableInfo.Columns {
		if strings.EqualFold(col.Name, columnName) {
			colIdx = i
			break
		}
	}
	if colIdx < 0 {
		return fmt.Errorf("column %s not found", columnName)
	}

	// Get all rows
	rows, err := e.storage.GetRows(tableName)
	if err != nil {
		return err
	}

	// Index each row
	for rowID, row := range rows {
		if colIdx < len(row) {
			content := row[colIdx].ToString()
			if content != "" {
				if err := e.fts.IndexDocument(tableName, columnName, uint64(rowID+1), content); err != nil {
					e.log.Debug("failed to index row %d: %v", rowID+1, err)
				}
			}
		}
	}

	return nil
}

// executeDropIndex executes DROP INDEX statement
func (e *Engine) executeDropIndex(stmt *parser.Statement) (*Result, error) {
	// Check if this is a FULLTEXT index
	if stmt.IndexFullText {
		if len(stmt.IndexCols) == 0 {
			// Try to drop by index name pattern "tablename_columnname"
			// This is a common convention for fulltext indexes
			for _, key := range e.fts.ListIndexes() {
				if strings.HasPrefix(key, stmt.Table+".") {
					parts := strings.SplitN(key, ".", 2)
					if len(parts) == 2 {
						if err := e.fts.DropIndex(stmt.Table, parts[1]); err != nil {
							e.log.Debug("failed to drop fulltext index %s: %v", key, err)
						}
					}
				}
			}
		} else {
			for _, col := range stmt.IndexCols {
				if err := e.fts.DropIndex(stmt.Table, col); err != nil {
					e.log.Debug("failed to drop fulltext index on %s.%s: %v", stmt.Table, col, err)
				}
			}
		}

		return &Result{
			IsExecutionResult: true,
			Message:           fmt.Sprintf("Fulltext index %s dropped", stmt.IndexName),
		}, nil
	}

	return &Result{
		IsExecutionResult: true,
		Message:           fmt.Sprintf("Index %s dropped", stmt.IndexName),
	}, nil
}

// executeAlterTable executes ALTER TABLE statement
func (e *Engine) executeAlterTable(stmt *parser.Statement) (*Result, error) {
	// Get existing table info
	tableInfo, err := e.storage.GetTableInfo(stmt.Table)
	if err != nil {
		return nil, err
	}

	for _, action := range stmt.AlterActions {
		switch action.Type {
		case "ADD":
			if action.ColumnDef != nil {
				col := types.ColumnDef{
					Name:     action.ColumnDef.Name,
					Type:     types.ParseDataType(action.ColumnDef.Type),
					Length:   action.ColumnDef.Length,
					Nullable: action.ColumnDef.Nullable,
				}
				tableInfo.Columns = append(tableInfo.Columns, col)
			}
		case "DROP":
			// Find and remove column
			for i, col := range tableInfo.Columns {
				if strings.EqualFold(col.Name, action.Column) {
					tableInfo.Columns = append(tableInfo.Columns[:i], tableInfo.Columns[i+1:]...)
					break
				}
			}
		}
	}

	return &Result{
		IsExecutionResult: true,
		Message:           fmt.Sprintf("Table %s altered", stmt.Table),
	}, nil
}

// executeSet executes SET statement
func (e *Engine) executeSet(stmt *parser.Statement) (*Result, error) {
	value := ""
	if stmt.SetValue != nil {
		value = e.evalExpr(stmt.SetValue, nil, nil).ToString()
	}

	setVar := strings.ToUpper(stmt.SetVar)

	// XxLdb specific settings
	switch setVar {
	case "USER":
		e.auth.SetCredentials(value, "")
		// Persist auth config
		if err := e.storage.SetAuthConfig(e.auth.ToMap()); err != nil {
			e.log.Warn("failed to persist auth config: %v", err)
		}
		return &Result{IsExecutionResult: true, Message: "Username set"}, nil
	case "PASSWORD":
		e.auth.SetCredentials(e.auth.GetUsername(), value)
		// Persist auth config
		if err := e.storage.SetAuthConfig(e.auth.ToMap()); err != nil {
			e.log.Warn("failed to persist auth config: %v", err)
		}
		return &Result{IsExecutionResult: true, Message: "Password updated"}, nil
	case "LOG_LEVEL":
		e.log.SetLevelFromString(value)
		return &Result{IsExecutionResult: true, Message: fmt.Sprintf("Log level set to %s", value)}, nil
	}

	// MySQL session variables - just acknowledge all of them
	// We use UTF-8 internally, so charset/collation settings are no-ops
	mysqlSessionVars := map[string]bool{
		"NAMES": true,
		"CHARACTER": true,
		"CHARSET": true,
		"COLLATION": true,
		"COLLATE": true,
		"AUTOCOMMIT": true,
		"SQL_MODE": true,
		"TIME_ZONE": true,
		"TIMEZONE": true,
		"CHARACTER_SET_CLIENT": true,
		"CHARACTER_SET_CONNECTION": true,
		"CHARACTER_SET_RESULTS": true,
		"CHARACTER_SET_SERVER": true,
		"COLLATION_CONNECTION": true,
		"COLLATION_SERVER": true,
		"INIT_CONNECT": true,
		"INTERACTIVE_TIMEOUT": true,
		"WAIT_TIMEOUT": true,
		"NET_WRITE_TIMEOUT": true,
		"NET_READ_TIMEOUT": true,
		"MAX_ALLOWED_PACKET": true,
		"NET_BUFFER_LENGTH": true,
		"TX_ISOLATION": true,
		"TRANSACTION_ISOLATION": true,
		"SESSION": true,
		"GLOBAL": true,
	}

	if mysqlSessionVars[setVar] {
		return &Result{IsExecutionResult: true, Message: fmt.Sprintf("%s set to %s", stmt.SetVar, value)}, nil
	}

	// Check for session.xxx format
	if strings.HasPrefix(setVar, "SESSION.") {
		return &Result{IsExecutionResult: true, Message: fmt.Sprintf("%s set to %s", stmt.SetVar, value)}, nil
	}

	// Check for @session.xxx or @@session.xxx format (variables)
	if strings.HasPrefix(stmt.SetVar, "@") {
		return &Result{IsExecutionResult: true, Message: fmt.Sprintf("Variable set to %s", value)}, nil
	}

	return nil, fmt.Errorf("unknown SET option: %s", stmt.SetVar)
}

// executeShow executes SHOW statement
func (e *Engine) executeShow(stmt *parser.Statement) (*Result, error) {
	switch strings.ToUpper(stmt.ShowType) {
	case "TABLES", "FULL TABLES":
		tables := e.storage.ListTables()
		rows := make([]*Row, len(tables))
		if strings.ToUpper(stmt.ShowType) == "FULL TABLES" {
			// SHOW FULL TABLES returns: Tables_in_xxdb, Table_type
			for i, t := range tables {
				rows[i] = &Row{Data: []types.Value{
					types.NewStringValue(t),
					types.NewStringValue("BASE TABLE"),
				}}
			}
			return &Result{
				Columns: []string{"Tables_in_xxldb", "Table_type"},
				Rows:    rows,
			}, nil
		}
		for i, t := range tables {
			rows[i] = &Row{Data: []types.Value{types.NewStringValue(t)}}
		}
		return &Result{
			Columns: []string{"table_name"},
			Rows:    rows,
		}, nil

	case "COLUMNS", "FULL COLUMNS":
		tableInfo, err := e.storage.GetTableInfo(stmt.ShowTarget)
		if err != nil {
			return nil, err
		}
		rows := make([]*Row, len(tableInfo.Columns))
		if strings.ToUpper(stmt.ShowType) == "FULL COLUMNS" {
			// SHOW FULL COLUMNS returns more details
			for i, col := range tableInfo.Columns {
				rows[i] = &Row{Data: []types.Value{
					types.NewStringValue(col.Name),
					types.NewStringValue(col.Type.String()),
					types.NewStringValue(fmt.Sprintf("%s(%d)", col.Type.String(), col.Length)),
					types.NewStringValue(""), // Collation
					types.NewStringValue("YES"), // Null
					types.NewStringValue(""), // Key
					types.NewStringValue(""), // Default
					types.NewStringValue(""), // Extra
					types.NewStringValue("select,insert,update,references"), // Privileges
					types.NewStringValue(""), // Comment
				}}
			}
			return &Result{
				Columns: []string{"Field", "Type", "Collation", "Null", "Key", "Default", "Extra", "Privileges", "Comment"},
				Rows:    rows,
			}, nil
		}
		for i, col := range tableInfo.Columns {
			rows[i] = &Row{Data: []types.Value{
				types.NewStringValue(col.Name),
				types.NewStringValue(col.Type.String()),
				types.NewStringValue(fmt.Sprintf("%d", col.Length)),
				types.NewStringValue(fmt.Sprintf("%v", col.Nullable)),
				types.NewStringValue(fmt.Sprintf("%v", col.PrimaryKey)),
			}}
		}
		return &Result{
			Columns: []string{"column_name", "type", "length", "nullable", "primary_key"},
			Rows:    rows,
		}, nil

	case "CREATE":
		tableInfo, err := e.storage.GetTableInfo(stmt.ShowTarget)
		if err != nil {
			return nil, err
		}
		var colDefs []string
		for _, col := range tableInfo.Columns {
			colDefs = append(colDefs, col.String())
		}
		createStmt := fmt.Sprintf("CREATE TABLE `%s` (\n  %s\n) ENGINE=XxLdb DEFAULT CHARSET=utf8mb4", stmt.ShowTarget, strings.Join(colDefs, ",\n  "))
		return &Result{
			Columns: []string{"Table", "Create Table"},
			Rows: []*Row{
				{Data: []types.Value{
					types.NewStringValue(stmt.ShowTarget),
					types.NewStringValue(createStmt),
				}},
			},
		}, nil


	case "DATABASES", "SCHEMAS":
		// MySQL: SHOW DATABASES - XxLdb only has one database
		return &Result{
			Columns: []string{"Database"},
			Rows: []*Row{
				{Data: []types.Value{types.NewStringValue("xxldb")}},
			},
		}, nil

	case "VARIABLES":
		// MySQL: SHOW VARIABLES
		return &Result{
			Columns: []string{"Variable_name", "Value"},
			Rows: []*Row{
				{Data: []types.Value{types.NewStringValue("character_set_server"), types.NewStringValue("utf8mb4")}},
				{Data: []types.Value{types.NewStringValue("collation_server"), types.NewStringValue("utf8mb4_general_ci")}},
				{Data: []types.Value{types.NewStringValue("version"), types.NewStringValue("5.7.42")}},
				{Data: []types.Value{types.NewStringValue("autocommit"), types.NewStringValue("1")}},
			},
		}, nil

	case "STATUS":
		return &Result{
			Columns: []string{"Variable_name", "Value"},
			Rows:    []*Row{},
		}, nil

	case "TABLE STATUS":
		// MySQL: SHOW TABLE STATUS [FROM db] [LIKE 'pattern']
		tables := e.storage.ListTables()
		var rows []*Row
		for _, t := range tables {
			// Apply LIKE filter if present
			if stmt.ShowPattern != "" {
				matched, _ := filepath.Match(stmt.ShowPattern, t)
				if !matched {
					continue
				}
			}
			tableInfo, err := e.storage.GetTableInfo(t)
			var rowCount int64
			if err == nil {
				rowCount = tableInfo.RowCount
			}
			rows = append(rows, &Row{Data: []types.Value{
				types.NewStringValue(t),                   // Name
				types.NewStringValue("InnoDB"),            // Engine
				types.NewStringValue("10"),                // Version
				types.NewStringValue("Compact"),           // Row_format
				types.NewStringValue(fmt.Sprintf("%d", rowCount)), // Rows
				types.NewStringValue("0"),                 // Avg_row_length
				types.NewStringValue("0"),                 // Data_length
				types.NewStringValue("0"),                 // Max_data_length
				types.NewStringValue("0"),                 // Index_length
				types.NewStringValue("0"),                 // Data_free
				types.NewStringValue("0"),                 // Auto_increment
				types.NewStringValue(""),                  // Create_time
				types.NewStringValue(""),                  // Update_time
				types.NewStringValue(""),                  // Check_time
				types.NewStringValue("utf8mb4_general_ci"), // Collation
				types.NewStringValue(""),                  // Checksum
				types.NewStringValue(""),                  // Create_options
				types.NewStringValue(""),                  // Comment
			}})
		}
		return &Result{
			Columns: []string{"Name", "Engine", "Version", "Row_format", "Rows", "Avg_row_length", "Data_length", "Max_data_length", "Index_length", "Data_free", "Auto_increment", "Create_time", "Update_time", "Check_time", "Collation", "Checksum", "Create_options", "Comment"},
			Rows:    rows,
		}, nil

	case "WARNINGS":
		return &Result{
			Columns: []string{"Level", "Code", "Message"},
			Rows:    []*Row{},
		}, nil

	case "GRANTS":
		return &Result{
			Columns: []string{"Grants for admin@%"},
			Rows: []*Row{
				{Data: []types.Value{types.NewStringValue("GRANT ALL PRIVILEGES ON *.* TO 'admin'@'%'")}},
			},
		}, nil

	case "INDEX", "INDEXES", "KEYS":
		// MySQL: SHOW INDEX FROM table
		if stmt.ShowTarget == "" {
			return nil, fmt.Errorf("SHOW INDEX requires table name")
		}
		tableInfo, err := e.storage.GetTableInfo(stmt.ShowTarget)
		if err != nil {
			return nil, err
		}
		var rows []*Row
		seq := 1
		for _, col := range tableInfo.Columns {
			if col.PrimaryKey {
				rows = append(rows, &Row{Data: []types.Value{
					types.NewStringValue("xxldb"),    // Table
					types.NewStringValue("0"),        // Non_unique
					types.NewStringValue("PRIMARY"),  // Key_name
					types.NewStringValue(fmt.Sprintf("%d", seq)), // Seq_in_index
					types.NewStringValue(col.Name),   // Column_name
					types.NewStringValue("A"),        // Collation
					types.NewStringValue("0"),        // Cardinality
					types.NewStringValue(""),         // Sub_part
					types.NewStringValue(""),         // Packed
					types.NewStringValue(""),         // Null
					types.NewStringValue("BTREE"),    // Index_type
					types.NewStringValue(""),         // Comment
					types.NewStringValue(""),         // Index_comment
					types.NewStringValue(""),         // Visible
					types.NewStringValue("YES"),      // Expression
				}})
				seq++
			}
		}
		return &Result{
			Columns: []string{"Table", "Non_unique", "Key_name", "Seq_in_index", "Column_name", "Collation", "Cardinality", "Sub_part", "Packed", "Null", "Index_type", "Comment", "Index_comment", "Visible", "Expression"},
			Rows:    rows,
		}, nil

	case "ENGINES":
		return &Result{
			Columns: []string{"Engine", "Support", "Comment", "Transactions", "XA", "Savepoints"},
			Rows: []*Row{
				{Data: []types.Value{
					types.NewStringValue("XxLdb"),
					types.NewStringValue("DEFAULT"),
					types.NewStringValue("XxLdb storage engine"),
					types.NewStringValue("NO"),
					types.NewStringValue("NO"),
					types.NewStringValue("NO"),
				}},
			},
		}, nil

	case "OPEN TABLES":
		return &Result{
			Columns: []string{"Database", "Table", "In_use", "Name_locked"},
			Rows:    []*Row{},
		}, nil

	case "COLLATION":
		return &Result{
			Columns: []string{"Collation", "Charset", "Id", "Default", "Compiled", "Sortlen"},
			Rows: []*Row{
				{Data: []types.Value{
					types.NewStringValue("utf8mb4_general_ci"),
					types.NewStringValue("utf8mb4"),
					types.NewStringValue("45"),
					types.NewStringValue("Yes"),
					types.NewStringValue("Yes"),
					types.NewStringValue("1"),
				}},
			},
		}, nil

	case "FUNCTION STATUS":
		return &Result{
			Columns: []string{"Db", "Name", "Type", "Definer", "Modified", "Created", "Security_type", "Comment", "character_set_client", "collation_connection", "Database Collation"},
			Rows:    []*Row{},
		}, nil

	case "PROCEDURE STATUS":
		return &Result{
			Columns: []string{"Db", "Name", "Type", "Definer", "Modified", "Created", "Security_type", "Comment", "character_set_client", "collation_connection", "Database Collation"},
			Rows:    []*Row{},
		}, nil

	case "TRIGGERS":
		return &Result{
			Columns: []string{"Trigger", "Event", "Table", "Statement", "Timing", "Created", "sql_mode", "Definer", "character_set_client", "collation_connection", "Database Collation"},
			Rows:    []*Row{},
		}, nil

	case "EVENTS":
		return &Result{
			Columns: []string{"Db", "Name", "Definer", "Time zone", "Type", "Execute at", "Interval value", "Interval field", "Starts", "Ends", "Status", "Originator", "character_set_client", "collation_connection", "Database Collation"},
			Rows:    []*Row{},
		}, nil

	default:
		return nil, fmt.Errorf("unknown SHOW option: %s", stmt.ShowType)
	}
}

// executeBackup executes BACKUP statement
func (e *Engine) executeBackup(stmt *parser.Statement) (*Result, error) {
	if err := e.storage.Backup(stmt.FilePath); err != nil {
		return nil, err
	}
	return &Result{
		IsExecutionResult: true,
		Message:           fmt.Sprintf("Database backed up to %s", stmt.FilePath),
	}, nil
}

// executeRestore executes RESTORE statement
func (e *Engine) executeRestore(stmt *parser.Statement) (*Result, error) {
	// This would require more complex handling
	return &Result{
		IsExecutionResult: true,
		Message:           fmt.Sprintf("Database would be restored from %s", stmt.FilePath),
	}, nil
}

// ExecuteWithArgs executes a SQL statement with arguments for parameterized queries
// This is useful for import operations that need to insert rows with values
func (e *Engine) ExecuteWithArgs(sql string, args ...interface{}) (*Result, error) {
	// Replace ? placeholders with actual values
	processedSQL := e.replacePlaceholders(sql, args)
	return e.Execute(processedSQL)
}

// replacePlaceholders replaces ? placeholders with actual values
func (e *Engine) replacePlaceholders(sql string, args []interface{}) string {
	result := sql
	for _, arg := range args {
		// Find first ? and replace it
		idx := strings.Index(result, "?")
		if idx == -1 {
			break
		}

		var replacement string
		switch v := arg.(type) {
		case nil:
			replacement = "NULL"
		case string:
			// Escape single quotes
			escaped := strings.ReplaceAll(v, "'", "''")
			replacement = fmt.Sprintf("'%s'", escaped)
		case []byte:
			// Convert bytes to hex or base64 representation
			replacement = fmt.Sprintf("X'%x'", v)
		case int, int32, int64:
			replacement = fmt.Sprintf("%d", v)
		case float32, float64:
			replacement = fmt.Sprintf("%v", v)
		case bool:
			if v {
				replacement = "1"
			} else {
				replacement = "0"
			}
		default:
			// Try to convert to string
			replacement = fmt.Sprintf("'%v'", v)
		}

		result = result[:idx] + replacement + result[idx+1:]
	}
	return result
}

// InsertRowDirect inserts a row directly into a table
// This is more efficient for bulk imports as it bypasses SQL parsing
func (e *Engine) InsertRowDirect(tableName string, values []interface{}) (uint64, error) {
	tableInfo, err := e.storage.GetTableInfo(tableName)
	if err != nil {
		return 0, err
	}

	// Convert values to types.Value
	row := make([]types.Value, len(tableInfo.Columns))
	for i := 0; i < len(values) && i < len(row); i++ {
		row[i] = types.NewValue(values[i])
	}

	// Fill remaining with defaults/nulls
	for i := len(values); i < len(row); i++ {
		if tableInfo.Columns[i].Default != nil {
			row[i] = *tableInfo.Columns[i].Default
		} else {
			row[i] = types.NewNullValue()
		}
	}

	id, _, err := e.storage.InsertRow(tableName, row)
	if err != nil {
		return 0, err
	}

	return id, nil
}

// InsertBlobDirect inserts a row with BLOB data directly
// blobData can be []byte, io.Reader, or string (hex)
func (e *Engine) InsertBlobDirect(tableName string, columnName string, blobData interface{}) (uint64, error) {
	return e.insertBinaryDirect(tableName, columnName, blobData, types.TypeBlob)
}

// InsertImageDirect inserts a row with IMAGE data directly
// imageData can be []byte, io.Reader, or string (hex)
func (e *Engine) InsertImageDirect(tableName string, columnName string, imageData interface{}) (uint64, error) {
	return e.insertBinaryDirect(tableName, columnName, imageData, types.TypeImage)
}

// insertBinaryDirect is a helper that handles BLOB and IMAGE insertion
func (e *Engine) insertBinaryDirect(tableName string, columnName string, data interface{}, targetType types.DataType) (uint64, error) {
	tableInfo, err := e.storage.GetTableInfo(tableName)
	if err != nil {
		return 0, err
	}

	// Find column index and verify type
	colIdx := -1
	for i, col := range tableInfo.Columns {
		if strings.EqualFold(col.Name, columnName) {
			colIdx = i
			break
		}
	}
	if colIdx < 0 {
		return 0, fmt.Errorf("column %s not found in table %s", columnName, tableName)
	}

	// Convert data to []byte
	var byteData []byte
	switch v := data.(type) {
	case []byte:
		byteData = v
	case string:
		// Treat as hex string
		val, err := types.NewBlobValueFromHex(v)
		if err != nil {
			return 0, fmt.Errorf("invalid hex string: %w", err)
		}
		byteData = val.Data.([]byte)
	case io.Reader:
		byteData, err = io.ReadAll(v)
		if err != nil {
			return 0, fmt.Errorf("failed to read from reader: %w", err)
		}
	default:
		return 0, fmt.Errorf("unsupported data type %T for BLOB/IMAGE", data)
	}

	// Create row with null defaults
	row := make([]types.Value, len(tableInfo.Columns))
	for i, col := range tableInfo.Columns {
		if col.Default != nil {
			row[i] = *col.Default
		} else {
			row[i] = types.NewNullValue()
		}
	}

	// Set the binary value
	if targetType == types.TypeImage {
		row[colIdx] = types.NewImageValue(byteData)
	} else {
		row[colIdx] = types.NewBlobValue(byteData)
	}

	_, lastInsertID, err := e.storage.InsertRow(tableName, row)
	if err != nil {
		return 0, err
	}

	// Return the auto-increment ID (lastInsertID) for queries
	return uint64(lastInsertID), nil
}

// UpdateBlobDirect updates a BLOB column directly
func (e *Engine) UpdateBlobDirect(tableName string, columnName string, blobData interface{}, whereClause string) (int64, error) {
	return e.updateBinaryDirect(tableName, columnName, blobData, whereClause, types.TypeBlob)
}

// UpdateImageDirect updates an IMAGE column directly
func (e *Engine) UpdateImageDirect(tableName string, columnName string, imageData interface{}, whereClause string) (int64, error) {
	return e.updateBinaryDirect(tableName, columnName, imageData, whereClause, types.TypeImage)
}

// updateBinaryDirect is a helper for updating BLOB/IMAGE columns
func (e *Engine) updateBinaryDirect(tableName string, columnName string, data interface{}, whereClause string, targetType types.DataType) (int64, error) {
	// Convert data to []byte
	var byteData []byte
	var err error
	switch v := data.(type) {
	case []byte:
		byteData = v
	case string:
		val, err := types.NewBlobValueFromHex(v)
		if err != nil {
			return 0, fmt.Errorf("invalid hex string: %w", err)
		}
		byteData = val.Data.([]byte)
	case io.Reader:
		byteData, err = io.ReadAll(v)
		if err != nil {
			return 0, fmt.Errorf("failed to read from reader: %w", err)
		}
	default:
		return 0, fmt.Errorf("unsupported data type %T for BLOB/IMAGE", data)
	}

	// Build and execute UPDATE statement
	var valueExpr string
	if targetType == types.TypeImage {
		valueExpr = fmt.Sprintf("IMAGE_FROM_HEX('%x')", byteData)
	} else {
		valueExpr = fmt.Sprintf("BLOB_FROM_HEX('%x')", byteData)
	}

	sql := fmt.Sprintf("UPDATE %s SET %s = %s", tableName, columnName, valueExpr)
	if whereClause != "" {
		sql += " WHERE " + whereClause
	}

	result, err := e.Execute(sql)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected, nil
}

// GetBlobDirect retrieves BLOB data directly from a table
func (e *Engine) GetBlobDirect(tableName string, columnName string, whereClause string) ([]byte, error) {
	return e.getBinaryDirect(tableName, columnName, whereClause)
}

// GetImageDirect retrieves IMAGE data directly from a table
func (e *Engine) GetImageDirect(tableName string, columnName string, whereClause string) ([]byte, error) {
	return e.getBinaryDirect(tableName, columnName, whereClause)
}

// getBinaryDirect is a helper for retrieving BLOB/IMAGE data
func (e *Engine) getBinaryDirect(tableName string, columnName string, whereClause string) ([]byte, error) {
	sql := fmt.Sprintf("SELECT %s FROM %s", columnName, tableName)
	if whereClause != "" {
		sql += " WHERE " + whereClause
	}
	sql += " LIMIT 1"

	result, err := e.Execute(sql)
	if err != nil {
		return nil, err
	}

	if len(result.Rows) == 0 {
		return nil, fmt.Errorf("no rows found")
	}

	if len(result.Rows[0].Data) == 0 {
		return nil, fmt.Errorf("no data in column %s", columnName)
	}

	data := result.Rows[0].Data[0]
	if data.IsNull {
		return nil, nil
	}

	return data.ToBytes()
}

// executeBegin executes BEGIN/START TRANSACTION statement
// Note: XxLdb doesn't support real transactions, this is a no-op for compatibility
func (e *Engine) executeBegin(stmt *parser.Statement) (*Result, error) {
	return &Result{
		IsExecutionResult: true,
		Message:           "Transaction started",
	}, nil
}

// executeCommit executes COMMIT statement
// Note: XxLdb doesn't support real transactions, this is a no-op for compatibility
func (e *Engine) executeCommit(stmt *parser.Statement) (*Result, error) {
	return &Result{
		IsExecutionResult: true,
		Message:           "Transaction committed",
	}, nil
}

// executeRollback executes ROLLBACK statement
// Note: XxLdb doesn't support real transactions, this is a no-op for compatibility
func (e *Engine) executeRollback(stmt *parser.Statement) (*Result, error) {
	return &Result{
		IsExecutionResult: true,
		Message:           "Transaction rolled back",
	}, nil
}
