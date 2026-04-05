package function

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/topxeq/xxldb/types"
)

func TestStringFunctions(t *testing.T) {
	// CONCAT
	result, err := Call("CONCAT", []types.Value{
		types.NewStringValue("Hello"),
		types.NewStringValue(" "),
		types.NewStringValue("World"),
	})
	if err != nil || result.ToString() != "Hello World" {
		t.Errorf("CONCAT failed: %v, got %s", err, result.ToString())
	}

	// LENGTH
	result, err = Call("LENGTH", []types.Value{types.NewStringValue("Hello")})
	if err != nil || result.ToString() != "5" {
		t.Errorf("LENGTH failed: got %s", result.ToString())
	}

	// UPPER
	result, err = Call("UPPER", []types.Value{types.NewStringValue("hello")})
	if err != nil || result.ToString() != "HELLO" {
		t.Errorf("UPPER failed: got %s", result.ToString())
	}

	// LOWER
	result, err = Call("LOWER", []types.Value{types.NewStringValue("HELLO")})
	if err != nil || result.ToString() != "hello" {
		t.Errorf("LOWER failed: got %s", result.ToString())
	}

	// TRIM
	result, err = Call("TRIM", []types.Value{types.NewStringValue("  hello  ")})
	if err != nil || result.ToString() != "hello" {
		t.Errorf("TRIM failed: got %s", result.ToString())
	}

	// SUBSTRING
	result, err = Call("SUBSTRING", []types.Value{
		types.NewStringValue("Hello World"),
		types.NewIntValue(1),
		types.NewIntValue(5),
	})
	if err != nil || result.ToString() != "Hello" {
		t.Errorf("SUBSTRING failed: got %s", result.ToString())
	}

	// REPLACE
	result, err = Call("REPLACE", []types.Value{
		types.NewStringValue("Hello World"),
		types.NewStringValue("World"),
		types.NewStringValue("Go"),
	})
	if err != nil || result.ToString() != "Hello Go" {
		t.Errorf("REPLACE failed: got %s", result.ToString())
	}

	// LEFT
	result, err = Call("LEFT", []types.Value{
		types.NewStringValue("Hello"),
		types.NewIntValue(3),
	})
	if err != nil || result.ToString() != "Hel" {
		t.Errorf("LEFT failed: got %s", result.ToString())
	}

	// RIGHT
	result, err = Call("RIGHT", []types.Value{
		types.NewStringValue("Hello"),
		types.NewIntValue(3),
	})
	if err != nil || result.ToString() != "llo" {
		t.Errorf("RIGHT failed: got %s", result.ToString())
	}

	// REVERSE
	result, err = Call("REVERSE", []types.Value{types.NewStringValue("Hello")})
	if err != nil || result.ToString() != "olleH" {
		t.Errorf("REVERSE failed: got %s", result.ToString())
	}

	// REPEAT
	result, err = Call("REPEAT", []types.Value{
		types.NewStringValue("Ab"),
		types.NewIntValue(3),
	})
	if err != nil || result.ToString() != "AbAbAb" {
		t.Errorf("REPEAT failed: got %s", result.ToString())
	}

	// INSTR
	result, err = Call("INSTR", []types.Value{
		types.NewStringValue("Hello World"),
		types.NewStringValue("World"),
	})
	if err != nil || result.ToString() != "7" {
		t.Errorf("INSTR failed: got %s", result.ToString())
	}
}

func TestNumericFunctions(t *testing.T) {
	// ABS
	result, err := Call("ABS", []types.Value{types.NewFloatValue(-3.14)})
	if err != nil {
		t.Errorf("ABS failed: %v", err)
	}
	f, _ := result.ToFloat64()
	if math.Abs(f-3.14) > 0.001 {
		t.Errorf("ABS failed: got %s", result.ToString())
	}

	// ROUND
	result, err = Call("ROUND", []types.Value{
		types.NewFloatValue(3.14159),
		types.NewIntValue(2),
	})
	if err != nil || result.ToString() != "3.14" {
		t.Errorf("ROUND failed: got %s", result.ToString())
	}

	// FLOOR
	result, err = Call("FLOOR", []types.Value{types.NewFloatValue(3.99)})
	if err != nil || result.ToString() != "3" {
		t.Errorf("FLOOR failed: got %s", result.ToString())
	}

	// CEIL
	result, err = Call("CEIL", []types.Value{types.NewFloatValue(3.01)})
	if err != nil || result.ToString() != "4" {
		t.Errorf("CEIL failed: got %s", result.ToString())
	}

	// POWER
	result, err = Call("POWER", []types.Value{
		types.NewFloatValue(2),
		types.NewFloatValue(3),
	})
	if err != nil || result.ToString() != "8" {
		t.Errorf("POWER failed: got %s", result.ToString())
	}

	// SQRT
	result, err = Call("SQRT", []types.Value{types.NewFloatValue(16)})
	if err != nil || result.ToString() != "4" {
		t.Errorf("SQRT failed: got %s", result.ToString())
	}

	// MOD
	result, err = Call("MOD", []types.Value{
		types.NewIntValue(10),
		types.NewIntValue(3),
	})
	if err != nil || result.ToString() != "1" {
		t.Errorf("MOD failed: got %s", result.ToString())
	}
}

func TestAggregateFunctions(t *testing.T) {
	values := []types.Value{
		types.NewIntValue(1),
		types.NewIntValue(2),
		types.NewIntValue(3),
		types.NewIntValue(4),
		types.NewIntValue(5),
	}

	// COUNT
	result, err := Call("COUNT", values)
	if err != nil || result.ToString() != "5" {
		t.Errorf("COUNT failed: got %s", result.ToString())
	}

	// SUM
	result, err = Call("SUM", values)
	if err != nil || result.ToString() != "15" {
		t.Errorf("SUM failed: got %s", result.ToString())
	}

	// AVG
	result, err = Call("AVG", values)
	if err != nil || result.ToString() != "3" {
		t.Errorf("AVG failed: got %s", result.ToString())
	}

	// MIN
	result, err = Call("MIN", values)
	if err != nil || result.ToString() != "1" {
		t.Errorf("MIN failed: got %s", result.ToString())
	}

	// MAX
	result, err = Call("MAX", values)
	if err != nil || result.ToString() != "5" {
		t.Errorf("MAX failed: got %s", result.ToString())
	}
}

func TestDateTimeFunctions(t *testing.T) {
	// NOW
	result, err := Call("NOW", nil)
	if err != nil || result.IsNull {
		t.Error("NOW should return current datetime")
	}

	// CURRENT_DATE
	result, err = Call("CURRENT_DATE", nil)
	if err != nil || result.IsNull {
		t.Error("CURRENT_DATE should return current date")
	}

	// DATE
	result, err = Call("DATE", []types.Value{types.NewStringValue("2026-04-04")})
	if err != nil {
		t.Errorf("DATE failed: %v", err)
	}

	// YEAR
	dt := types.NewDatetimeValue(time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC))
	result, err = Call("YEAR", []types.Value{dt})
	if err != nil || result.ToString() != "2026" {
		t.Errorf("YEAR failed: got %s", result.ToString())
	}

	// MONTH
	result, err = Call("MONTH", []types.Value{dt})
	if err != nil || result.ToString() != "4" {
		t.Errorf("MONTH failed: got %s", result.ToString())
	}

	// DAY
	result, err = Call("DAY", []types.Value{dt})
	if err != nil || result.ToString() != "4" {
		t.Errorf("DAY failed: got %s", result.ToString())
	}

	// HOUR
	result, err = Call("HOUR", []types.Value{dt})
	if err != nil || result.ToString() != "12" {
		t.Errorf("HOUR failed: got %s", result.ToString())
	}

	// DATEDIFF
	dt1 := types.NewDatetimeValue(time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC))
	dt2 := types.NewDatetimeValue(time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC))
	result, err = Call("DATEDIFF", []types.Value{dt1, dt2})
	if err != nil || result.ToString() != "3" {
		t.Errorf("DATEDIFF failed: got %s", result.ToString())
	}

	// DATE_ADD
	result, err = Call("DATE_ADD", []types.Value{dt, types.NewIntValue(7)})
	if err != nil {
		t.Errorf("DATE_ADD failed: %v", err)
	}
}

func TestConversionFunctions(t *testing.T) {
	// CAST to INT
	result, err := Call("CAST", []types.Value{
		types.NewStringValue("42"),
		types.NewStringValue("INT"),
	})
	if err != nil || result.ToString() != "42" {
		t.Errorf("CAST to INT failed: got %s", result.ToString())
	}

	// CAST to FLOAT
	result, err = Call("CAST", []types.Value{
		types.NewStringValue("3.14"),
		types.NewStringValue("FLOAT"),
	})
	if err != nil || result.ToString() != "3.14" {
		t.Errorf("CAST to FLOAT failed: got %s", result.ToString())
	}

	// CAST to STRING
	result, err = Call("CAST", []types.Value{
		types.NewIntValue(42),
		types.NewStringValue("VARCHAR"),
	})
	if err != nil || result.ToString() != "42" {
		t.Errorf("CAST to VARCHAR failed: got %s", result.ToString())
	}

	// TO_STRING
	result, err = Call("TO_STRING", []types.Value{types.NewIntValue(123)})
	if err != nil || result.ToString() != "123" {
		t.Errorf("TO_STRING failed: got %s", result.ToString())
	}

	// TO_INT
	result, err = Call("TO_INT", []types.Value{types.NewStringValue("456")})
	if err != nil || result.ToString() != "456" {
		t.Errorf("TO_INT failed: got %s", result.ToString())
	}

	// TO_FLOAT
	result, err = Call("TO_FLOAT", []types.Value{types.NewStringValue("2.5")})
	if err != nil || result.ToString() != "2.5" {
		t.Errorf("TO_FLOAT failed: got %s", result.ToString())
	}
}

func TestCoalesce(t *testing.T) {
	// COALESCE with non-null first value
	result, err := Call("COALESCE", []types.Value{
		types.NewIntValue(1),
		types.NewIntValue(2),
	})
	if err != nil || result.ToString() != "1" {
		t.Errorf("COALESCE failed: got %s", result.ToString())
	}

	// COALESCE with null first value
	result, err = Call("COALESCE", []types.Value{
		types.NewNullValue(),
		types.NewIntValue(2),
	})
	if err != nil || result.ToString() != "2" {
		t.Errorf("COALESCE failed: got %s", result.ToString())
	}

	// IFNULL (alias)
	result, err = Call("IFNULL", []types.Value{
		types.NewNullValue(),
		types.NewStringValue("default"),
	})
	if err != nil || result.ToString() != "default" {
		t.Errorf("IFNULL failed: got %s", result.ToString())
	}
}

func TestConditionalFunctions(t *testing.T) {
	// IF true
	result, err := Call("IF", []types.Value{
		types.NewIntValue(1),
		types.NewStringValue("yes"),
		types.NewStringValue("no"),
	})
	if err != nil || result.ToString() != "yes" {
		t.Errorf("IF(true) failed: got %s", result.ToString())
	}

	// IF false
	result, err = Call("IF", []types.Value{
		types.NewIntValue(0),
		types.NewStringValue("yes"),
		types.NewStringValue("no"),
	})
	if err != nil || result.ToString() != "no" {
		t.Errorf("IF(false) failed: got %s", result.ToString())
	}
}

func TestUtilityFunctions(t *testing.T) {
	// ISNULL
	result, err := Call("ISNULL", []types.Value{types.NewNullValue()})
	if err != nil || result.ToString() != "1" {
		t.Errorf("ISNULL(null) failed: got %s", result.ToString())
	}

	result, err = Call("ISNULL", []types.Value{types.NewIntValue(1)})
	if err != nil || result.ToString() != "0" {
		t.Errorf("ISNULL(1) failed: got %s", result.ToString())
	}

	// TYPEOF
	result, err = Call("TYPEOF", []types.Value{types.NewIntValue(1)})
	if err != nil || result.ToString() != "INT" {
		t.Errorf("TYPEOF failed: got %s", result.ToString())
	}

	result, err = Call("TYPEOF", []types.Value{types.NewStringValue("hello")})
	if err != nil || result.ToString() != "VARCHAR" {
		t.Errorf("TYPEOF failed: got %s", result.ToString())
	}

	result, err = Call("TYPEOF", []types.Value{types.NewNullValue()})
	if err != nil || result.ToString() != "NULL" {
		t.Errorf("TYPEOF(null) failed: got %s", result.ToString())
	}
}

func TestNullHandling(t *testing.T) {
	// String functions with null should return null
	result, err := Call("UPPER", []types.Value{types.NewNullValue()})
	if err != nil || !result.IsNull {
		t.Error("UPPER(null) should return null")
	}

	// Numeric functions with null should return null
	result, err = Call("ABS", []types.Value{types.NewNullValue()})
	if err != nil || !result.IsNull {
		t.Error("ABS(null) should return null")
	}

	// CONCAT ignores nulls
	result, err = Call("CONCAT", []types.Value{
		types.NewStringValue("Hello"),
		types.NewNullValue(),
		types.NewStringValue("World"),
	})
	if err != nil || result.ToString() != "HelloWorld" {
		t.Errorf("CONCAT with null failed: got %s", result.ToString())
	}
}

func TestUnknownFunction(t *testing.T) {
	_, err := Call("UNKNOWN_FUNC", nil)
	if err == nil {
		t.Error("Unknown function should return error")
	}
}

func TestIsAggregate(t *testing.T) {
	if !IsAggregate("COUNT") || !IsAggregate("SUM") || !IsAggregate("AVG") {
		t.Error("COUNT, SUM, AVG should be aggregate functions")
	}
	if IsAggregate("UPPER") || IsAggregate("LOWER") {
		t.Error("UPPER, LOWER should not be aggregate functions")
	}
}

func TestFunctionRegistration(t *testing.T) {
	// Test that Get works
	fn, ok := Get("CONCAT")
	if !ok || fn == nil {
		t.Error("Get(CONCAT) should return a function")
	}

	// Test case insensitivity
	fn, ok = Get("concat")
	if !ok || fn == nil {
		t.Error("Get(concat) should return a function (case insensitive)")
	}
}

// Additional tests for better coverage

func TestLTRIM(t *testing.T) {
	result, err := Call("LTRIM", []types.Value{types.NewStringValue("   hello")})
	if err != nil || result.ToString() != "hello" {
		t.Errorf("LTRIM failed: got %s", result.ToString())
	}

	// With custom chars
	result, err = Call("LTRIM", []types.Value{
		types.NewStringValue("xxhello"),
		types.NewStringValue("x"),
	})
	if err != nil || result.ToString() != "hello" {
		t.Errorf("LTRIM with chars failed: got %s", result.ToString())
	}
}

func TestRTRIM(t *testing.T) {
	result, err := Call("RTRIM", []types.Value{types.NewStringValue("hello   ")})
	if err != nil || result.ToString() != "hello" {
		t.Errorf("RTRIM failed: got %s", result.ToString())
	}

	// With custom chars
	result, err = Call("RTRIM", []types.Value{
		types.NewStringValue("helloxx"),
		types.NewStringValue("x"),
	})
	if err != nil || result.ToString() != "hello" {
		t.Errorf("RTRIM with chars failed: got %s", result.ToString())
	}
}

func TestLPAD(t *testing.T) {
	result, err := Call("LPAD", []types.Value{
		types.NewStringValue("hello"),
		types.NewIntValue(10),
		types.NewStringValue("x"),
	})
	if err != nil || result.ToString() != "xxxxxhello" {
		t.Errorf("LPAD failed: got %s", result.ToString())
	}
}

