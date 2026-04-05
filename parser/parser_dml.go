// Package parser provides additional SQL parsing for xxldb
package parser

import (
	"fmt"
	"strings"
)

// parseInsert parses INSERT statement
func (p *Parser) parseInsert() (*Statement, error) {
	stmt := &Statement{Type: StmtInsert}
	stmt.Updates = make(map[string]*Expression)

	p.advance() // Skip INSERT

	// INTO keyword
	if err := p.expect(TokKeyword, "INTO"); err != nil {
		return nil, err
	}

	// Table name
	if !p.match(TokIdent) {
		return nil, fmt.Errorf("expected table name, got %s", p.current())
	}
	stmt.Table = p.parseTableName()

	// Column list
	if p.match(TokLParen) {
		p.advance()
		for {
			if !p.match(TokIdent) {
				return nil, fmt.Errorf("expected column name, got %s", p.current())
			}
			stmt.Columns2 = append(stmt.Columns2, p.advance().Value)

			if !p.match(TokComma) {
				break
			}
			p.advance()
		}
		if err := p.expect(TokRParen, ""); err != nil {
			return nil, err
		}
	}

	// VALUES or SELECT
	if p.matchKeyword("VALUES") {
		p.advance()

		// Single or multiple row values
		for {
			if err := p.expect(TokLParen, ""); err != nil {
				return nil, err
			}

			var row []*Expression
			for {
				expr, err := p.parseExpression()
				if err != nil {
					return nil, err
				}
				row = append(row, expr)

				if !p.match(TokComma) {
					break
				}
				p.advance()
			}

			if err := p.expect(TokRParen, ""); err != nil {
				return nil, err
			}

			stmt.ValuesList = append(stmt.ValuesList, row)

			// Check for more rows
			if !p.match(TokComma) {
				break
			}
			p.advance()
		}

		// Also store in Values for compatibility
		if len(stmt.ValuesList) > 0 {
			stmt.Values = stmt.ValuesList[0]
		}
	} else if p.matchKeyword("SELECT") {
		// INSERT ... SELECT
		selectStmt, err := p.parseSelect()
		if err != nil {
			return nil, err
		}
		stmt.SelectStmt = selectStmt
	} else if p.matchKeyword("DEFAULT") {
		p.advance()
		if err := p.expect(TokKeyword, "VALUES"); err != nil {
			return nil, err
		}
		// DEFAULT VALUES - use default for all columns
	} else {
		return nil, fmt.Errorf("expected VALUES or SELECT, got %s", p.current())
	}

	return stmt, nil
}

// parseUpdate parses UPDATE statement
func (p *Parser) parseUpdate() (*Statement, error) {
	stmt := &Statement{Type: StmtUpdate}
	stmt.Updates = make(map[string]*Expression)

	p.advance() // Skip UPDATE

	// Table name
	if !p.match(TokIdent) {
		return nil, fmt.Errorf("expected table name, got %s", p.current())
	}
	stmt.Table = p.parseTableName()

	// SET clause
	if err := p.expect(TokKeyword, "SET"); err != nil {
		return nil, err
	}

	for {
		// Column name
		if !p.match(TokIdent) {
			return nil, fmt.Errorf("expected column name, got %s", p.current())
		}
		colName := p.advance().Value

		// =
		if err := p.expect(TokOperator, "="); err != nil {
			return nil, err
		}

		// Value expression
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		stmt.Updates[colName] = expr

		if !p.match(TokComma) {
			break
		}
		p.advance()
	}

	// WHERE clause
	if p.matchKeyword("WHERE") {
		p.advance()
		where, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		stmt.Where = where
	}

	// LIMIT clause (for UPDATE)
	if p.matchKeyword("LIMIT") {
		p.advance()
		limit, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		stmt.Limit = limit
	}

	return stmt, nil
}

// parseDelete parses DELETE statement
func (p *Parser) parseDelete() (*Statement, error) {
	stmt := &Statement{Type: StmtDelete}

	p.advance() // Skip DELETE

	// FROM clause
	if err := p.expect(TokKeyword, "FROM"); err != nil {
		return nil, err
	}

	// Table name
	if !p.match(TokIdent) {
		return nil, fmt.Errorf("expected table name, got %s", p.current())
	}
	stmt.Table = p.parseTableName()

	// WHERE clause
	if p.matchKeyword("WHERE") {
		p.advance()
		where, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		stmt.Where = where
	}

	// LIMIT clause
	if p.matchKeyword("LIMIT") {
		p.advance()
		limit, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		stmt.Limit = limit
	}

	return stmt, nil
}

