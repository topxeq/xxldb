// Package parser provides SQL parsing for xxldb
package parser

import (
	"fmt"
	"strconv"
	"strings"
)

// Parser parses SQL statements
type Parser struct {
	tokens []Token
	pos    int
	err    error
}

// NewParser creates a new parser
func NewParser(input string) *Parser {
	return &Parser{
		tokens: Tokenize(input),
		pos:    0,
	}
}

// Parse parses the input and returns a statement
func (p *Parser) Parse() (*Statement, error) {
	if p.err != nil {
		return nil, p.err
	}

	tok := p.current()
	if tok.Type != TokKeyword {
		return nil, fmt.Errorf("expected keyword, got %s", tok)
	}

	switch strings.ToUpper(tok.Value) {
	case "SELECT":
		return p.parseSelect()
	case "INSERT":
		return p.parseInsert()
	case "UPDATE":
		return p.parseUpdate()
	case "DELETE":
		return p.parseDelete()
	case "CREATE":
		return p.parseCreate()
	case "DROP":
		return p.parseDrop()
	case "ALTER":
		return p.parseAlter()
	case "SET":
		return p.parseSet()
	case "SHOW":
		return p.parseShow()
	case "USE":
		return p.parseUse()
	case "DESCRIBE", "DESC":
		return p.parseDescribe()
	case "BACKUP":
		return p.parseBackup()
	case "RESTORE":
		return p.parseRestore()
	case "BEGIN":
		return p.parseBegin()
	case "START":
		return p.parseStartTransaction()
	case "COMMIT":
		return p.parseCommit()
	case "ROLLBACK":
		return p.parseRollback()
	default:
		return nil, fmt.Errorf("unsupported statement: %s", tok.Value)
	}
}

// current returns the current token
func (p *Parser) current() Token {
	if p.pos >= len(p.tokens) {
		return Token{Type: TokEOF}
	}
	return p.tokens[p.pos]
}

// peek returns the token at offset from current position
func (p *Parser) peek(offset int) Token {
	pos := p.pos + offset
	if pos >= len(p.tokens) {
		return Token{Type: TokEOF}
	}
	return p.tokens[pos]
}

// advance moves to the next token
func (p *Parser) advance() Token {
	tok := p.current()
	p.pos++
	return tok
}

// expect expects a specific token type or value
func (p *Parser) expect(typ TokenType, value string) error {
	tok := p.current()
	if tok.Type != typ {
		return fmt.Errorf("expected %v, got %v", typ, tok.Type)
	}
	if value != "" && strings.ToUpper(tok.Value) != strings.ToUpper(value) {
		return fmt.Errorf("expected %s, got %s", value, tok.Value)
	}
	p.advance()
	return nil
}

// match checks if current token matches
func (p *Parser) match(typ TokenType, values ...string) bool {
	tok := p.current()
	if tok.Type != typ {
		return false
	}
	if len(values) == 0 {
		return true
	}
	for _, v := range values {
		if strings.ToUpper(tok.Value) == strings.ToUpper(v) {
			return true
		}
	}
	return false
}

// matchKeyword checks if current token is a keyword
func (p *Parser) matchKeyword(keywords ...string) bool {
	return p.match(TokKeyword, keywords...)
}

// parseTableName parses a table name, handling database.table format
// Returns just the table name (strips database prefix if present)
func (p *Parser) parseTableName() string {
	_, table := p.parseTableNameWithDB()
	return table
}

// parseTableNameWithDB parses a table name, returning both database prefix (if any) and table name
func (p *Parser) parseTableNameWithDB() (database string, table string) {
	// Accept both identifiers and keywords as table names
	if !p.match(TokIdent) && !p.match(TokKeyword) {
		return "", ""
	}
	name := p.advance().Value

	// Check for database.table format
	if p.match(TokDot) {
		p.advance() // Skip the dot
		if p.match(TokIdent) || p.match(TokKeyword) {
			// The second part is the actual table name
			return name, p.advance().Value
		}
	}

	return "", name
}