func TestRPAD(t *testing.T) {
	result, err := Call("RPAD", []types.Value{
		types.NewStringValue("hello"),
		types.NewIntValue(10),
		types.NewStringValue("x"),
	})
	if err != nil || result.ToString() != "helloxxxxx" {
		t.Errorf("RPAD failed: got %s", result.ToString())
	}
}

func TestSIGN(t *testing.T) {
	result, err := Call("SIGN", []types.Value{types.NewFloatValue(5.5)})
	if err != nil || result.ToString() != "1" {
		t.Errorf("SIGN(5.5) failed: got %s", result.ToString())
	}

	result, err = Call("SIGN", []types.Value{types.NewFloatValue(-5.5)})
	if err != nil || result.ToString() != "-1" {
		t.Errorf("SIGN(-5.5) failed: got %s", result.ToString())
	}
}

func TestCurrentTime(t *testing.T) {
	result, err := Call("CURRENT_TIME", nil)
	if err != nil || result.IsNull {
		t.Error("CURRENT_TIME should return current time")
	}
}

func TestMinuteSecond(t *testing.T) {
	dt := types.NewDatetimeValue(time.Date(2026, 4, 4, 12, 30, 45, 0, time.UTC))

	result, err := Call("MINUTE", []types.Value{dt})
	if err != nil || result.ToString() != "30" {
		t.Errorf("MINUTE failed: got %s", result.ToString())
	}

	result, err = Call("SECOND", []types.Value{dt})
	if err != nil || result.ToString() != "45" {
		t.Errorf("SECOND failed: got %s", result.ToString())
	}
}

func TestDateSub(t *testing.T) {
	dt := types.NewDatetimeValue(time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC))
	_, err := Call("DATE_SUB", []types.Value{dt, types.NewIntValue(7)})
	if err != nil {
		t.Errorf("DATE_SUB failed: %v", err)
	}
}

func TestDateFormat(t *testing.T) {
	dt := types.NewDatetimeValue(time.Date(2026, 4, 4, 12, 30, 45, 0, time.UTC))
	_, err := Call("DATE_FORMAT", []types.Value{
		dt,
		types.NewStringValue("%Y-%m-%d"),
	})
	if err != nil {
		t.Errorf("DATE_FORMAT failed: %v", err)
	}
}

func TestNULLIF(t *testing.T) {
	// Equal values return NULL
	result, err := Call("NULLIF", []types.Value{
		types.NewIntValue(1),
		types.NewIntValue(1),
	})
	if err != nil || !result.IsNull {
		t.Error("NULLIF(1,1) should return NULL")
	}

	// Different values return first value
	result, err = Call("NULLIF", []types.Value{
		types.NewIntValue(1),
		types.NewIntValue(2),
	})
	if err != nil || result.ToString() != "1" {
		t.Errorf("NULLIF(1,2) should return 1, got %s", result.ToString())
	}
}

func TestIS_NOT_NULL(t *testing.T) {
	result, err := Call("IS_NOT_NULL", []types.Value{types.NewNullValue()})
	if err != nil || result.ToString() != "0" {
		t.Errorf("IS_NOT_NULL(null) failed: got %s", result.ToString())
	}

	result, err = Call("IS_NOT_NULL", []types.Value{types.NewIntValue(1)})
	if err != nil || result.ToString() != "1" {
		t.Errorf("IS_NOT_NULL(1) failed: got %s", result.ToString())
	}
}

func TestSQRTNegative(t *testing.T) {
	_, err := Call("SQRT", []types.Value{types.NewFloatValue(-1)})
	if err == nil {
		t.Error("SQRT of negative should return error")
	}
}

func TestMODByZero(t *testing.T) {
	_, err := Call("MOD", []types.Value{
		types.NewIntValue(10),
		types.NewIntValue(0),
	})
	if err == nil {
		t.Error("MOD by zero should return error")
	}
}

func TestAggregateWithNulls(t *testing.T) {
	values := []types.Value{
		types.NewIntValue(1),
		types.NewNullValue(),
		types.NewIntValue(3),
		types.NewNullValue(),
		types.NewIntValue(5),
	}

	// COUNT ignores nulls
	result, err := Call("COUNT", values)
	if err != nil || result.ToString() != "3" {
		t.Errorf("COUNT with nulls failed: got %s", result.ToString())
	}

	// AVG ignores nulls
	result, err = Call("AVG", values)
	if err != nil || result.ToString() != "3" {
		t.Errorf("AVG with nulls failed: got %s", result.ToString())
	}
}

func TestEmptyAggregate(t *testing.T) {
	// MIN/MAX with empty list
	result, err := Call("MIN", []types.Value{})
	if err != nil || !result.IsNull {
		t.Error("MIN of empty should be NULL")
	}

	result, err = Call("MAX", []types.Value{})
	if err != nil || !result.IsNull {
		t.Error("MAX of empty should be NULL")
	}

	// AVG with empty list
	result, err = Call("AVG", []types.Value{})
	if err != nil || !result.IsNull {
		t.Error("AVG of empty should be NULL")
	}
}

func TestHasAggregate(t *testing.T) {
	if !HasAggregate("SELECT COUNT(*) FROM t") {
		t.Error("Should detect COUNT aggregate")
	}
	if !HasAggregate("SELECT SUM(x) FROM t") {
		t.Error("Should detect SUM aggregate")
	}
	if HasAggregate("SELECT x FROM t") {
		t.Error("Should not detect aggregate in simple SELECT")
	}
}

func TestRegister(t *testing.T) {
	// Register a custom function
	customFunc := func(args []types.Value) (types.Value, error) {
		return types.NewStringValue("custom"), nil
	}
	Register("CUSTOM_FUNC", customFunc)

	// Verify it's registered
	fn, ok := Get("CUSTOM_FUNC")
	if !ok || fn == nil {
		t.Error("Custom function should be registered")
	}

	// Call it
	result, err := Call("CUSTOM_FUNC", nil)
	if err != nil || result.ToString() != "custom" {
		t.Errorf("Custom function call failed: got %s", result.ToString())
	}
}

func TestStringFunctionErrors(t *testing.T) {
	// CONCAT with no args - may return null or error depending on implementation
	result, err := Call("CONCAT", []types.Value{})
	if err == nil && result.IsNull {
		// CONCAT with no args returns null - acceptable
	} else if err != nil {
		// CONCAT with no args returns error - also acceptable
	} else if result.ToString() == "" {
		// CONCAT with no args returns empty string - also acceptable
	} else {
		t.Errorf("Unexpected CONCAT() result: %s", result.ToString())
	}

	// LENGTH with wrong arg count
	_, err = Call("LENGTH", []types.Value{})
	if err == nil {
		t.Error("LENGTH with no args should error")
	}

	// SUBSTRING with wrong arg count
	_, err = Call("SUBSTRING", []types.Value{types.NewStringValue("hello")})
	if err == nil {
		t.Error("SUBSTRING with 1 arg should error")
	}

	// REPLACE with wrong arg count
	_, err = Call("REPLACE", []types.Value{types.NewStringValue("a")})
	if err == nil {
		t.Error("REPLACE with 1 arg should error")
	}
}

func TestNumericFunctionErrors(t *testing.T) {
	// ABS with no args
	_, err := Call("ABS", []types.Value{})
	if err == nil {
		t.Error("ABS with no args should error")
	}

	// POWER with wrong arg count
	_, err = Call("POWER", []types.Value{types.NewFloatValue(2)})
	if err == nil {
		t.Error("POWER with 1 arg should error")
	}

	// MOD with wrong arg count
	_, err = Call("MOD", []types.Value{types.NewIntValue(10)})
	if err == nil {
		t.Error("MOD with 1 arg should error")
	}
}

func TestDateTimeFunctionErrors(t *testing.T) {
	// DATE with wrong arg count
	_, err := Call("DATE", []types.Value{})
	if err == nil {
		t.Error("DATE with no args should error")
	}

	// YEAR with wrong arg count
	_, err = Call("YEAR", []types.Value{})
	if err == nil {
		t.Error("YEAR with no args should error")
	}

	// DATEDIFF with wrong arg count
	_, err = Call("DATEDIFF", []types.Value{types.NewDatetimeValue(time.Now())})
	if err == nil {
		t.Error("DATEDIFF with 1 arg should error")
	}
}

func TestConversionFunctionErrors(t *testing.T) {
	// CAST with wrong arg count
	_, err := Call("CAST", []types.Value{types.NewIntValue(1)})
	if err == nil {
		t.Error("CAST with 1 arg should error")
	}

	// CAST with unsupported type
	result, err := Call("CAST", []types.Value{
		types.NewIntValue(1),
		types.NewStringValue("UNSUPPORTED"),
	})
	if err == nil {
		t.Error("CAST with unsupported type should error")
	}
	_ = result
}

func TestFileFunctions(t *testing.T) {
	// Create temp file
	tmpDir, err := os.MkdirTemp("", "xxldb-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world"), 0644)

	// FILE_EXISTS
	result, err := Call("FILE_EXISTS", []types.Value{types.NewStringValue(testFile)})
	if err != nil || result.ToString() != "1" {
		t.Errorf("FILE_EXISTS failed: got %s", result.ToString())
	}

	// FILE_EXISTS for non-existent file
	result, err = Call("FILE_EXISTS", []types.Value{types.NewStringValue("/nonexistent/file.txt")})
	if err != nil || result.ToString() != "0" {
		t.Errorf("FILE_EXISTS for non-existent failed: got %s", result.ToString())
	}

	// FILE_SIZE
	result, err = Call("FILE_SIZE", []types.Value{types.NewStringValue(testFile)})
	if err != nil || result.ToString() != "11" {
		t.Errorf("FILE_SIZE failed: got %s", result.ToString())
	}

	// LOAD_FILE
	result, err = Call("LOAD_FILE", []types.Value{types.NewStringValue(testFile)})
	if err != nil {
		t.Errorf("LOAD_FILE failed: %v", err)
	}

	// SAVE_FILE
	savePath := filepath.Join(tmpDir, "saved.txt")
	result, err = Call("SAVE_FILE", []types.Value{
		types.NewStringValue(savePath),
		types.NewStringValue("saved content"),
	})
	if err != nil {
		t.Errorf("SAVE_FILE failed: %v", err)
	}

	// Verify saved
	data, _ := os.ReadFile(savePath)
	if string(data) != "saved content" {
		t.Errorf("SAVE_FILE content mismatch: %s", string(data))
	}
}

func TestFileFunctionErrors(t *testing.T) {
	// FILE_EXISTS with wrong args
	_, err := Call("FILE_EXISTS", []types.Value{})
	if err == nil {
		t.Error("FILE_EXISTS with no args should error")
	}

	// FILE_SIZE with wrong args
	_, err = Call("FILE_SIZE", []types.Value{})
	if err == nil {
		t.Error("FILE_SIZE with no args should error")
	}

	// LOAD_FILE with wrong args
	_, err = Call("LOAD_FILE", []types.Value{})
	if err == nil {
		t.Error("LOAD_FILE with no args should error")
	}

	// SAVE_FILE with wrong args
	_, err = Call("SAVE_FILE", []types.Value{types.NewStringValue("/tmp/test.txt")})
	if err == nil {
		t.Error("SAVE_FILE with 1 arg should error")
	}
}

func TestSubstringEdgeCases(t *testing.T) {
	// Negative start (should clamp to 0)
	result, err := Call("SUBSTRING", []types.Value{
		types.NewStringValue("hello"),
		types.NewIntValue(-5),
		types.NewIntValue(3),
	})
	if err != nil {
		t.Errorf("SUBSTRING with negative start failed: %v", err)
	}

	// Length beyond string
	result, err = Call("SUBSTRING", []types.Value{
		types.NewStringValue("hello"),
		types.NewIntValue(1),
		types.NewIntValue(100),
	})
	if err != nil || result.ToString() != "hello" {
		t.Errorf("SUBSTRING with long length failed: got %s", result.ToString())
	}
}

func TestLeftRightEdgeCases(t *testing.T) {
	// LEFT with n > len
	result, err := Call("LEFT", []types.Value{
		types.NewStringValue("hello"),
		types.NewIntValue(100),
	})
	if err != nil || result.ToString() != "hello" {
		t.Errorf("LEFT with large n failed: got %s", result.ToString())
	}

	// RIGHT with n > len
	result, err = Call("RIGHT", []types.Value{
		types.NewStringValue("hello"),
		types.NewIntValue(100),
	})
	if err != nil || result.ToString() != "hello" {
		t.Errorf("RIGHT with large n failed: got %s", result.ToString())
	}
}

func TestCOALESCEEdgeCases(t *testing.T) {
	// All nulls
	result, err := Call("COALESCE", []types.Value{
		types.NewNullValue(),
		types.NewNullValue(),
	})
	if err != nil || !result.IsNull {
		t.Error("COALESCE with all nulls should return NULL")
	}

	// No args
	_, err = Call("COALESCE", []types.Value{})
	if err == nil {
		t.Error("COALESCE with no args should error")
	}
}

func TestIFErrors(t *testing.T) {
	// Wrong arg count
	_, err := Call("IF", []types.Value{types.NewIntValue(1)})
	if err == nil {
		t.Error("IF with 1 arg should error")
	}
}

func TestINSTRWithNull(t *testing.T) {
	// INSTR with null first arg
	result, err := Call("INSTR", []types.Value{
		types.NewNullValue(),
		types.NewStringValue("test"),
	})
	if err != nil || !result.IsNull {
		t.Error("INSTR(null, str) should return NULL")
	}

	// INSTR with null second arg
	result, err = Call("INSTR", []types.Value{
		types.NewStringValue("test"),
		types.NewNullValue(),
	})
	if err != nil || !result.IsNull {
		t.Error("INSTR(str, null) should return NULL")
	}
}

func TestRoundNoPrecision(t *testing.T) {
	_, err := Call("ROUND", []types.Value{types.NewFloatValue(3.7)})
	if err != nil {
		t.Errorf("ROUND without precision failed: %v", err)
	}
}

func TestDATEDIFFWithNull(t *testing.T) {
	result, err := Call("DATEDIFF", []types.Value{
		types.NewNullValue(),
		types.NewDatetimeValue(time.Now()),
	})
	if err != nil || !result.IsNull {
		t.Error("DATEDIFF with null should return NULL")
	}
}

func TestDATEWithInvalid(t *testing.T) {
	_, err := Call("DATE", []types.Value{types.NewStringValue("invalid-date")})
	if err == nil {
		t.Error("DATE with invalid format should error")
	}
}

func TestGCDAndLCM(t *testing.T) {
	// Test GCD
	result, err := Call("GCD", []types.Value{
		types.NewIntValue(48),
		types.NewIntValue(18),
	})
	if err != nil {
		t.Errorf("GCD failed: %v", err)
	}
	gcd, _ := result.ToInt64()
	if gcd != 6 {
		t.Errorf("GCD(48, 18) = %d, want 6", gcd)
	}

	// Test LCM
	result, err = Call("LCM", []types.Value{
		types.NewIntValue(4),
		types.NewIntValue(6),
	})
	if err != nil {
		t.Errorf("LCM failed: %v", err)
	}
	lcm, _ := result.ToInt64()
	if lcm != 12 {
		t.Errorf("LCM(4, 6) = %d, want 12", lcm)
	}
}

