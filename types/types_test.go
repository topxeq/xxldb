package types

import (
	"testing"
	"time"
)

func TestDataTypeString(t *testing.T) {
	tests := []struct {
		dt       DataType
		expected string
	}{
		{TypeNull, "NULL"},
		{TypeSeq, "SEQ"},
		{TypeInt, "INT"},
		{TypeFloat, "FLOAT"},
		{TypeChar, "CHAR"},
		{TypeVarchar, "VARCHAR"},
		{TypeText, "TEXT"},
		{TypeDate, "DATE"},
		{TypeTime, "TIME"},
		{TypeDatetime, "DATETIME"},
		{TypeBlob, "BLOB"},
		{TypeFile, "FILE"},
		{TypeUnknown, "UNKNOWN"},
	}

	for _, tt := range tests {
		if got := tt.dt.String(); got != tt.expected {
			t.Errorf("DataType(%d).String() = %s, want %s", tt.dt, got, tt.expected)
		}
	}
}

func TestParseDataType(t *testing.T) {
	tests := []struct {
		input    string
		expected DataType
	}{
		{"SEQ", TypeSeq},
		{"SERIAL", TypeSeq},
		{"AUTO_INCREMENT", TypeSeq},
		{"INT", TypeInt},
		{"INTEGER", TypeInt},
		{"BIGINT", TypeInt},
		{"FLOAT", TypeFloat},
		{"DOUBLE", TypeFloat},
		{"DECIMAL", TypeFloat},
		{"CHAR", TypeChar},
		{"VARCHAR", TypeVarchar},
		{"TEXT", TypeVarchar},
		{"DATE", TypeDate},
		{"TIME", TypeTime},
		{"DATETIME", TypeDatetime},
		{"TIMESTAMP", TypeDatetime},
		{"BLOB", TypeBlob},
		{"FILE", TypeFile},
		{"VARCHAR(100)", TypeVarchar},
		{"CHAR(50)", TypeChar},
		{"UNKNOWN_TYPE", TypeUnknown},
	}

	for _, tt := range tests {
		if got := ParseDataType(tt.input); got != tt.expected {
			t.Errorf("ParseDataType(%s) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestDataTypeCategories(t *testing.T) {
	// IsNumeric
	if !TypeInt.IsNumeric() || !TypeFloat.IsNumeric() || !TypeSeq.IsNumeric() {
		t.Error("INT, FLOAT, SEQ should be numeric")
	}
	if TypeVarchar.IsNumeric() {
		t.Error("VARCHAR should not be numeric")
	}

	// IsString
	if !TypeChar.IsString() || !TypeVarchar.IsString() || !TypeText.IsString() {
		t.Error("CHAR, VARCHAR, TEXT should be string types")
	}
	if TypeInt.IsString() {
		t.Error("INT should not be string type")
	}

	// IsDateTime
	if !TypeDate.IsDateTime() || !TypeTime.IsDateTime() || !TypeDatetime.IsDateTime() {
		t.Error("DATE, TIME, DATETIME should be datetime types")
	}
	if TypeInt.IsDateTime() {
		t.Error("INT should not be datetime type")
	}

	// IsBinary
	if !TypeBlob.IsBinary() || !TypeFile.IsBinary() {
		t.Error("BLOB, FILE should be binary types")
	}
	if TypeInt.IsBinary() {
		t.Error("INT should not be binary type")
	}
}

func TestDataTypeSize(t *testing.T) {
	if TypeInt.Size() != 8 {
		t.Error("INT size should be 8 bytes")
	}
	if TypeFloat.Size() != 8 {
		t.Error("FLOAT size should be 8 bytes")
	}
	if TypeVarchar.Size() != -1 {
		t.Error("VARCHAR size should be -1 (variable)")
	}
}

func TestNewValue(t *testing.T) {
	// Test nil
	v := NewValue(nil)
	if !v.IsNull {
		t.Error("NewValue(nil) should be null")
	}

	// Test int
	v = NewValue(42)
	if v.Type != TypeInt {
		t.Error("NewValue(42) should be INT type")
	}
	if v.Data.(int) != 42 {
		t.Error("NewValue(42) data should be 42")
	}

	// Test string
	v = NewValue("hello")
	if v.Type != TypeVarchar {
		t.Error("NewValue(string) should be VARCHAR type")
	}

	// Test float
	v = NewValue(3.14)
	if v.Type != TypeFloat {
		t.Error("NewValue(float) should be FLOAT type")
	}

	// Test []byte
	v = NewValue([]byte("binary"))
	if v.Type != TypeBlob {
		t.Error("NewValue([]byte) should be BLOB type")
	}

	// Test time
	v = NewValue(time.Now())
	if v.Type != TypeDatetime {
		t.Error("NewValue(time) should be DATETIME type")
	}
}

func TestValueToString(t *testing.T) {
	tests := []struct {
		value    Value
		expected string
	}{
		{NewNullValue(), "NULL"},
		{NewIntValue(42), "42"},
		{NewIntValue(-100), "-100"},
		{NewFloatValue(3.14), "3.14"},
		{NewFloatValue(100.0), "100"},
		{NewStringValue("hello"), "hello"},
		{NewBoolValue(true), "1"},
		{NewBoolValue(false), "0"},
	}

	for _, tt := range tests {
		if got := tt.value.ToString(); got != tt.expected {
			t.Errorf("ToString() = %s, want %s", got, tt.expected)
		}
	}
}

func TestValueToInt64(t *testing.T) {
	// From int
	v := NewIntValue(42)
	n, err := v.ToInt64()
	if err != nil || n != 42 {
		t.Errorf("ToInt64 from int: got %d, want 42", n)
	}

	// From float
	v = NewFloatValue(3.99)
	n, err = v.ToInt64()
	if err != nil || n != 3 {
		t.Errorf("ToInt64 from float: got %d, want 3", n)
	}

	// From string
	v = NewStringValue("123")
	n, err = v.ToInt64()
	if err != nil || n != 123 {
		t.Errorf("ToInt64 from string: got %d, want 123", n)
	}

	// From null
	v = NewNullValue()
	_, err = v.ToInt64()
	if err == nil {
		t.Error("ToInt64 from null should error")
	}
}

func TestValueToFloat64(t *testing.T) {
	// From int
	v := NewIntValue(42)
	f, err := v.ToFloat64()
	if err != nil || f != 42.0 {
		t.Errorf("ToFloat64 from int: got %f, want 42.0", f)
	}

	// From float
	v = NewFloatValue(3.14)
	f, err = v.ToFloat64()
	if err != nil || f != 3.14 {
		t.Errorf("ToFloat64 from float: got %f, want 3.14", f)
	}

	// From string
	v = NewStringValue("2.5")
	f, err = v.ToFloat64()
	if err != nil || f != 2.5 {
		t.Errorf("ToFloat64 from string: got %f, want 2.5", f)
	}
}

func TestValueToBool(t *testing.T) {
	tests := []struct {
		value    Value
		expected bool
	}{
		{NewIntValue(1), true},
		{NewIntValue(0), false},
		{NewIntValue(-1), true},
		{NewFloatValue(0.0), false},
		{NewFloatValue(0.1), true},
		{NewStringValue("true"), true},
		{NewStringValue("false"), false},
		{NewStringValue("1"), true},
		{NewStringValue("yes"), true},
		{NewNullValue(), false},
	}

	for _, tt := range tests {
		if got := tt.value.ToBool(); got != tt.expected {
			t.Errorf("ToBool() = %v, want %v", got, tt.expected)
		}
	}
}

func TestValueToTime(t *testing.T) {
	// From time.Time
	now := time.Now()
	v := NewDatetimeValue(now)
	tm, err := v.ToTime()
	if err != nil || !tm.Equal(now) {
		t.Error("ToTime from time.Time failed")
	}

	// From string
	v = NewStringValue("2026-04-04 12:00:00")
	tm, err = v.ToTime()
	if err != nil {
		t.Errorf("ToTime from string failed: %v", err)
	}

	// Invalid string
	v = NewStringValue("not a date")
	_, err = v.ToTime()
	if err == nil {
		t.Error("ToTime from invalid string should error")
	}
}

func TestValueCompare(t *testing.T) {
	// Int comparisons
	if NewIntValue(1).Compare(NewIntValue(2)) >= 0 {
		t.Error("1 should be less than 2")
	}
	if NewIntValue(2).Compare(NewIntValue(1)) <= 0 {
		t.Error("2 should be greater than 1")
	}
	if NewIntValue(5).Compare(NewIntValue(5)) != 0 {
		t.Error("5 should equal 5")
	}

	// String comparisons
	if NewStringValue("a").Compare(NewStringValue("b")) >= 0 {
		t.Error("'a' should be less than 'b'")
	}
	if NewStringValue("z").Compare(NewStringValue("a")) <= 0 {
		t.Error("'z' should be greater than 'a'")
	}

	// Null comparisons
	null := NewNullValue()
	nonNull := NewIntValue(1)
	if null.Compare(null) != 0 {
		t.Error("NULL should equal NULL")
	}
	if null.Compare(nonNull) >= 0 {
		t.Error("NULL should be less than non-null")
	}
	if nonNull.Compare(null) <= 0 {
		t.Error("Non-null should be greater than NULL")
	}
}

func TestValueEquals(t *testing.T) {
	if !NewIntValue(42).Equals(NewIntValue(42)) {
		t.Error("42 should equal 42")
	}
	if NewIntValue(1).Equals(NewIntValue(2)) {
		t.Error("1 should not equal 2")
	}
	if !NewStringValue("hello").Equals(NewStringValue("hello")) {
		t.Error("'hello' should equal 'hello'")
	}
}

func TestValueHash(t *testing.T) {
	// Same values should have same hash
	h1 := NewIntValue(42).Hash()
	h2 := NewIntValue(42).Hash()
	if h1 != h2 {
		t.Error("Same values should have same hash")
	}

	// Different values should have different hashes
	h3 := NewIntValue(43).Hash()
	if h1 == h3 {
		t.Error("Different values should have different hashes")
	}

	// Null should have hash 0
	if NewNullValue().Hash() != 0 {
		t.Error("NULL hash should be 0")
	}
}

func TestValueClone(t *testing.T) {
	// Clone int
	v1 := NewIntValue(42)
	v2 := v1.Clone()
	if !v1.Equals(v2) {
		t.Error("Cloned int should equal original")
	}

	// Clone blob
	blob := []byte{1, 2, 3, 4, 5}
	v1 = NewBlobValue(blob)
	v2 = v1.Clone()
	if !v1.Equals(v2) {
		t.Error("Cloned blob should equal original")
	}
}

func TestColumnDef(t *testing.T) {
	col := NewColumnDef("id", TypeInt, 0).
		WithPrimaryKey(true).
		WithAutoInc(true).
		WithNullable(false)

	if col.Name != "id" {
		t.Error("Column name should be 'id'")
	}
	if col.Type != TypeInt {
		t.Error("Column type should be INT")
	}
	if !col.PrimaryKey {
		t.Error("Column should be primary key")
	}
	if !col.AutoInc {
		t.Error("Column should be auto increment")
	}
	if col.Nullable {
		t.Error("Primary key should not be nullable")
	}
}

func TestColumnDefString(t *testing.T) {
	col := NewColumnDef("name", TypeVarchar, 100).WithNullable(false)
	expected := "name VARCHAR(100) NOT NULL"
	if col.String() != expected {
		t.Errorf("ColumnDef.String() = %s, want %s", col.String(), expected)
	}

	col = NewColumnDef("id", TypeSeq, 0).WithPrimaryKey(true).WithAutoInc(true)
	s := col.String()
	if !containsAll(s, "id", "SEQ", "PRIMARY KEY", "AUTO INCREMENT") {
		t.Errorf("ColumnDef.String() = %s, missing parts", s)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !contains(s, sub) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || (len(s) > 0 && findSubstring(s, sub)))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestTableInfo(t *testing.T) {
	columns := []ColumnDef{
		{Name: "id", Type: TypeSeq, PrimaryKey: true},
		{Name: "name", Type: TypeVarchar, Length: 100},
		{Name: "age", Type: TypeInt},
	}

	ti := NewTableInfo(1, "users", columns)

	if ti.Name != "users" {
		t.Error("Table name should be 'users'")
	}
	if len(ti.Columns) != 3 {
		t.Error("Table should have 3 columns")
	}

	// PrimaryKeyColumn
	pk := ti.PrimaryKeyColumn()
	if pk == nil || pk.Name != "id" {
		t.Error("Primary key column should be 'id'")
	}

	// ColumnIndex
	idx, ok := ti.ColumnIndex("name")
	if !ok || idx != 1 {
		t.Error("ColumnIndex('name') should return 1")
	}

	// ColumnNames
	names := ti.ColumnNames()
	if len(names) != 3 || names[0] != "id" {
		t.Error("ColumnNames should return all column names")
	}
}

func TestValueJSON(t *testing.T) {
	// Test marshaling
	v := NewIntValue(42)
	data, err := v.MarshalJSON()
	if err != nil {
		t.Errorf("MarshalJSON failed: %v", err)
	}

	// Test unmarshaling
	var v2 Value
	err = v2.UnmarshalJSON(data)
	if err != nil {
		t.Errorf("UnmarshalJSON failed: %v", err)
	}
	if v2.ToString() != "42" {
		t.Errorf("Unmarshaled value = %s, want 42", v2.ToString())
	}

	// Test null
	v = NewNullValue()
	data, _ = v.MarshalJSON()
	var v3 Value
	v3.UnmarshalJSON(data)
	if !v3.IsNull {
		t.Error("Unmarshaled null should be null")
	}

	// Test datetime
	now := time.Now()
	v = NewDatetimeValue(now)
	data, _ = v.MarshalJSON()
	var v4 Value
	v4.UnmarshalJSON(data)
	if v4.Type != TypeDatetime {
		t.Error("Unmarshaled datetime should be datetime type")
	}
}

func TestRow(t *testing.T) {
	data := []Value{NewIntValue(1), NewStringValue("test")}
	row := NewRow(data)

	if len(row.Data) != 2 {
		t.Error("Row should have 2 values")
	}

	// Clone
	cloned := row.Clone()
	if !cloned.Data[0].Equals(row.Data[0]) {
		t.Error("Cloned row data should equal original")
	}
}
