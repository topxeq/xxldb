// Package parser provides SQL lexical analysis for xxldb
package parser

import (
	"fmt"
	"strings"
	"unicode"
)

// TokenType represents the type of a token
type TokenType int

const (
	TokEOF TokenType = iota
	TokError
	TokIdent      // Identifier
	TokNumber     // Number literal
	TokString     // String literal
	TokKeyword    // SQL keyword
	TokOperator   // Operator (=, <, >, etc.)
	TokComma      // ,
	TokLParen     // (
	TokRParen     // )
	TokLBracket   // [
	TokRBracket   // ]
	TokSemicolon  // ;
	TokDot        // .
	TokStar       // *
	TokParameter  // ? or $1, $2, etc.
)

// Token represents a lexical token
type Token struct {
	Type  TokenType
	Value string
	Pos   int // Position in input
}

// String returns a string representation of the token
func (t Token) String() string {
	switch t.Type {
	case TokEOF:
		return "EOF"
	case TokError:
		return fmt.Sprintf("ERROR(%s)", t.Value)
	case TokIdent:
		return fmt.Sprintf("IDENT(%s)", t.Value)
	case TokNumber:
		return fmt.Sprintf("NUMBER(%s)", t.Value)
	case TokString:
		return fmt.Sprintf("STRING(%s)", t.Value)
	case TokKeyword:
		return fmt.Sprintf("KEYWORD(%s)", t.Value)
	case TokOperator:
		return fmt.Sprintf("OP(%s)", t.Value)
	case TokComma:
		return ","
	case TokLParen:
		return "("
	case TokRParen:
		return ")"
	case TokLBracket:
		return "["
	case TokRBracket:
		return "]"
	case TokSemicolon:
		return ";"
	case TokDot:
		return "."
	case TokStar:
		return "*"
	case TokParameter:
		return fmt.Sprintf("PARAM(%s)", t.Value)
	default:
		return t.Value
	}
}

// SQL keywords
var keywords = map[string]bool{
	// DDL
	"CREATE": true, "DROP": true, "ALTER": true, "TABLE": true, "INDEX": true,
	// DML
	"SELECT": true, "INSERT": true, "UPDATE": true, "DELETE": true,
	"INTO": true, "VALUES": true, "SET": true,
	// DQL
	"FROM": true, "WHERE": true, "AND": true, "OR": true, "NOT": true,
	"ORDER": true, "BY": true, "GROUP": true, "HAVING": true,
	"LIMIT": true, "OFFSET": true, "ASC": true, "DESC": true,
	"DISTINCT": true, "ALL": true,
	// Joins
	"JOIN": true, "INNER": true, "LEFT": true, "RIGHT": true, "OUTER": true,
	"CROSS": true, "ON": true, "USING": true,
	// Set operations
	"UNION": true, "INTERSECT": true, "EXCEPT": true,
	// Data types
	"SEQ": true, "INT": true, "INTEGER": true, "BIGINT": true, "SMALLINT": true,
	"FLOAT": true, "DOUBLE": true, "DECIMAL": true, "NUMERIC": true,
	"CHAR": true, "CHARACTER": true, "VARCHAR": true, "TEXT": true, "CLOB": true,
	"DATE": true, "TIME": true, "DATETIME": true, "TIMESTAMP": true,
	"BLOB": true, "BINARY": true, "FILE": true,
	"BOOLEAN": true, "BOOL": true,
	// Constraints
	"PRIMARY": true, "KEY": true, "FOREIGN": true, "REFERENCES": true,
	"UNIQUE": true, "NULL": true, "DEFAULT": true, "CHECK": true,
	"AUTO_INCREMENT": true, "AUTOINCREMENT": true,
	// Misc
	"AS": true, "IN": true, "BETWEEN": true, "LIKE": true, "IS": true,
	"EXISTS": true, "CASE": true, "WHEN": true, "THEN": true, "ELSE": true, "END": true,
	"IF": true, "NULLIF": true, "COALESCE": true,
	"TRUE": true, "FALSE": true,
	"SHOW": true, "TABLES": true, "DATABASES": true, "COLUMNS": true,
	"USE": true, "DATABASE": true, "SCHEMA": true,
	"LOAD": true, "OUTFILE": true, "FOLDER": true,
	"BACKUP": true, "RESTORE": true,
	"USER": true, "PASSWORD": true,
	"LOG": true, "LEVEL": true,
	"BEGIN": true, "COMMIT": true, "ROLLBACK": true, "TRANSACTION": true,
}