// parseSelect parses a SELECT statement
func (p *Parser) parseSelect() (*Statement, error) {
	stmt := &Statement{Type: StmtSelect}

	p.advance() // Skip SELECT

	// DISTINCT
	if p.matchKeyword("DISTINCT") {
		stmt.Distinct = true
		p.advance()
	}

	// Columns
	cols, err := p.parseSelectColumns()
	if err != nil {
		return nil, err
	}
	stmt.Columns = cols

	// FROM
	if p.matchKeyword("FROM") {
		p.advance()
		from, err := p.parseFromClause()
		if err != nil {
			return nil, err
		}
		stmt.Table = from.Table
		stmt.From = from
	}

	// JOINs
	for p.matchKeyword("JOIN", "INNER", "LEFT", "RIGHT", "CROSS") {
		join, err := p.parseJoinClause()
		if err != nil {
			return nil, err
		}
		stmt.Joins = append(stmt.Joins, *join)
	}

	// WHERE
	if p.matchKeyword("WHERE") {
		p.advance()
		where, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		stmt.Where = where
	}

	// GROUP BY
	if p.matchKeyword("GROUP") {
		p.advance()
		if err := p.expect(TokKeyword, "BY"); err != nil {
			return nil, err
		}
		groupBy, err := p.parseGroupByList()
		if err != nil {
			return nil, err
		}
		stmt.GroupBy = groupBy
	}

	// HAVING
	if p.matchKeyword("HAVING") {
		p.advance()
		having, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		stmt.Having = having
	}

	// ORDER BY
	if p.matchKeyword("ORDER") {
		p.advance()
		if err := p.expect(TokKeyword, "BY"); err != nil {
			return nil, err
		}
		orderBy, err := p.parseOrderByList()
		if err != nil {
			return nil, err
		}
		stmt.OrderBy = orderBy
	}

	// LIMIT
	if p.matchKeyword("LIMIT") {
		p.advance()
		limit, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		stmt.Limit = limit
	}

	// OFFSET
	if p.matchKeyword("OFFSET") {
		p.advance()
		offset, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		stmt.Offset = offset
	}

	// UNION
	for p.matchKeyword("UNION") {
		p.advance()
		unionAll := false
		if p.matchKeyword("ALL") {
			unionAll = true
			p.advance()
		}
		right, err := p.parseSelect()
		if err != nil {
			return nil, err
		}
		stmt.Union = append(stmt.Union, right)
		stmt.UnionAll = unionAll
	}

	return stmt, nil
}

// parseSelectColumns parses SELECT column list
func (p *Parser) parseSelectColumns() ([]SelectColumn, error) {
	var cols []SelectColumn

	for {
		col, err := p.parseSelectColumn()
		if err != nil {
			return nil, err
		}
		cols = append(cols, *col)

		if !p.match(TokComma) {
			break
		}
		p.advance()
	}

	return cols, nil
}

// parseSelectColumn parses a single SELECT column
func (p *Parser) parseSelectColumn() (*SelectColumn, error) {
	col := &SelectColumn{}

	// Check for *
	if p.match(TokStar) {
		col.All = true
		p.advance()
		return col, nil
	}

	// Check for table.*
	if p.match(TokIdent) && p.peek(1).Type == TokDot && p.peek(2).Type == TokStar {
		tok := p.advance()
		p.advance() // .
		p.advance() // *
		col.TableName = tok.Value
		col.All = true
		return col, nil
	}

	// Parse expression
	expr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	col.Expr = expr

	// Check for AS alias
	if p.matchKeyword("AS") {
		p.advance()
		if p.match(TokIdent) {
			col.Alias = p.advance().Value
		}
	} else if p.match(TokIdent) && !p.isKeyword(p.current().Value) {
		// Implicit alias
		col.Alias = p.advance().Value
	}

	return col, nil
}

// isKeyword checks if a string is a keyword
func (p *Parser) isKeyword(s string) bool {
	return keywords[strings.ToUpper(s)]
}

// parseFromClause parses FROM clause
func (p *Parser) parseFromClause() (*FromClause, error) {
	from := &FromClause{}

	if p.match(TokLParen) {
		// Subquery
		p.advance()
		subq, err := p.parseSelect()
		if err != nil {
			return nil, err
		}
		if err := p.expect(TokRParen, ""); err != nil {
			return nil, err
		}
		from.Subquery = subq
	} else {
		if !p.match(TokIdent) {
			return nil, fmt.Errorf("expected table name, got %s", p.current())
		}
		from.Database, from.Table = p.parseTableNameWithDB()
	}

	// Alias
	if p.matchKeyword("AS") {
		p.advance()
	}
	if p.match(TokIdent) && !p.isKeyword(p.current().Value) {
		from.Alias = p.advance().Value
	}

	return from, nil
}

