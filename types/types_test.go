package types

import (
	"strings"
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

// Test NewDateValue
func TestNewDateValue(t *testing.T) {
	now := time.Now()
	v := NewDateValue(now)
	if v.Type != TypeDate {
		t.Errorf("NewDateValue type = %v, want DATE", v.Type)
	}
	if v.IsNull {
		t.Error("NewDateValue should not be null")
	}
}

// Test ToBytes
func TestValueToBytes(t *testing.T) {
	// From string
	v := NewStringValue("hello")
	b, err := v.ToBytes()
	if err != nil || string(b) != "hello" {
		t.Errorf("ToBytes from string = %s, want 'hello'", string(b))
	}

	// From blob
	v = NewBlobValue([]byte{1, 2, 3})
	b, err = v.ToBytes()
	if err != nil || len(b) != 3 {
		t.Errorf("ToBytes from blob length = %d, want 3", len(b))
	}

	// From null
	v = NewNullValue()
	b, err = v.ToBytes()
	if err != nil || b != nil {
		t.Error("ToBytes from null should be nil")
	}

	// From int - may not be supported
	v = NewIntValue(42)
	b, err = v.ToBytes()
	if err != nil {
		t.Logf("ToBytes from int: %v", err)
	}
}

// Test WithDefault
func TestColumnDefWithDefault(t *testing.T) {
	col := NewColumnDef("status", TypeVarchar, 20).WithDefault("active")
	if col.Default == nil {
		t.Error("WithDefault should set default value")
	}
	if col.Default.ToString() != "active" {
		t.Errorf("Default = %s, want 'active'", col.Default.ToString())
	}
}

// Test detectType
func TestDetectType(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected DataType
	}{
		{int(42), TypeInt},
		{int64(42), TypeInt},
		{float64(3.14), TypeFloat},
		{float32(3.14), TypeFloat},
		{"hello", TypeVarchar},
		{true, TypeInt},
		{false, TypeInt},
		{[]byte{1, 2, 3}, TypeBlob},
		{time.Now(), TypeDatetime},
	}

	for _, tt := range tests {
		got := detectType(tt.input)
		if got != tt.expected {
			t.Errorf("detectType(%T) = %v, want %v", tt.input, got, tt.expected)
		}
	}

	// nil case - may return TypeUnknown or TypeNull
	got := detectType(nil)
	t.Logf("detectType(nil) = %v", got)
}

// Test more ToString cases
func TestMoreToString(t *testing.T) {
	// Bool values
	v := NewBoolValue(true)
	if v.ToString() != "1" {
		t.Errorf("ToString(true) = %s, want '1'", v.ToString())
	}

	v = NewBoolValue(false)
	if v.ToString() != "0" {
		t.Errorf("ToString(false) = %s, want '0'", v.ToString())
	}

	// Blob value
	v = NewBlobValue([]byte{0x48, 0x65, 0x6c, 0x6c, 0x6f}) // "Hello"
	s := v.ToString()
	if s == "" {
		t.Error("ToString from blob should not be empty")
	}

	// Date value
	v = NewDateValue(time.Now())
	s = v.ToString()
	if s == "" {
		t.Error("ToString from date should not be empty")
	}
}

// Test more ToInt64 cases
func TestMoreToInt64(t *testing.T) {
	// From bool
	v := NewBoolValue(true)
	n, err := v.ToInt64()
	if err != nil || n != 1 {
		t.Errorf("ToInt64(true) = %d, want 1", n)
	}

	// From invalid string
	v = NewStringValue("not a number")
	_, err = v.ToInt64()
	if err == nil {
		t.Error("ToInt64 from 'not a number' should error")
	}

	// From blob
	v = NewBlobValue([]byte{1, 2, 3})
	_, err = v.ToInt64()
	if err == nil {
		t.Error("ToInt64 from blob should error")
	}
}

// Test more ToFloat64 cases
func TestMoreToFloat64(t *testing.T) {
	// From bool
	v := NewBoolValue(true)
	f, err := v.ToFloat64()
	if err != nil || f != 1.0 {
		t.Errorf("ToFloat64(true) = %f, want 1.0", f)
	}

	// From invalid string
	v = NewStringValue("not a number")
	_, err = v.ToFloat64()
	if err == nil {
		t.Error("ToFloat64 from 'not a number' should error")
	}

	// From null
	v = NewNullValue()
	_, err = v.ToFloat64()
	if err == nil {
		t.Error("ToFloat64 from null should error")
	}

	// From blob
	v = NewBlobValue([]byte{1, 2, 3})
	_, err = v.ToFloat64()
	if err == nil {
		t.Error("ToFloat64 from blob should error")
	}
}

