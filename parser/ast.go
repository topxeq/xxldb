// Package parser provides AST definitions for xxldb SQL
package parser

import (
	"fmt"
	"strings"
)

// StatementType represents the type of SQL statement
type StatementType int

const (
	StmtUnknown StatementType = iota
	StmtSelect
	StmtInsert
	StmtUpdate
	StmtDelete
	StmtCreateTable
	StmtDropTable
	StmtAlterTable
	StmtCreateIndex
	StmtDropIndex
	StmtSet
	StmtShow
	StmtUse
	StmtBackup
	StmtRestore
	StmtBegin
	StmtCommit
	StmtRollback
)

// String returns the string representation
func (t StatementType) String() string {
	switch t {
	case StmtSelect:
		return "SELECT"
	case StmtInsert:
		return "INSERT"
	case StmtUpdate:
		return "UPDATE"
	case StmtDelete:
		return "DELETE"
	case StmtCreateTable:
		return "CREATE TABLE"
	case StmtDropTable:
		return "DROP TABLE"
	case StmtAlterTable:
		return "ALTER TABLE"
	case StmtCreateIndex:
		return "CREATE INDEX"
	case StmtDropIndex:
		return "DROP INDEX"
	case StmtSet:
		return "SET"
	case StmtShow:
		return "SHOW"
	case StmtUse:
		return "USE"
	case StmtBackup:
		return "BACKUP"
	case StmtRestore:
		return "RESTORE"
	case StmtBegin:
		return "BEGIN"
	case StmtCommit:
		return "COMMIT"
	case StmtRollback:
		return "ROLLBACK"
	default:
		return "UNKNOWN"
	}
}

// Statement represents a SQL statement
type Statement struct {
	Type StatementType

	// SELECT, INSERT, UPDATE, DELETE, CREATE TABLE, DROP TABLE
	Table  string
	Tables []string // For multiple tables (JOIN)

	// SELECT
	Columns    []SelectColumn
	Distinct   bool
	From       *FromClause
	Where      *Expression
	GroupBy    []string
	Having     *Expression
	OrderBy    []OrderByClause
	Limit      *Expression
	Offset     *Expression
	Joins      []JoinClause
	Union      []*Statement
	UnionAll   bool

	// INSERT
	Columns2   []string // Column names for INSERT
	Values     []*Expression
	ValuesList [][]*Expression // For multi-row INSERT
	SelectStmt *Statement      // For INSERT ... SELECT

	// UPDATE
	Updates    map[string]*Expression

	// CREATE TABLE
	IfNotExists bool
	ColumnDefs  []ColumnDef
	Constraints []Constraint

	// ALTER TABLE
	AlterActions []AlterAction

	// CREATE INDEX / DROP INDEX
	IndexName   string
	IndexCols   []string
	IndexUnique bool

	// SET
	SetVar   string
	SetValue *Expression

	// SHOW
	ShowType string // TABLES, COLUMNS, etc.
	ShowTarget string

	// BACKUP / RESTORE
	FilePath string
}

// SelectColumn represents a column in SELECT
type SelectColumn struct {
	Expr      *Expression
	Alias     string
	TableName string // For table.column
	All       bool   // For *
}

// String returns string representation
func (c SelectColumn) String() string {
	if c.All {
		return "*"
	}
	if c.Alias != "" {
		return fmt.Sprintf("%s AS %s", c.Expr, c.Alias)
	}
	return c.Expr.String()
}

// FromClause represents a FROM clause
type FromClause struct {
	Table     string
	Alias     string
	Join      *JoinClause
	Subquery  *Statement
}

// JoinClause represents a JOIN clause
type JoinClause struct {
	Type       string // INNER, LEFT, RIGHT, CROSS
	Table      string
	Alias      string
	On         *Expression
	Using      []string
	Next       *JoinClause // For chain joins
}

// OrderByClause represents an ORDER BY clause
type OrderByClause struct {
	Expr      *Expression
	Direction string // ASC, DESC
}

// ColumnDef represents a column definition
type ColumnDef struct {
	Name       string
	Type       string
	Length     int
	Precision  int
	Scale      int
	Nullable   bool
	Default    *Expression
	AutoInc    bool
	PrimaryKey bool
	Unique     bool
	References string
}

// String returns string representation
func (c ColumnDef) String() string {
	var sb strings.Builder
	sb.WriteString(c.Name)
	sb.WriteString(" ")
	sb.WriteString(c.Type)
	if c.Length > 0 {
		sb.WriteString(fmt.Sprintf("(%d", c.Length))
		if c.Scale > 0 {
			sb.WriteString(fmt.Sprintf(", %d", c.Scale))
		}
		sb.WriteString(")")
	}
	if c.PrimaryKey {
		sb.WriteString(" PRIMARY KEY")
	}
	if c.AutoInc {
		sb.WriteString(" AUTO_INCREMENT")
	}
	if !c.Nullable && !c.PrimaryKey {
		sb.WriteString(" NOT NULL")
	}
	if c.Unique && !c.PrimaryKey {
		sb.WriteString(" UNIQUE")
	}
	if c.Default != nil {
		sb.WriteString(" DEFAULT ")
		sb.WriteString(c.Default.String())
	}
	return sb.String()
}

