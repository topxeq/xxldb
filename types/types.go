// Package types defines the data types and value system for xxldb
package types

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"strconv"
	"strings"
	"time"
)

// DataType represents a column data type
type DataType int

const (
	TypeUnknown DataType = iota
	TypeNull             // NULL type
	TypeSeq              // Auto-increment sequence (int64)
	TypeInt              // Integer (int64)
	TypeFloat            // Floating point (float64)
	TypeChar             // Fixed length string
	TypeVarchar          // Variable length string
	TypeText             // Large text
	TypeDate             // Date only
	TypeTime             // Time only
	TypeDatetime         // Date and time
	TypeBlob             // Binary large object
	TypeFile             // File reference
	TypeImage            // Image with metadata
)

// String returns the string representation of the data type
func (dt DataType) String() string {
	switch dt {
	case TypeNull:
		return "NULL"
	case TypeSeq:
		return "SEQ"
	case TypeInt:
		return "INT"
	case TypeFloat:
		return "FLOAT"
	case TypeChar:
		return "CHAR"
	case TypeVarchar:
		return "VARCHAR"
	case TypeText:
		return "TEXT"
	case TypeDate:
		return "DATE"
	case TypeTime:
		return "TIME"
	case TypeDatetime:
		return "DATETIME"
	case TypeBlob:
		return "BLOB"
	case TypeFile:
		return "FILE"
	case TypeImage:
		return "IMAGE"
	default:
		return "UNKNOWN"
	}
}

// ParseDataType parses a string to DataType
func ParseDataType(s string) DataType {
	s = strings.ToUpper(strings.TrimSpace(s))

	// Handle type with length specifier
	if idx := strings.Index(s, "("); idx > 0 {
		s = s[:idx]
	}

	switch s {
	case "SEQ", "SERIAL", "AUTO_INCREMENT", "BIGSERIAL":
		return TypeSeq
	case "INT", "INTEGER", "BIGINT", "SMALLINT", "TINYINT", "INT64":
		return TypeInt
	case "FLOAT", "DOUBLE", "DECIMAL", "NUMERIC", "REAL", "DOUBLE PRECISION":
		return TypeFloat
	case "CHAR", "CHARACTER":
		return TypeChar
	case "VARCHAR", "VARCHAR2", "NVARCHAR", "TEXT":
		return TypeVarchar
	case "CLOB", "LONGTEXT":
		return TypeText
	case "DATE":
		return TypeDate
	case "TIME":
		return TypeTime
	case "DATETIME", "TIMESTAMP", "TIMESTAMP WITHOUT TIME ZONE":
		return TypeDatetime
	case "BLOB", "BINARY", "VARBINARY", "BYTEA", "LONGBLOB":
		return TypeBlob
	case "FILE":
		return TypeFile
	case "IMAGE", "IMG", "PICTURE":
		return TypeImage
	default:
		return TypeUnknown
	}
}

// IsNumeric returns true if the type is numeric
func (dt DataType) IsNumeric() bool {
	return dt == TypeInt || dt == TypeFloat || dt == TypeSeq
}

// IsString returns true if the type is string-like
func (dt DataType) IsString() bool {
	return dt == TypeChar || dt == TypeVarchar || dt == TypeText
}

// IsDateTime returns true if the type is date/time
func (dt DataType) IsDateTime() bool {
	return dt == TypeDate || dt == TypeTime || dt == TypeDatetime
}

// IsBinary returns true if the type is binary
func (dt DataType) IsBinary() bool {
	return dt == TypeBlob || dt == TypeFile || dt == TypeImage
}

// Size returns the fixed size of the type in bytes, or -1 for variable
func (dt DataType) Size() int {
	switch dt {
	case TypeNull:
		return 0
	case TypeSeq, TypeInt:
		return 8
	case TypeFloat:
		return 8
	case TypeDate, TypeTime, TypeDatetime:
		return 8
	default:
		return -1 // Variable length
	}
}