// parseJoinClause parses JOIN clause
func (p *Parser) parseJoinClause() (*JoinClause, error) {
	join := &JoinClause{}

	// Join type
	switch strings.ToUpper(p.current().Value) {
	case "INNER":
		join.Type = "INNER"
		p.advance()
		p.expect(TokKeyword, "JOIN")
	case "LEFT":
		join.Type = "LEFT"
		p.advance()
		if p.matchKeyword("OUTER") {
			p.advance()
		}
		p.expect(TokKeyword, "JOIN")
	case "RIGHT":
		join.Type = "RIGHT"
		p.advance()
		if p.matchKeyword("OUTER") {
			p.advance()
		}
		p.expect(TokKeyword, "JOIN")
	case "CROSS":
		join.Type = "CROSS"
		p.advance()
		p.expect(TokKeyword, "JOIN")
	default:
		join.Type = "INNER"
		p.advance() // JOIN
	}

	// Table name
	if !p.match(TokIdent) {
		return nil, fmt.Errorf("expected table name, got %s", p.current())
	}
	join.Table = p.parseTableName()

	// Alias
	if p.matchKeyword("AS") {
		p.advance()
	}
	if p.match(TokIdent) && !p.isKeyword(p.current().Value) {
		join.Alias = p.advance().Value
	}

	// ON clause
	if p.matchKeyword("ON") {
		p.advance()
		on, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		join.On = on
	}

	// USING clause
	if p.matchKeyword("USING") {
		p.advance()
		if err := p.expect(TokLParen, ""); err != nil {
			return nil, err
		}
		for {
			if !p.match(TokIdent) {
				return nil, fmt.Errorf("expected column name, got %s", p.current())
			}
			join.Using = append(join.Using, p.advance().Value)
			if !p.match(TokComma) {
				break
			}
			p.advance()
		}
		if err := p.expect(TokRParen, ""); err != nil {
			return nil, err
		}
	}

	return join, nil
}

// parseGroupByList parses GROUP BY column list
func (p *Parser) parseGroupByList() ([]string, error) {
	var cols []string

	for {
		if !p.match(TokIdent) {
			return nil, fmt.Errorf("expected column name, got %s", p.current())
		}
		cols = append(cols, p.advance().Value)

		if !p.match(TokComma) {
			break
		}
		p.advance()
	}

	return cols, nil
}

// parseOrderByList parses ORDER BY list
func (p *Parser) parseOrderByList() ([]OrderByClause, error) {
	var list []OrderByClause

	for {
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}

		ob := OrderByClause{Expr: expr, Direction: "ASC"}

		if p.matchKeyword("ASC") {
			ob.Direction = "ASC"
			p.advance()
		} else if p.matchKeyword("DESC") {
			ob.Direction = "DESC"
			p.advance()
		}

		list = append(list, ob)

		if !p.match(TokComma) {
			break
		}
		p.advance()
	}

	return list, nil
}

// parseExpression parses an expression
func (p *Parser) parseExpression() (*Expression, error) {
	return p.parseOrExpr()
}

// parseOrExpr parses OR expression
func (p *Parser) parseOrExpr() (*Expression, error) {
	left, err := p.parseAndExpr()
	if err != nil {
		return nil, err
	}

	for p.matchKeyword("OR") {
		p.advance()
		right, err := p.parseAndExpr()
		if err != nil {
			return nil, err
		}
		left = NewBinaryExpr(left, "OR", right)
	}

	return left, nil
}

// parseAndExpr parses AND expression
func (p *Parser) parseAndExpr() (*Expression, error) {
	left, err := p.parseNotExpr()
	if err != nil {
		return nil, err
	}

	for p.matchKeyword("AND") {
		p.advance()
		right, err := p.parseNotExpr()
		if err != nil {
			return nil, err
		}
		left = NewBinaryExpr(left, "AND", right)
	}

	return left, nil
}

