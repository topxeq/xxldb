package function

import (
	"math"
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