// Value represents a typed value in the database
type Value struct {
	Type    DataType
	Data    interface{}
	IsNull  bool
	BlobRef *BlobRef // Optional: reference to external blob storage
}

// BlobRef represents a reference to an external blob
type BlobRef struct {
	ID   uint64 `json:"id"`
	Size int64  `json:"size"`
}

// NewValue creates a new Value from any Go value
func NewValue(data interface{}) Value {
	if data == nil {
		return Value{Type: TypeNull, IsNull: true}
	}
	return Value{
		Type: detectType(data),
		Data: data,
	}
}

// NewNullValue creates a null value
func NewNullValue() Value {
	return Value{Type: TypeNull, IsNull: true}
}

// NewIntValue creates an integer value
func NewIntValue(n int64) Value {
	return Value{Type: TypeInt, Data: n}
}

// NewFloatValue creates a float value
func NewFloatValue(f float64) Value {
	return Value{Type: TypeFloat, Data: f}
}

// NewStringValue creates a string value
func NewStringValue(s string) Value {
	return Value{Type: TypeVarchar, Data: s}
}

// NewBoolValue creates a boolean value (stored as int)
func NewBoolValue(b bool) Value {
	if b {
		return Value{Type: TypeInt, Data: int64(1)}
	}
	return Value{Type: TypeInt, Data: int64(0)}
}

// NewDateValue creates a date value
func NewDateValue(t time.Time) Value {
	return Value{Type: TypeDate, Data: t}
}

// NewDatetimeValue creates a datetime value
func NewDatetimeValue(t time.Time) Value {
	return Value{Type: TypeDatetime, Data: t}
}

// NewBlobValue creates a blob value
func NewBlobValue(data []byte) Value {
	return Value{Type: TypeBlob, Data: data}
}

// NewImageValue creates an image value
func NewImageValue(data []byte) Value {
	return Value{Type: TypeImage, Data: data}
}

// NewBlobValueFromReader creates a blob value from an io.Reader
func NewBlobValueFromReader(r io.Reader) (Value, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Value{}, fmt.Errorf("failed to read blob data: %w", err)
	}
	return NewBlobValue(data), nil
}

// NewImageValueFromReader creates an image value from an io.Reader
func NewImageValueFromReader(r io.Reader) (Value, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Value{}, fmt.Errorf("failed to read image data: %w", err)
	}
	return NewImageValue(data), nil
}

// NewBlobValueFromHex creates a blob value from a hex string
func NewBlobValueFromHex(hexStr string) (Value, error) {
	// Remove optional 0x prefix
	hexStr = strings.TrimPrefix(hexStr, "0x")
	hexStr = strings.TrimPrefix(hexStr, "0X")

	// Remove whitespace
	hexStr = strings.ReplaceAll(hexStr, " ", "")
	hexStr = strings.ReplaceAll(hexStr, "\n", "")
	hexStr = strings.ReplaceAll(hexStr, "\r", "")
	hexStr = strings.ReplaceAll(hexStr, "\t", "")

	if len(hexStr)%2 != 0 {
		return Value{}, fmt.Errorf("invalid hex string: length must be even")
	}

	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return Value{}, fmt.Errorf("invalid hex data: %w", err)
	}

	return NewBlobValue(data), nil
}

// NewImageValueFromHex creates an image value from a hex string
func NewImageValueFromHex(hexStr string) (Value, error) {
	// Remove optional 0x prefix
	hexStr = strings.TrimPrefix(hexStr, "0x")
	hexStr = strings.TrimPrefix(hexStr, "0X")

	// Remove whitespace
	hexStr = strings.ReplaceAll(hexStr, " ", "")
	hexStr = strings.ReplaceAll(hexStr, "\n", "")
	hexStr = strings.ReplaceAll(hexStr, "\r", "")
	hexStr = strings.ReplaceAll(hexStr, "\t", "")

	if len(hexStr)%2 != 0 {
		return Value{}, fmt.Errorf("invalid hex string: length must be even")
	}

	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return Value{}, fmt.Errorf("invalid hex data: %w", err)
	}

	return NewImageValue(data), nil
}