// parseNotExpr parses NOT expression
func (p *Parser) parseNotExpr() (*Expression, error) {
	if p.matchKeyword("NOT") {
		p.advance()
		right, err := p.parseComparisonExpr()
		if err != nil {
			return nil, err
		}
		return NewUnaryExpr("NOT", right), nil
	}
	return p.parseComparisonExpr()
}

// parseComparisonExpr parses comparison expression
func (p *Parser) parseComparisonExpr() (*Expression, error) {
	left, err := p.parseAdditiveExpr()
	if err != nil {
		return nil, err
	}

	// Handle comparison operators
	op := ""
	if p.match(TokOperator, "=", "<", ">", "<=", ">=", "<>", "!=") {
		op = p.advance().Value
	} else if p.matchKeyword("IS") {
		p.advance()
		if p.matchKeyword("NOT") {
			p.advance()
			if p.matchKeyword("NULL") {
				p.advance()
				return NewBinaryExpr(left, "IS NOT", NewLiteralExpr(nil)), nil
			}
			return nil, fmt.Errorf("expected NULL after IS NOT")
		}
		if p.matchKeyword("NULL") {
			p.advance()
			return NewBinaryExpr(left, "IS", NewLiteralExpr(nil)), nil
		}
		return nil, fmt.Errorf("expected NULL after IS")
	} else if p.matchKeyword("IN") {
		p.advance()
		return p.parseInExpr(left)
	} else if p.matchKeyword("BETWEEN") {
		p.advance()
		return p.parseBetweenExpr(left)
	} else if p.matchKeyword("LIKE") {
		p.advance()
		op = "LIKE"
	} else {
		return left, nil
	}

	right, err := p.parseAdditiveExpr()
	if err != nil {
		return nil, err
	}

	return NewBinaryExpr(left, op, right), nil
}

// parseInExpr parses IN expression
func (p *Parser) parseInExpr(left *Expression) (*Expression, error) {
	if err := p.expect(TokLParen, ""); err != nil {
		return nil, err
	}

	// Check for subquery: IN (SELECT ...)
	if p.matchKeyword("SELECT") {
		subq, err := p.parseSelect()
		if err != nil {
			return nil, err
		}
		if err := p.expect(TokRParen, ""); err != nil {
			return nil, err
		}
		return &Expression{Type: ExprIn, Left: left, Subquery: subq}, nil
	}

	var list []*Expression
	for {
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		list = append(list, expr)

		if !p.match(TokComma) {
			break
		}
		p.advance()
	}

	if err := p.expect(TokRParen, ""); err != nil {
		return nil, err
	}

	return &Expression{Type: ExprIn, Left: left, List: list}, nil
}

// parseBetweenExpr parses BETWEEN expression
func (p *Parser) parseBetweenExpr(left *Expression) (*Expression, error) {
	low, err := p.parseAdditiveExpr()
	if err != nil {
		return nil, err
	}

	if err := p.expect(TokKeyword, "AND"); err != nil {
		return nil, err
	}

	high, err := p.parseAdditiveExpr()
	if err != nil {
		return nil, err
	}

	return &Expression{Type: ExprBetween, Left: left, List: []*Expression{low, high}}, nil
}

// parseAdditiveExpr parses additive expression (+, -, ||)
func (p *Parser) parseAdditiveExpr() (*Expression, error) {
	left, err := p.parseMultiplicativeExpr()
	if err != nil {
		return nil, err
	}

	for {
		var op string
		if p.match(TokOperator, "+", "-") {
			op = p.advance().Value
		} else if p.match(TokOperator, "||") {
			op = "||"
			p.advance()
		} else {
			break
		}

		right, err := p.parseMultiplicativeExpr()
		if err != nil {
			return nil, err
		}
		left = NewBinaryExpr(left, op, right)
	}

	return left, nil
}

// parseMultiplicativeExpr parses multiplicative expression (*, /, %)
func (p *Parser) parseMultiplicativeExpr() (*Expression, error) {
	left, err := p.parseUnaryExpr()
	if err != nil {
		return nil, err
	}

	for p.match(TokOperator, "*", "/", "%") {
		op := p.advance().Value
		right, err := p.parseUnaryExpr()
		if err != nil {
			return nil, err
		}
		left = NewBinaryExpr(left, op, right)
	}

	return left, nil
}