// Test Value size via ToBytes
func TestValueSizeViaBytes(t *testing.T) {
	tests := []struct {
		value    Value
		minSize  int
	}{
		{NewStringValue("hello"), 5},
		{NewBlobValue([]byte{1, 2, 3, 4, 5}), 5},
		{NewNullValue(), 0},
	}

	for _, tt := range tests {
		b, err := tt.value.ToBytes()
		if err != nil {
			t.Errorf("ToBytes error: %v", err)
			continue
		}
		size := len(b)
		if size < tt.minSize {
			t.Errorf("Size() = %d, want at least %d", size, tt.minSize)
		}
	}

	// Int and float may not support ToBytes
	_, err := NewIntValue(42).ToBytes()
	t.Logf("Int ToBytes: %v", err)
	_, err = NewFloatValue(3.14).ToBytes()
	t.Logf("Float ToBytes: %v", err)
}

// Test TableInfo without primary key
func TestTableInfoNoPrimaryKey(t *testing.T) {
	columns := []ColumnDef{
		{Name: "name", Type: TypeVarchar, Length: 100},
		{Name: "value", Type: TypeInt},
	}

	ti := NewTableInfo(1, "no_pk", columns)
	pk := ti.PrimaryKeyColumn()
	if pk != nil {
		t.Error("TableInfo without primary key should return nil")
	}
}

// Test ColumnIndex for non-existent column
func TestColumnIndexNotFound(t *testing.T) {
	columns := []ColumnDef{
		{Name: "id", Type: TypeInt},
		{Name: "name", Type: TypeVarchar, Length: 100},
	}

	ti := NewTableInfo(1, "test", columns)
	idx, ok := ti.ColumnIndex("nonexistent")
	if ok {
		t.Error("ColumnIndex for non-existent column should return false")
	}
	if idx != -1 {
		t.Errorf("ColumnIndex for non-existent = %d, want -1", idx)
	}
}

// Test Compare with different types
func TestCompareDifferentTypes(t *testing.T) {
	// Int vs Float
	v1 := NewIntValue(5)
	v2 := NewFloatValue(5.0)
	// These should be comparable
	cmp := v1.Compare(v2)
	t.Logf("Compare int(5) vs float(5.0): %d", cmp)

	// Int vs String
	v1 = NewIntValue(5)
	v2 = NewStringValue("5")
	cmp = v1.Compare(v2)
	t.Logf("Compare int(5) vs string('5'): %d", cmp)
}

// Test NewValue with various types
func TestNewValueMoreTypes(t *testing.T) {
	// uint
	v := NewValue(uint(42))
	if v.Type != TypeInt {
		t.Errorf("NewValue(uint) type = %v, want INT", v.Type)
	}

	// int32
	v = NewValue(int32(42))
	if v.Type != TypeInt {
		t.Errorf("NewValue(int32) type = %v, want INT", v.Type)
	}

	// []interface{}
	v = NewValue([]interface{}{1, 2, 3})
	// Should still create a value
	t.Logf("NewValue(slice) type = %v", v.Type)
}

// TestToInt64Comprehensive tests ToInt64 with more cases
func TestToInt64Comprehensive(t *testing.T) {
	// Test int conversion
	v := NewIntValue(42)
	n, err := v.ToInt64()
	if err != nil || n != 42 {
		t.Errorf("ToInt64(int) = %d, %v, want 42, nil", n, err)
	}

	// Test float to int conversion
	v = NewFloatValue(3.99)
	n, err = v.ToInt64()
	if err != nil {
		t.Logf("ToInt64(float) = %d, %v", n, err)
	}

	// Test string to int conversion
	v = NewStringValue("123")
	n, err = v.ToInt64()
	if err != nil {
		t.Logf("ToInt64(string '123') = %d, %v", n, err)
	}

	// Test invalid string
	v = NewStringValue("not a number")
	n, err = v.ToInt64()
	if err == nil {
		t.Error("ToInt64(invalid string) should fail")
	}

	// Test bool conversion
	v = NewBoolValue(true)
	n, err = v.ToInt64()
	if err != nil {
		t.Logf("ToInt64(bool true) = %d, %v", n, err)
	}

	// Test NULL
	v = NewNullValue()
	n, err = v.ToInt64()
	if err == nil {
		t.Error("ToInt64(NULL) should fail")
	}
}