// Constraint represents a table constraint
type Constraint struct {
	Name       string
	Type       string // PRIMARY KEY, FOREIGN KEY, UNIQUE, CHECK
	Columns    []string
	RefTable   string
	RefColumns []string
	Expr       *Expression
}

// AlterAction represents an ALTER TABLE action
type AlterAction struct {
	Type      string // ADD, DROP, MODIFY, RENAME
	Column    string
	ColumnDef *ColumnDef
	NewName   string
}

// Expression represents a SQL expression
type Expression struct {
	Type ExprType

	// Literal values
	Literal interface{}

	// Column reference
	Column   string
	Table    string
	Database string

	// Function call
	FuncName string
	Args     []*Expression

	// Unary operator
	Op    string
	Right *Expression

	// Binary operator
	Left  *Expression

	// CASE expression
	CaseExpr     *Expression
	WhenClauses  []WhenClause
	ElseExpr     *Expression

	// IN / BETWEEN
	List     []*Expression
	Subquery *Statement

	// EXISTS
	Exists bool
}

// ExprType represents the type of expression
type ExprType int

const (
	ExprUnknown ExprType = iota
	ExprLiteral
	ExprColumn
	ExprFunction
	ExprUnaryOp
	ExprBinaryOp
	ExprCase
	ExprIn
	ExprBetween
	ExprExists
	ExprSubquery
	ExprStar
)

// String returns string representation
func (e *Expression) String() string {
	if e == nil {
		return ""
	}

	switch e.Type {
	case ExprLiteral:
		switch v := e.Literal.(type) {
		case string:
			return fmt.Sprintf("'%s'", v)
		default:
			return fmt.Sprintf("%v", v)
		}
	case ExprColumn:
		if e.Table != "" {
			return fmt.Sprintf("%s.%s", e.Table, e.Column)
		}
		return e.Column
	case ExprFunction:
		args := make([]string, len(e.Args))
		for i, arg := range e.Args {
			args[i] = arg.String()
		}
		return fmt.Sprintf("%s(%s)", e.FuncName, strings.Join(args, ", "))
	case ExprUnaryOp:
		return fmt.Sprintf("%s%s", e.Op, e.Right)
	case ExprBinaryOp:
		return fmt.Sprintf("%s %s %s", e.Left, e.Op, e.Right)
	case ExprCase:
		var sb strings.Builder
		sb.WriteString("CASE")
		if e.CaseExpr != nil {
			sb.WriteString(" ")
			sb.WriteString(e.CaseExpr.String())
		}
		for _, w := range e.WhenClauses {
			sb.WriteString(" WHEN ")
			sb.WriteString(w.Cond.String())
			sb.WriteString(" THEN ")
			sb.WriteString(w.Then.String())
		}
		if e.ElseExpr != nil {
			sb.WriteString(" ELSE ")
			sb.WriteString(e.ElseExpr.String())
		}
		sb.WriteString(" END")
		return sb.String()
	case ExprIn:
		list := make([]string, len(e.List))
		for i, item := range e.List {
			list[i] = item.String()
		}
		return fmt.Sprintf("%s IN (%s)", e.Left, strings.Join(list, ", "))
	case ExprBetween:
		return fmt.Sprintf("%s BETWEEN %s AND %s", e.Left, e.List[0], e.List[1])
	case ExprStar:
		if e.Table != "" {
			return fmt.Sprintf("%s.*", e.Table)
		}
		return "*"
	default:
		return ""
	}
}

// WhenClause represents a WHEN clause in CASE expression
type WhenClause struct {
	Cond *Expression
	Then *Expression
}

// NewLiteralExpr creates a literal expression
func NewLiteralExpr(v interface{}) *Expression {
	return &Expression{Type: ExprLiteral, Literal: v}
}

// NewColumnExpr creates a column expression
func NewColumnExpr(name string) *Expression {
	return &Expression{Type: ExprColumn, Column: name}
}

// NewQualifiedColumnExpr creates a qualified column expression (table.column)
func NewQualifiedColumnExpr(table, column string) *Expression {
	return &Expression{Type: ExprColumn, Table: table, Column: column}
}

// NewFunctionExpr creates a function call expression
func NewFunctionExpr(name string, args ...*Expression) *Expression {
	return &Expression{Type: ExprFunction, FuncName: strings.ToUpper(name), Args: args}
}

// NewBinaryExpr creates a binary expression
func NewBinaryExpr(left *Expression, op string, right *Expression) *Expression {
	return &Expression{Type: ExprBinaryOp, Left: left, Op: op, Right: right}
}

// NewUnaryExpr creates a unary expression
func NewUnaryExpr(op string, right *Expression) *Expression {
	return &Expression{Type: ExprUnaryOp, Op: op, Right: right}
}

// NewStarExpr creates a star expression
func NewStarExpr(table string) *Expression {
	return &Expression{Type: ExprStar, Table: table}
}