// parseCreate parses CREATE statement
func (p *Parser) parseCreate() (*Statement, error) {
	p.advance() // Skip CREATE

	if p.matchKeyword("TABLE") {
		return p.parseCreateTable()
	} else if p.matchKeyword("FULLTEXT") {
		return p.parseCreateFullTextIndex()
	} else if p.matchKeyword("INDEX") || p.matchKeyword("UNIQUE") {
		return p.parseCreateIndex()
	} else if p.matchKeyword("DATABASE") || p.matchKeyword("SCHEMA") {
		return p.parseCreateDatabase()
	}

	return nil, fmt.Errorf("unsupported CREATE statement: %s", p.current().Value)
}

// parseCreateTable parses CREATE TABLE statement
func (p *Parser) parseCreateTable() (*Statement, error) {
	stmt := &Statement{Type: StmtCreateTable}

	p.advance() // Skip TABLE

	// IF NOT EXISTS
	if p.matchKeyword("IF") {
		p.advance()
		if err := p.expect(TokKeyword, "NOT"); err != nil {
			return nil, err
		}
		if err := p.expect(TokKeyword, "EXISTS"); err != nil {
			return nil, err
		}
		stmt.IfNotExists = true
	}

	// Table name
	if !p.match(TokIdent) {
		return nil, fmt.Errorf("expected table name, got %s", p.current())
	}
	stmt.Table = p.parseTableName()

	// Column definitions
	if err := p.expect(TokLParen, ""); err != nil {
		return nil, err
	}

	for {
		if p.match(TokRParen) {
			break
		}

		// Check for table-level constraint
		if p.matchKeyword("PRIMARY", "FOREIGN", "UNIQUE", "CHECK", "CONSTRAINT") {
			constraint, err := p.parseConstraint()
			if err != nil {
				return nil, err
			}
			stmt.Constraints = append(stmt.Constraints, *constraint)
		} else {
			// Column definition
			col, err := p.parseColumnDef()
			if err != nil {
				return nil, err
			}
			stmt.ColumnDefs = append(stmt.ColumnDefs, *col)
		}

		if !p.match(TokComma) {
			break
		}
		p.advance()
	}

	if err := p.expect(TokRParen, ""); err != nil {
		return nil, err
	}

	return stmt, nil
}

// parseColumnDef parses column definition
func (p *Parser) parseColumnDef() (*ColumnDef, error) {
	col := &ColumnDef{Nullable: true}

	// Column name
	if !p.match(TokIdent) {
		return nil, fmt.Errorf("expected column name, got %s", p.current())
	}
	col.Name = p.advance().Value

	// Data type
	if !p.match(TokKeyword) && !p.match(TokIdent) {
		return nil, fmt.Errorf("expected data type, got %s", p.current())
	}
	col.Type = p.advance().Value

	// Type length/precision
	if p.match(TokLParen) {
		p.advance()
		if !p.match(TokNumber) {
			return nil, fmt.Errorf("expected length, got %s", p.current())
		}
		col.Length = parseInt(p.advance().Value)

		if p.match(TokComma) {
			p.advance()
			if !p.match(TokNumber) {
				return nil, fmt.Errorf("expected scale, got %s", p.current())
			}
			col.Scale = parseInt(p.advance().Value)
		}

		if err := p.expect(TokRParen, ""); err != nil {
			return nil, err
		}
	}

	// Column constraints
	for {
		if p.matchKeyword("PRIMARY") {
			p.advance()
			if err := p.expect(TokKeyword, "KEY"); err != nil {
				return nil, err
			}
			col.PrimaryKey = true
			col.Nullable = false
		} else if p.matchKeyword("NOT") {
			p.advance()
			if err := p.expect(TokKeyword, "NULL"); err != nil {
				return nil, err
			}
			col.Nullable = false
		} else if p.matchKeyword("NULL") {
			p.advance()
			col.Nullable = true
		} else if p.matchKeyword("UNIQUE") {
			p.advance()
			col.Unique = true
		} else if p.matchKeyword("DEFAULT") {
			p.advance()
			expr, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			col.Default = expr
		} else if p.matchKeyword("AUTO_INCREMENT", "AUTOINCREMENT") {
			p.advance()
			col.AutoInc = true
		} else if p.matchKeyword("REFERENCES") {
			p.advance()
			if !p.match(TokIdent) {
				return nil, fmt.Errorf("expected table name, got %s", p.current())
			}
			col.References = p.advance().Value
		} else {
			break
		}
	}

	return col, nil
}