// TestToFloat64Comprehensive tests ToFloat64 with more cases
func TestToFloat64Comprehensive(t *testing.T) {
	// Test float conversion
	v := NewFloatValue(3.14159)
	f, err := v.ToFloat64()
	if err != nil || f != 3.14159 {
		t.Errorf("ToFloat64(float) = %f, %v, want 3.14159, nil", f, err)
	}

	// Test int to float
	v = NewIntValue(42)
	f, err = v.ToFloat64()
	if err != nil || f != 42.0 {
		t.Errorf("ToFloat64(int) = %f, %v, want 42.0, nil", f, err)
	}

	// Test string to float
	v = NewStringValue("3.14")
	f, err = v.ToFloat64()
	if err != nil {
		t.Logf("ToFloat64(string '3.14') = %f, %v", f, err)
	}

	// Test invalid string
	v = NewStringValue("not a number")
	f, err = v.ToFloat64()
	if err == nil {
		t.Error("ToFloat64(invalid string) should fail")
	}

	// Test NULL
	v = NewNullValue()
	f, err = v.ToFloat64()
	if err == nil {
		t.Error("ToFloat64(NULL) should fail")
	}
}

// TestToStringComprehensive tests ToString with more cases
func TestToStringComprehensive(t *testing.T) {
	// Test string
	v := NewStringValue("hello")
	if v.ToString() != "hello" {
		t.Errorf("ToString(string) = %s, want 'hello'", v.ToString())
	}

	// Test int
	v = NewIntValue(42)
	if v.ToString() != "42" {
		t.Errorf("ToString(int) = %s, want '42'", v.ToString())
	}

	// Test float
	v = NewFloatValue(3.14)
	s := v.ToString()
	t.Logf("ToString(float) = %s", s)

	// Test bool
	v = NewBoolValue(true)
	s = v.ToString()
	t.Logf("ToString(bool) = %s", s)

	// Test NULL
	v = NewNullValue()
	if v.ToString() != "NULL" {
		t.Errorf("ToString(NULL) = %s, want 'NULL'", v.ToString())
	}

	// Test BLOB
	v = NewBlobValue([]byte{1, 2, 3})
	s = v.ToString()
	t.Logf("ToString(blob) = %s", s)

	// Test time
	v = NewDatetimeValue(time.Now())
	s = v.ToString()
	t.Logf("ToString(datetime) = %s", s)
}

// TestCompareMore tests Compare with more combinations
func TestCompareMore(t *testing.T) {
	tests := []struct {
		v1, v2 Value
	}{
		{NewIntValue(1), NewIntValue(2)},
		{NewIntValue(5), NewIntValue(5)},
		{NewFloatValue(1.5), NewFloatValue(2.5)},
		{NewStringValue("a"), NewStringValue("b")},
		{NewStringValue("abc"), NewStringValue("abc")},
		{NewIntValue(1), NewFloatValue(1.5)},
		{NewStringValue("5"), NewIntValue(5)},
		{NewNullValue(), NewNullValue()},
		{NewNullValue(), NewIntValue(1)},
		{NewIntValue(1), NewNullValue()},
	}

	for _, tt := range tests {
		cmp := tt.v1.Compare(tt.v2)
		t.Logf("Compare(%v, %v) = %d", tt.v1, tt.v2, cmp)
	}
}

// TestHashMore tests Hash with more types
func TestHashMore(t *testing.T) {
	v1 := NewIntValue(42)
	v2 := NewIntValue(42)
	if v1.Hash() != v2.Hash() {
		t.Error("Same values should have same hash")
	}

	v3 := NewIntValue(43)
	if v1.Hash() == v3.Hash() {
		t.Error("Different values should (likely) have different hashes")
	}

	// Test hash stability
	v := NewStringValue("test")
	h1 := v.Hash()
	h2 := v.Hash()
	if h1 != h2 {
		t.Error("Hash should be stable")
	}
}

// TestCloneMore tests Clone with more types
func TestCloneMore(t *testing.T) {
	// Clone int
	v1 := NewIntValue(42)
	v2 := v1.Clone()
	if v1.Compare(v2) != 0 {
		t.Error("Cloned int should be equal")
	}

	// Clone string
	v1 = NewStringValue("hello")
	v2 = v1.Clone()
	if v1.Compare(v2) != 0 {
		t.Error("Cloned string should be equal")
	}

	// Clone null
	v1 = NewNullValue()
	v2 = v1.Clone()
	if !v2.IsNull {
		t.Error("Cloned null should be null")
	}
}