// ToHex converts a blob/image value to hex string
func (v Value) ToHex() (string, error) {
	if v.IsNull {
		return "", nil
	}

	data, err := v.ToBytes()
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(data), nil
}

// detectType detects the DataType from interface{}
func detectType(data interface{}) DataType {
	switch data.(type) {
	case int:
		return TypeInt
	case int8:
		return TypeInt
	case int16:
		return TypeInt
	case int32:
		return TypeInt
	case int64:
		return TypeInt
	case uint:
		return TypeInt
	case uint8:
		return TypeInt
	case uint16:
		return TypeInt
	case uint32:
		return TypeInt
	case uint64:
		return TypeInt
	case float32:
		return TypeFloat
	case float64:
		return TypeFloat
	case string:
		return TypeVarchar
	case time.Time:
		return TypeDatetime
	case []byte:
		return TypeBlob
	case bool:
		return TypeInt
	default:
		return TypeUnknown
	}
}

// ToString converts value to string
func (v Value) ToString() string {
	if v.IsNull {
		return "NULL"
	}

	switch v.Type {
	case TypeInt, TypeSeq:
		switch val := v.Data.(type) {
		case int:
			return fmt.Sprintf("%d", val)
		case int64:
			return fmt.Sprintf("%d", val)
		case int32:
			return fmt.Sprintf("%d", val)
		case float64:
			return fmt.Sprintf("%d", int64(val))
		case float32:
			return fmt.Sprintf("%d", int64(val))
		default:
			return fmt.Sprintf("%v", val)
		}
	case TypeFloat:
		switch val := v.Data.(type) {
		case float64:
			// Format with reasonable precision
			if val == float64(int64(val)) {
				return fmt.Sprintf("%.0f", val)
			}
			return fmt.Sprintf("%v", val)
		case float32:
			return fmt.Sprintf("%v", val)
		default:
			return fmt.Sprintf("%v", val)
		}
	case TypeDate:
		if t, ok := v.Data.(time.Time); ok {
			return t.Format("2006-01-02")
		}
	case TypeTime:
		if t, ok := v.Data.(time.Time); ok {
			return t.Format("15:04:05")
		}
	case TypeDatetime:
		if t, ok := v.Data.(time.Time); ok {
			return t.Format("2006-01-02 15:04:05")
		}
	case TypeChar, TypeVarchar, TypeText:
		return fmt.Sprintf("%s", v.Data)
	case TypeBlob:
		if data, ok := v.Data.([]byte); ok {
			return fmt.Sprintf("[BLOB %d bytes]", len(data))
		}
	case TypeFile:
		return fmt.Sprintf("[FILE %s]", v.Data)
	}

	return fmt.Sprintf("%v", v.Data)
}

// ToInt64 converts value to int64
func (v Value) ToInt64() (int64, error) {
	if v.IsNull {
		return 0, fmt.Errorf("cannot convert NULL to int64")
	}

	switch val := v.Data.(type) {
	case int:
		return int64(val), nil
	case int8:
		return int64(val), nil
	case int16:
		return int64(val), nil
	case int32:
		return int64(val), nil
	case int64:
		return val, nil
	case uint:
		return int64(val), nil
	case uint8:
		return int64(val), nil
	case uint16:
		return int64(val), nil
	case uint32:
		return int64(val), nil
	case uint64:
		return int64(val), nil
	case float32:
		return int64(val), nil
	case float64:
		return int64(val), nil
	case string:
		return strconv.ParseInt(val, 10, 64)
	case bool:
		if val {
			return 1, nil
		}
		return 0, nil
	default:
		return 0, fmt.Errorf("cannot convert %T to int64", v.Data)
	}
}