// parseConstraint parses table constraint
func (p *Parser) parseConstraint() (*Constraint, error) {
	constraint := &Constraint{}

	// Optional constraint name
	if p.matchKeyword("CONSTRAINT") {
		p.advance()
		if !p.match(TokIdent) {
			return nil, fmt.Errorf("expected constraint name, got %s", p.current())
		}
		constraint.Name = p.advance().Value
	}

	if p.matchKeyword("PRIMARY") {
		p.advance()
		if err := p.expect(TokKeyword, "KEY"); err != nil {
			return nil, err
		}
		constraint.Type = "PRIMARY KEY"

		// Column list
		if err := p.expect(TokLParen, ""); err != nil {
			return nil, err
		}
		for {
			if !p.match(TokIdent) {
				return nil, fmt.Errorf("expected column name, got %s", p.current())
			}
			constraint.Columns = append(constraint.Columns, p.advance().Value)
			if !p.match(TokComma) {
				break
			}
			p.advance()
		}
		if err := p.expect(TokRParen, ""); err != nil {
			return nil, err
		}
	} else if p.matchKeyword("FOREIGN") {
		p.advance()
		if err := p.expect(TokKeyword, "KEY"); err != nil {
			return nil, err
		}
		constraint.Type = "FOREIGN KEY"

		// Column list
		if err := p.expect(TokLParen, ""); err != nil {
			return nil, err
		}
		for {
			if !p.match(TokIdent) {
				return nil, fmt.Errorf("expected column name, got %s", p.current())
			}
			constraint.Columns = append(constraint.Columns, p.advance().Value)
			if !p.match(TokComma) {
				break
			}
			p.advance()
		}
		if err := p.expect(TokRParen, ""); err != nil {
			return nil, err
		}

		// REFERENCES
		if err := p.expect(TokKeyword, "REFERENCES"); err != nil {
			return nil, err
		}
		if !p.match(TokIdent) {
			return nil, fmt.Errorf("expected table name, got %s", p.current())
		}
		constraint.RefTable = p.parseTableName()

		// Referenced columns
		if p.match(TokLParen) {
			p.advance()
			for {
				if !p.match(TokIdent) {
					return nil, fmt.Errorf("expected column name, got %s", p.current())
				}
				constraint.RefColumns = append(constraint.RefColumns, p.advance().Value)
				if !p.match(TokComma) {
					break
				}
				p.advance()
			}
			if err := p.expect(TokRParen, ""); err != nil {
				return nil, err
			}
		}
	} else if p.matchKeyword("UNIQUE") {
		p.advance()
		constraint.Type = "UNIQUE"

		// Column list
		if p.match(TokLParen) {
			p.advance()
			for {
				if !p.match(TokIdent) {
					return nil, fmt.Errorf("expected column name, got %s", p.current())
				}
				constraint.Columns = append(constraint.Columns, p.advance().Value)
				if !p.match(TokComma) {
					break
				}
				p.advance()
			}
			if err := p.expect(TokRParen, ""); err != nil {
				return nil, err
			}
		}
	} else if p.matchKeyword("CHECK") {
		p.advance()
		constraint.Type = "CHECK"

		if err := p.expect(TokLParen, ""); err != nil {
			return nil, err
		}
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		constraint.Expr = expr
		if err := p.expect(TokRParen, ""); err != nil {
			return nil, err
		}
	}

	return constraint, nil
}