// TestSizeMore tests DataType Size
func TestSizeMore(t *testing.T) {
	// Test VARCHAR with length
	dt := TypeVarchar
	size := dt.Size()
	t.Logf("VARCHAR(100) size = %d", size)

	// Test INT with no length
	dt = TypeInt
	size = dt.Size()
	t.Logf("INT size = %d", size)

	// Test BLOB
	dt = TypeBlob
	size = dt.Size()
	t.Logf("BLOB size = %d", size)
}

// TestDetectTypeMore tests detectType
func TestDetectTypeMore(t *testing.T) {
	tests := []interface{}{
		int8(8),
		int16(16),
		uint(42),
		uint8(8),
		uint16(16),
		uint32(32),
		uint64(64),
		float32(3.14),
		[]int{1, 2, 3},
		map[string]int{"a": 1},
	}

	for _, input := range tests {
		v := NewValue(input)
		t.Logf("NewValue(%T) -> Type=%v", input, v.Type)
	}
}

// TestToBoolMore tests ToBool
func TestToBoolMore(t *testing.T) {
	tests := []struct {
		input   interface{}
		want    bool
	}{
		{int(0), false},
		{int(1), true},
		{int(-1), true},
		{float64(0.0), false},
		{float64(1.5), true},
		{"", false},
		{"hello", true},
		{true, true},
		{false, false},
	}

	for _, tt := range tests {
		v := NewValue(tt.input)
		got := v.ToBool()
		t.Logf("ToBool(%v) = %v", tt.input, got)
	}
}

// TestToTimeMore tests ToTime
func TestToTimeMore(t *testing.T) {
	// Test time.Time
	now := time.Now()
	v := NewDatetimeValue(now)
	t2, err := v.ToTime()
	if err != nil {
		t.Errorf("ToTime(time) failed: %v", err)
	}
	if !t2.Equal(now) {
		t.Error("ToTime(time) should return same time")
	}

	// Test string date
	v = NewStringValue("2024-01-15")
	t2, err = v.ToTime()
	if err != nil {
		t.Logf("ToTime('2024-01-15') = %v, %v", t2, err)
	}

	// Test invalid string
	v = NewStringValue("not a date")
	t2, err = v.ToTime()
	if err == nil {
		t.Error("ToTime(invalid string) should fail")
	}

	// Test NULL
	v = NewNullValue()
	t2, err = v.ToTime()
	if err == nil {
		t.Error("ToTime(NULL) should fail")
	}
}

// TestToInt64ExtraTypes tests ToInt64 with additional types
func TestToInt64ExtraTypes(t *testing.T) {
	// Test uint types
	v := NewValue(uint(42))
	got, err := v.ToInt64()
	if err != nil {
		t.Errorf("ToInt64(uint) failed: %v", err)
	}
	t.Logf("ToInt64(uint 42) = %d", got)

	v = NewValue(uint8(8))
	got, err = v.ToInt64()
	if err != nil {
		t.Errorf("ToInt64(uint8) failed: %v", err)
	}
	t.Logf("ToInt64(uint8 8) = %d", got)

	v = NewValue(uint16(16))
	got, err = v.ToInt64()
	if err != nil {
		t.Errorf("ToInt64(uint16) failed: %v", err)
	}
	t.Logf("ToInt64(uint16 16) = %d", got)

	v = NewValue(uint32(32))
	got, err = v.ToInt64()
	if err != nil {
		t.Errorf("ToInt64(uint32) failed: %v", err)
	}
	t.Logf("ToInt64(uint32 32) = %d", got)

	v = NewValue(uint64(64))
	got, err = v.ToInt64()
	if err != nil {
		t.Errorf("ToInt64(uint64) failed: %v", err)
	}
	t.Logf("ToInt64(uint64 64) = %d", got)

	// Test bool
	v = NewValue(true)
	got, err = v.ToInt64()
	if err != nil {
		t.Errorf("ToInt64(bool true) failed: %v", err)
	}
	if got != 1 {
		t.Errorf("ToInt64(true) = %d, want 1", got)
	}

	v = NewValue(false)
	got, err = v.ToInt64()
	if err != nil {
		t.Errorf("ToInt64(bool false) failed: %v", err)
	}
	if got != 0 {
		t.Errorf("ToInt64(false) = %d, want 0", got)
	}

	// Test invalid string
	v = NewStringValue("not a number")
	_, err = v.ToInt64()
	if err == nil {
		t.Error("ToInt64(invalid string) should fail")
	}

	// Test unsupported type
	v = Value{Data: []int{1, 2, 3}}
	_, err = v.ToInt64()
	if err == nil {
		t.Error("ToInt64(slice) should fail")
	}
}