// ToFloat64 converts value to float64
func (v Value) ToFloat64() (float64, error) {
	if v.IsNull {
		return 0, fmt.Errorf("cannot convert NULL to float64")
	}

	switch val := v.Data.(type) {
	case int:
		return float64(val), nil
	case int8:
		return float64(val), nil
	case int16:
		return float64(val), nil
	case int32:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case uint:
		return float64(val), nil
	case uint8:
		return float64(val), nil
	case uint16:
		return float64(val), nil
	case uint32:
		return float64(val), nil
	case uint64:
		return float64(val), nil
	case float32:
		return float64(val), nil
	case float64:
		return val, nil
	case string:
		return strconv.ParseFloat(val, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", v.Data)
	}
}

// ToBool converts value to bool
func (v Value) ToBool() bool {
	if v.IsNull {
		return false
	}

	switch val := v.Data.(type) {
	case bool:
		return val
	case int, int8, int16, int32, int64:
		i, _ := v.ToInt64()
		return i != 0
	case float32, float64:
		f, _ := v.ToFloat64()
		return f != 0
	case string:
		s := strings.ToLower(val)
		return s == "true" || s == "1" || s == "yes"
	default:
		return false
	}
}

// ToTime converts value to time.Time
func (v Value) ToTime() (time.Time, error) {
	if v.IsNull {
		return time.Time{}, fmt.Errorf("cannot convert NULL to time")
	}

	switch val := v.Data.(type) {
	case time.Time:
		return val, nil
	case string:
		// Try common date/time formats
		formats := []string{
			"2006-01-02 15:04:05",
			"2006-01-02T15:04:05",
			"2006-01-02 15:04:05.999",
			"2006-01-02T15:04:05.999",
			"2006-01-02",
			"15:04:05",
		}
		for _, format := range formats {
			if t, err := time.Parse(format, val); err == nil {
				return t, nil
			}
		}
		return time.Time{}, fmt.Errorf("cannot parse time: %s", val)
	default:
		return time.Time{}, fmt.Errorf("cannot convert %T to time", v.Data)
	}
}

// ToBytes converts value to []byte
func (v Value) ToBytes() ([]byte, error) {
	if v.IsNull {
		return nil, nil
	}

	// If this is a blob reference, the actual data needs to be loaded from storage
	// The storage layer should handle this before calling ToBytes
	switch val := v.Data.(type) {
	case []byte:
		return val, nil
	case string:
		return []byte(val), nil
	default:
		return nil, fmt.Errorf("cannot convert %T to []byte", v.Data)
	}
}

// IsBlobRef returns true if this value is a reference to external blob storage
func (v Value) IsBlobRef() bool {
	return v.BlobRef != nil
}

// BlobSize returns the size of the blob data
// For blob references, returns the stored size; otherwise returns the actual data length
func (v Value) BlobSize() int64 {
	if v.IsNull {
		return 0
	}

	if v.BlobRef != nil {
		return v.BlobRef.Size
	}

	switch val := v.Data.(type) {
	case []byte:
		return int64(len(val))
	case string:
		return int64(len(val))
	default:
		return 0
	}
}

// NewBlobRefValue creates a blob value with external storage reference
func NewBlobRefValue(blobID uint64, size int64) Value {
	return Value{
		Type: TypeBlob,
		Data: nil, // Data is stored externally
		BlobRef: &BlobRef{
			ID:   blobID,
			Size: size,
		},
	}
}

// Compare compares two values
// Returns: -1 if v < other, 0 if v == other, 1 if v > other
func (v Value) Compare(other Value) int {
	// Handle NULL comparison
	if v.IsNull && other.IsNull {
		return 0
	}
	if v.IsNull {
		return -1 // NULL is considered less than any value
	}
	if other.IsNull {
		return 1
	}

	// Try numeric comparison first
	vFloat, vErr := v.ToFloat64()
	oFloat, oErr := other.ToFloat64()
	if vErr == nil && oErr == nil {
		if vFloat < oFloat {
			return -1
		} else if vFloat > oFloat {
			return 1
		}
		return 0
	}

	// Try time comparison
	vTime, vTErr := v.ToTime()
	oTime, oTErr := other.ToTime()
	if vTErr == nil && oTErr == nil {
		if vTime.Before(oTime) {
			return -1
		} else if vTime.After(oTime) {
			return 1
		}
		return 0
	}

	// Fall back to string comparison
	vStr := v.ToString()
	oStr := other.ToString()
	if vStr < oStr {
		return -1
	} else if vStr > oStr {
		return 1
	}
	return 0
}

// Equals checks if two values are equal
func (v Value) Equals(other Value) bool {
	return v.Compare(other) == 0
}

// Hash returns a hash value for the Value
func (v Value) Hash() uint64 {
	if v.IsNull {
		return 0
	}

	h := fnv.New64a()
	h.Write([]byte{byte(v.Type)})

	switch v.Type {
	case TypeInt, TypeSeq:
		i, _ := v.ToInt64()
		binary.Write(h, binary.LittleEndian, i)
	case TypeFloat:
		f, _ := v.ToFloat64()
		binary.Write(h, binary.LittleEndian, f)
	default:
		h.Write([]byte(v.ToString()))
	}

	return h.Sum64()
}

// Clone creates a deep copy of the value
func (v Value) Clone() Value {
	if v.IsNull {
		return NewNullValue()
	}

	copied := Value{Type: v.Type, Data: v.Data}

	// Copy blob reference if present
	if v.BlobRef != nil {
		copied.BlobRef = &BlobRef{
			ID:   v.BlobRef.ID,
			Size: v.BlobRef.Size,
		}
	}

	switch val := v.Data.(type) {
	case []byte:
		if v.BlobRef == nil {
			// Only copy bytes if not using external storage
			data := make([]byte, len(val))
			copy(data, val)
			copied.Data = data
		}
	}

	return copied
}

// ColumnDef defines a column structure
type ColumnDef struct {
	Name       string  `json:"name"`
	Type       DataType `json:"type"`
	Length     int     `json:"length,omitempty"`     // For CHAR, VARCHAR
	Precision  int     `json:"precision,omitempty"`  // For FLOAT, DECIMAL
	Nullable   bool    `json:"nullable"`
	Default    *Value  `json:"default,omitempty"`
	PrimaryKey bool    `json:"primary_key"`
	AutoInc    bool    `json:"auto_inc"`
}

// NewColumnDef creates a new column definition
func NewColumnDef(name string, typ DataType, length int) *ColumnDef {
	return &ColumnDef{
		Name:     name,
		Type:     typ,
		Length:   length,
		Nullable: true,
	}
}

// WithNullable sets the nullable flag
func (c *ColumnDef) WithNullable(nullable bool) *ColumnDef {
	c.Nullable = nullable
	return c
}

// WithPrimaryKey sets the primary key flag
func (c *ColumnDef) WithPrimaryKey(pk bool) *ColumnDef {
	c.PrimaryKey = pk
	if pk {
		c.Nullable = false
	}
	return c
}

// WithAutoInc sets the auto increment flag
func (c *ColumnDef) WithAutoInc(auto bool) *ColumnDef {
	c.AutoInc = auto
	return c
}

// WithDefault sets the default value
func (c *ColumnDef) WithDefault(val interface{}) *ColumnDef {
	c.Default = &Value{Data: val, Type: c.Type}
	return c
}

// String returns a string representation of the column definition
func (c ColumnDef) String() string {
	var sb strings.Builder
	sb.WriteString(c.Name)
	sb.WriteString(" ")
	sb.WriteString(c.Type.String())
	if c.Length > 0 {
		sb.WriteString(fmt.Sprintf("(%d)", c.Length))
	}
	if c.PrimaryKey {
		sb.WriteString(" PRIMARY KEY")
	}
	if c.AutoInc {
		sb.WriteString(" AUTO INCREMENT")
	}
	if !c.Nullable && !c.PrimaryKey {
		sb.WriteString(" NOT NULL")
	}
	return sb.String()
}

// TableInfo stores table metadata
type TableInfo struct {
	ID        uint64       `json:"id"`
	Name      string       `json:"name"`
	Columns   []ColumnDef  `json:"columns"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
	RowCount  int64        `json:"row_count"`
}

// NewTableInfo creates a new table info
func NewTableInfo(id uint64, name string, columns []ColumnDef) *TableInfo {
	return &TableInfo{
		ID:        id,
		Name:      name,
		Columns:   columns,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// PrimaryKeyColumn returns the primary key column, if any
func (t *TableInfo) PrimaryKeyColumn() *ColumnDef {
	for i := range t.Columns {
		if t.Columns[i].PrimaryKey {
			return &t.Columns[i]
		}
	}
	return nil
}

// ColumnIndex returns the index of a column by name
func (t *TableInfo) ColumnIndex(name string) (int, bool) {
	name = strings.ToLower(name)
	for i, col := range t.Columns {
		if strings.ToLower(col.Name) == name {
			return i, true
		}
	}
	return -1, false
}

// ColumnNames returns a list of column names
func (t *TableInfo) ColumnNames() []string {
	names := make([]string, len(t.Columns))
	for i, col := range t.Columns {
		names[i] = col.Name
	}
	return names
}

// Row represents a row of values
type Row struct {
	ID    uint64
	Data  []Value
}

// NewRow creates a new row
func NewRow(data []Value) *Row {
	return &Row{Data: data}
}

// Clone creates a deep copy of the row
func (r *Row) Clone() *Row {
	data := make([]Value, len(r.Data))
	for i, v := range r.Data {
		data[i] = v.Clone()
	}
	return &Row{ID: r.ID, Data: data}
}

// Value marshaling/unmarshaling for JSON

type valueJSON struct {
	Type    string      `json:"type"`
	Data    interface{} `json:"data,omitempty"`
	IsNull  bool        `json:"is_null"`
	BlobRef *BlobRef    `json:"blob_ref,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (v Value) MarshalJSON() ([]byte, error) {
	vj := valueJSON{
		Type:   v.Type.String(),
		IsNull: v.IsNull,
	}

	if !v.IsNull {
		// If there's a blob reference, include it instead of the data
		if v.BlobRef != nil {
			vj.BlobRef = v.BlobRef
		} else {
			switch v.Type {
			case TypeDate, TypeTime, TypeDatetime:
				if t, ok := v.Data.(time.Time); ok {
					vj.Data = t.Format(time.RFC3339)
				}
			case TypeBlob:
				if data, ok := v.Data.([]byte); ok {
					vj.Data = data
				}
			default:
				vj.Data = v.Data
			}
		}
	}

	return json.Marshal(vj)
}

// UnmarshalJSON implements json.Unmarshaler
func (v *Value) UnmarshalJSON(data []byte) error {
	var vj valueJSON
	if err := json.Unmarshal(data, &vj); err != nil {
		return err
	}

	v.Type = ParseDataType(vj.Type)
	v.IsNull = vj.IsNull
	v.BlobRef = vj.BlobRef

	if !vj.IsNull && vj.Data != nil && vj.BlobRef == nil {
		switch v.Type {
		case TypeInt, TypeSeq:
			// JSON numbers are float64
			if f, ok := vj.Data.(float64); ok {
				v.Data = int64(f)
			} else {
				v.Data = vj.Data
			}
		case TypeDate, TypeTime, TypeDatetime:
			if s, ok := vj.Data.(string); ok {
				t, _ := time.Parse(time.RFC3339, s)
				v.Data = t
			}
		default:
			v.Data = vj.Data
		}
	}

	return nil
}