// parseCreateIndex parses CREATE INDEX statement
func (p *Parser) parseCreateIndex() (*Statement, error) {
	stmt := &Statement{Type: StmtCreateIndex}

	if p.matchKeyword("UNIQUE") {
		stmt.IndexUnique = true
		p.advance()
	}

	if err := p.expect(TokKeyword, "INDEX"); err != nil {
		return nil, err
	}

	// Index name
	if !p.match(TokIdent) {
		return nil, fmt.Errorf("expected index name, got %s", p.current())
	}
	stmt.IndexName = p.advance().Value

	// ON table
	if err := p.expect(TokKeyword, "ON"); err != nil {
		return nil, err
	}

	if !p.match(TokIdent) {
		return nil, fmt.Errorf("expected table name, got %s", p.current())
	}
	stmt.Table = p.parseTableName()

	// Column list
	if err := p.expect(TokLParen, ""); err != nil {
		return nil, err
	}
	for {
		if !p.match(TokIdent) {
			return nil, fmt.Errorf("expected column name, got %s", p.current())
		}
		stmt.IndexCols = append(stmt.IndexCols, p.advance().Value)
		if !p.match(TokComma) {
			break
		}
		p.advance()
	}
	if err := p.expect(TokRParen, ""); err != nil {
		return nil, err
	}

	return stmt, nil
}

// parseCreateDatabase parses CREATE DATABASE statement
func (p *Parser) parseCreateDatabase() (*Statement, error) {
	p.advance() // Skip DATABASE or SCHEMA

	stmt := &Statement{Type: StmtCreateDatabase}

	// Check for IF NOT EXISTS
	if p.matchKeyword("IF") {
		p.advance() // Skip IF
		if err := p.expect(TokKeyword, "NOT"); err != nil {
			return nil, err
		}
		if err := p.expect(TokKeyword, "EXISTS"); err != nil {
			return nil, err
		}
		stmt.IfNotExists = true
	}

	// Database name
	if !p.match(TokIdent) {
		return nil, fmt.Errorf("expected database name, got %s", p.current())
	}
	stmt.Table = p.parseTableName()

	return stmt, nil
}

// parseCreateFullTextIndex parses CREATE FULLTEXT INDEX statement
func (p *Parser) parseCreateFullTextIndex() (*Statement, error) {
	stmt := &Statement{
		Type:         StmtCreateIndex,
		IndexFullText: true,
	}

	p.advance() // Skip FULLTEXT

	if err := p.expect(TokKeyword, "INDEX"); err != nil {
		return nil, err
	}

	// Index name (optional)
	if p.match(TokIdent) {
		stmt.IndexName = p.advance().Value
	}

	// ON table
	if err := p.expect(TokKeyword, "ON"); err != nil {
		return nil, err
	}

	if !p.match(TokIdent) {
		return nil, fmt.Errorf("expected table name, got %s", p.current())
	}
	stmt.Table = p.parseTableName()

	// Column list
	if err := p.expect(TokLParen, ""); err != nil {
		return nil, err
	}

	if !p.match(TokIdent) {
		return nil, fmt.Errorf("expected column name, got %s", p.current())
	}
	stmt.IndexCols = append(stmt.IndexCols, p.advance().Value)

	if err := p.expect(TokRParen, ""); err != nil {
		return nil, err
	}

	return stmt, nil
}

// parseDrop parses DROP statement
func (p *Parser) parseDrop() (*Statement, error) {
	p.advance() // Skip DROP

	if p.matchKeyword("TABLE") {
		return p.parseDropTable()
	} else if p.matchKeyword("INDEX") {
		return p.parseDropIndex()
	} else if p.matchKeyword("DATABASE", "SCHEMA") {
		return p.parseDropDatabase()
	}

	return nil, fmt.Errorf("unsupported DROP statement: %s", p.current().Value)
}

// parseDropTable parses DROP TABLE statement
func (p *Parser) parseDropTable() (*Statement, error) {
	stmt := &Statement{Type: StmtDropTable}

	p.advance() // Skip TABLE

	// IF EXISTS
	if p.matchKeyword("IF") {
		p.advance()
		if err := p.expect(TokKeyword, "EXISTS"); err != nil {
			return nil, err
		}
		stmt.IfExists = true
	}

	// Table name
	if !p.match(TokIdent) {
		return nil, fmt.Errorf("expected table name, got %s", p.current())
	}
	stmt.Table = p.parseTableName()

	return stmt, nil
}

// parseDropIndex parses DROP INDEX statement
func (p *Parser) parseDropIndex() (*Statement, error) {
	stmt := &Statement{Type: StmtDropIndex}

	p.advance() // Skip INDEX

	// Index name
	if !p.match(TokIdent) {
		return nil, fmt.Errorf("expected index name, got %s", p.current())
	}
	stmt.IndexName = p.advance().Value

	// ON table (optional)
	if p.matchKeyword("ON") {
		p.advance()
		if !p.match(TokIdent) {
			return nil, fmt.Errorf("expected table name, got %s", p.current())
		}
		stmt.Table = p.parseTableName()
	}

	return stmt, nil
}

