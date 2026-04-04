// Package executor provides SQL execution for xxldb
package executor

import (
	"fmt"
	"strings"
	"sync"

	"github.com/topxeq/xxldb/auth"
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
	mu      sync.RWMutex
	storage *storage.Storage
	auth    *auth.Auth
	log     *logger.Logger
	scripts *script.Manager
	config  Config
}

// Config holds engine configuration
type Config struct {
	Path         string
	InMemory     bool
	LogLevel     string
	Username     string
	Password     string
	AutoCommit   bool
	SyncInterval int
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

	logLevel := logger.INFO
	if config.LogLevel != "" {
		logLevel = logger.ParseLevel(config.LogLevel)
	}

	engine := &Engine{
		storage: store,
		auth:    auth.NewAuth(),
		log:     logger.NewLogger(logLevel, nil),
		scripts: script.NewManager(),
		config:  config,
	}

	// Initialize system tables
	if err := engine.initSystemTables(); err != nil {
		engine.log.Warn("failed to initialize system tables: %v", err)
	}

	// Set credentials if provided
	if config.Username != "" && config.Password != "" {
		engine.auth.SetCredentials(config.Username, config.Password)
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

// Close closes the engine
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.storage.Close()
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

	// Get table info
	tableInfo, err := e.storage.GetTableInfo(stmt.Table)
	if err != nil {
		return nil, err
	}

	// Get all rows (simplified - will be optimized with indexes)
	rows, err := e.storage.GetRows(stmt.Table)
	if err != nil {
		return nil, err
	}

	// Build column index map
	colIndex := make(map[string]int)
	for i, col := range tableInfo.Columns {
		colIndex[strings.ToLower(col.Name)] = i
	}

	// Convert to result rows
	resultRows := make([]*Row, len(rows))
	for i, data := range rows {
		resultRows[i] = &Row{Data: data}
	}

	// Apply WHERE filter
	if stmt.Where != nil {
		resultRows = e.filterRows(resultRows, stmt.Where, colIndex)
	}

	// Handle GROUP BY
	if len(stmt.GroupBy) > 0 {
		resultRows = e.groupRows(resultRows, stmt.GroupBy, colIndex, stmt.Columns)
	}


		// Handle aggregate functions
		if e.hasAggregateFunctions(stmt.Columns) {
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
					data[j] = e.evalExpr(col.Expr, row.Data, colIndex)
				}
			}
			projectedRows[i] = &Row{Data: data}
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
			data[i] = e.evalExpr(col.Expr, nil, nil)
		}
	}

	return &Result{
		Columns: columns,
		Rows:    []*Row{{Data: data}},
	}, nil
}

// filterRows filters rows by WHERE condition
func (e *Engine) filterRows(rows []*Row, where *parser.Expression, colIndex map[string]int) []*Row {
	var result []*Row
	for _, row := range rows {
		if e.evalBool(where, row.Data, colIndex) {
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
			// For aggregate functions, compute results
			result = append(result, groupRows[0])
		}
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
			val1 = e.evalExpr(ob.Expr, row1.Data, colIndex)
			val2 = e.evalExpr(ob.Expr, row2.Data, colIndex)
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
								val := e.evalExpr(col.Expr.Args[0], row.Data, colIndex)
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
				data[i] = e.evalExpr(col.Expr, rows[0].Data, colIndex)
			}
		} else if col.Expr != nil {
			// Non-function expression
			data[i] = e.evalExpr(col.Expr, rows[0].Data, colIndex)
		}
	}

	return []*Row{{Data: data}}
}

// evalExpr evaluates an expression
func (e *Engine) evalExpr(expr *parser.Expression, row []types.Value, colIndex map[string]int) types.Value {
	if expr == nil {
		return types.NewNullValue()
	}

	switch expr.Type {
	case parser.ExprLiteral:
		return types.NewValue(expr.Literal)

	case parser.ExprColumn:
		idx := colIndex[strings.ToLower(expr.Column)]
		if idx < len(row) {
			return row[idx]
		}
		return types.NewNullValue()

	case parser.ExprFunction:
		return e.evalFunction(expr.FuncName, expr.Args, row, colIndex)

	case parser.ExprBinaryOp:
		return e.evalBinaryOp(expr, row, colIndex)

	case parser.ExprUnaryOp:
		return e.evalUnaryOp(expr, row, colIndex)

	case parser.ExprCase:
		return e.evalCase(expr, row, colIndex)

	default:
		return types.NewNullValue()
	}
}

// evalBool evaluates an expression as boolean
func (e *Engine) evalBool(expr *parser.Expression, row []types.Value, colIndex map[string]int) bool {
	val := e.evalExpr(expr, row, colIndex)
	return val.ToBool()
}

// evalInt evaluates an expression as integer
func (e *Engine) evalInt(expr *parser.Expression) int64 {
	val := e.evalExpr(expr, nil, nil)
	n, _ := val.ToInt64()
	return n
}

// evalFunction evaluates a function call
func (e *Engine) evalFunction(name string, args []*parser.Expression, row []types.Value, colIndex map[string]int) types.Value {
	// Evaluate arguments
	values := make([]types.Value, len(args))
	for i, arg := range args {
		values[i] = e.evalExpr(arg, row, colIndex)
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
func (e *Engine) evalBinaryOp(expr *parser.Expression, row []types.Value, colIndex map[string]int) types.Value {
	left := e.evalExpr(expr.Left, row, colIndex)
	right := e.evalExpr(expr.Right, row, colIndex)

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
func (e *Engine) evalUnaryOp(expr *parser.Expression, row []types.Value, colIndex map[string]int) types.Value {
	right := e.evalExpr(expr.Right, row, colIndex)

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
func (e *Engine) evalCase(expr *parser.Expression, row []types.Value, colIndex map[string]int) types.Value {
	for _, when := range expr.WhenClauses {
		if e.evalBool(when.Cond, row, colIndex) {
			return e.evalExpr(when.Then, row, colIndex)
		}
	}

	if expr.ElseExpr != nil {
		return e.evalExpr(expr.ElseExpr, row, colIndex)
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
func (e *Engine) matchLike(str, pattern string) bool {
	// Simplified LIKE matching
	si, pi := 0, 0
	starIdx, matchIdx := -1, 0

	for si < len(str) {
		if pi < len(pattern) && (pattern[pi] == '_' || pattern[pi] == str[si]) {
			si++
			pi++
		} else if pi < len(pattern) && pattern[pi] == '%' {
			starIdx = pi
			matchIdx = si
			pi++
		} else if starIdx != -1 {
			pi = starIdx + 1
			matchIdx++
			si = matchIdx
		} else {
			return false
		}
	}

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
