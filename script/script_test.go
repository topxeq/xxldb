package script

import (
	"testing"

	"github.com/topxeq/xxldb/types"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.scripts == nil {
		t.Error("scripts map should be initialized")
	}
}

func TestRegisterAndGet(t *testing.T) {
	m := NewManager()

	m.Register("xx_test", "$1 + $2")

	script, ok := m.Get("xx_test")
	if !ok || script != "$1 + $2" {
		t.Error("Get should return registered script")
	}

	// Case insensitive
	script, ok = m.Get("XX_TEST")
	if !ok || script != "$1 + $2" {
		t.Error("Get should be case insensitive")
	}

	// Non-existent
	_, ok = m.Get("xx_nonexistent")
	if ok {
		t.Error("Get for non-existent script should return false")
	}
}

func TestRemove(t *testing.T) {
	m := NewManager()

	m.Register("xx_test", "script")
	m.Remove("xx_test")

	_, ok := m.Get("xx_test")
	if ok {
		t.Error("Removed script should not be found")
	}
}

func TestList(t *testing.T) {
	m := NewManager()

	m.Register("xx_a", "script_a")
	m.Register("xx_b", "script_b")
	m.Register("xx_c", "script_c")

	list := m.List()
	if len(list) != 3 {
		t.Errorf("List should return 3 scripts, got %d", len(list))
	}
}

func TestExecute(t *testing.T) {
	m := NewManager()

	// Test with $1, $2 placeholders
	m.Register("xx_add", "$1 + $2")

	result, err := m.Execute("xx_add", []types.Value{
		types.NewIntValue(10),
		types.NewIntValue(5),
	})
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}
	if result.ToString() != "15" {
		t.Errorf("Execute result = %s, want 15", result.ToString())
	}

	// Test with ${1}, ${2} placeholders
	m.Register("xx_multiply", "${1} * ${2}")

	result, err = m.Execute("xx_multiply", []types.Value{
		types.NewIntValue(3),
		types.NewIntValue(4),
	})
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}
	if result.ToString() != "12" {
		t.Errorf("Execute result = %s, want 12", result.ToString())
	}
}

func TestExecuteNonExistent(t *testing.T) {
	m := NewManager()

	_, err := m.Execute("xx_nonexistent", nil)
	if err == nil {
		t.Error("Execute for non-existent script should return error")
	}
}

func TestEvaluate(t *testing.T) {
	m := NewManager()

	tests := []struct {
		input    string
		expected string
	}{
		{"42", "42"},
		{"-10", "-10"},
		{"3.14", "3.14"},
		{"-2.5", "-2.5"},
		{"true", "1"},
		{"false", "0"},
		{"null", "NULL"},
		{"'hello'", "hello"},
		{"\"world\"", "world"},
		{"2 + 3", "5"},
		{"10 - 4", "6"},
		{"3 * 4", "12"},
		{"15 / 3", "5"},
		{"(2 + 3) * 4", "20"},
		{"-5 + 10", "5"},
	}

	for _, tt := range tests {
		result, err := m.evaluate(tt.input)
		if err != nil {
			t.Errorf("evaluate(%s) failed: %v", tt.input, err)
			continue
		}
		if result.ToString() != tt.expected {
			t.Errorf("evaluate(%s) = %s, want %s", tt.input, result.ToString(), tt.expected)
		}
	}
}

func TestEvaluateArithmetic(t *testing.T) {
	m := NewManager()

	tests := []struct {
		expr     string
		expected float64
	}{
		{"1 + 2", 3},
		{"10 - 3", 7},
		{"4 * 5", 20},
		{"20 / 4", 5},
		{"2 + 3 * 4", 14},
		{"(2 + 3) * 4", 20},
		{"10 / 2 + 3", 8},
		{"-5 + 10", 5},
	}

	for _, tt := range tests {
		result, err := m.evaluateArithmetic(tt.expr)
		if err != nil {
			t.Errorf("evaluateArithmetic(%s) failed: %v", tt.expr, err)
			continue
		}
		f, _ := result.ToFloat64()
		if f != tt.expected {
			t.Errorf("evaluateArithmetic(%s) = %f, want %f", tt.expr, f, tt.expected)
		}
	}
}

func TestDivisionByZero(t *testing.T) {
	m := NewManager()

	_, err := m.evaluateArithmetic("10 / 0")
	if err == nil {
		t.Error("Division by zero should return error")
	}
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"1 + 2", []string{"1", "+", "2"}},
		{"10 - 3 * 4", []string{"10", "-", "3", "*", "4"}},
		{"( 2 + 3 )", []string{"(", "2", "+", "3", ")"}},
		{"-5", []string{"-", "5"}},
	}

	for _, tt := range tests {
		result := tokenize(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("tokenize(%s) returned %d tokens, want %d", tt.input, len(result), len(tt.expected))
			continue
		}
		for i, tok := range result {
			if tok != tt.expected[i] {
				t.Errorf("tokenize(%s)[%d] = %s, want %s", tt.input, i, tok, tt.expected[i])
			}
		}
	}
}

func TestIsScriptFunc(t *testing.T) {
	if !IsScriptFunc("xx_test") {
		t.Error("xx_test should be a script function")
	}
	if !IsScriptFunc("XX_TEST") {
		t.Error("XX_TEST should be a script function (case insensitive)")
	}
	if IsScriptFunc("regular_func") {
		t.Error("regular_func should not be a script function")
	}
	if IsScriptFunc("test") {
		t.Error("test should not be a script function")
	}
}

func TestExecuteEmptyScript(t *testing.T) {
	m := NewManager()
	m.Register("xx_empty", "")

	result, err := m.Execute("xx_empty", nil)
	if err != nil {
		t.Errorf("Execute empty script failed: %v", err)
	}
	if !result.IsNull {
		t.Error("Empty script should return null")
	}
}

func TestComplexExpression(t *testing.T) {
	m := NewManager()

	m.Register("xx_complex", "($1 * 2) + ($2 * 3)")

	result, err := m.Execute("xx_complex", []types.Value{
		types.NewIntValue(5),
		types.NewIntValue(4),
	})
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}
	if result.ToString() != "22" {
		t.Errorf("Execute result = %s, want 22", result.ToString())
	}
}

func TestStringResult(t *testing.T) {
	m := NewManager()

	m.Register("xx_greet", "'Hello, ' || $1")

	// Note: || is not supported in arithmetic, should return as string
	result, err := m.Execute("xx_greet", []types.Value{
		types.NewStringValue("World"),
	})
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}
	// The result should contain the substituted value
	if result.ToString() == "" {
		t.Error("String result should not be empty")
	}
}