// parseDropDatabase parses DROP DATABASE statement
func (p *Parser) parseDropDatabase() (*Statement, error) {
	p.advance() // Skip DATABASE or SCHEMA

	stmt := &Statement{Type: StmtDropDatabase}

	// IF EXISTS
	if p.matchKeyword("IF") {
		p.advance()
		if err := p.expect(TokKeyword, "EXISTS"); err != nil {
			return nil, err
		}
		stmt.IfExists = true
	}

	// Database name
	if !p.match(TokIdent) {
		return nil, fmt.Errorf("expected database name, got %s", p.current())
	}
	stmt.Table = p.parseTableName()

	return stmt, nil
}

// parseAlter parses ALTER TABLE statement
func (p *Parser) parseAlter() (*Statement, error) {
	p.advance() // Skip ALTER

	if err := p.expect(TokKeyword, "TABLE"); err != nil {
		return nil, err
	}

	stmt := &Statement{Type: StmtAlterTable}

	// Table name
	if !p.match(TokIdent) {
		return nil, fmt.Errorf("expected table name, got %s", p.current())
	}
	stmt.Table = p.parseTableName()

	// Parse alter actions
	for {
		action := AlterAction{}

		if p.matchKeyword("ADD") {
			action.Type = "ADD"
			p.advance()

			// Column or constraint
			if p.matchKeyword("COLUMN") {
				p.advance()
				col, err := p.parseColumnDef()
				if err != nil {
					return nil, err
				}
				action.ColumnDef = col
			} else if p.matchKeyword("PRIMARY", "FOREIGN", "UNIQUE", "CHECK", "CONSTRAINT") {
				constraint, err := p.parseConstraint()
				if err != nil {
					return nil, err
				}
				_ = constraint // Store in action
			} else {
				col, err := p.parseColumnDef()
				if err != nil {
					return nil, err
				}
				action.ColumnDef = col
			}
		} else if p.matchKeyword("DROP") {
			action.Type = "DROP"
			p.advance()

			if p.matchKeyword("COLUMN") {
				p.advance()
			}

			if !p.match(TokIdent) {
				return nil, fmt.Errorf("expected column name, got %s", p.current())
			}
			action.Column = p.advance().Value
		} else if p.matchKeyword("MODIFY", "ALTER") {
			action.Type = "MODIFY"
			p.advance()

			if p.matchKeyword("COLUMN") {
				p.advance()
			}

			col, err := p.parseColumnDef()
			if err != nil {
				return nil, err
			}
			action.ColumnDef = col
		} else if p.matchKeyword("RENAME") {
			action.Type = "RENAME"
			p.advance()

			if p.matchKeyword("COLUMN") {
				p.advance()
				if !p.match(TokIdent) {
					return nil, fmt.Errorf("expected column name, got %s", p.current())
				}
				action.Column = p.advance().Value

				if err := p.expect(TokKeyword, "TO"); err != nil {
					return nil, err
				}

				if !p.match(TokIdent) {
					return nil, fmt.Errorf("expected new column name, got %s", p.current())
				}
				action.NewName = p.advance().Value
			} else if p.matchKeyword("TO") {
				p.advance()
				if !p.match(TokIdent) {
					return nil, fmt.Errorf("expected new table name, got %s", p.current())
				}
				action.NewName = p.advance().Value
			}
		} else {
			break
		}

		stmt.AlterActions = append(stmt.AlterActions, action)

		if !p.match(TokComma) {
			break
		}
		p.advance()
	}

	return stmt, nil
}

// parseInt parses an integer from string
func parseInt(s string) int {
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}

// parseSet parses SET statement
func (p *Parser) parseSet() (*Statement, error) {
	stmt := &Statement{Type: StmtSet}

	p.advance() // Skip SET

	// Variable name
	if !p.match(TokIdent) && !p.match(TokKeyword) {
		return nil, fmt.Errorf("expected variable name, got %s", p.current())
	}
	stmt.SetVar = p.advance().Value

	// Handle special cases like LOG LEVEL
	if strings.ToUpper(stmt.SetVar) == "LOG" && p.match(TokKeyword, "LEVEL") {
		p.advance()
		stmt.SetVar = "LOG_LEVEL"
	}

	// =
	if p.match(TokOperator, "=") {
		p.advance()
	}

	// Value
	expr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	stmt.SetValue = expr

	return stmt, nil
}