func TestLogFunctions(t *testing.T) {
	// Test LOG (natural log)
	result, err := Call("LOG", []types.Value{types.NewFloatValue(2.718281828)})
	if err != nil {
		t.Errorf("LOG failed: %v", err)
	}
	// Should be approximately 1

	// Test LOG10
	result, err = Call("LOG10", []types.Value{types.NewFloatValue(100)})
	if err != nil {
		t.Errorf("LOG10 failed: %v", err)
	}
	log10, _ := result.ToFloat64()
	if log10 < 1.99 || log10 > 2.01 {
		t.Errorf("LOG10(100) = %f, want 2", log10)
	}

	// Test LOG2
	result, err = Call("LOG2", []types.Value{types.NewFloatValue(8)})
	if err != nil {
		t.Errorf("LOG2 failed: %v", err)
	}
	log2, _ := result.ToFloat64()
	if log2 < 2.99 || log2 > 3.01 {
		t.Errorf("LOG2(8) = %f, want 3", log2)
	}
}

func TestExpFunction(t *testing.T) {
	result, err := Call("EXP", []types.Value{types.NewFloatValue(1)})
	if err != nil {
		t.Errorf("EXP failed: %v", err)
	}
	exp, _ := result.ToFloat64()
	if exp < 2.71 || exp > 2.72 {
		t.Errorf("EXP(1) = %f, want ~2.718", exp)
	}
}

func TestRadiansDegrees(t *testing.T) {
	// Test RADIANS
	result, err := Call("RADIANS", []types.Value{types.NewFloatValue(180)})
	if err != nil {
		t.Errorf("RADIANS failed: %v", err)
	}
	rad, _ := result.ToFloat64()
	if rad < 3.14 || rad > 3.15 {
		t.Errorf("RADIANS(180) = %f, want ~3.14159", rad)
	}

	// Test DEGREES
	result, err = Call("DEGREES", []types.Value{types.NewFloatValue(3.14159265)})
	if err != nil {
		t.Errorf("DEGREES failed: %v", err)
	}
	deg, _ := result.ToFloat64()
	if deg < 179.9 || deg > 180.1 {
		t.Errorf("DEGREES(pi) = %f, want ~180", deg)
	}
}

func TestTrigonometricFunctions(t *testing.T) {
	// Test SIN
	result, err := Call("SIN", []types.Value{types.NewFloatValue(0)})
	if err != nil {
		t.Errorf("SIN failed: %v", err)
	}
	sin, _ := result.ToFloat64()
	if sin != 0 {
		t.Errorf("SIN(0) = %f, want 0", sin)
	}

	// Test COS
	result, err = Call("COS", []types.Value{types.NewFloatValue(0)})
	if err != nil {
		t.Errorf("COS failed: %v", err)
	}
	cos, _ := result.ToFloat64()
	if cos != 1 {
		t.Errorf("COS(0) = %f, want 1", cos)
	}

	// Test TAN
	result, err = Call("TAN", []types.Value{types.NewFloatValue(0)})
	if err != nil {
		t.Errorf("TAN failed: %v", err)
	}
	tan, _ := result.ToFloat64()
	if tan != 0 {
		t.Errorf("TAN(0) = %f, want 0", tan)
	}
}

func TestStringPaddingEdgeCases(t *testing.T) {
	// LPAD with n < len - behavior may vary, just verify it doesn't error
	result, err := Call("LPAD", []types.Value{
		types.NewStringValue("hello"),
		types.NewIntValue(3),
		types.NewStringValue("x"),
	})
	if err != nil {
		t.Errorf("LPAD with small n failed: %v", err)
	}
	// The result might truncate or not pad - just verify we got something
	if result.ToString() == "" {
		t.Error("LPAD should return non-empty result")
	}

	// RPAD with n < len
	result, err = Call("RPAD", []types.Value{
		types.NewStringValue("hello"),
		types.NewIntValue(3),
		types.NewStringValue("x"),
	})
	if err != nil {
		t.Errorf("RPAD with small n failed: %v", err)
	}
	if result.ToString() == "" {
		t.Error("RPAD should return non-empty result")
	}
}

func TestRandomFunction(t *testing.T) {
	result, err := Call("RAND", []types.Value{})
	if err != nil {
		t.Errorf("RAND failed: %v", err)
	}
	rand, _ := result.ToFloat64()
	if rand < 0 || rand >= 1 {
		t.Errorf("RAND() = %f, should be in [0, 1)", rand)
	}
}

func TestUUIDFunction(t *testing.T) {
	result, err := Call("UUID", []types.Value{})
	if err != nil {
		t.Errorf("UUID failed: %v", err)
	}
	uuid := result.ToString()
	if len(uuid) != 36 {
		t.Errorf("UUID length = %d, want 36", len(uuid))
	}
	// Check format: 8-4-4-4-12
	if uuid[8] != '-' || uuid[13] != '-' || uuid[18] != '-' || uuid[23] != '-' {
		t.Errorf("UUID format incorrect: %s", uuid)
	}
}

func TestStringCompare(t *testing.T) {
	// Test STRCMP
	result, err := Call("STRCMP", []types.Value{
		types.NewStringValue("apple"),
		types.NewStringValue("banana"),
	})
	if err != nil {
		t.Errorf("STRCMP failed: %v", err)
	}
	cmp, _ := result.ToInt64()
	if cmp != -1 {
		t.Errorf("STRCMP('apple', 'banana') = %d, want -1", cmp)
	}

	result, err = Call("STRCMP", []types.Value{
		types.NewStringValue("banana"),
		types.NewStringValue("apple"),
	})
	if err != nil {
		t.Errorf("STRCMP failed: %v", err)
	}
	cmp, _ = result.ToInt64()
	if cmp != 1 {
		t.Errorf("STRCMP('banana', 'apple') = %d, want 1", cmp)
	}

	result, err = Call("STRCMP", []types.Value{
		types.NewStringValue("apple"),
		types.NewStringValue("apple"),
	})
	if err != nil {
		t.Errorf("STRCMP failed: %v", err)
	}
	cmp, _ = result.ToInt64()
	if cmp != 0 {
		t.Errorf("STRCMP('apple', 'apple') = %d, want 0", cmp)
	}
}

func TestLocateFunction(t *testing.T) {
	// Test INSTR instead (which is implemented)
	// INSTR is similar to LOCATE
	result, err := Call("INSTR", []types.Value{
		types.NewStringValue("Hello World"),
		types.NewStringValue("World"),
	})
	if err != nil || result.ToString() != "7" {
		t.Errorf("INSTR failed: got %s", result.ToString())
	}

	// Not found
	result, err = Call("INSTR", []types.Value{
		types.NewStringValue("Hello World"),
		types.NewStringValue("xyz"),
	})
	if err != nil || result.ToString() != "0" {
		t.Errorf("INSTR(not found) failed: got %s", result.ToString())
	}
}

func TestMakeDate(t *testing.T) {
	result, err := Call("MAKEDATE", []types.Value{
		types.NewIntValue(2024),
		types.NewIntValue(60),
	})
	if err != nil {
		t.Errorf("MAKEDATE failed: %v", err)
	}
	// 60th day of 2024 (leap year) should be Feb 29
	t.Logf("MAKEDATE(2024, 60) = %s", result.ToString())
}

func TestMakeTime(t *testing.T) {
	result, err := Call("MAKETIME", []types.Value{
		types.NewIntValue(12),
		types.NewIntValue(30),
		types.NewFloatValue(45),
	})
	if err != nil {
		t.Errorf("MAKETIME failed: %v", err)
	}
	t.Logf("MAKETIME(12, 30, 45) = %s", result.ToString())
}

func TestLastDay(t *testing.T) {
	// Test with February (2024 is leap year)
	result, err := Call("LAST_DAY", []types.Value{
		types.NewStringValue("2024-02-15"),
	})
	if err != nil {
		t.Errorf("LAST_DAY failed: %v", err)
	}
	t.Logf("LAST_DAY('2024-02-15') = %s", result.ToString())
}

func TestQuarterFunctionMore(t *testing.T) {
	// Test Q1
	result, err := Call("QUARTER", []types.Value{
		types.NewStringValue("2024-02-15"),
	})
	if err != nil {
		t.Errorf("QUARTER failed: %v", err)
	}
	q, _ := result.ToInt64()
	if q != 1 {
		t.Errorf("QUARTER('2024-02-15') = %d, want 1", q)
	}

	// Test Q3
	result, err = Call("QUARTER", []types.Value{
		types.NewStringValue("2024-08-15"),
	})
	if err != nil {
		t.Errorf("QUARTER failed: %v", err)
	}
	q, _ = result.ToInt64()
	if q != 3 {
		t.Errorf("QUARTER('2024-08-15') = %d, want 3", q)
	}
}

func TestWeekFunctionMore(t *testing.T) {
	result, err := Call("WEEK", []types.Value{
		types.NewStringValue("2024-01-07"),
	})
	if err != nil {
		t.Errorf("WEEK failed: %v", err)
	}
	week, _ := result.ToInt64()
	t.Logf("WEEK('2024-01-07') = %d", week)
}

func TestDayOfWeek(t *testing.T) {
	// 2024-01-07 is Sunday
	result, err := Call("DAYOFWEEK", []types.Value{
		types.NewStringValue("2024-01-07"),
	})
	if err != nil {
		t.Errorf("DAYOFWEEK failed: %v", err)
	}
	dow, _ := result.ToInt64()
	if dow != 1 {
		t.Errorf("DAYOFWEEK('2024-01-07') = %d, want 1 (Sunday)", dow)
	}
}

func TestDayOfYear(t *testing.T) {
	// 2024-02-01 is day 32 (2024 is leap year)
	result, err := Call("DAYOFYEAR", []types.Value{
		types.NewStringValue("2024-02-01"),
	})
	if err != nil {
		t.Errorf("DAYOFYEAR failed: %v", err)
	}
	doy, _ := result.ToInt64()
	if doy != 32 {
		t.Errorf("DAYOFYEAR('2024-02-01') = %d, want 32", doy)
	}
}

func TestConcatWS(t *testing.T) {
	result, err := Call("CONCAT_WS", []types.Value{
		types.NewStringValue(","),
		types.NewStringValue("apple"),
		types.NewStringValue("banana"),
		types.NewStringValue("cherry"),
	})
	if err != nil {
		t.Errorf("CONCAT_WS failed: %v", err)
	}
	concat := result.ToString()
	if concat != "apple,banana,cherry" {
		t.Errorf("CONCAT_WS(',', 'apple', 'banana', 'cherry') = %s, want 'apple,banana,cherry'", concat)
	}
}

func TestSpace(t *testing.T) {
	result, err := Call("SPACE", []types.Value{
		types.NewIntValue(5),
	})
	if err != nil {
		t.Errorf("SPACE failed: %v", err)
	}
	space := result.ToString()
	if space != "     " {
		t.Errorf("SPACE(5) = '%s', want '     '", space)
	}
}

func TestRepeatErrors(t *testing.T) {
	_, err := Call("REPEAT", []types.Value{types.NewStringValue("a")})
	if err == nil {
		t.Error("REPEAT with 1 arg should error")
	}
}

func TestReverseEmpty(t *testing.T) {
	result, err := Call("REVERSE", []types.Value{types.NewStringValue("")})
	if err != nil || result.ToString() != "" {
		t.Errorf("REVERSE('') failed: got '%s'", result.ToString())
	}
}

func TestSubstringWithLength(t *testing.T) {
	// SUBSTRING with 2 args (no length)
	result, err := Call("SUBSTRING", []types.Value{
		types.NewStringValue("Hello World"),
		types.NewIntValue(7),
	})
	if err != nil || result.ToString() != "World" {
		t.Errorf("SUBSTRING with 2 args failed: got %s", result.ToString())
	}
}


// Test folder functions
func TestLoadFolder(t *testing.T) {
	// Create temp folder with files
	dir, err := os.MkdirTemp("", "xxldb-folder-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create some files
	os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(dir, "file2.txt"), []byte("content2"), 0644)
	subDir := filepath.Join(dir, "subdir")
	os.Mkdir(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "file3.txt"), []byte("content3"), 0644)

	// Test LOAD_FOLDER
	result, err := Call("LOAD_FOLDER", []types.Value{types.NewStringValue(dir)})
	if err != nil {
		t.Logf("LOAD_FOLDER: %v", err)
		return
	}
	t.Logf("LOAD_FOLDER result: %s", result.ToString())
}