// parseUnaryExpr parses unary expression (+, -, NOT)
func (p *Parser) parseUnaryExpr() (*Expression, error) {
	if p.match(TokOperator, "+", "-") {
		op := p.advance().Value
		right, err := p.parseUnaryExpr()
		if err != nil {
			return nil, err
		}
		return NewUnaryExpr(op, right), nil
	}
	return p.parsePrimaryExpr()
}

// parsePrimaryExpr parses primary expression
func (p *Parser) parsePrimaryExpr() (*Expression, error) {
	tok := p.current()

	switch tok.Type {
	case TokNumber:
		return p.parseNumberLiteral()
	case TokString:
		p.advance()
		return NewLiteralExpr(tok.Value), nil
	case TokKeyword:
		if strings.ToUpper(tok.Value) == "NULL" {
			p.advance()
			return NewLiteralExpr(nil), nil
		}
		if strings.ToUpper(tok.Value) == "TRUE" {
			p.advance()
			return NewLiteralExpr(true), nil
		}
		if strings.ToUpper(tok.Value) == "FALSE" {
			p.advance()
			return NewLiteralExpr(false), nil
		}
		if strings.ToUpper(tok.Value) == "CASE" {
			return p.parseCaseExpr()
		}
		if strings.ToUpper(tok.Value) == "EXISTS" {
			return p.parseExistsExpr()
		}
		if strings.ToUpper(tok.Value) == "MATCH" {
			return p.parseMatchAgainst()
		}
		// Might be a function
		return p.parseIdentOrFunction()
	case TokIdent:
		return p.parseIdentOrFunction()
	case TokLParen:
		return p.parseParenExpr()
	case TokStar:
		p.advance()
		return NewStarExpr(""), nil
	default:
		return nil, fmt.Errorf("unexpected token: %s", tok)
	}
}

// parseNumberLiteral parses a number literal
func (p *Parser) parseNumberLiteral() (*Expression, error) {
	tok := p.advance()
	value := tok.Value

	// Check for integer
	if !strings.Contains(value, ".") && !strings.Contains(strings.ToUpper(value), "E") {
		n, err := strconv.ParseInt(value, 10, 64)
		if err == nil {
			return NewLiteralExpr(n), nil
		}
	}

	// Parse as float
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid number: %s", value)
	}
	return NewLiteralExpr(f), nil
}

// parseMatchAgainst parses MATCH(column) AGAINST('query')
func (p *Parser) parseMatchAgainst() (*Expression, error) {
	p.advance() // Skip MATCH

	// Open paren
	if err := p.expect(TokLParen, ""); err != nil {
		return nil, fmt.Errorf("expected ( after MATCH: %v", err)
	}

	// Column name
	if !p.match(TokIdent) {
		return nil, fmt.Errorf("expected column name in MATCH: %s", p.current())
	}
	column := p.advance().Value

	// Close paren
	if err := p.expect(TokRParen, ""); err != nil {
		return nil, err
	}

	// AGAINST
	if !p.matchKeyword("AGAINST") {
		return nil, fmt.Errorf("expected AGAINST after MATCH(column): %s", p.current())
	}
	p.advance()

	// Open paren
	if err := p.expect(TokLParen, ""); err != nil {
		return nil, err
	}

	// Query string
	if !p.match(TokString) {
		return nil, fmt.Errorf("expected search query string: %s", p.current())
	}
	query := p.advance().Value

	// Optional mode: WITH QUERY EXPANSION or WITH BOOLEAN MODE
	mode := ""
	if p.matchKeyword("WITH") {
		p.advance()
		if p.matchKeyword("QUERY") {
			p.advance()
			if err := p.expect(TokKeyword, "EXPANSION"); err != nil {
				return nil, err
			}
			mode = "QUERY EXPANSION"
		} else if p.matchKeyword("BOOLEAN") {
			p.advance()
			if err := p.expect(TokKeyword, "MODE"); err != nil {
				return nil, err
			}
			mode = "BOOLEAN"
		}
	}

	// Optional LEVEL n
	level := 0 // 0 means all levels
	if p.matchKeyword("LEVEL") {
		p.advance()
		if !p.match(TokNumber) {
			return nil, fmt.Errorf("expected level number after LEVEL: %s", p.current())
		}
		levelStr := p.advance().Value
		if n, err := strconv.Atoi(levelStr); err == nil && n >= 1 && n <= 3 {
			level = n
		}
	}

	// Close paren
	if err := p.expect(TokRParen, ""); err != nil {
		return nil, err
	}

	return &Expression{
		Type:        ExprMatch,
		MatchColumn: column,
		MatchQuery:  query,
		MatchMode:   mode,
		MatchLevel:  level,
	}, nil
}