// parseShow parses SHOW statement
func (p *Parser) parseShow() (*Statement, error) {
	stmt := &Statement{Type: StmtShow}

	p.advance() // Skip SHOW

	// Skip optional GLOBAL / SESSION modifier
	if p.matchKeyword("GLOBAL") || p.matchKeyword("SESSION") {
		p.advance()
	}

	// Check for optional FULL keyword
	isFull := false
	if p.matchKeyword("FULL") {
		isFull = true
		p.advance()
	}

	// What to show
	if p.matchKeyword("TABLES") {
		stmt.ShowType = "TABLES"
		if isFull {
			stmt.ShowType = "FULL TABLES"
		}
		p.advance()
	} else if p.matchKeyword("DATABASES") {
		stmt.ShowType = "DATABASES"
		p.advance()
	} else if p.matchKeyword("COLUMNS") {
		stmt.ShowType = "COLUMNS"
		if isFull {
			stmt.ShowType = "FULL COLUMNS"
		}
		p.advance()
		if p.matchKeyword("FROM") {
			p.advance()
			if !p.match(TokIdent) {
				return nil, fmt.Errorf("expected table name, got %s", p.current())
			}
			stmt.ShowTarget = p.advance().Value
		}
	} else if p.matchKeyword("CREATE") {
		stmt.ShowType = "CREATE"
		p.advance()
		if p.matchKeyword("TABLE") {
			p.advance()
			if !p.match(TokIdent) {
				return nil, fmt.Errorf("expected table name, got %s", p.current())
			}
			// Support db.table format: parse both parts, use only the table name
			stmt.ShowTarget = p.parseTableName()
		}
	} else if p.matchKeyword("VARIABLES") {
		stmt.ShowType = "VARIABLES"
		p.advance()
	} else if p.matchKeyword("STATUS") {
		stmt.ShowType = "STATUS"
		p.advance()
	} else if p.matchKeyword("WARNINGS") {
		stmt.ShowType = "WARNINGS"
		p.advance()
	} else if p.matchKeyword("GRANTS") {
		stmt.ShowType = "GRANTS"
		p.advance()
	} else if p.matchKeyword("INDEX") || p.matchKeyword("INDEXES") || p.matchKeyword("KEYS") {
		stmt.ShowType = "INDEX"
		p.advance()
		if p.matchKeyword("FROM") {
			p.advance()
			if !p.match(TokIdent) {
				return nil, fmt.Errorf("expected table name, got %s", p.current())
			}
			stmt.ShowTarget = p.advance().Value
		}
	} else if p.matchKeyword("PROCESSLIST") {
		stmt.ShowType = "PROCESSLIST"
		p.advance()
	} else if p.matchKeyword("ENGINES") || p.matchKeyword("ENGINE") {
		stmt.ShowType = "ENGINES"
		p.advance()
	} else if p.matchKeyword("OPEN") {
		// SHOW OPEN TABLES [FROM db] [WHERE ...]
		stmt.ShowType = "OPEN TABLES"
		for p.current().Type != TokEOF && p.current().Type != TokSemicolon {
			p.advance()
		}
	} else if p.matchKeyword("COLLATION") {
		stmt.ShowType = "COLLATION"
		for p.current().Type != TokEOF && p.current().Type != TokSemicolon {
			p.advance()
		}
	} else if p.matchKeyword("FUNCTION") {
		p.advance()
		if p.matchKeyword("STATUS") {
			p.advance()
		}
		stmt.ShowType = "FUNCTION STATUS"
		// skip optional WHERE clause
		for p.current().Type != TokEOF && p.current().Type != TokSemicolon {
			p.advance()
		}
	} else if p.matchKeyword("PROCEDURE") {
		p.advance()
		if p.matchKeyword("STATUS") {
			p.advance()
		}
		stmt.ShowType = "PROCEDURE STATUS"
		for p.current().Type != TokEOF && p.current().Type != TokSemicolon {
			p.advance()
		}
	} else if p.matchKeyword("TRIGGERS") {
		p.advance()
		stmt.ShowType = "TRIGGERS"
		if p.matchKeyword("FROM") || p.matchKeyword("IN") {
			p.advance()
			if p.match(TokIdent) {
				stmt.ShowTarget = p.advance().Value
			}
		}
		for p.current().Type != TokEOF && p.current().Type != TokSemicolon {
			p.advance()
		}
	} else if p.matchKeyword("EVENTS") {
		p.advance()
		stmt.ShowType = "EVENTS"
		for p.current().Type != TokEOF && p.current().Type != TokSemicolon {
			p.advance()
		}
	} else if p.matchKeyword("TABLE") {
		p.advance()
		if p.matchKeyword("STATUS") {
			stmt.ShowType = "TABLE STATUS"
			p.advance()
			// Handle FROM database clause
			if p.matchKeyword("FROM") {
				p.advance()
				if p.match(TokIdent) {
					stmt.ShowTarget = p.advance().Value
				}
			}
			// Handle LIKE clause
			if p.matchKeyword("LIKE") {
				p.advance()
				if p.match(TokString) {
					stmt.ShowPattern = p.advance().Value
				}
			}
		}
	} else {
		return nil, fmt.Errorf("unsupported SHOW statement: %s", p.current().Value)
	}

	return stmt, nil
}