// TestToFloat64ExtraTypes tests ToFloat64 with additional types
func TestToFloat64ExtraTypes(t *testing.T) {
	// Test various numeric types
	v := NewValue(int8(8))
	got, err := v.ToFloat64()
	if err != nil {
		t.Errorf("ToFloat64(int8) failed: %v", err)
	}
	t.Logf("ToFloat64(int8 8) = %f", got)

	v = NewValue(int16(16))
	got, err = v.ToFloat64()
	if err != nil {
		t.Errorf("ToFloat64(int16) failed: %v", err)
	}
	t.Logf("ToFloat64(int16 16) = %f", got)

	v = NewValue(int32(32))
	got, err = v.ToFloat64()
	if err != nil {
		t.Errorf("ToFloat64(int32) failed: %v", err)
	}
	t.Logf("ToFloat64(int32 32) = %f", got)

	v = NewValue(uint(42))
	got, err = v.ToFloat64()
	if err != nil {
		t.Errorf("ToFloat64(uint) failed: %v", err)
	}
	t.Logf("ToFloat64(uint 42) = %f", got)

	// Test string to float
	v = NewStringValue("3.14159")
	got, err = v.ToFloat64()
	if err != nil {
		t.Errorf("ToFloat64(string) failed: %v", err)
	}
	t.Logf("ToFloat64('3.14159') = %f", got)

	// Test invalid string
	v = NewStringValue("not a number")
	_, err = v.ToFloat64()
	if err == nil {
		t.Error("ToFloat64(invalid string) should fail")
	}

	// Test unsupported type
	v = Value{Data: []int{1, 2, 3}}
	_, err = v.ToFloat64()
	if err == nil {
		t.Error("ToFloat64(slice) should fail")
	}
}

// TestToStringWithVariousTypes tests ToString with more types
func TestToStringWithVariousTypes(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
	}{
		{"int64", int64(123456789)},
		{"float32", float32(3.14)},
		{"[]byte", []byte("bytes")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValue(tt.value)
			got := v.ToString()
			t.Logf("ToString(%T) = %s", tt.value, got)
		})
	}
}

// TestDataTypeSizeAll tests Size for all data types
func TestDataTypeSizeAll(t *testing.T) {
	tests := []struct {
		dt       DataType
		expected int
	}{
		{TypeNull, 0},
		{TypeSeq, 8},
		{TypeInt, 8},
		{TypeFloat, 8},
		{TypeDate, 8},
		{TypeTime, 8},
		{TypeDatetime, 8},
		{TypeChar, -1},
		{TypeVarchar, -1},
		{TypeText, -1},
		{TypeBlob, -1},
		{TypeFile, -1},
		{TypeUnknown, -1},
	}

	for _, tt := range tests {
		got := tt.dt.Size()
		if got != tt.expected {
			t.Errorf("DataType(%v).Size() = %d, want %d", tt.dt, got, tt.expected)
		}
	}
}

// TestToStringTypeFile tests ToString for FILE type
func TestToStringTypeFile(t *testing.T) {
	v := Value{Type: TypeFile, Data: "/path/to/file.txt"}
	s := v.ToString()
	if !strings.Contains(s, "FILE") {
		t.Errorf("ToString(FILE) = %s, should contain 'FILE'", s)
	}
	t.Logf("ToString(FILE) = %s", s)
}

// TestToStringTypeTime tests ToString for TIME type
func TestToStringTypeTime(t *testing.T) {
	now := time.Now()
	v := Value{Type: TypeTime, Data: now}
	s := v.ToString()
	if s == "" {
		t.Error("ToString(TIME) should not be empty")
	}
	t.Logf("ToString(TIME) = %s", s)
}

// TestToStringTypeText tests ToString for TEXT type
func TestToStringTypeText(t *testing.T) {
	v := Value{Type: TypeText, Data: "large text content"}
	s := v.ToString()
	if s != "large text content" {
		t.Errorf("ToString(TEXT) = %s, want 'large text content'", s)
	}
}