// parseIdentOrFunction parses an identifier or function call
func (p *Parser) parseIdentOrFunction() (*Expression, error) {
	name := p.advance().Value

	// Check for function call
	if p.match(TokLParen) {
		p.advance()
		var args []*Expression

		// Check for COUNT(*)
		if p.match(TokStar) {
			p.advance()
			args = append(args, NewStarExpr(""))
		} else if !p.match(TokRParen) {
			for {
				// Check for DISTINCT in aggregate functions
				if p.matchKeyword("DISTINCT") {
					p.advance()
					expr, err := p.parseExpression()
					if err != nil {
						return nil, err
					}
					args = append(args, NewUnaryExpr("DISTINCT", expr))
				} else {
					expr, err := p.parseExpression()
					if err != nil {
						return nil, err
					}
					args = append(args, expr)
				}

				if !p.match(TokComma) {
					break
				}
				p.advance()
			}
		}

		if err := p.expect(TokRParen, ""); err != nil {
			return nil, err
		}

		return NewFunctionExpr(name, args...), nil
	}

	// Check for qualified name (table.column)
	if p.match(TokDot) {
		p.advance()
		// Allow keywords as column names in qualified names (e.g., t1.status)
		if !p.match(TokIdent) && !p.match(TokKeyword) {
			return nil, fmt.Errorf("expected column name after '.'")
		}
		col := p.advance().Value
		return NewQualifiedColumnExpr(name, col), nil
	}

	return NewColumnExpr(name), nil
}

// parseParenExpr parses parenthesized expression
func (p *Parser) parseParenExpr() (*Expression, error) {
	p.advance() // (

	// Check for subquery
	if p.matchKeyword("SELECT") {
		subq, err := p.parseSelect()
		if err != nil {
			return nil, err
		}
		if err := p.expect(TokRParen, ""); err != nil {
			return nil, err
		}
		return &Expression{Type: ExprSubquery, Subquery: subq}, nil
	}

	expr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	if err := p.expect(TokRParen, ""); err != nil {
		return nil, err
	}

	return expr, nil
}

// parseCaseExpr parses CASE expression
func (p *Parser) parseCaseExpr() (*Expression, error) {
	p.advance() // CASE

	expr := &Expression{Type: ExprCase}

	// Optional case expression
	if !p.matchKeyword("WHEN") {
		caseExpr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		expr.CaseExpr = caseExpr
	}

	// WHEN clauses
	for p.matchKeyword("WHEN") {
		p.advance()
		cond, err := p.parseExpression()
		if err != nil {
			return nil, err
		}

		if err := p.expect(TokKeyword, "THEN"); err != nil {
			return nil, err
		}

		then, err := p.parseExpression()
		if err != nil {
			return nil, err
		}

		expr.WhenClauses = append(expr.WhenClauses, WhenClause{Cond: cond, Then: then})
	}

	// ELSE clause
	if p.matchKeyword("ELSE") {
		p.advance()
		elseExpr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		expr.ElseExpr = elseExpr
	}

	if err := p.expect(TokKeyword, "END"); err != nil {
		return nil, err
	}

	return expr, nil
}

// parseExistsExpr parses EXISTS expression
func (p *Parser) parseExistsExpr() (*Expression, error) {
	p.advance() // EXISTS

	if err := p.expect(TokLParen, ""); err != nil {
		return nil, err
	}

	subq, err := p.parseSelect()
	if err != nil {
		return nil, err
	}

	if err := p.expect(TokRParen, ""); err != nil {
		return nil, err
	}

	return &Expression{Type: ExprExists, Subquery: subq}, nil
}

// ParseString is a convenience function to parse SQL
func ParseString(sql string) (*Statement, error) {
	return NewParser(sql).Parse()
}