// parseUse parses USE statement
func (p *Parser) parseUse() (*Statement, error) {
	stmt := &Statement{Type: StmtUse}

	p.advance() // Skip USE

	// Database name
	if !p.match(TokIdent) {
		return nil, fmt.Errorf("expected database name, got %s", p.current())
	}
	stmt.Table = p.parseTableName() // Reuse Table field

	return stmt, nil
}

// parseDescribe parses DESCRIBE/DESC statement
func (p *Parser) parseDescribe() (*Statement, error) {
	stmt := &Statement{Type: StmtDescribe}

	p.advance() // Skip DESCRIBE or DESC

	// Table name
	if !p.match(TokIdent) {
		return nil, fmt.Errorf("expected table name, got %s", p.current())
	}
	stmt.Table = p.parseTableName()

	return stmt, nil
}

// parseBackup parses BACKUP statement
func (p *Parser) parseBackup() (*Statement, error) {
	stmt := &Statement{Type: StmtBackup}

	p.advance() // Skip BACKUP

	// TO file path
	if p.matchKeyword("TO") {
		p.advance()
	}

	if p.match(TokString) {
		stmt.FilePath = p.advance().Value
	} else if p.match(TokIdent) {
		stmt.FilePath = p.advance().Value
	} else {
		return nil, fmt.Errorf("expected file path, got %s", p.current())
	}

	return stmt, nil
}

// parseRestore parses RESTORE statement
func (p *Parser) parseRestore() (*Statement, error) {
	stmt := &Statement{Type: StmtRestore}

	p.advance() // Skip RESTORE

	// FROM file path
	if p.matchKeyword("FROM") {
		p.advance()
	}

	if p.match(TokString) {
		stmt.FilePath = p.advance().Value
	} else if p.match(TokIdent) {
		stmt.FilePath = p.advance().Value
	} else {
		return nil, fmt.Errorf("expected file path, got %s", p.current())
	}

	return stmt, nil
}

// parseBegin parses BEGIN statement
func (p *Parser) parseBegin() (*Statement, error) {
	stmt := &Statement{Type: StmtBegin}
	p.advance() // Skip BEGIN

	if p.matchKeyword("TRANSACTION") {
		p.advance()
	}

	return stmt, nil
}

// parseStartTransaction parses START TRANSACTION statement
func (p *Parser) parseStartTransaction() (*Statement, error) {
	stmt := &Statement{Type: StmtBegin}
	p.advance() // Skip START

	if err := p.expect(TokKeyword, "TRANSACTION"); err != nil {
		return nil, err
	}

	return stmt, nil
}

// parseCommit parses COMMIT statement
func (p *Parser) parseCommit() (*Statement, error) {
	stmt := &Statement{Type: StmtCommit}
	p.advance() // Skip COMMIT

	if p.matchKeyword("TRANSACTION") {
		p.advance()
	}

	return stmt, nil
}

// parseRollback parses ROLLBACK statement
func (p *Parser) parseRollback() (*Statement, error) {
	stmt := &Statement{Type: StmtRollback}
	p.advance() // Skip ROLLBACK

	if p.matchKeyword("TRANSACTION") {
		p.advance()
	}

	return stmt, nil
}