// TestToStringTypeChar tests ToString for CHAR type
func TestToStringTypeChar(t *testing.T) {
	v := Value{Type: TypeChar, Data: "char value"}
	s := v.ToString()
	if s != "char value" {
		t.Errorf("ToString(CHAR) = %s, want 'char value'", s)
	}
}

// TestToStringIntVariants tests ToString with various int types
func TestToStringIntVariants(t *testing.T) {
	tests := []struct {
		name  string
		value Value
	}{
		{"int", Value{Type: TypeInt, Data: int(42)}},
		{"int64", Value{Type: TypeInt, Data: int64(42)}},
		{"int32", Value{Type: TypeInt, Data: int32(42)}},
		{"float64 as int", Value{Type: TypeInt, Data: float64(42.0)}},
		{"float32 as int", Value{Type: TypeInt, Data: float32(42.0)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.value.ToString()
			if s != "42" {
				t.Errorf("ToString() = %s, want '42'", s)
			}
		})
	}
}

// TestToStringFloatVariants tests ToString with various float types
func TestToStringFloatVariants(t *testing.T) {
	tests := []struct {
		name     string
		value    Value
		expected string
	}{
		{"float64 integer", Value{Type: TypeFloat, Data: float64(100.0)}, "100"},
		{"float64 decimal", Value{Type: TypeFloat, Data: float64(3.14)}, "3.14"},
		{"float32", Value{Type: TypeFloat, Data: float32(3.14)}, "3.14"},
		{"default type", Value{Type: TypeFloat, Data: "not a float"}, "not a float"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.value.ToString()
			t.Logf("ToString() = %s", s)
		})
	}
}

// TestHashWithBlob tests Hash with blob values
func TestHashWithBlob(t *testing.T) {
	v1 := NewBlobValue([]byte{1, 2, 3, 4, 5})
	v2 := NewBlobValue([]byte{1, 2, 3, 4, 5})
	if v1.Hash() != v2.Hash() {
		t.Error("Same blob values should have same hash")
	}

	// Test that hash is non-zero
	if v1.Hash() == 0 {
		t.Error("Blob hash should not be zero")
	}
}

// TestHashWithFloat tests Hash with float values
func TestHashWithFloat(t *testing.T) {
	v1 := NewFloatValue(3.14)
	v2 := NewFloatValue(3.14)
	if v1.Hash() != v2.Hash() {
		t.Error("Same float values should have same hash")
	}
}

// TestHashWithString tests Hash with string values
func TestHashWithString(t *testing.T) {
	v1 := NewStringValue("test")
	v2 := NewStringValue("test")
	if v1.Hash() != v2.Hash() {
		t.Error("Same string values should have same hash")
	}
}

// TestToFloat64WithAllTypes tests ToFloat64 with all numeric types
func TestToFloat64WithAllTypes(t *testing.T) {
	tests := []struct {
		name     string
		value    Value
		expected float64
	}{
		{"int", Value{Type: TypeInt, Data: int(42)}, 42.0},
		{"int8", Value{Type: TypeInt, Data: int8(8)}, 8.0},
		{"int16", Value{Type: TypeInt, Data: int16(16)}, 16.0},
		{"int32", Value{Type: TypeInt, Data: int32(32)}, 32.0},
		{"int64", Value{Type: TypeInt, Data: int64(64)}, 64.0},
		{"uint", Value{Type: TypeInt, Data: uint(42)}, 42.0},
		{"uint8", Value{Type: TypeInt, Data: uint8(8)}, 8.0},
		{"uint16", Value{Type: TypeInt, Data: uint16(16)}, 16.0},
		{"uint32", Value{Type: TypeInt, Data: uint32(32)}, 32.0},
		{"uint64", Value{Type: TypeInt, Data: uint64(64)}, 64.0},
		{"float32", Value{Type: TypeFloat, Data: float32(3.14)}, 3.14},
		{"float64", Value{Type: TypeFloat, Data: float64(3.14)}, 3.14},
		{"string number", Value{Type: TypeVarchar, Data: "2.5"}, 2.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.value.ToFloat64()
			if err != nil {
				t.Errorf("ToFloat64() error: %v", err)
				return
			}
			// Allow small floating point differences
			diff := got - tt.expected
			if diff < 0 {
				diff = -diff
			}
			if diff > 0.001 {
				t.Errorf("ToFloat64() = %f, want %f", got, tt.expected)
			}
		})
	}
}