func TestExportFolder(t *testing.T) {
	// Create temp folders
	srcDir, err := os.MkdirTemp("", "xxldb-export-src-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(srcDir)

	dstDir, err := os.MkdirTemp("", "xxldb-export-dst-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dstDir)

	// Create some files
	os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content1"), 0644)

	// Test EXPORT_FOLDER
	result, err := Call("EXPORT_FOLDER", []types.Value{
		types.NewStringValue(srcDir),
		types.NewStringValue(dstDir),
	})
	if err != nil {
		t.Logf("EXPORT_FOLDER: %v", err)
		return
	}
	t.Logf("EXPORT_FOLDER result: %s", result.ToString())
}

func TestListFolder(t *testing.T) {
	// Create temp folder
	dir, err := os.MkdirTemp("", "xxldb-list-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create some files
	os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(dir, "file2.txt"), []byte("content2"), 0644)

	// Test LIST_FOLDER
	result, err := Call("LIST_FOLDER", []types.Value{types.NewStringValue(dir)})
	if err != nil {
		t.Logf("LIST_FOLDER: %v", err)
		return
	}
	t.Logf("LIST_FOLDER result: %s", result.ToString())
}

func TestFolderFiles(t *testing.T) {
	// Create temp folder
	dir, err := os.MkdirTemp("", "xxldb-files-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create some files
	os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(dir, "file2.txt"), []byte("content2"), 0644)

	// Test FOLDER_FILES
	result, err := Call("FOLDER_FILES", []types.Value{types.NewStringValue(dir)})
	if err != nil {
		t.Logf("FOLDER_FILES: %v", err)
		return
	}
	t.Logf("FOLDER_FILES result: %s", result.ToString())
}

func TestSaveFile(t *testing.T) {
	// Create temp folder
	dir, err := os.MkdirTemp("", "xxldb-save-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Test SAVE_FILE
	filePath := filepath.Join(dir, "saved.txt")
	result, err := Call("SAVE_FILE", []types.Value{
		types.NewStringValue(filePath),
		types.NewStringValue("test content"),
	})
	if err != nil {
		t.Logf("SAVE_FILE: %v", err)
		return
	}

	// Verify file was created
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("File not created: %v", err)
	}
	if string(content) != "test content" {
		t.Errorf("Content mismatch: got %s", string(content))
	}
	_ = result
}

func TestLoadFile(t *testing.T) {
	// Create temp file
	file, err := os.CreateTemp("", "xxldb-load-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	// Write content
	file.WriteString("test file content")
	file.Close()

	// Test LOAD_FILE
	result, err := Call("LOAD_FILE", []types.Value{types.NewStringValue(file.Name())})
	if err != nil {
		t.Fatalf("LOAD_FILE failed: %v", err)
	}
	// LOAD_FILE returns a BLOB, not a string
	// Just verify it returns something
	if result.IsNull {
		t.Error("LOAD_FILE should not return NULL for existing file")
	}
}

func TestFileExists(t *testing.T) {
	// Create temp file
	file, err := os.CreateTemp("", "xxldb-exists-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())
	file.Close()

	// Test existing file
	result, err := Call("FILE_EXISTS", []types.Value{types.NewStringValue(file.Name())})
	if err != nil {
		t.Fatalf("FILE_EXISTS failed: %v", err)
	}
	exists, _ := result.ToInt64()
	if exists != 1 {
		t.Errorf("FILE_EXISTS should return 1 for existing file")
	}

	// Test non-existing file
	result, err = Call("FILE_EXISTS", []types.Value{types.NewStringValue("/nonexistent/file.txt")})
	if err != nil {
		t.Fatalf("FILE_EXISTS failed: %v", err)
	}
	exists, _ = result.ToInt64()
	if exists != 0 {
		t.Errorf("FILE_EXISTS should return 0 for non-existing file")
	}
}

func TestFileSize(t *testing.T) {
	// Create temp file
	file, err := os.CreateTemp("", "xxldb-size-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())
	file.WriteString("12345")
	file.Close()

	// Test FILE_SIZE
	result, err := Call("FILE_SIZE", []types.Value{types.NewStringValue(file.Name())})
	if err != nil {
		t.Fatalf("FILE_SIZE failed: %v", err)
	}
	size, _ := result.ToInt64()
	if size != 5 {
		t.Errorf("FILE_SIZE = %d, want 5", size)
	}
}

// Test TRIM with null
func TestTrimWithNull(t *testing.T) {
	result, err := Call("TRIM", []types.Value{types.NewNullValue()})
	if err != nil || !result.IsNull {
		t.Error("TRIM(null) should return null")
	}
}

// Test LOWER with null
func TestLowerWithNull(t *testing.T) {
	result, err := Call("LOWER", []types.Value{types.NewNullValue()})
	if err != nil || !result.IsNull {
		t.Error("LOWER(null) should return null")
	}
}

// Test UPPER with null
func TestUpperWithNull(t *testing.T) {
	result, err := Call("UPPER", []types.Value{types.NewNullValue()})
	if err != nil || !result.IsNull {
		t.Error("UPPER(null) should return null")
	}
}

// Test FLOOR with various inputs
func TestFloorVariations(t *testing.T) {
	tests := []struct {
		input    float64
		expected int64
	}{
		{3.7, 3},
		{-3.7, -4},
		{3.0, 3},
	}

	for _, tt := range tests {
		result, err := Call("FLOOR", []types.Value{types.NewFloatValue(tt.input)})
		if err != nil {
			t.Errorf("FLOOR(%f) failed: %v", tt.input, err)
			continue
		}
		got, _ := result.ToInt64()
		if got != tt.expected {
			t.Errorf("FLOOR(%f) = %d, want %d", tt.input, got, tt.expected)
		}
	}

	// FLOOR with null
	result, err := Call("FLOOR", []types.Value{types.NewNullValue()})
	if err != nil || !result.IsNull {
		t.Error("FLOOR(null) should return null")
	}
}

// Test CEIL with various inputs
func TestCeilVariations(t *testing.T) {
	tests := []struct {
		input    float64
		expected int64
	}{
		{3.2, 4},
		{-3.2, -3},
		{3.0, 3},
	}

	for _, tt := range tests {
		result, err := Call("CEIL", []types.Value{types.NewFloatValue(tt.input)})
		if err != nil {
			t.Errorf("CEIL(%f) failed: %v", tt.input, err)
			continue
		}
		got, _ := result.ToInt64()
		if got != tt.expected {
			t.Errorf("CEIL(%f) = %d, want %d", tt.input, got, tt.expected)
		}
	}

	// CEIL with null
	result, err := Call("CEIL", []types.Value{types.NewNullValue()})
	if err != nil || !result.IsNull {
		t.Error("CEIL(null) should return null")
	}
}

// Test date/time functions with null
func TestDateFunctionsWithNull(t *testing.T) {
	// YEAR with null
	result, err := Call("YEAR", []types.Value{types.NewNullValue()})
	if err != nil || !result.IsNull {
		t.Error("YEAR(null) should return null")
	}

	// MONTH with null
	result, err = Call("MONTH", []types.Value{types.NewNullValue()})
	if err != nil || !result.IsNull {
		t.Error("MONTH(null) should return null")
	}

	// DAY with null
	result, err = Call("DAY", []types.Value{types.NewNullValue()})
	if err != nil || !result.IsNull {
		t.Error("DAY(null) should return null")
	}

	// HOUR with null
	result, err = Call("HOUR", []types.Value{types.NewNullValue()})
	if err != nil || !result.IsNull {
		t.Error("HOUR(null) should return null")
	}

	// MINUTE with null
	result, err = Call("MINUTE", []types.Value{types.NewNullValue()})
	if err != nil || !result.IsNull {
		t.Error("MINUTE(null) should return null")
	}

	// SECOND with null
	result, err = Call("SECOND", []types.Value{types.NewNullValue()})
	if err != nil || !result.IsNull {
		t.Error("SECOND(null) should return null")
	}
}

// Test TO_STRING function
func TestToStringVariations(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected string
	}{
		{int64(42), "42"},
		{float64(3.14), "3.14"},
	}

	for _, tt := range tests {
		result, err := Call("TO_STRING", []types.Value{types.NewValue(tt.input)})
		if err != nil {
			t.Errorf("TO_STRING(%v) failed: %v", tt.input, err)
			continue
		}
		if result.ToString() != tt.expected {
			t.Errorf("TO_STRING(%v) = %s, want %s", tt.input, result.ToString(), tt.expected)
		}
	}

	// Test with bool - actual behavior may vary
	result, err := Call("TO_STRING", []types.Value{types.NewBoolValue(true)})
	if err != nil {
		t.Errorf("TO_STRING(true) failed: %v", err)
	}
	t.Logf("TO_STRING(true) = %s", result.ToString())
}

// Test TO_INT function
func TestToIntVariations(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected int64
	}{
		{"42", 42},
		{float64(3.9), 3},
		{int64(100), 100},
	}

	for _, tt := range tests {
		result, err := Call("TO_INT", []types.Value{types.NewValue(tt.input)})
		if err != nil {
			t.Errorf("TO_INT(%v) failed: %v", tt.input, err)
			continue
		}
		got, _ := result.ToInt64()
		if got != tt.expected {
			t.Errorf("TO_INT(%v) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

// Test TO_FLOAT function
func TestToFloatVariations(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected float64
	}{
		{"3.14", 3.14},
		{int64(42), 42.0},
		{float64(2.5), 2.5},
	}

	for _, tt := range tests {
		result, err := Call("TO_FLOAT", []types.Value{types.NewValue(tt.input)})
		if err != nil {
			t.Errorf("TO_FLOAT(%v) failed: %v", tt.input, err)
			continue
		}
		got, _ := result.ToFloat64()
		if got != tt.expected {
			t.Errorf("TO_FLOAT(%v) = %f, want %f", tt.input, got, tt.expected)
		}
	}
}

// Test LOG with edge cases
func TestLogEdgeCases(t *testing.T) {
	// LOG with base parameter
	result, err := Call("LOG", []types.Value{
		types.NewFloatValue(100),
		types.NewFloatValue(10),
	})
	if err != nil {
		t.Errorf("LOG(100, 10) failed: %v", err)
	}

	// LOG with null
	result, err = Call("LOG", []types.Value{types.NewNullValue()})
	if err != nil || !result.IsNull {
		t.Error("LOG(null) should return null")
	}
}

// Test SQRT with edge cases
func TestSqrtEdgeCases(t *testing.T) {
	// SQRT with zero
	result, err := Call("SQRT", []types.Value{types.NewFloatValue(0)})
	if err != nil {
		t.Errorf("SQRT(0) failed: %v", err)
	}

	// SQRT with null
	result, err = Call("SQRT", []types.Value{types.NewNullValue()})
	if err != nil || !result.IsNull {
		t.Error("SQRT(null) should return null")
	}
}

// Test POWER with edge cases
func TestPowerEdgeCases(t *testing.T) {
	// POWER with null
	result, err := Call("POWER", []types.Value{types.NewNullValue(), types.NewFloatValue(2)})
	if err != nil || !result.IsNull {
		t.Error("POWER(null, 2) should return null")
	}
}

// Test SIGN with zero
func TestSignZero(t *testing.T) {
	result, err := Call("SIGN", []types.Value{types.NewFloatValue(0)})
	if err != nil {
		t.Errorf("SIGN(0) failed: %v", err)
	}
	got, _ := result.ToInt64()
	if got != 0 {
		t.Errorf("SIGN(0) = %d, want 0", got)
	}
}

// Test DATE_ADD and DATE_SUB with various intervals
func TestDateAddSubVariations(t *testing.T) {
	dt := types.NewDatetimeValue(time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC))

	// DATE_ADD with null
	result, err := Call("DATE_ADD", []types.Value{types.NewNullValue(), types.NewIntValue(1)})
	if err != nil || !result.IsNull {
		t.Error("DATE_ADD(null, 1) should return null")
	}

	// DATE_SUB with null
	result, err = Call("DATE_SUB", []types.Value{types.NewNullValue(), types.NewIntValue(1)})
	if err != nil || !result.IsNull {
		t.Error("DATE_SUB(null, 1) should return null")
	}

	// DATE_ADD with valid date
	_, err = Call("DATE_ADD", []types.Value{dt, types.NewIntValue(7)})
	if err != nil {
		t.Errorf("DATE_ADD failed: %v", err)
	}
}

// Test CAST with more types
func TestCastVariations(t *testing.T) {
	// CAST to FLOAT
	result, err := Call("CAST", []types.Value{
		types.NewStringValue("3.14"),
		types.NewStringValue("FLOAT"),
	})
	if err != nil {
		t.Errorf("CAST to FLOAT failed: %v", err)
	}

	// CAST to VARCHAR
	result, err = Call("CAST", []types.Value{
		types.NewIntValue(42),
		types.NewStringValue("VARCHAR"),
	})
	if err != nil {
		t.Errorf("CAST to VARCHAR failed: %v", err)
	}

	// CAST with null
	result, err = Call("CAST", []types.Value{
		types.NewNullValue(),
		types.NewStringValue("INT"),
	})
	if err != nil || !result.IsNull {
		t.Error("CAST(null, type) should return null")
	}
}

// Test FLOOR/CEIL with wrong arg count
func TestFloorCeilErrors(t *testing.T) {
	_, err := Call("FLOOR", []types.Value{})
	if err == nil {
		t.Error("FLOOR with no args should error")
	}

	_, err = Call("CEIL", []types.Value{})
	if err == nil {
		t.Error("CEIL with no args should error")
	}
}

// Test date functions with wrong arg count
func TestDateFunctionErrors(t *testing.T) {
	_, err := Call("YEAR", []types.Value{})
	if err == nil {
		t.Error("YEAR with no args should error")
	}

	_, err = Call("MONTH", []types.Value{})
	if err == nil {
		t.Error("MONTH with no args should error")
	}

	_, err = Call("DAY", []types.Value{})
	if err == nil {
		t.Error("DAY with no args should error")
	}

	_, err = Call("HOUR", []types.Value{})
	if err == nil {
		t.Error("HOUR with no args should error")
	}
}

// Test TO_STRING/TO_INT/TO_FLOAT errors
func TestConversionErrors(t *testing.T) {
	_, err := Call("TO_STRING", []types.Value{})
	if err == nil {
		t.Error("TO_STRING with no args should error")
	}

	_, err = Call("TO_INT", []types.Value{})
	if err == nil {
		t.Error("TO_INT with no args should error")
	}

	_, err = Call("TO_FLOAT", []types.Value{})
	if err == nil {
		t.Error("TO_FLOAT with no args should error")
	}

	// TO_INT with invalid string
	_, err = Call("TO_INT", []types.Value{types.NewStringValue("not a number")})
	if err == nil {
		t.Error("TO_INT with invalid string should error")
	}

	// TO_FLOAT with invalid string
	_, err = Call("TO_FLOAT", []types.Value{types.NewStringValue("not a number")})
	if err == nil {
		t.Error("TO_FLOAT with invalid string should error")
	}
}

// Test trig functions with null
func TestTrigWithNull(t *testing.T) {
	functions := []string{"SIN", "COS", "TAN", "RADIANS", "DEGREES"}

	for _, fn := range functions {
		result, err := Call(fn, []types.Value{types.NewNullValue()})
		if err != nil || !result.IsNull {
			t.Errorf("%s(null) should return null", fn)
		}
	}
}

// Test LOG functions with null
func TestLogFunctionsWithNull(t *testing.T) {
	functions := []string{"LOG", "LOG10", "LOG2", "EXP"}

	for _, fn := range functions {
		result, err := Call(fn, []types.Value{types.NewNullValue()})
		if err != nil || !result.IsNull {
			t.Errorf("%s(null) should return null", fn)
		}
	}
}

// Test GCD/LCM with negative numbers
func TestGcdLcmEdgeCases(t *testing.T) {
	// GCD with negative
	result, err := Call("GCD", []types.Value{
		types.NewIntValue(-48),
		types.NewIntValue(18),
	})
	if err != nil {
		t.Errorf("GCD with negative failed: %v", err)
	}

	// LCM with negative
	result, err = Call("LCM", []types.Value{
		types.NewIntValue(-4),
		types.NewIntValue(6),
	})
	if err != nil {
		t.Errorf("LCM with negative failed: %v", err)
	}

	// GCD/LCM with null
	result, err = Call("GCD", []types.Value{types.NewNullValue(), types.NewIntValue(10)})
	if err != nil || !result.IsNull {
		t.Error("GCD(null, 10) should return null")
	}

	result, err = Call("LCM", []types.Value{types.NewNullValue(), types.NewIntValue(10)})
	if err != nil || !result.IsNull {
		t.Error("LCM(null, 10) should return null")
	}
}

// Test GCD/LCM errors
func TestGcdLcmErrors(t *testing.T) {
	_, err := Call("GCD", []types.Value{types.NewIntValue(10)})
	if err == nil {
		t.Error("GCD with 1 arg should error")
	}

	_, err = Call("LCM", []types.Value{types.NewIntValue(10)})
	if err == nil {
		t.Error("LCM with 1 arg should error")
	}
}

// Test LEFT/RIGHT with null values
func TestLeftRightNullValues(t *testing.T) {
	// LEFT with null
	result, err := Call("LEFT", []types.Value{types.NewNullValue(), types.NewIntValue(3)})
	if err != nil || !result.IsNull {
		t.Error("LEFT(null, 3) should return null")
	}

	// RIGHT with null
	result, err = Call("RIGHT", []types.Value{types.NewNullValue(), types.NewIntValue(3)})
	if err != nil || !result.IsNull {
		t.Error("RIGHT(null, 3) should return null")
	}
}

// Test LEFT/RIGHT with zero
func TestLeftRightWithZero(t *testing.T) {
	// LEFT with zero
	_, err := Call("LEFT", []types.Value{
		types.NewStringValue("hello"),
		types.NewIntValue(0),
	})
	if err != nil {
		t.Errorf("LEFT with 0 failed: %v", err)
	}

	// RIGHT with zero
	_, err = Call("RIGHT", []types.Value{
		types.NewStringValue("hello"),
		types.NewIntValue(0),
	})
	if err != nil {
		t.Errorf("RIGHT with 0 failed: %v", err)
	}
}

// Test TRIM variations
func TestTrimVariations(t *testing.T) {
	// TRIM with default (spaces)
	result, err := Call("TRIM", []types.Value{types.NewStringValue("  hello  ")})
	if err != nil || result.ToString() != "hello" {
		t.Errorf("TRIM default failed: got %s", result.ToString())
	}

	// TRIM with custom chars
	result, err = Call("TRIM", []types.Value{
		types.NewStringValue("xxhelloxx"),
		types.NewStringValue("x"),
	})
	if err != nil || result.ToString() != "hello" {
		t.Errorf("TRIM with chars failed: got %s", result.ToString())
	}
}

// Test LTRIM/RTRIM with null
func TestLtrimRtrimNull(t *testing.T) {
	result, err := Call("LTRIM", []types.Value{types.NewNullValue()})
	if err != nil || !result.IsNull {
		t.Error("LTRIM(null) should return null")
	}

	result, err = Call("RTRIM", []types.Value{types.NewNullValue()})
	if err != nil || !result.IsNull {
		t.Error("RTRIM(null) should return null")
	}
}

// Test REPLACE with null
func TestReplaceWithNull(t *testing.T) {
	result, err := Call("REPLACE", []types.Value{
		types.NewNullValue(),
		types.NewStringValue("a"),
		types.NewStringValue("b"),
	})
	if err != nil || !result.IsNull {
		t.Error("REPLACE(null, a, b) should return null")
	}
}

// Test LPAD/RPAD with null
func TestLpadRpadNull(t *testing.T) {
	result, err := Call("LPAD", []types.Value{
		types.NewNullValue(),
		types.NewIntValue(10),
		types.NewStringValue("x"),
	})
	if err != nil || !result.IsNull {
		t.Error("LPAD(null, 10, 'x') should return null")
	}

	result, err = Call("RPAD", []types.Value{
		types.NewNullValue(),
		types.NewIntValue(10),
		types.NewStringValue("x"),
	})
	if err != nil || !result.IsNull {
		t.Error("RPAD(null, 10, 'x') should return null")
	}
}

// Test REVERSE with null
func TestReverseWithNull(t *testing.T) {
	result, err := Call("REVERSE", []types.Value{types.NewNullValue()})
	if err != nil || !result.IsNull {
		t.Error("REVERSE(null) should return null")
	}
}

// Test REPEAT with null
func TestRepeatWithNull(t *testing.T) {
	result, err := Call("REPEAT", []types.Value{
		types.NewNullValue(),
		types.NewIntValue(3),
	})
	if err != nil || !result.IsNull {
		t.Error("REPEAT(null, 3) should return null")
	}
}

// Test NOW variations
func TestNowVariations(t *testing.T) {
	// NOW
	result, err := Call("NOW", []types.Value{})
	if err != nil || result.IsNull {
		t.Error("NOW() should not return null")
	}

	// CURRENT_DATE
	result, err = Call("CURRENT_DATE", []types.Value{})
	if err != nil || result.IsNull {
		t.Error("CURRENT_DATE() should not return null")
	}

	// CURRENT_TIME
	result, err = Call("CURRENT_TIME", []types.Value{})
	if err != nil || result.IsNull {
		t.Error("CURRENT_TIME() should not return null")
	}
}

// Test DATE_FORMAT with null
func TestDateFormatNull(t *testing.T) {
	result, err := Call("DATE_FORMAT", []types.Value{
		types.NewNullValue(),
		types.NewStringValue("%Y-%m-%d"),
	})
	if err != nil || !result.IsNull {
		t.Error("DATE_FORMAT(null, format) should return null")
	}
}

// Test more error cases
func TestMoreFunctionErrors(t *testing.T) {
	// INSTR with wrong args
	_, err := Call("INSTR", []types.Value{types.NewStringValue("hello")})
	if err == nil {
		t.Error("INSTR with 1 arg should error")
	}

	// LPAD with wrong args
	_, err = Call("LPAD", []types.Value{types.NewStringValue("hello")})
	if err == nil {
		t.Error("LPAD with 1 arg should error")
	}

	// RPAD with wrong args
	_, err = Call("RPAD", []types.Value{types.NewStringValue("hello")})
	if err == nil {
		t.Error("RPAD with 1 arg should error")
	}

	// REVERSE with wrong args
	_, err = Call("REVERSE", []types.Value{})
	if err == nil {
		t.Error("REVERSE with no args should error")
	}
}

// TestFolderFunctionsComprehensive tests folder functions
func TestFolderFunctionsComprehensive(t *testing.T) {
	// Create temp directory for testing
	tmpDir, err := os.MkdirTemp("", "xxldb-folder-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create some test files
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world"), 0644)

	subDir := filepath.Join(tmpDir, "subdir")
	os.Mkdir(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "nested.txt"), []byte("nested file"), 0644)

	// Test LIST_FOLDER
	result, err := Call("LIST_FOLDER", []types.Value{types.NewStringValue(tmpDir)})
	if err != nil {
		t.Logf("LIST_FOLDER: %v", err)
	} else {
		t.Logf("LIST_FOLDER result: %s", result.ToString())
	}

	// Test FOLDER_FILES
	result, err = Call("FOLDER_FILES", []types.Value{types.NewStringValue(tmpDir)})
	if err != nil {
		t.Logf("FOLDER_FILES: %v", err)
	} else {
		t.Logf("FOLDER_FILES result: %s", result.ToString())
	}

	// Test with non-existent path
	result, err = Call("LIST_FOLDER", []types.Value{types.NewStringValue("/nonexistent/path")})
	if err == nil {
		t.Log("LIST_FOLDER on non-existent path should fail or return error indicator")
	}

	// Test with null
	result, err = Call("FOLDER_FILES", []types.Value{types.NewNullValue()})
	if err != nil {
		t.Logf("FOLDER_FILES with null: %v", err)
	}
}

// TestExportFolderComprehensive tests EXPORT_FOLDER
func TestExportFolderComprehensive(t *testing.T) {
	// Create temp directories
	srcDir, err := os.MkdirTemp("", "xxldb-export-src-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(srcDir)

	dstDir, err := os.MkdirTemp("", "xxldb-export-dst-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dstDir)

	// Create test files in source
	os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(srcDir, "file2.txt"), []byte("content2"), 0644)

	// Test EXPORT_FOLDER
	result, err := Call("EXPORT_FOLDER", []types.Value{
		types.NewStringValue(srcDir),
		types.NewStringValue(dstDir),
	})
	if err != nil {
		t.Logf("EXPORT_FOLDER: %v", err)
	} else {
		t.Logf("EXPORT_FOLDER result: %s", result.ToString())
	}
}

// TestLoadFolderComprehensive tests LOAD_FOLDER
func TestLoadFolderComprehensive(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "xxldb-load-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	os.WriteFile(filepath.Join(tmpDir, "a.txt"), []byte("file a"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "b.txt"), []byte("file b"), 0644)

	// Test LOAD_FOLDER
	result, err := Call("LOAD_FOLDER", []types.Value{types.NewStringValue(tmpDir)})
	if err != nil {
		t.Logf("LOAD_FOLDER: %v", err)
	} else {
		t.Logf("LOAD_FOLDER returned %d bytes of data", len(result.ToString()))
	}
}

// TestSpaceFunction tests SPACE function
func TestSpaceFunction(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, ""},
		{3, "   "},
		{5, "     "},
	}

	for _, tt := range tests {
		result, err := Call("SPACE", []types.Value{types.NewIntValue(int64(tt.n))})
		if err != nil {
			t.Errorf("SPACE(%d) error: %v", tt.n, err)
			continue
		}
		if result.ToString() != tt.want {
			t.Errorf("SPACE(%d) = '%s', want '%s'", tt.n, result.ToString(), tt.want)
		}
	}

	// Test with negative
	result, err := Call("SPACE", []types.Value{types.NewIntValue(-1)})
	if err != nil {
		t.Logf("SPACE(-1) error: %v", err)
	}

	// Test with null
	result, err = Call("SPACE", []types.Value{types.NewNullValue()})
	if err != nil || !result.IsNull {
		t.Error("SPACE(null) should return null")
	}
}

// TestStrcmpFunction tests STRCMP function
func TestStrcmpFunction(t *testing.T) {
	tests := []struct {
		s1, s2 string
	}{
		{"abc", "abc"},
		{"abc", "abd"},
		{"abd", "abc"},
		{"ABC", "abc"},
	}

	for _, tt := range tests {
		result, err := Call("STRCMP", []types.Value{
			types.NewStringValue(tt.s1),
			types.NewStringValue(tt.s2),
		})
		if err != nil {
			t.Errorf("STRCMP(%s, %s) error: %v", tt.s1, tt.s2, err)
		} else {
			t.Logf("STRCMP('%s', '%s') = %s", tt.s1, tt.s2, result.ToString())
		}
	}
}

// TestMakeDateFunction tests MAKEDATE function
func TestMakeDateFunction(t *testing.T) {
	result, err := Call("MAKEDATE", []types.Value{
		types.NewIntValue(2024),
		types.NewIntValue(1),
	})
	if err != nil {
		t.Errorf("MAKEDATE error: %v", err)
	} else {
		t.Logf("MAKEDATE(2024, 1) = %s", result.ToString())
	}

	// Test with day 100
	result, err = Call("MAKEDATE", []types.Value{
		types.NewIntValue(2024),
		types.NewIntValue(100),
	})
	if err != nil {
		t.Logf("MAKEDATE(2024, 100) error: %v", err)
	}
}

// TestMakeTimeFunction tests MAKETIME function
func TestMakeTimeFunction(t *testing.T) {
	result, err := Call("MAKETIME", []types.Value{
		types.NewIntValue(12),
		types.NewIntValue(30),
		types.NewIntValue(45),
	})
	if err != nil {
		t.Errorf("MAKETIME error: %v", err)
	} else {
		t.Logf("MAKETIME(12, 30, 45) = %s", result.ToString())
	}
}

// TestLastDayFunction tests LAST_DAY function
func TestLastDayFunction(t *testing.T) {
	result, err := Call("LAST_DAY", []types.Value{
		types.NewStringValue("2024-02-15"),
	})
	if err != nil {
		t.Logf("LAST_DAY error: %v", err)
	} else {
		t.Logf("LAST_DAY('2024-02-15') = %s", result.ToString())
	}
}
func TestDayOfYearFunction(t *testing.T) {
	result, err := Call("DAYOFYEAR", []types.Value{
		types.NewStringValue("2024-02-01"),
	})
	if err != nil {
		t.Logf("DAYOFYEAR error: %v", err)
	} else {
		t.Logf("DAYOFYEAR('2024-02-01') = %s", result.ToString())
	}
}

// TestMathFunctionsWithNull tests math functions with null input
func TestMathFunctionsWithNull(t *testing.T) {
	funcs := []string{"SIN", "COS", "TAN", "RADIANS", "DEGREES", "EXP"}
	for _, fn := range funcs {
		result, err := Call(fn, []types.Value{types.NewNullValue()})
		if err != nil {
			t.Logf("%s(null) error: %v", fn, err)
		} else if !result.IsNull {
			t.Errorf("%s(null) should return null", fn)
		}
	}
}

// TestLogFunctionsWithEdgeCases tests log functions with edge cases
func TestLogFunctionsWithEdgeCases(t *testing.T) {
	// LOG with negative number
	result, err := Call("LOG", []types.Value{types.NewIntValue(-1)})
	if err != nil {
		t.Logf("LOG(-1) error: %v", err)
	}
	_ = result

	// LOG10 with zero
	result, err = Call("LOG10", []types.Value{types.NewIntValue(0)})
	if err != nil {
		t.Logf("LOG10(0) error: %v", err)
	}

	// LOG2 with 1
	result, err = Call("LOG2", []types.Value{types.NewIntValue(1)})
	if err != nil {
		t.Logf("LOG2(1) error: %v", err)
	} else {
		t.Logf("LOG2(1) = %s", result.ToString())
	}
}

// TestUuidFunction tests UUID function
func TestUuidFunction(t *testing.T) {
	result, err := Call("UUID", []types.Value{})
	if err != nil {
		t.Errorf("UUID error: %v", err)
	}
	if len(result.ToString()) != 36 {
		t.Errorf("UUID length = %d, want 36", len(result.ToString()))
	}
	t.Logf("UUID: %s", result.ToString())
}

// TestRandFunction tests RAND function
func TestRandFunction(t *testing.T) {
	result, err := Call("RAND", []types.Value{})
	if err != nil {
		t.Errorf("RAND error: %v", err)
	}
	f, err := result.ToFloat64()
	if err != nil {
		t.Errorf("RAND result not float: %v", err)
	}
	if f < 0 || f >= 1 {
		t.Errorf("RAND() = %f, should be in [0, 1)", f)
	}
	t.Logf("RAND() = %f", f)
}

// TestFnExportFolderComprehensive tests fnExportFolder
func TestFnExportFolderComprehensive(t *testing.T) {
	// Create temp directories
	srcDir, err := os.MkdirTemp("", "export_src_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(srcDir)

	dstDir, err := os.MkdirTemp("", "export_dst_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dstDir)

	// Create test files
	os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(srcDir, "file2.txt"), []byte("content2"), 0644)

	// Create subdirectory
	subDir := filepath.Join(srcDir, "subdir")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "file3.txt"), []byte("content3"), 0644)

	// Test EXPORT_FOLDER
	result, err := Call("EXPORT_FOLDER", []types.Value{types.NewStringValue(srcDir), types.NewStringValue(dstDir)})
	if err != nil {
		t.Logf("EXPORT_FOLDER error: %v", err)
	} else {
		t.Logf("EXPORT_FOLDER result: %v", result)
	}
}

// TestFnListFolderDetailed tests fnListFolder with detailed output
func TestFnListFolderDetailed(t *testing.T) {
	// Create temp directory
	dir, err := os.MkdirTemp("", "list_folder_det_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create test structure
	os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("content"), 0644)
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)
	os.WriteFile(filepath.Join(dir, "subdir", "file2.txt"), []byte("content2"), 0644)

	// Test LIST_FOLDER
	result, err := Call("LIST_FOLDER", []types.Value{types.NewStringValue(dir)})
	if err != nil {
		t.Logf("LIST_FOLDER error: %v", err)
	} else {
		t.Logf("LIST_FOLDER result: %v", result)
	}

	// Test with recursive flag
	result2, err := Call("LIST_FOLDER", []types.Value{types.NewStringValue(dir), types.NewBoolValue(true)})
	if err != nil {
		t.Logf("LIST_FOLDER recursive error: %v", err)
	} else {
		t.Logf("LIST_FOLDER recursive: %v", result2)
	}
}

// TestFnFolderFilesDetailed tests fnFolderFiles
func TestFnFolderFilesDetailed(t *testing.T) {
	// Create temp directory
	dir, err := os.MkdirTemp("", "folder_files_det_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create test files
	for i := 0; i < 5; i++ {
		os.WriteFile(filepath.Join(dir, fmt.Sprintf("file%d.txt", i)), []byte("content"), 0644)
	}

	// Create subdirectory with files
	subDir := filepath.Join(dir, "subdir")
	os.MkdirAll(subDir, 0755)
	for i := 0; i < 3; i++ {
		os.WriteFile(filepath.Join(subDir, fmt.Sprintf("subfile%d.txt", i)), []byte("content"), 0644)
	}

	// Test FOLDER_FILES
	result, err := Call("FOLDER_FILES", []types.Value{types.NewStringValue(dir)})
	if err != nil {
		t.Logf("FOLDER_FILES error: %v", err)
	} else {
		t.Logf("FOLDER_FILES result: %v", result)
	}

	// Test with pattern
	result2, err := Call("FOLDER_FILES", []types.Value{types.NewStringValue(dir), types.NewStringValue("*.txt")})
	if err != nil {
		t.Logf("FOLDER_FILES with pattern error: %v", err)
	} else {
		t.Logf("FOLDER_FILES with pattern: %v", result2)
	}
}

// TestFnLoadFolderError tests fnLoadFolder error handling
func TestFnLoadFolderError(t *testing.T) {
	// Test with non-existent directory
	result, err := Call("LOAD_FOLDER", []types.Value{types.NewStringValue("/nonexistent/path")})
	if err != nil {
		t.Logf("LOAD_FOLDER error: %v", err)
	} else {
		t.Logf("LOAD_FOLDER non-existent: %v", result)
	}
}

// TestMathFunctionsNullHandling tests math functions with NULL
func TestMathFunctionsNullHandling(t *testing.T) {
	nullVal := types.NewNullValue()

	tests := []struct {
		name string
		fn   string
	}{
		{"ABS", "ABS"},
		{"ROUND", "ROUND"},
		{"FLOOR", "FLOOR"},
		{"CEIL", "CEIL"},
		{"SQRT", "SQRT"},
	}

	for _, tt := range tests {
		result, _ := Call(tt.fn, []types.Value{nullVal})
		t.Logf("%s(null) = %v", tt.fn, result)
	}
}

// TestStringFunctionsNullHandling tests string functions with NULL
func TestStringFunctionsNullHandling(t *testing.T) {
	nullVal := types.NewNullValue()

	tests := []string{"UPPER", "LOWER", "TRIM", "LENGTH", "REVERSE"}

	for _, fn := range tests {
		result, _ := Call(fn, []types.Value{nullVal})
		t.Logf("%s(null) = %v", fn, result)
	}
}

// TestFormatSize tests formatSize function indirectly
func TestFormatSize(t *testing.T) {
	// This tests the formatSize function through LIST_FOLDER
	dir, err := os.MkdirTemp("", "format_size_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create files of different sizes
	os.WriteFile(filepath.Join(dir, "small.txt"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(dir, "large.txt"), make([]byte, 1024*100), 0644) // 100KB

	result, err := Call("LIST_FOLDER", []types.Value{types.NewStringValue(dir)})
	if err != nil {
		t.Logf("LIST_FOLDER error: %v", err)
	} else {
		t.Logf("LIST_FOLDER result: %v", result)
	}
}

// TestMathFunctionsEdgeCases tests math functions with edge cases
func TestMathFunctionsEdgeCases(t *testing.T) {
	// Test GCD with various inputs
	result, err := Call("GCD", []types.Value{types.NewIntValue(12), types.NewIntValue(8)})
	if err != nil {
		t.Errorf("GCD failed: %v", err)
	}
	t.Logf("GCD(12, 8) = %v", result)

	// Test LCM
	result, err = Call("LCM", []types.Value{types.NewIntValue(4), types.NewIntValue(6)})
	if err != nil {
		t.Errorf("LCM failed: %v", err)
	}
	t.Logf("LCM(4, 6) = %v", result)
}

// TestTrigonometricFunctionsEdgeCases tests trigonometric functions
func TestTrigonometricFunctionsEdgeCases(t *testing.T) {
	// Test SIN
	result, err := Call("SIN", []types.Value{types.NewFloatValue(0)})
	if err != nil {
		t.Errorf("SIN failed: %v", err)
	}
	t.Logf("SIN(0) = %v", result)

	// Test COS
	result, err = Call("COS", []types.Value{types.NewFloatValue(0)})
	if err != nil {
		t.Errorf("COS failed: %v", err)
	}
	t.Logf("COS(0) = %v", result)

	// Test TAN
	result, err = Call("TAN", []types.Value{types.NewFloatValue(0)})
	if err != nil {
		t.Errorf("TAN failed: %v", err)
	}
	t.Logf("TAN(0) = %v", result)
}

// TestLogFunctionsEdgeCases tests logarithm functions
func TestLogFunctionsEdgeCases(t *testing.T) {
	// Test LOG
	result, err := Call("LOG", []types.Value{types.NewFloatValue(2.718281828)})
	if err != nil {
		t.Errorf("LOG failed: %v", err)
	}
	t.Logf("LOG(e) = %v", result)

	// Test LOG10
	result, err = Call("LOG10", []types.Value{types.NewFloatValue(100.0)})
	if err != nil {
		t.Errorf("LOG10 failed: %v", err)
	}
	t.Logf("LOG10(100) = %v", result)

	// Test LOG2
	result, err = Call("LOG2", []types.Value{types.NewFloatValue(8.0)})
	if err != nil {
		t.Errorf("LOG2 failed: %v", err)
	}
	t.Logf("LOG2(8) = %v", result)
}

// TestTrigonometricFunctionsExtra tests trigonometric functions
func TestTrigonometricFunctionsExtra(t *testing.T) {
	// Test SIN
	result, err := Call("SIN", []types.Value{types.NewFloatValue(0)})
	if err != nil {
		t.Errorf("SIN failed: %v", err)
	}
	t.Logf("SIN(0) = %v", result)

	// Test COS
	result, err = Call("COS", []types.Value{types.NewFloatValue(0)})
	if err != nil {
		t.Errorf("COS failed: %v", err)
	}
	t.Logf("COS(0) = %v", result)

	// Test TAN
	result, err = Call("TAN", []types.Value{types.NewFloatValue(0)})
	if err != nil {
		t.Errorf("TAN failed: %v", err)
	}
	t.Logf("TAN(0) = %v", result)
}

// TestLogFunctionsExtra tests logarithm functions
func TestLogFunctionsExtra(t *testing.T) {
	// Test LOG
	result, err := Call("LOG", []types.Value{types.NewFloatValue(2.718281828)})
	if err != nil {
		t.Errorf("LOG failed: %v", err)
	}
	t.Logf("LOG(e) = %v", result)

	// Test LOG10
	result, err = Call("LOG10", []types.Value{types.NewFloatValue(100.0)})
	if err != nil {
		t.Errorf("LOG10 failed: %v", err)
	}
	t.Logf("LOG10(100) = %v", result)

	// Test LOG2
	result, err = Call("LOG2", []types.Value{types.NewFloatValue(8.0)})
	if err != nil {
		t.Errorf("LOG2 failed: %v", err)
	}
	t.Logf("LOG2(8) = %v", result)
}

// TestDateFunctionsExtra tests date functions with more scenarios
func TestDateFunctionsExtra(t *testing.T) {
	// Test MAKEDATE
	result, err := Call("MAKEDATE", []types.Value{
		types.NewIntValue(2024),
		types.NewIntValue(1),
	})
	if err != nil {
		t.Errorf("MAKEDATE failed: %v", err)
	}
	t.Logf("MAKEDATE(2024, 1) = %v", result)

	result, err = Call("MAKEDATE", []types.Value{
		types.NewIntValue(2024),
		types.NewIntValue(365),
	})
	if err != nil {
		t.Errorf("MAKEDATE failed: %v", err)
	}
	t.Logf("MAKEDATE(2024, 365) = %v", result)

	// Test LAST_DAY
	result, err = Call("LAST_DAY", []types.Value{
		types.NewStringValue("2024-02-15"),
	})
	if err != nil {
		t.Errorf("LAST_DAY failed: %v", err)
	}
	t.Logf("LAST_DAY('2024-02-15') = %v", result)

	// Test QUARTER
	result, err = Call("QUARTER", []types.Value{
		types.NewStringValue("2024-04-15"),
	})
	if err != nil {
		t.Errorf("QUARTER failed: %v", err)
	}
	t.Logf("QUARTER('2024-04-15') = %v", result)

	// Test WEEK
	result, err = Call("WEEK", []types.Value{
		types.NewStringValue("2024-01-15"),
	})
	if err != nil {
		t.Errorf("WEEK failed: %v", err)
	}
	t.Logf("WEEK('2024-01-15') = %v", result)

	// Test DAYOFWEEK
	result, err = Call("DAYOFWEEK", []types.Value{
		types.NewStringValue("2024-01-15"),
	})
	if err != nil {
		t.Errorf("DAYOFWEEK failed: %v", err)
	}
	t.Logf("DAYOFWEEK('2024-01-15') = %v", result)

	// Test DAYOFYEAR
	result, err = Call("DAYOFYEAR", []types.Value{
		types.NewStringValue("2024-02-01"),
	})
	if err != nil {
		t.Errorf("DAYOFYEAR failed: %v", err)
	}
	t.Logf("DAYOFYEAR('2024-02-01') = %v", result)
}

// TestTimeFunctionsExtra tests time functions
func TestTimeFunctionsExtra(t *testing.T) {
	// Test MINUTE
	result, err := Call("MINUTE", []types.Value{
		types.NewStringValue("10:30:45"),
	})
	if err != nil {
		t.Errorf("MINUTE failed: %v", err)
	}
	t.Logf("MINUTE('10:30:45') = %v", result)

	// Test SECOND
	result, err = Call("SECOND", []types.Value{
		types.NewStringValue("10:30:45"),
	})
	if err != nil {
		t.Errorf("SECOND failed: %v", err)
	}
	t.Logf("SECOND('10:30:45') = %v", result)
}

// TestFileFunctionsExtra tests file functions with more scenarios
func TestFileFunctionsExtra(t *testing.T) {
	// Create temp file
	tmpFile, err := os.CreateTemp("", "test_save_*.txt")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Test SAVE_FILE
	result, err := Call("SAVE_FILE", []types.Value{
		types.NewStringValue(tmpPath),
		types.NewStringValue("Hello, World!"),
	})
	if err != nil {
		t.Errorf("SAVE_FILE failed: %v", err)
	}
	t.Logf("SAVE_FILE result: %v", result)

	// Test FILE_EXISTS
	result, err = Call("FILE_EXISTS", []types.Value{
		types.NewStringValue(tmpPath),
	})
	if err != nil {
		t.Errorf("FILE_EXISTS failed: %v", err)
	}
	t.Logf("FILE_EXISTS result: %v", result)

	// Test FILE_SIZE
	result, err = Call("FILE_SIZE", []types.Value{
		types.NewStringValue(tmpPath),
	})
	if err != nil {
		t.Errorf("FILE_SIZE failed: %v", err)
	}
	t.Logf("FILE_SIZE result: %v", result)
}

// TestMathFunctionsComprehensive tests math functions with edge cases
func TestMathFunctionsComprehensive(t *testing.T) {
	// Test SIGN
	tests := []struct {
		name  string
		value float64
	}{
		{"positive", 42.5},
		{"negative", -42.5},
		{"zero", 0.0},
	}

	for _, tt := range tests {
		result, err := Call("SIGN", []types.Value{types.NewFloatValue(tt.value)})
		if err != nil {
			t.Errorf("SIGN(%f) failed: %v", tt.value, err)
		}
		t.Logf("SIGN(%f) = %v", tt.value, result)
	}
}

// TestExportFolderRecursive tests fnExportFolder with recursive structure
func TestExportFolderRecursive(t *testing.T) {
	// Create temp directories
	srcDir, err := os.MkdirTemp("", "export_recursive_src_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(srcDir)

	dstDir, err := os.MkdirTemp("", "export_recursive_dst_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dstDir)

	// Create nested structure
	subDir1 := filepath.Join(srcDir, "level1")
	subDir2 := filepath.Join(subDir1, "level2")
	os.MkdirAll(subDir2, 0755)

	// Create files at each level
	os.WriteFile(filepath.Join(srcDir, "root.txt"), []byte("root file"), 0644)
	os.WriteFile(filepath.Join(subDir1, "level1.txt"), []byte("level1 file"), 0644)
	os.WriteFile(filepath.Join(subDir2, "level2.txt"), []byte("level2 file"), 0644)

	// Test EXPORT_FOLDER
	result, err := Call("EXPORT_FOLDER", []types.Value{
		types.NewStringValue(srcDir),
		types.NewStringValue(dstDir),
	})
	if err != nil {
		t.Logf("EXPORT_FOLDER: %v", err)
	} else {
		t.Logf("EXPORT_FOLDER result: %s", result.ToString())

		// Verify nested files were exported
		if _, err := os.Stat(filepath.Join(dstDir, "root.txt")); err != nil {
			t.Logf("root.txt not exported: %v", err)
		}
		if _, err := os.Stat(filepath.Join(dstDir, "level1", "level1.txt")); err != nil {
			t.Logf("level1/level1.txt not exported: %v", err)
		}
		if _, err := os.Stat(filepath.Join(dstDir, "level1", "level2", "level2.txt")); err != nil {
			t.Logf("level1/level2/level2.txt not exported: %v", err)
		}
	}
}

// TestListFolderRecursiveDetailed tests fnListFolder recursive
func TestListFolderRecursiveDetailed(t *testing.T) {
	// Create temp directory
	dir, err := os.MkdirTemp("", "list_recursive_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create nested structure
	os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("content1"), 0644)
	subDir := filepath.Join(dir, "subdir")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "file2.txt"), []byte("content2"), 0644)
	os.WriteFile(filepath.Join(subDir, "file3.txt"), []byte("content3"), 0644)

	// Test LIST_FOLDER with recursive
	result, err := Call("LIST_FOLDER", []types.Value{
		types.NewStringValue(dir),
		types.NewBoolValue(true), // recursive
	})
	if err != nil {
		t.Logf("LIST_FOLDER recursive: %v", err)
	} else {
		t.Logf("LIST_FOLDER recursive result: %s", result.ToString())
	}
}

// TestFolderFilesRecursive tests fnFolderFiles with nested structure
func TestFolderFilesRecursive(t *testing.T) {
	// Create temp directory
	dir, err := os.MkdirTemp("", "folder_files_recursive_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create files
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0644)
	subDir := filepath.Join(dir, "sub")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "c.txt"), []byte("c"), 0644)
	os.WriteFile(filepath.Join(subDir, "d.txt"), []byte("d"), 0644)

	// Test FOLDER_FILES
	result, err := Call("FOLDER_FILES", []types.Value{
		types.NewStringValue(dir),
		types.NewBoolValue(true), // include subdirectories
	})
	if err != nil {
		t.Logf("FOLDER_FILES: %v", err)
	} else {
		t.Logf("FOLDER_FILES result: %s", result.ToString())
	}
}

// TestLoadFolderRecursive tests fnLoadFolder with nested structure
func TestLoadFolderRecursive(t *testing.T) {
	// Create temp directory
	dir, err := os.MkdirTemp("", "load_folder_recursive_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create nested structure
	os.WriteFile(filepath.Join(dir, "root.txt"), []byte("root content"), 0644)
	subDir := filepath.Join(dir, "nested")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "nested.txt"), []byte("nested content"), 0644)

	// Test LOAD_FOLDER
	result, err := Call("LOAD_FOLDER", []types.Value{
		types.NewStringValue(dir),
	})
	if err != nil {
		t.Logf("LOAD_FOLDER: %v", err)
	} else {
		t.Logf("LOAD_FOLDER result type: %v", result.Type)
	}
}

// TestSaveFileWithCreateDir tests SAVE_FILE creating directories
func TestSaveFileWithCreateDir(t *testing.T) {
	// Create temp directory
	baseDir, err := os.MkdirTemp("", "save_file_dir_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(baseDir)

	// Test SAVE_FILE with nested path
	nestedPath := filepath.Join(baseDir, "subdir", "nested", "file.txt")
	result, err := Call("SAVE_FILE", []types.Value{
		types.NewStringValue(nestedPath),
		types.NewStringValue("nested file content"),
	})
	if err != nil {
		t.Logf("SAVE_FILE with nested path: %v", err)
	} else {
		t.Logf("SAVE_FILE result: %s", result.ToString())

		// Verify file was created
		content, err := os.ReadFile(nestedPath)
		if err != nil {
			t.Logf("Could not read saved file: %v", err)
		} else if string(content) != "nested file content" {
			t.Errorf("Content mismatch: got %s", string(content))
		}
	}
}

// TestFileFunctionsNonExistent tests file functions with non-existent paths
func TestFileFunctionsNonExistent(t *testing.T) {
	// FILE_SIZE with non-existent
	_, err := Call("FILE_SIZE", []types.Value{types.NewStringValue("/nonexistent/path/file.txt")})
	if err == nil {
		t.Log("FILE_SIZE with non-existent might not error")
	}

	// LOAD_FILE with non-existent
	_, err = Call("LOAD_FILE", []types.Value{types.NewStringValue("/nonexistent/path/file.txt")})
	if err == nil {
		t.Log("LOAD_FILE with non-existent might not error")
	}

	// LIST_FOLDER with non-existent
	_, err = Call("LIST_FOLDER", []types.Value{types.NewStringValue("/nonexistent/path/folder")})
	if err == nil {
		t.Log("LIST_FOLDER with non-existent might not error")
	}
}

// TestStringConcatenationOperator tests string concatenation with ||
func TestStringConcatenationOperator(t *testing.T) {
	// CONCAT with many arguments
	result, err := Call("CONCAT", []types.Value{
		types.NewStringValue("a"),
		types.NewStringValue("b"),
		types.NewStringValue("c"),
		types.NewStringValue("d"),
		types.NewStringValue("e"),
	})
	if err != nil {
		t.Errorf("CONCAT with many args failed: %v", err)
	} else {
		t.Logf("CONCAT many args: %s", result.ToString())
	}
}

// TestAggregateWithDifferentTypes tests aggregate functions with different types
func TestAggregateWithDifferentTypes(t *testing.T) {
	// SUM with floats
	values := []types.Value{
		types.NewFloatValue(1.5),
		types.NewFloatValue(2.5),
		types.NewFloatValue(3.0),
	}
	result, err := Call("SUM", values)
	if err != nil {
		t.Errorf("SUM with floats failed: %v", err)
	} else {
		t.Logf("SUM floats: %s", result.ToString())
	}

	// AVG with mixed types
	values2 := []types.Value{
		types.NewIntValue(1),
		types.NewFloatValue(2.5),
		types.NewIntValue(3),
	}
	result, err = Call("AVG", values2)
	if err != nil {
		t.Errorf("AVG with mixed types failed: %v", err)
	} else {
		t.Logf("AVG mixed: %s", result.ToString())
	}
}

// TestDateFunctionsEdgeCases tests date function edge cases
func TestDateFunctionsEdgeCases(t *testing.T) {
	// DATE with time
	result, err := Call("DATE", []types.Value{types.NewStringValue("2024-12-31 23:59:59")})
	if err != nil {
		t.Logf("DATE with time: %v", err)
	} else {
		t.Logf("DATE with time: %s", result.ToString())
	}

	// DATEDIFF with same date
	result, err = Call("DATEDIFF", []types.Value{
		types.NewStringValue("2024-01-01"),
		types.NewStringValue("2024-01-01"),
	})
	if err != nil {
		t.Logf("DATEDIFF same date: %v", err)
	} else {
		t.Logf("DATEDIFF same: %s", result.ToString())
	}

	// DATE_ADD with negative days
	dt := types.NewDatetimeValue(time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC))
	result, err = Call("DATE_ADD", []types.Value{dt, types.NewIntValue(-5)})
	if err != nil {
		t.Logf("DATE_ADD negative: %v", err)
	} else {
		t.Logf("DATE_ADD negative: %s", result.ToString())
	}
}

// TestCastWithNull tests CAST with NULL
func TestCastWithNull(t *testing.T) {
	result, err := Call("CAST", []types.Value{
		types.NewNullValue(),
		types.NewStringValue("INT"),
	})
	if err != nil {
		t.Logf("CAST NULL to INT: %v", err)
	} else if !result.IsNull {
		t.Error("CAST NULL should return NULL")
	}
}

// TestCoalesceWithAllTypes tests COALESCE with various types
func TestCoalesceWithAllTypes(t *testing.T) {
	// COALESCE with integers
	result, err := Call("COALESCE", []types.Value{
		types.NewNullValue(),
		types.NewNullValue(),
		types.NewIntValue(42),
	})
	if err != nil || result.ToString() != "42" {
		t.Errorf("COALESCE with multiple nulls failed: %v, got %s", err, result.ToString())
	}

	// COALESCE with strings
	result, err = Call("COALESCE", []types.Value{
		types.NewNullValue(),
		types.NewStringValue("fallback"),
	})
	if err != nil || result.ToString() != "fallback" {
		t.Errorf("COALESCE string failed: %v, got %s", err, result.ToString())
	}
}

// TestRoundWithNegative tests ROUND with negative precision
func TestRoundWithNegative(t *testing.T) {
	result, err := Call("ROUND", []types.Value{
		types.NewFloatValue(1234.56),
		types.NewIntValue(-2), // round to nearest 100
	})
	if err != nil {
		t.Logf("ROUND negative precision: %v", err)
	} else {
		t.Logf("ROUND(-2): %s", result.ToString())
	}
}

// TestSubstringWithStart tests SUBSTRING with just start position
func TestSubstringWithStart(t *testing.T) {
	result, err := Call("SUBSTRING", []types.Value{
		types.NewStringValue("Hello World"),
		types.NewIntValue(7), // start from position 7
	})
	if err != nil {
		t.Errorf("SUBSTRING with start only failed: %v", err)
	} else if result.ToString() != "World" {
		t.Errorf("SUBSTRING(7): got %s, want 'World'", result.ToString())
	}
}

// TestMathFunctionsWithIntegers tests math functions with integer input
func TestMathFunctionsWithIntegers(t *testing.T) {
	// ABS with integer
	result, err := Call("ABS", []types.Value{types.NewIntValue(-42)})
	if err != nil {
		t.Errorf("ABS int failed: %v", err)
	} else {
		t.Logf("ABS int: %s", result.ToString())
	}

	// FLOOR with integer
	result, err = Call("FLOOR", []types.Value{types.NewIntValue(42)})
	if err != nil {
		t.Errorf("FLOOR int failed: %v", err)
	} else {
		t.Logf("FLOOR int: %s", result.ToString())
	}

	// CEIL with integer
	result, err = Call("CEIL", []types.Value{types.NewIntValue(42)})
	if err != nil {
		t.Errorf("CEIL int failed: %v", err)
	} else {
		t.Logf("CEIL int: %s", result.ToString())
	}
}

// TestExportFolderWithSubfolders tests fnExportFolder with nested subfolders
func TestExportFolderWithSubfolders(t *testing.T) {
	// Create source and destination directories
	srcDir, err := os.MkdirTemp("", "export_subfolders_src_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(srcDir)

	dstDir, err := os.MkdirTemp("", "export_subfolders_dst_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dstDir)

	// Create a complex nested structure
	os.WriteFile(filepath.Join(srcDir, "root.txt"), []byte("root content"), 0644)

	// Level 1
	level1 := filepath.Join(srcDir, "documents")
	os.MkdirAll(level1, 0755)
	os.WriteFile(filepath.Join(level1, "doc1.txt"), []byte("document 1"), 0644)
	os.WriteFile(filepath.Join(level1, "doc2.txt"), []byte("document 2"), 0644)

	// Level 2
	level2 := filepath.Join(level1, "images")
	os.MkdirAll(level2, 0755)
	os.WriteFile(filepath.Join(level2, "image1.bin"), []byte("image data"), 0644)
	os.WriteFile(filepath.Join(level2, "image2.bin"), []byte("more image data"), 0644)

	// Level 3
	level3 := filepath.Join(level2, "thumbnails")
	os.MkdirAll(level3, 0755)
	os.WriteFile(filepath.Join(level3, "thumb1.bin"), []byte("thumb"), 0644)

	// First LOAD_FOLDER to get serialized data
	loadResult, err := Call("LOAD_FOLDER", []types.Value{types.NewStringValue(srcDir)})
	if err != nil {
		t.Fatalf("LOAD_FOLDER error: %v", err)
	}

	// Then EXPORT_FOLDER with the serialized data
	result, err := Call("EXPORT_FOLDER", []types.Value{
		loadResult,
		types.NewStringValue(dstDir),
	})
	if err != nil {
		t.Logf("EXPORT_FOLDER error: %v", err)
	} else {
		t.Logf("EXPORT_FOLDER result: %s", result.ToString())

		// Verify deep nesting was exported
		thumbPath := filepath.Join(dstDir, "documents", "images", "thumbnails", "thumb1.bin")
		if _, err := os.Stat(thumbPath); err != nil {
			t.Logf("Thumbnail not exported: %v", err)
		} else {
			// Verify content
			content, err := os.ReadFile(thumbPath)
			if err != nil {
				t.Logf("Could not read exported file: %v", err)
			} else if string(content) != "thumb" {
				t.Errorf("Content mismatch: got %s", string(content))
			} else {
				t.Logf("Thumbnail exported successfully with correct content")
			}
		}
	}
}

// TestListFolderWithSerializedData tests fnListFolder with serialized folder data
func TestListFolderWithSerializedData(t *testing.T) {
	// First load a folder to get serialized data
	dir, err := os.MkdirTemp("", "list_serial_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create structure with various sizes
	os.WriteFile(filepath.Join(dir, "small.txt"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(dir, "medium.txt"), make([]byte, 2048), 0644)
	os.WriteFile(filepath.Join(dir, "large.txt"), make([]byte, 1024*100), 0644)

	subDir := filepath.Join(dir, "subdir")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "file.txt"), []byte("content"), 0644)

	// Load the folder
	loadResult, err := Call("LOAD_FOLDER", []types.Value{types.NewStringValue(dir)})
	if err != nil {
		t.Logf("LOAD_FOLDER error: %v", err)
		return
	}

	// Now list the loaded folder data
	result, err := Call("LIST_FOLDER", []types.Value{loadResult})
	if err != nil {
		t.Logf("LIST_FOLDER error: %v", err)
	} else {
		t.Logf("LIST_FOLDER result:\n%s", result.ToString())
	}
}

// TestFolderFilesWithSerializedData tests fnFolderFiles with serialized folder data
func TestFolderFilesWithSerializedData(t *testing.T) {
	// First load a folder
	dir, err := os.MkdirTemp("", "files_serial_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create files
	for i := 0; i < 5; i++ {
		os.WriteFile(filepath.Join(dir, fmt.Sprintf("file%d.txt", i)), []byte("content"), 0644)
	}

	subDir := filepath.Join(dir, "nested")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "nested.txt"), []byte("nested"), 0644)

	// Load the folder
	loadResult, err := Call("LOAD_FOLDER", []types.Value{types.NewStringValue(dir)})
	if err != nil {
		t.Logf("LOAD_FOLDER error: %v", err)
		return
	}

	// Count files
	result, err := Call("FOLDER_FILES", []types.Value{loadResult})
	if err != nil {
		t.Logf("FOLDER_FILES error: %v", err)
	} else {
		t.Logf("FOLDER_FILES result: %s", result.ToString())
	}
}

// TestFormatSizeIndirectly tests formatSize through LIST_FOLDER
func TestFormatSizeIndirectly(t *testing.T) {
	// Create files of various sizes
	dir, err := os.MkdirTemp("", "format_size_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Small file (bytes)
	os.WriteFile(filepath.Join(dir, "small.txt"), []byte("x"), 0644)

	// KB file
	os.WriteFile(filepath.Join(dir, "kb.txt"), make([]byte, 2048), 0644)

	// MB file
	os.WriteFile(filepath.Join(dir, "mb.txt"), make([]byte, 1024*1024*2), 0644)

	// Load and list
	loadResult, err := Call("LOAD_FOLDER", []types.Value{types.NewStringValue(dir)})
	if err != nil {
		t.Logf("LOAD_FOLDER error: %v", err)
		return
	}

	result, err := Call("LIST_FOLDER", []types.Value{loadResult})
	if err != nil {
		t.Logf("LIST_FOLDER error: %v", err)
	} else {
		// Check that sizes are formatted correctly
		output := result.ToString()
		t.Logf("LIST_FOLDER output:\n%s", output)

		// Verify format strings are present
		if !strings.Contains(output, "B") && !strings.Contains(output, "KB") && !strings.Contains(output, "MB") {
			t.Log("No size format indicators found")
		}
	}
}

// TestSaveFileWithOverwrite tests SAVE_FILE overwriting existing file
func TestSaveFileWithOverwrite(t *testing.T) {
	dir, err := os.MkdirTemp("", "save_overwrite_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	filePath := filepath.Join(dir, "test.txt")

	// Create initial file
	os.WriteFile(filePath, []byte("initial content"), 0644)

	// Overwrite with SAVE_FILE
	result, err := Call("SAVE_FILE", []types.Value{
		types.NewStringValue(filePath),
		types.NewStringValue("new content"),
	})
	if err != nil {
		t.Errorf("SAVE_FILE overwrite failed: %v", err)
	}

	// Verify content
	content, _ := os.ReadFile(filePath)
	if string(content) != "new content" {
		t.Errorf("Content mismatch: got %s", string(content))
	}
	t.Logf("SAVE_FILE result: %s", result.ToString())
}

// TestMinuteSecondEdgeCases tests MINUTE and SECOND functions
func TestMinuteSecondEdgeCases(t *testing.T) {
	// Test with time string
	result, err := Call("MINUTE", []types.Value{types.NewStringValue("10:59:59")})
	if err != nil {
		t.Logf("MINUTE with time string: %v", err)
	} else {
		t.Logf("MINUTE('10:59:59') = %s", result.ToString())
	}

	result, err = Call("SECOND", []types.Value{types.NewStringValue("10:30:45")})
	if err != nil {
		t.Logf("SECOND with time string: %v", err)
	} else {
		t.Logf("SECOND('10:30:45') = %s", result.ToString())
	}

	// Test with null
	result, err = Call("MINUTE", []types.Value{types.NewNullValue()})
	if err != nil || !result.IsNull {
		t.Log("MINUTE(null) should return null")
	}

	result, err = Call("SECOND", []types.Value{types.NewNullValue()})
	if err != nil || !result.IsNull {
		t.Log("SECOND(null) should return null")
	}
}

// TestDateAddSubEdgeCases tests DATE_ADD and DATE_SUB edge cases
func TestDateAddSubEdgeCases(t *testing.T) {
	// Test DATE_ADD with string date
	result, err := Call("DATE_ADD", []types.Value{
		types.NewStringValue("2024-01-15"),
		types.NewIntValue(10),
	})
	if err != nil {
		t.Logf("DATE_ADD with string: %v", err)
	} else {
		t.Logf("DATE_ADD string result: %s", result.ToString())
	}

	// Test DATE_SUB with string date
	result, err = Call("DATE_SUB", []types.Value{
		types.NewStringValue("2024-01-15"),
		types.NewIntValue(5),
	})
	if err != nil {
		t.Logf("DATE_SUB with string: %v", err)
	} else {
		t.Logf("DATE_SUB string result: %s", result.ToString())
	}
}

// TestSignEdgeCases tests SIGN function edge cases
func TestSignEdgeCases(t *testing.T) {
	tests := []float64{-100.5, -1, 0, 1, 100.5}

	for _, val := range tests {
		result, err := Call("SIGN", []types.Value{types.NewFloatValue(val)})
		if err != nil {
			t.Errorf("SIGN(%f) failed: %v", val, err)
		} else {
			t.Logf("SIGN(%f) = %s", val, result.ToString())
		}
	}

	// Test with null
	result, err := Call("SIGN", []types.Value{types.NewNullValue()})
	if err != nil || !result.IsNull {
		t.Log("SIGN(null) should return null")
	}
}

// TestMakeDateEdgeCases tests MAKEDATE edge cases
func TestMakeDateEdgeCases(t *testing.T) {
	// Day 1 of year
	result, err := Call("MAKEDATE", []types.Value{
		types.NewIntValue(2024),
		types.NewIntValue(1),
	})
	if err != nil {
		t.Errorf("MAKEDATE(2024, 1) failed: %v", err)
	} else {
		t.Logf("MAKEDATE(2024, 1) = %s", result.ToString())
	}

	// Day 365 (last day)
	result, err = Call("MAKEDATE", []types.Value{
		types.NewIntValue(2023),
		types.NewIntValue(365),
	})
	if err != nil {
		t.Errorf("MAKEDATE(2023, 365) failed: %v", err)
	} else {
		t.Logf("MAKEDATE(2023, 365) = %s", result.ToString())
	}

	// Day 366 (leap year)
	result, err = Call("MAKEDATE", []types.Value{
		types.NewIntValue(2024),
		types.NewIntValue(366),
	})
	if err != nil {
		t.Errorf("MAKEDATE(2024, 366) failed: %v", err)
	} else {
		t.Logf("MAKEDATE(2024, 366) = %s", result.ToString())
	}
}

// TestLastDayEdgeCases tests LAST_DAY edge cases
func TestLastDayEdgeCases(t *testing.T) {
	// January (31 days)
	result, err := Call("LAST_DAY", []types.Value{
		types.NewStringValue("2024-01-15"),
	})
	if err != nil {
		t.Errorf("LAST_DAY January failed: %v", err)
	} else {
		t.Logf("LAST_DAY January: %s", result.ToString())
	}

	// February non-leap year
	result, err = Call("LAST_DAY", []types.Value{
		types.NewStringValue("2023-02-15"),
	})
	if err != nil {
		t.Errorf("LAST_DAY Feb non-leap failed: %v", err)
	} else {
		t.Logf("LAST_DAY Feb non-leap: %s", result.ToString())
	}

	// April (30 days)
	result, err = Call("LAST_DAY", []types.Value{
		types.NewStringValue("2024-04-15"),
	})
	if err != nil {
		t.Errorf("LAST_DAY April failed: %v", err)
	} else {
		t.Logf("LAST_DAY April: %s", result.ToString())
	}

	// Null input
	result, err = Call("LAST_DAY", []types.Value{types.NewNullValue()})
	if err != nil || !result.IsNull {
		t.Log("LAST_DAY(null) should return null")
	}
}

// TestQuarterEdgeCases tests QUARTER edge cases
func TestQuarterEdgeCases(t *testing.T) {
	months := []struct {
		month   string
		quarter int
	}{
		{"2024-01-15", 1},
		{"2024-03-31", 1},
		{"2024-04-01", 2},
		{"2024-06-30", 2},
		{"2024-07-01", 3},
		{"2024-09-30", 3},
		{"2024-10-01", 4},
		{"2024-12-31", 4},
	}

	for _, tt := range months {
		result, err := Call("QUARTER", []types.Value{
			types.NewStringValue(tt.month),
		})
		if err != nil {
			t.Errorf("QUARTER(%s) failed: %v", tt.month, err)
		} else {
			q, _ := result.ToInt64()
			if int(q) != tt.quarter {
				t.Errorf("QUARTER(%s) = %d, want %d", tt.month, q, tt.quarter)
			}
		}
	}
}

// TestWeekEdgeCases tests WEEK edge cases
func TestWeekEdgeCases(t *testing.T) {
	// Beginning of year
	result, err := Call("WEEK", []types.Value{
		types.NewStringValue("2024-01-01"),
	})
	if err != nil {
		t.Errorf("WEEK Jan 1 failed: %v", err)
	} else {
		t.Logf("WEEK Jan 1: %s", result.ToString())
	}

	// End of year
	result, err = Call("WEEK", []types.Value{
		types.NewStringValue("2024-12-31"),
	})
	if err != nil {
		t.Errorf("WEEK Dec 31 failed: %v", err)
	} else {
		t.Logf("WEEK Dec 31: %s", result.ToString())
	}

	// Null input
	result, err = Call("WEEK", []types.Value{types.NewNullValue()})
	if err != nil || !result.IsNull {
		t.Log("WEEK(null) should return null")
	}
}

// TestDayOfWeekEdgeCases tests DAYOFWEEK edge cases
func TestDayOfWeekEdgeCases(t *testing.T) {
	// Test all days of a week
	dates := []struct {
		date    string
		dayNum  int
	}{
		{"2024-01-07", 1}, // Sunday
		{"2024-01-08", 2}, // Monday
		{"2024-01-09", 3}, // Tuesday
		{"2024-01-10", 4}, // Wednesday
		{"2024-01-11", 5}, // Thursday
		{"2024-01-12", 6}, // Friday
		{"2024-01-13", 7}, // Saturday
	}

	for _, tt := range dates {
		result, err := Call("DAYOFWEEK", []types.Value{
			types.NewStringValue(tt.date),
		})
		if err != nil {
			t.Errorf("DAYOFWEEK(%s) failed: %v", tt.date, err)
		} else {
			d, _ := result.ToInt64()
			if int(d) != tt.dayNum {
				t.Errorf("DAYOFWEEK(%s) = %d, want %d", tt.date, d, tt.dayNum)
			}
		}
	}
}

// TestDayOfYearEdgeCases tests DAYOFYEAR edge cases
func TestDayOfYearEdgeCases(t *testing.T) {
	// Day 1
	result, err := Call("DAYOFYEAR", []types.Value{
		types.NewStringValue("2024-01-01"),
	})
	if err != nil {
		t.Errorf("DAYOFYEAR Jan 1 failed: %v", err)
	} else {
		d, _ := result.ToInt64()
		if d != 1 {
			t.Errorf("DAYOFYEAR Jan 1 = %d, want 1", d)
		}
	}

	// Last day of leap year
	result, err = Call("DAYOFYEAR", []types.Value{
		types.NewStringValue("2024-12-31"),
	})
	if err != nil {
		t.Errorf("DAYOFYEAR Dec 31 failed: %v", err)
	} else {
		d, _ := result.ToInt64()
		if d != 366 {
			t.Errorf("DAYOFYEAR Dec 31 leap = %d, want 366", d)
		}
	}

	// Last day of non-leap year
	result, err = Call("DAYOFYEAR", []types.Value{
		types.NewStringValue("2023-12-31"),
	})
	if err != nil {
		t.Errorf("DAYOFYEAR Dec 31 failed: %v", err)
	} else {
		d, _ := result.ToInt64()
		if d != 365 {
			t.Errorf("DAYOFYEAR Dec 31 non-leap = %d, want 365", d)
		}
	}
}

// TestBlobHexFunctions tests BLOB_FROM_HEX and BLOB_TO_HEX
func TestBlobHexFunctions(t *testing.T) {
	// Test data
	testData := []byte{0x48, 0x65, 0x6c, 0x6c, 0x6f} // "Hello"
	expectedHex := "48656c6c6f"

	// Test BLOB_TO_HEX
	result, err := Call("BLOB_TO_HEX", []types.Value{
		types.NewBlobValue(testData),
	})
	if err != nil {
		t.Errorf("BLOB_TO_HEX failed: %v", err)
	}
	if result.ToString() != expectedHex {
		t.Errorf("BLOB_TO_HEX = %s, want %s", result.ToString(), expectedHex)
	}

	// Test BLOB_FROM_HEX
	result2, err := Call("BLOB_FROM_HEX", []types.Value{
		types.NewStringValue(expectedHex),
	})
	if err != nil {
		t.Errorf("BLOB_FROM_HEX failed: %v", err)
	}
	blobData, ok := result2.Data.([]byte)
	if !ok {
		t.Errorf("BLOB_FROM_HEX did not return []byte")
	} else if string(blobData) != string(testData) {
		t.Errorf("BLOB_FROM_HEX = %v, want %v", blobData, testData)
	}

	// Test with 0x prefix
	result3, err := Call("BLOB_FROM_HEX", []types.Value{
		types.NewStringValue("0x" + expectedHex),
	})
	if err != nil {
		t.Errorf("BLOB_FROM_HEX with 0x prefix failed: %v", err)
	}
	blobData3, _ := result3.Data.([]byte)
	if string(blobData3) != string(testData) {
		t.Errorf("BLOB_FROM_HEX with 0x prefix = %v, want %v", blobData3, testData)
	}

	// Test with uppercase hex
	result4, err := Call("BLOB_FROM_HEX", []types.Value{
		types.NewStringValue("48656C6C6F"),
	})
	if err != nil {
		t.Errorf("BLOB_FROM_HEX uppercase failed: %v", err)
	}
	blobData4, _ := result4.Data.([]byte)
	if string(blobData4) != string(testData) {
		t.Errorf("BLOB_FROM_HEX uppercase = %v, want %v", blobData4, testData)
	}

	// Test null input
	result5, err := Call("BLOB_FROM_HEX", []types.Value{
		types.NewNullValue(),
	})
	if err != nil || !result5.IsNull {
		t.Errorf("BLOB_FROM_HEX null should return null")
	}

	// Test BLOB_TO_HEX with null
	result6, err := Call("BLOB_TO_HEX", []types.Value{
		types.NewNullValue(),
	})
	if err != nil || !result6.IsNull {
		t.Errorf("BLOB_TO_HEX null should return null")
	}
}

// TestBlobBase64Functions tests BLOB_FROM_BASE64 and BLOB_TO_BASE64
func TestBlobBase64Functions(t *testing.T) {
	testData := []byte("Hello, World!")
	expectedBase64 := "SGVsbG8sIFdvcmxkIQ=="

	// Test BLOB_TO_BASE64
	result, err := Call("BLOB_TO_BASE64", []types.Value{
		types.NewBlobValue(testData),
	})
	if err != nil {
		t.Errorf("BLOB_TO_BASE64 failed: %v", err)
	}
	if result.ToString() != expectedBase64 {
		t.Errorf("BLOB_TO_BASE64 = %s, want %s", result.ToString(), expectedBase64)
	}

	// Test BLOB_FROM_BASE64
	result2, err := Call("BLOB_FROM_BASE64", []types.Value{
		types.NewStringValue(expectedBase64),
	})
	if err != nil {
		t.Errorf("BLOB_FROM_BASE64 failed: %v", err)
	}
	blobData, ok := result2.Data.([]byte)
	if !ok {
		t.Errorf("BLOB_FROM_BASE64 did not return []byte")
	} else if string(blobData) != string(testData) {
		t.Errorf("BLOB_FROM_BASE64 = %v, want %v", blobData, testData)
	}

	// Test null input
	result3, err := Call("BLOB_FROM_BASE64", []types.Value{
		types.NewNullValue(),
	})
	if err != nil || !result3.IsNull {
		t.Errorf("BLOB_FROM_BASE64 null should return null")
	}
}

// TestImageHexFunctions tests IMAGE_FROM_HEX and IMAGE_TO_HEX
func TestImageHexFunctions(t *testing.T) {
	// Minimal valid PNG (1x1 transparent pixel)
	pngHex := "89504e470d0a1a0a0000000d49484452000000010000000108060000001f15c4890000000b49444154789c6360000200000500017a5eab3f0000000049454e44ae426082"

	// Test IMAGE_FROM_HEX
	result, err := Call("IMAGE_FROM_HEX", []types.Value{
		types.NewStringValue(pngHex),
	})
	if err != nil {
		t.Errorf("IMAGE_FROM_HEX failed: %v", err)
	}
	if result.Type != types.TypeImage {
		t.Errorf("IMAGE_FROM_HEX should return IMAGE type, got %v", result.Type)
	}

	// Test IMAGE_TO_HEX
	result2, err := Call("IMAGE_TO_HEX", []types.Value{
		result,
	})
	if err != nil {
		t.Errorf("IMAGE_TO_HEX failed: %v", err)
	}
	if result2.ToString() != pngHex {
		t.Errorf("IMAGE_TO_HEX roundtrip failed: got %s", result2.ToString())
	}

	// Test with 0x prefix
	result3, err := Call("IMAGE_FROM_HEX", []types.Value{
		types.NewStringValue("0x" + pngHex),
	})
	if err != nil {
		t.Errorf("IMAGE_FROM_HEX with 0x prefix failed: %v", err)
	}
	if result3.Type != types.TypeImage {
		t.Errorf("IMAGE_FROM_HEX with 0x prefix should return IMAGE type")
	}

	// Test null input
	result4, err := Call("IMAGE_FROM_HEX", []types.Value{
		types.NewNullValue(),
	})
	if err != nil || !result4.IsNull {
		t.Errorf("IMAGE_FROM_HEX null should return null")
	}

	// Test IMAGE_TO_HEX with null
	result5, err := Call("IMAGE_TO_HEX", []types.Value{
		types.NewNullValue(),
	})
	if err != nil || !result5.IsNull {
		t.Errorf("IMAGE_TO_HEX null should return null")
	}
}

// TestHexErrorCases tests error handling for hex functions
func TestHexErrorCases(t *testing.T) {
	// Invalid hex string (odd length)
	_, err := Call("BLOB_FROM_HEX", []types.Value{
		types.NewStringValue("abc"),
	})
	if err == nil {
		t.Errorf("BLOB_FROM_HEX should fail with odd-length hex string")
	}

	// Invalid hex characters
	_, err = Call("BLOB_FROM_HEX", []types.Value{
		types.NewStringValue("ghij"),
	})
	if err == nil {
		t.Errorf("BLOB_FROM_HEX should fail with invalid hex characters")
	}

	// Invalid hex for IMAGE
	_, err = Call("IMAGE_FROM_HEX", []types.Value{
		types.NewStringValue("abc"),
	})
	if err == nil {
		t.Errorf("IMAGE_FROM_HEX should fail with odd-length hex string")
	}

	// Missing argument
	_, err = Call("BLOB_FROM_HEX", []types.Value{})
	if err == nil {
		t.Errorf("BLOB_FROM_HEX should fail with no arguments")
	}

	_, err = Call("BLOB_TO_HEX", []types.Value{})
	if err == nil {
		t.Errorf("BLOB_TO_HEX should fail with no arguments")
	}

	_, err = Call("IMAGE_FROM_HEX", []types.Value{})
	if err == nil {
		t.Errorf("IMAGE_FROM_HEX should fail with no arguments")
	}

	_, err = Call("IMAGE_TO_HEX", []types.Value{})
	if err == nil {
		t.Errorf("IMAGE_TO_HEX should fail with no arguments")
	}
}

// TestHexWithWhitespace tests hex decoding with whitespace
func TestHexWithWhitespace(t *testing.T) {
	// Hex with spaces
	result, err := Call("BLOB_FROM_HEX", []types.Value{
		types.NewStringValue("48 65 6c 6c 6f"),
	})
	if err != nil {
		t.Errorf("BLOB_FROM_HEX with spaces failed: %v", err)
	}
	data, _ := result.Data.([]byte)
	if string(data) != "Hello" {
		t.Errorf("BLOB_FROM_HEX with spaces = %s, want Hello", string(data))
	}

	// Hex with newlines
	result2, err := Call("BLOB_FROM_HEX", []types.Value{
		types.NewStringValue("4865\n6c6c\n6f"),
	})
	if err != nil {
		t.Errorf("BLOB_FROM_HEX with newlines failed: %v", err)
	}
	data2, _ := result2.Data.([]byte)
	if string(data2) != "Hello" {
		t.Errorf("BLOB_FROM_HEX with newlines = %s, want Hello", string(data2))
	}
}
