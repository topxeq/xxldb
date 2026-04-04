// Package script provides script function support for xxldb
// Script functions are user-defined functions stored in the xxscript table
package script

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/topxeq/xxldb/types"
)

// Manager manages script functions
type Manager struct {
	scripts map[string]string
}

// NewManager creates a new script manager
func NewManager() *Manager {
	return &Manager{
		scripts: make(map[string]string),
	}
}

// Register registers a script function
func (m *Manager) Register(name, script string) {
	m.scripts[strings.ToLower(name)] = script
}

// Get retrieves a script by name
func (m *Manager) Get(name string) (string, bool) {
	script, ok := m.scripts[strings.ToLower(name)]
	return script, ok
}

// Remove removes a script function
func (m *Manager) Remove(name string) {
	delete(m.scripts, strings.ToLower(name))
}

// List lists all script function names
func (m *Manager) List() []string {
	names := make([]string, 0, len(m.scripts))
	for name := range m.scripts {
		names = append(names, name)
	}
	return names
}

// Execute executes a script function with arguments
func (m *Manager) Execute(name string, args []types.Value) (types.Value, error) {
	script, ok := m.Get(name)
	if !ok {
		return types.Value{}, fmt.Errorf("script function not found: %s", name)
	}

	// Replace placeholders with argument values
	result := script
	for i, arg := range args {
		// Support ${1}, ${2}, etc.
		placeholder := fmt.Sprintf("${%d}", i+1)
		result = strings.ReplaceAll(result, placeholder, arg.ToString())

		// Support $1, $2, etc.
		placeholder = fmt.Sprintf("$%d", i+1)
		result = strings.ReplaceAll(result, placeholder, arg.ToString())
	}

	// Evaluate the result
	return m.evaluate(result)
}

// evaluate evaluates a script expression
func (m *Manager) evaluate(expr string) (types.Value, error) {
	expr = strings.TrimSpace(expr)

	if expr == "" {
		return types.NewNullValue(), nil
	}

	// Check for string literal
	if (strings.HasPrefix(expr, "'") && strings.HasSuffix(expr, "'")) ||
		(strings.HasPrefix(expr, "\"") && strings.HasSuffix(expr, "\"")) {
		return types.NewStringValue(expr[1 : len(expr)-1]), nil
	}

	// Check for integer
	if matched, _ := regexp.MatchString(`^-?\d+$`, expr); matched {
		n, _ := strconv.ParseInt(expr, 10, 64)
		return types.NewIntValue(n), nil
	}

	// Check for float
	if matched, _ := regexp.MatchString(`^-?\d+\.\d+$`, expr); matched {
		f, _ := strconv.ParseFloat(expr, 64)
		return types.NewFloatValue(f), nil
	}

	// Check for boolean
	switch strings.ToLower(expr) {
	case "true", "yes", "1":
		return types.NewBoolValue(true), nil
	case "false", "no", "0":
		return types.NewBoolValue(false), nil
	case "null", "nil":
		return types.NewNullValue(), nil
	}

	// Check for arithmetic expression
	if strings.ContainsAny(expr, "+-*/") {
		if result, err := m.evaluateArithmetic(expr); err == nil {
			return result, nil
		}
	}

	// Default: return as string
	return types.NewStringValue(expr), nil
}

// evaluateArithmetic evaluates a simple arithmetic expression
func (m *Manager) evaluateArithmetic(expr string) (types.Value, error) {
	// Simple arithmetic parser
	tokens := tokenize(expr)
	if len(tokens) == 0 {
		return types.Value{}, fmt.Errorf("invalid expression")
	}

	result, err := parseExpression(tokens)
	if err != nil {
		return types.Value{}, err
	}

	return types.NewFloatValue(result), nil
}

// tokenize tokenizes an arithmetic expression
func tokenize(expr string) []string {
	var tokens []string
	var current strings.Builder

	for i := 0; i < len(expr); i++ {
		c := expr[i]

		if c == ' ' || c == '\t' {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			continue
		}

		if c == '+' || c == '-' || c == '*' || c == '/' || c == '(' || c == ')' {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			tokens = append(tokens, string(c))
			continue
		}

		current.WriteByte(c)
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// parseExpression parses and evaluates a tokenized expression
func parseExpression(tokens []string) (float64, error) {
	pos := 0

	var parseExpr func() (float64, error)
	var parseTerm func() (float64, error)
	var parseFactor func() (float64, error)

	parseExpr = func() (float64, error) {
		left, err := parseTerm()
		if err != nil {
			return 0, err
		}

		for pos < len(tokens) && (tokens[pos] == "+" || tokens[pos] == "-") {
			op := tokens[pos]
			pos++
			right, err := parseTerm()
			if err != nil {
				return 0, err
			}
			if op == "+" {
				left += right
			} else {
				left -= right
			}
		}

		return left, nil
	}

	parseTerm = func() (float64, error) {
		left, err := parseFactor()
		if err != nil {
			return 0, err
		}

		for pos < len(tokens) && (tokens[pos] == "*" || tokens[pos] == "/") {
			op := tokens[pos]
			pos++
			right, err := parseFactor()
			if err != nil {
				return 0, err
			}
			if op == "*" {
				left *= right
			} else {
				if right == 0 {
					return 0, fmt.Errorf("division by zero")
				}
				left /= right
			}
		}

		return left, nil
	}

	parseFactor = func() (float64, error) {
		if pos >= len(tokens) {
			return 0, fmt.Errorf("unexpected end of expression")
		}

		token := tokens[pos]

		if token == "(" {
			pos++
			result, err := parseExpr()
			if err != nil {
				return 0, err
			}
			if pos >= len(tokens) || tokens[pos] != ")" {
				return 0, fmt.Errorf("missing closing parenthesis")
			}
			pos++
			return result, nil
		}

		if token == "-" {
			pos++
			val, err := parseFactor()
			if err != nil {
				return 0, err
			}
			return -val, nil
		}

		pos++
		val, err := strconv.ParseFloat(token, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid number: %s", token)
		}
		return val, nil
	}

	return parseExpr()
}

// IsScriptFunc checks if a function name is a script function (starts with xx_)
func IsScriptFunc(name string) bool {
	return strings.HasPrefix(strings.ToLower(name), "xx_")
}