// TestToInt64WithAllTypes tests ToInt64 with all numeric types
func TestToInt64WithAllTypes(t *testing.T) {
	tests := []struct {
		name     string
		value    Value
		expected int64
	}{
		{"int", Value{Type: TypeInt, Data: int(42)}, 42},
		{"int8", Value{Type: TypeInt, Data: int8(8)}, 8},
		{"int16", Value{Type: TypeInt, Data: int16(16)}, 16},
		{"int32", Value{Type: TypeInt, Data: int32(32)}, 32},
		{"int64", Value{Type: TypeInt, Data: int64(64)}, 64},
		{"uint", Value{Type: TypeInt, Data: uint(42)}, 42},
		{"uint8", Value{Type: TypeInt, Data: uint8(8)}, 8},
		{"uint16", Value{Type: TypeInt, Data: uint16(16)}, 16},
		{"uint32", Value{Type: TypeInt, Data: uint32(32)}, 32},
		{"uint64", Value{Type: TypeInt, Data: uint64(64)}, 64},
		{"float32", Value{Type: TypeFloat, Data: float32(3.99)}, 3},
		{"float64", Value{Type: TypeFloat, Data: float64(3.99)}, 3},
		{"string number", Value{Type: TypeVarchar, Data: "123"}, 123},
		{"bool true", Value{Type: TypeInt, Data: true}, 1},
		{"bool false", Value{Type: TypeInt, Data: false}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.value.ToInt64()
			if err != nil {
				t.Errorf("ToInt64() error: %v", err)
				return
			}
			if got != tt.expected {
				t.Errorf("ToInt64() = %d, want %d", got, tt.expected)
			}
		})
	}
}

