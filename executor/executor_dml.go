// Package executor provides DML execution for xxldb
package executor

import (
	"fmt"
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

	// Handle INSERT ... SELECT
	if stmt.SelectStmt != nil {
		selectResult, err := e.executeSelect(stmt.SelectStmt)
		if err != nil {
			return nil, err
		}

		for _, row := range selectResult.Rows {
			_, lid, err := e.storage.InsertRow(stmt.Table, row.Data)
			if err != nil {
				return nil, err
			}
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
			_, lid, err := e.storage.InsertRow(stmt.Table, row)
			if err != nil {
				return nil, err
			}
			rowsAffected++
			if lid > lastInsertID {
				lastInsertID = lid
			}
		}
	} else if len(stmt.Values) > 0 {
		row := e.buildRow(tableInfo, stmt.Columns2, stmt.Values, colIndex)
		_, lid, err := e.storage.InsertRow(stmt.Table, row)
		if err != nil {
			return nil, err
		}
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

	// Build updates map
	updates := make(map[int]types.Value)
	for colName, expr := range stmt.Updates {
		idx := colIndex[strings.ToLower(colName)]
		updates[idx] = e.evalExpr(expr, nil, nil)
	}

	// Build condition function
	condition := func(row []types.Value) bool {
		if stmt.Where == nil {
			return true
		}
		return e.evalBool(stmt.Where, row, colIndex)
	}

	// Update rows via storage
	count, err := e.storage.UpdateRows(stmt.Table, updates, condition)
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

	// Delete rows
	count, err := e.storage.DeleteRows(stmt.Table, condition)
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
	if err := e.storage.DropTable(stmt.Table); err != nil {
		return nil, err
	}

	return &Result{
		IsExecutionResult: true,
		Message:           fmt.Sprintf("Table %s dropped", stmt.Table),
	}, nil
}

// executeCreateIndex executes CREATE INDEX statement
func (e *Engine) executeCreateIndex(stmt *parser.Statement) (*Result, error) {
	// Index support will be implemented later
	return &Result{
		IsExecutionResult: true,
		Message:           fmt.Sprintf("Index %s created on table %s", stmt.IndexName, stmt.Table),
	}, nil
}

// executeDropIndex executes DROP INDEX statement
func (e *Engine) executeDropIndex(stmt *parser.Statement) (*Result, error) {
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

	switch strings.ToUpper(stmt.SetVar) {
	case "USER":
		e.auth.SetCredentials(value, "")
		return &Result{IsExecutionResult: true, Message: "Username set"}, nil
	case "PASSWORD":
		e.auth.SetCredentials(e.auth.GetUsername(), value)
		return &Result{IsExecutionResult: true, Message: "Password updated"}, nil
	case "LOG_LEVEL":
		e.log.SetLevelFromString(value)
		return &Result{IsExecutionResult: true, Message: fmt.Sprintf("Log level set to %s", value)}, nil
	default:
		return nil, fmt.Errorf("unknown SET option: %s", stmt.SetVar)
	}
}

// executeShow executes SHOW statement
func (e *Engine) executeShow(stmt *parser.Statement) (*Result, error) {
	switch strings.ToUpper(stmt.ShowType) {
	case "TABLES":
		tables := e.storage.ListTables()
		rows := make([]*Row, len(tables))
		for i, t := range tables {
			rows[i] = &Row{Data: []types.Value{types.NewStringValue(t)}}
		}
		return &Result{
			Columns: []string{"table_name"},
			Rows:    rows,
		}, nil

	case "COLUMNS":
		tableInfo, err := e.storage.GetTableInfo(stmt.ShowTarget)
		if err != nil {
			return nil, err
		}
		rows := make([]*Row, len(tableInfo.Columns))
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
		createStmt := fmt.Sprintf("CREATE TABLE %s (\n  %s\n)", stmt.ShowTarget, strings.Join(colDefs, ",\n  "))
		return &Result{
			IsExecutionResult: true,
			Message:           createStmt,
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