// Lexer tokenizes SQL input
type Lexer struct {
	input string
	pos   int
	start int
	width int
}

// NewLexer creates a new lexer
func NewLexer(input string) *Lexer {
	return &Lexer{input: input}
}

// NextToken returns the next token
func (l *Lexer) NextToken() Token {
	l.skipWhitespace()

	if l.pos >= len(l.input) {
		return Token{Type: TokEOF, Pos: l.pos}
	}

	l.start = l.pos

	r := l.peek()

	switch {
	case r == 0:
		return Token{Type: TokEOF, Pos: l.pos}
	case isLetter(r) || r == '_':
		return l.scanIdentOrKeyword()
	case isDigit(r):
		return l.scanNumber()
	case r == '\'' || r == '"':
		return l.scanString()
	case r == '?':
		l.next()
		return Token{Type: TokParameter, Value: "?", Pos: l.start}
	case r == '$':
		return l.scanParameter()
	case r == '(':
		l.next()
		return Token{Type: TokLParen, Value: "(", Pos: l.start}
	case r == ')':
		l.next()
		return Token{Type: TokRParen, Value: ")", Pos: l.start}
	case r == '[':
		l.next()
		return Token{Type: TokLBracket, Value: "[", Pos: l.start}
	case r == ']':
		l.next()
		return Token{Type: TokRBracket, Value: "]", Pos: l.start}
	case r == ',':
		l.next()
		return Token{Type: TokComma, Value: ",", Pos: l.start}
	case r == ';':
		l.next()
		return Token{Type: TokSemicolon, Value: ";", Pos: l.start}
	case r == '.':
		l.next()
		return Token{Type: TokDot, Value: ".", Pos: l.start}
	case r == '*':
		l.next()
		return Token{Type: TokStar, Value: "*", Pos: l.start}
	case r == '=' || r == '<' || r == '>' || r == '!' || r == '|':
		return l.scanOperator()
	case r == '-':
		// Check for comment
		if l.peekAt(1) == '-' {
			l.skipLineComment()
			return l.NextToken()
		}
		return l.scanOperator()
	case r == '/':
		// Check for comment
		if l.peekAt(1) == '*' {
			l.skipBlockComment()
			return l.NextToken()
		}
		return l.scanOperator()
	default:
		l.next()
		return Token{Type: TokError, Value: fmt.Sprintf("unexpected character: %c", r), Pos: l.start}
	}
}

// scanIdentOrKeyword scans an identifier or keyword
func (l *Lexer) scanIdentOrKeyword() Token {
	for {
		r := l.peek()
		if !isLetter(r) && !isDigit(r) && r != '_' {
			break
		}
		l.next()
	}

	value := l.input[l.start:l.pos]
	upper := strings.ToUpper(value)

	if keywords[upper] {
		return Token{Type: TokKeyword, Value: upper, Pos: l.start}
	}
	return Token{Type: TokIdent, Value: value, Pos: l.start}
}

// scanNumber scans a numeric literal
func (l *Lexer) scanNumber() Token {
	hasDot := false
	hasExp := false

	for {
		r := l.peek()
		switch {
		case isDigit(r):
			l.next()
		case r == '.' && !hasDot && !hasExp:
			hasDot = true
			l.next()
		case (r == 'e' || r == 'E') && !hasExp:
			hasExp = true
			l.next()
			// Optional sign after exponent
			if r := l.peek(); r == '+' || r == '-' {
				l.next()
			}
		default:
			goto done
		}
	}

done:
	value := l.input[l.start:l.pos]
	return Token{Type: TokNumber, Value: value, Pos: l.start}
}

// scanString scans a string literal
func (l *Lexer) scanString() Token {
	quote := l.next() // Opening quote

	var sb strings.Builder
	for {
		r := l.peek()
		if r == 0 {
			return Token{Type: TokError, Value: "unterminated string", Pos: l.start}
		}

		if r == quote {
			l.next()
			// Check for escaped quote ('')
			if l.peek() == quote {
				l.next()
				sb.WriteRune(quote)
				continue
			}
			break
		}

		if r == '\\' {
			l.next()
			escaped := l.peek()
			if escaped == 0 {
				return Token{Type: TokError, Value: "unterminated escape sequence", Pos: l.start}
			}
			l.next()
			switch escaped {
			case 'n':
				sb.WriteByte('\n')
			case 't':
				sb.WriteByte('\t')
			case 'r':
				sb.WriteByte('\r')
			case '\\':
				sb.WriteByte('\\')
			case quote:
				sb.WriteRune(quote)
			default:
				sb.WriteByte('\\')
				sb.WriteRune(escaped)
			}
			continue
		}

		l.next()
		sb.WriteRune(r)
	}

	return Token{Type: TokString, Value: sb.String(), Pos: l.start}
}