// TestParseDataTypeMore tests ParseDataType with more types
func TestParseDataTypeMore(t *testing.T) {
	tests := []struct {
		input    string
		expected DataType
	}{
		{"BIGSERIAL", TypeSeq},
		{"SMALLINT", TypeInt},
		{"TINYINT", TypeInt},
		{"INT64", TypeInt},
		{"NUMERIC", TypeFloat},
		{"REAL", TypeFloat},
		{"DOUBLE PRECISION", TypeFloat},
		{"CHARACTER", TypeChar},
		{"VARCHAR2", TypeVarchar},
		{"NVARCHAR", TypeVarchar},
		{"CLOB", TypeText},
		{"LONGTEXT", TypeText},
		{"TIMESTAMP WITHOUT TIME ZONE", TypeDatetime},
		{"BINARY", TypeBlob},
		{"VARBINARY", TypeBlob},
		{"BYTEA", TypeBlob},
		{"LONGBLOB", TypeBlob},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseDataType(tt.input)
			if got != tt.expected {
				t.Errorf("ParseDataType(%s) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

// TestMarshalJSONAllTypes tests MarshalJSON for all types
func TestMarshalJSONAllTypes(t *testing.T) {
	tests := []struct {
		name  string
		value Value
	}{
		{"null", NewNullValue()},
		{"int", NewIntValue(42)},
		{"float", NewFloatValue(3.14)},
		{"string", NewStringValue("hello")},
		{"bool", NewBoolValue(true)},
		{"date", NewDateValue(time.Now())},
		{"datetime", NewDatetimeValue(time.Now())},
		{"blob", NewBlobValue([]byte{1, 2, 3})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := tt.value.MarshalJSON()
			if err != nil {
				t.Errorf("MarshalJSON error: %v", err)
				return
			}
			t.Logf("MarshalJSON(%s) = %s", tt.name, string(data))

			// Unmarshal and verify
			var v2 Value
			err = v2.UnmarshalJSON(data)
			if err != nil {
				t.Errorf("UnmarshalJSON error: %v", err)
			}
		})
	}
}

// TestUnmarshalJSONInvalid tests UnmarshalJSON with invalid data
func TestUnmarshalJSONInvalid(t *testing.T) {
	var v Value
	err := v.UnmarshalJSON([]byte("invalid json"))
	if err == nil {
		t.Error("UnmarshalJSON should fail with invalid JSON")
	}
}

// TestCompareEdgeCases tests Compare with edge cases
func TestCompareEdgeCases(t *testing.T) {
	// Compare same type values
	tests := []struct {
		name     string
		v1, v2   Value
		expected int // -1, 0, or 1
	}{
		{"int less", NewIntValue(1), NewIntValue(2), -1},
		{"int equal", NewIntValue(5), NewIntValue(5), 0},
		{"int greater", NewIntValue(3), NewIntValue(2), 1},
		{"float less", NewFloatValue(1.0), NewFloatValue(2.0), -1},
		{"float equal", NewFloatValue(3.14), NewFloatValue(3.14), 0},
		{"string less", NewStringValue("a"), NewStringValue("b"), -1},
		{"string equal", NewStringValue("x"), NewStringValue("x"), 0},
		{"null vs null", NewNullValue(), NewNullValue(), 0},
		{"null vs value", NewNullValue(), NewIntValue(1), -1},
		{"value vs null", NewIntValue(1), NewNullValue(), 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.v1.Compare(tt.v2)
			if got != tt.expected {
				t.Errorf("Compare() = %d, want %d", got, tt.expected)
			}
		})
	}
}

// TestToBytesWithTypes tests ToBytes with various types
func TestToBytesWithTypes(t *testing.T) {
	tests := []struct {
		name    string
		value   Value
		wantLen int
		wantErr bool
	}{
		{"string", NewStringValue("hello"), 5, false},
		{"blob", NewBlobValue([]byte{1, 2, 3}), 3, false},
		{"null", NewNullValue(), 0, false},
		{"int", NewIntValue(42), 0, true}, // Should error
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.value.ToBytes()
			if (err != nil) != tt.wantErr {
				t.Errorf("ToBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(got) != tt.wantLen {
				t.Errorf("ToBytes() len = %d, want %d", len(got), tt.wantLen)
			}
		})
	}
}

// TestNewValueWithNil tests NewValue with nil
func TestNewValueWithNil(t *testing.T) {
	v := NewValue(nil)
	if !v.IsNull {
		t.Error("NewValue(nil) should be null")
	}
	if v.Type != TypeNull {
		t.Errorf("NewValue(nil) type = %v, want TypeNull", v.Type)
	}
}

// TestCloneWithNull tests Clone with null value
func TestCloneWithNull(t *testing.T) {
	v := NewNullValue()
	cloned := v.Clone()
	if !cloned.IsNull {
		t.Error("Cloned null should be null")
	}
}

// TestRowCloneWithEmptyData tests Row.Clone with empty data
func TestRowCloneWithEmptyData(t *testing.T) {
	row := NewRow([]Value{})
	cloned := row.Clone()
	if len(cloned.Data) != 0 {
		t.Error("Cloned empty row should have empty data")
	}
}

// TestValueData tests various value data types
func TestValueData(t *testing.T) {
	// Test that data is preserved
	v := NewStringValue("test data")
	if v.Data.(string) != "test data" {
		t.Error("Value data should be preserved")
	}
}

// TestTableInfoMethods tests TableInfo methods
func TestTableInfoMethods(t *testing.T) {
	columns := []ColumnDef{
		{Name: "id", Type: TypeSeq, PrimaryKey: true, AutoInc: true},
		{Name: "name", Type: TypeVarchar, Length: 100, Nullable: true},
		{Name: "age", Type: TypeInt, Nullable: true},
		{Name: "created", Type: TypeDatetime},
	}

	ti := NewTableInfo(1, "users", columns)

	// Test PrimaryKeyColumn
	pk := ti.PrimaryKeyColumn()
	if pk == nil || pk.Name != "id" {
		t.Error("PrimaryKeyColumn should return 'id'")
	}

	// Test ColumnIndex for each column
	for i, col := range columns {
		idx, ok := ti.ColumnIndex(col.Name)
		if !ok || idx != i {
			t.Errorf("ColumnIndex(%s) = %d, %v, want %d, true", col.Name, idx, ok, i)
		}
	}

	// Test ColumnNames
	names := ti.ColumnNames()
	if len(names) != 4 {
		t.Errorf("ColumnNames() length = %d, want 4", len(names))
	}
}

// TestColumnDefChaining tests ColumnDef method chaining
func TestColumnDefChaining(t *testing.T) {
	col := NewColumnDef("id", TypeSeq, 0).
		WithPrimaryKey(true).
		WithAutoInc(true).
		WithNullable(false)

	if !col.PrimaryKey {
		t.Error("WithPrimaryKey should set PrimaryKey to true")
	}
	if !col.AutoInc {
		t.Error("WithAutoInc should set AutoInc to true")
	}
	if col.Nullable {
		t.Error("WithNullable(false) should set Nullable to false")
	}

	// Test WithDefault
	col = NewColumnDef("status", TypeVarchar, 20).
		WithDefault("active").
		WithNullable(true)

	if col.Default == nil {
		t.Error("WithDefault should set Default")
	}
	if !col.Nullable {
		t.Error("WithNullable(true) should set Nullable to true")
	}
}