// scanParameter scans a parameter placeholder ($1, $2, etc.)
func (l *Lexer) scanParameter() Token {
	l.next() // Skip $

	// Check if it's a numbered parameter
	start := l.pos
	for isDigit(l.peek()) {
		l.next()
	}

	if l.pos > start {
		return Token{Type: TokParameter, Value: l.input[l.start:l.pos], Pos: l.start}
	}

	// Just a $ sign
	return Token{Type: TokIdent, Value: "$", Pos: l.start}
}

// scanOperator scans an operator
func (l *Lexer) scanOperator() Token {
	r := l.next()

	switch r {
	case '=':
		return Token{Type: TokOperator, Value: "=", Pos: l.start}
	case '<':
		switch l.peek() {
		case '=':
			l.next()
			return Token{Type: TokOperator, Value: "<=", Pos: l.start}
		case '>':
			l.next()
			return Token{Type: TokOperator, Value: "<>", Pos: l.start}
		case '<':
			l.next()
			return Token{Type: TokOperator, Value: "<<", Pos: l.start}
		default:
			return Token{Type: TokOperator, Value: "<", Pos: l.start}
		}
	case '>':
		switch l.peek() {
		case '=':
			l.next()
			return Token{Type: TokOperator, Value: ">=", Pos: l.start}
		case '>':
			l.next()
			return Token{Type: TokOperator, Value: ">>", Pos: l.start}
		default:
			return Token{Type: TokOperator, Value: ">", Pos: l.start}
		}
	case '!':
		if l.peek() == '=' {
			l.next()
			return Token{Type: TokOperator, Value: "!=", Pos: l.start}
		}
		return Token{Type: TokOperator, Value: "!", Pos: l.start}
	case '|':
		if l.peek() == '|' {
			l.next()
			return Token{Type: TokOperator, Value: "||", Pos: l.start}
		}
		return Token{Type: TokOperator, Value: "|", Pos: l.start}
	case '+':
		return Token{Type: TokOperator, Value: "+", Pos: l.start}
	case '-':
		return Token{Type: TokOperator, Value: "-", Pos: l.start}
	case '/':
		return Token{Type: TokOperator, Value: "/", Pos: l.start}
	case '%':
		return Token{Type: TokOperator, Value: "%", Pos: l.start}
	default:
		return Token{Type: TokOperator, Value: string(r), Pos: l.start}
	}
}

// skipWhitespace skips whitespace characters
func (l *Lexer) skipWhitespace() {
	for {
		r := l.peek()
		if !unicode.IsSpace(r) {
			break
		}
		l.next()
	}
}

// skipLineComment skips a line comment (-- ...)
func (l *Lexer) skipLineComment() {
	for {
		r := l.peek()
		if r == 0 || r == '\n' {
			break
		}
		l.next()
	}
}

// skipBlockComment skips a block comment (/* ... */)
func (l *Lexer) skipBlockComment() {
	l.next() // Skip /
	l.next() // Skip *

	for {
		r := l.peek()
		if r == 0 {
			return
		}
		if r == '*' && l.peekAt(1) == '/' {
			l.next()
			l.next()
			return
		}
		l.next()
	}
}

// next returns the next character and advances position
func (l *Lexer) next() rune {
	if l.pos >= len(l.input) {
		return 0
	}
	r, w := rune(l.input[l.pos]), 1
	l.width = w
	l.pos += w
	return r
}

// peek returns the next character without advancing
func (l *Lexer) peek() rune {
	if l.pos >= len(l.input) {
		return 0
	}
	return rune(l.input[l.pos])
}

// peekAt returns the character at offset from current position
func (l *Lexer) peekAt(offset int) rune {
	pos := l.pos + offset
	if pos >= len(l.input) {
		return 0
	}
	return rune(l.input[pos])
}

// isLetter returns true if r is a letter
func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// isDigit returns true if r is a digit
func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

// Tokenize returns all tokens from the input
func Tokenize(input string) []Token {
	lexer := NewLexer(input)
	var tokens []Token

	for {
		tok := lexer.NextToken()
		tokens = append(tokens, tok)
		if tok.Type == TokEOF || tok.Type == TokError {
			break
		}
	}

	return tokens
}
