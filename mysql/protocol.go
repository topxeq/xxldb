// Package mysql provides MySQL protocol server for xxldb
package mysql

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"io"
	"strconv"
)

// MySQL protocol constants
const (
	// Protocol version
	ProtocolVersion = 10

	// Default server capabilities
	DEFAULT_CAPABILITIES = CLIENT_LONG_PASSWORD |
		CLIENT_LONG_FLAG |
		CLIENT_LOCAL_FILES |
		CLIENT_PROTOCOL_41 |
		CLIENT_TRANSACTIONS |
		CLIENT_SECURE_CONN |
		CLIENT_MULTI_STATEMENTS |
		CLIENT_MULTI_RESULTS |
		CLIENT_PS_MULTI_RESULTS |
		CLIENT_PLUGIN_AUTH |
		CLIENT_CONNECT_ATTRS |
		CLIENT_PLUGIN_AUTH_LENENC_CLIENT_DATA

	// Client capabilities
	CLIENT_LONG_PASSWORD             = 0x00000001
	CLIENT_FOUND_ROWS                = 0x00000002
	CLIENT_LONG_FLAG                 = 0x00000004
	CLIENT_CONNECT_WITH_DB           = 0x00000008
	CLIENT_NO_SCHEMA                 = 0x00000010
	CLIENT_COMPRESS                  = 0x00000020
	CLIENT_ODBC                      = 0x00000040
	CLIENT_LOCAL_FILES               = 0x00000080
	CLIENT_IGNORE_SPACE              = 0x00000100
	CLIENT_PROTOCOL_41               = 0x00000200
	CLIENT_SSL                       = 0x00000800
	CLIENT_TRANSACTIONS              = 0x00002000
	CLIENT_SECURE_CONN               = 0x00008000
	CLIENT_MULTI_STATEMENTS          = 0x00010000
	CLIENT_MULTI_RESULTS             = 0x00020000
	CLIENT_PS_MULTI_RESULTS          = 0x00040000
	CLIENT_PLUGIN_AUTH               = 0x00080000
	CLIENT_CONNECT_ATTRS             = 0x00100000
	CLIENT_PLUGIN_AUTH_LENENC_CLIENT_DATA = 0x00200000

	// Auth plugin names
	AUTH_PLUGIN_NAME              = "mysql_native_password"
	AUTH_PLUGIN_CACHING_SHA2      = "caching_sha2_password"

	// Auth switch response
	AUTH_SWITCH_REQUEST = 0xfe

	// Command types
	COM_SLEEP          = 0x00
	COM_QUIT           = 0x01
	COM_INIT_DB        = 0x02
	COM_QUERY          = 0x03
	COM_FIELD_LIST     = 0x04
	COM_CREATE_DB      = 0x05
	COM_DROP_DB        = 0x06
	COM_REFRESH        = 0x07
	COM_SHUTDOWN       = 0x08
	COM_STATISTICS     = 0x09
	COM_PROCESS_INFO   = 0x0a
	COM_CONNECT        = 0x0b
	COM_PROCESS_KILL   = 0x0c
	COM_DEBUG          = 0x0d
	COM_PING           = 0x0e
	COM_CHANGE_USER    = 0x11
	COM_BINLOG_DUMP    = 0x12
	COM_TABLE_DUMP     = 0x13
	COM_CONNECT_OUT    = 0x14
	COM_REGISTER_SLAVE = 0x15
	COM_STMT_PREPARE   = 0x16
	COM_STMT_EXECUTE   = 0x17
	COM_STMT_SEND_LONG_DATA = 0x18
	COM_STMT_CLOSE     = 0x19
	COM_STMT_RESET     = 0x1a
	COM_SET_OPTION     = 0x1b
	COM_STMT_FETCH     = 0x1c
	COM_DAEMON         = 0x1d
	COM_BINLOG_DUMP_GTID = 0x1e
	COM_RESET_CONNECTION = 0x1f

	// Column types
	MYSQL_TYPE_DECIMAL    = 0x00
	MYSQL_TYPE_TINY       = 0x01
	MYSQL_TYPE_SHORT      = 0x02
	MYSQL_TYPE_LONG       = 0x03
	MYSQL_TYPE_FLOAT      = 0x04
	MYSQL_TYPE_DOUBLE     = 0x05
	MYSQL_TYPE_NULL       = 0x06
	MYSQL_TYPE_TIMESTAMP  = 0x07
	MYSQL_TYPE_LONGLONG   = 0x08
	MYSQL_TYPE_INT24      = 0x09
	MYSQL_TYPE_DATE       = 0x0a
	MYSQL_TYPE_TIME       = 0x0b
	MYSQL_TYPE_DATETIME   = 0x0c
	MYSQL_TYPE_YEAR       = 0x0d
	MYSQL_TYPE_VARCHAR    = 0xfd
	MYSQL_TYPE_BIT        = 0xfe
	MYSQL_TYPE_JSON       = 0xf5
	MYSQL_TYPE_NEWDECIMAL = 0xf6
	MYSQL_TYPE_ENUM       = 0xf7
	MYSQL_TYPE_SET        = 0xf8
	MYSQL_TYPE_TINY_BLOB  = 0xf9
	MYSQL_TYPE_MEDIUM_BLOB = 0xfa
	MYSQL_TYPE_LONG_BLOB  = 0xfb
	MYSQL_TYPE_BLOB       = 0xfc
	MYSQL_TYPE_VAR_STRING = 0xfd
	MYSQL_TYPE_STRING     = 0xfe
	MYSQL_TYPE_GEOMETRY   = 0xff

	// OK/EOF packet header
	OK_HEADER  = 0x00
	EOF_HEADER = 0xfe
	ERR_HEADER = 0xff
)

// PacketReader reads MySQL packets
type PacketReader struct {
	r io.Reader
}

// NewPacketReader creates a new packet reader
func NewPacketReader(r io.Reader) *PacketReader {
	return &PacketReader{r: r}
}

// ReadPacket reads a MySQL packet
func (r *PacketReader) ReadPacket() ([]byte, error) {
	var header [4]byte
	_, err := io.ReadFull(r.r, header[:])
	if err != nil {
		return nil, err
	}

	length := int(uint32(header[0]) | uint32(header[1])<<8 | uint32(header[2])<<16)
	// sequence := header[3]

	if length == 0 {
		return nil, nil
	}

	data := make([]byte, length)
	_, err = io.ReadFull(r.r, data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// PacketWriter writes MySQL packets
type PacketWriter struct {
	w         io.Writer
	sequence  uint8
}

// NewPacketWriter creates a new packet writer
func NewPacketWriter(w io.Writer) *PacketWriter {
	return &PacketWriter{w: w, sequence: 0}
}

// WritePacket writes a MySQL packet
func (w *PacketWriter) WritePacket(data []byte) error {
	length := len(data)
	if length > 0xffffff {
		// Split large packets
		for len(data) > 0 {
			chunk := data
			if len(chunk) > 0xffffff {
				chunk = data[:0xffffff]
			}
			if err := w.writePacketInternal(chunk); err != nil {
				return err
			}
			data = data[len(chunk):]
		}
		return nil
	}
	return w.writePacketInternal(data)
}

func (w *PacketWriter) writePacketInternal(data []byte) error {
	length := len(data)

	header := []byte{
		byte(length),
		byte(length >> 8),
		byte(length >> 16),
		w.sequence,
	}
	w.sequence++

	if _, err := w.w.Write(header); err != nil {
		return err
	}

	if len(data) > 0 {
		_, err := w.w.Write(data)
		return err
	}

	return nil
}

// ResetSequence resets the sequence number
func (w *PacketWriter) ResetSequence() {
	w.sequence = 0
}

// WriteOK writes an OK packet
func (w *PacketWriter) WriteOK(affectedRows, insertID uint64, status uint16, warnings uint16, message string) error {
	var buf bytes.Buffer

	buf.WriteByte(OK_HEADER)
	buf.Write(writeLengthEncodedInt(affectedRows))
	buf.Write(writeLengthEncodedInt(insertID))
	buf.Write(writeUint16(status))
	buf.Write(writeUint16(warnings))
	if message != "" {
		buf.Write(writeLengthEncodedString(message))
	}

	return w.WritePacket(buf.Bytes())
}

// WriteEOF writes an EOF packet
func (w *PacketWriter) WriteEOF(warnings, status uint16) error {
	data := []byte{
		EOF_HEADER,
		0x00, 0x00, // warnings
		byte(status), byte(status >> 8),
	}
	return w.WritePacket(data)
}

// WriteError writes an error packet
func (w *PacketWriter) WriteError(code uint16, sqlState string, message string) error {
	var buf bytes.Buffer

	buf.WriteByte(ERR_HEADER)
	buf.Write(writeUint16(code))
	buf.WriteByte('#') // SQL state marker
	if len(sqlState) < 5 {
		sqlState = "HY000" // Default
	}
	buf.WriteString(sqlState[:5])
	buf.WriteString(message)

	return w.WritePacket(buf.Bytes())
}

// WriteResultHeader writes result set header packet
func (w *PacketWriter) WriteResultHeader(columnCount uint64) error {
	return w.WritePacket(writeLengthEncodedInt(columnCount))
}

// WriteColumnDefinition writes a column definition packet
func (w *PacketWriter) WriteColumnDefinition(schema, table, orgTable, name, orgName string,
	columnLength uint32, charset uint16, columnType byte, decimals byte, flags uint16) error {

	var buf bytes.Buffer

	// Catalog (always "def")
	buf.Write(writeLengthEncodedString("def"))

	// Schema
	buf.Write(writeLengthEncodedString(schema))

	// Table
	buf.Write(writeLengthEncodedString(table))

	// Org table
	buf.Write(writeLengthEncodedString(orgTable))

	// Name
	buf.Write(writeLengthEncodedString(name))

	// Org name
	buf.Write(writeLengthEncodedString(orgName))

	// Length of fixed-length fields (0x0c)
	buf.WriteByte(0x0c)

	// Character set
	buf.Write(writeUint16(charset))

	// Column length
	buf.Write(writeUint32(columnLength))

	// Column type
	buf.WriteByte(columnType)

	// Flags
	buf.Write(writeUint16(flags))

	// Decimals
	buf.WriteByte(decimals)

	// Filler
	buf.Write([]byte{0x00, 0x00})

	// No extended metadata

	return w.WritePacket(buf.Bytes())
}

// WriteRowData writes a row data packet
func (w *PacketWriter) WriteRowData(values []interface{}) error {
	var buf bytes.Buffer

	for _, v := range values {
		if v == nil {
			buf.WriteByte(0xfb) // NULL
		} else {
			buf.Write(writeLengthEncodedString(anyToString(v)))
		}
	}

	return w.WritePacket(buf.Bytes())
}

// WriteBinaryRowData writes a binary row data packet for prepared statements
// Binary protocol row format: header(0x00) + null_bitmap + values (no type info)
func (w *PacketWriter) WriteBinaryRowData(values []interface{}) error {
	var buf bytes.Buffer

	// Header: 0x00
	buf.WriteByte(0x00)

	// Null bitmap: (num_columns + 7 + 2) / 8 bytes
	// The first 2 bits are reserved
	numCols := len(values)
	nullBitmapSize := (numCols + 7 + 2) / 8
	nullBitmap := make([]byte, nullBitmapSize)

	// Build null bitmap
	for i, v := range values {
		if v == nil {
			bytePos := (i + 2) / 8
			bitPos := (i + 2) % 8
			nullBitmap[bytePos] |= (1 << uint(bitPos))
		}
	}
	buf.Write(nullBitmap)

	// Write values in binary format
	// Since we declared columns as MYSQL_TYPE_VAR_STRING, all values should be sent as strings
	for _, v := range values {
		if v == nil {
			continue // NULL is indicated in bitmap
		}
		// Convert to string and write as length-encoded string
		buf.Write(writeLengthEncodedString(anyToString(v)))
	}

	return w.WritePacket(buf.Bytes())
}

// Helper functions

func writeLengthEncodedInt(n uint64) []byte {
	switch {
	case n <= 250:
		return []byte{byte(n)}
	case n <= 0xffff:
		return []byte{0xfc, byte(n), byte(n >> 8)}
	case n <= 0xffffff:
		return []byte{0xfd, byte(n), byte(n >> 8), byte(n >> 16)}
	default:
		return []byte{0xfe, byte(n), byte(n >> 8), byte(n >> 16), byte(n >> 24),
			byte(n >> 32), byte(n >> 40), byte(n >> 48), byte(n >> 56)}
	}
}

func writeLengthEncodedString(s string) []byte {
	data := writeLengthEncodedInt(uint64(len(s)))
	return append(data, []byte(s)...)
}

func writeUint16(n uint16) []byte {
	return []byte{byte(n), byte(n >> 8)}
}

func writeUint32(n uint32) []byte {
	return []byte{byte(n), byte(n >> 8), byte(n >> 16), byte(n >> 24)}
}

func anyToString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case []byte:
		return string(val)
	case string:
		return val
	default:
		return anyToStringGeneric(v)
	}
}

func anyToStringGeneric(v interface{}) string {
	if v == nil {
		return ""
	}

	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	case int:
		return int64ToString(int64(val))
	case int8:
		return int64ToString(int64(val))
	case int16:
		return int64ToString(int64(val))
	case int32:
		return int64ToString(int64(val))
	case int64:
		return int64ToString(val)
	case uint:
		return uint64ToString(uint64(val))
	case uint8:
		return uint64ToString(uint64(val))
	case uint16:
		return uint64ToString(uint64(val))
	case uint32:
		return uint64ToString(uint64(val))
	case uint64:
		return uint64ToString(val)
	case float32:
		return float64ToString(float64(val))
	case float64:
		return float64ToString(val)
	case bool:
		if val {
			return "1"
		}
		return "0"
	default:
		return ""
	}
}

func int64ToString(n int64) string {
	if n == 0 {
		return "0"
	}

	// Handle overflow case for math.MinInt64
	if n == -9223372036854775808 {
		return "-9223372036854775808"
	}

	neg := n < 0
	if neg {
		n = -n
	}

	// Count digits
	tmp := n
	digits := 0
	for tmp > 0 {
		digits++
		tmp /= 10
	}

	// Create buffer for result
	result := make([]byte, digits)
	if neg {
		result = make([]byte, digits+1)
		result[0] = '-'
	}

	// Fill digits from right to left
	for i := len(result) - 1; i >= 0 && n > 0; i-- {
		result[i] = byte('0' + n%10)
		n /= 10
	}

	return string(result)
}

func uint64ToString(n uint64) string {
	if n == 0 {
		return "0"
	}

	// Count digits
	tmp := n
	digits := 0
	for tmp > 0 {
		digits++
		tmp /= 10
	}

	// Create buffer for result
	result := make([]byte, digits)

	// Fill digits from right to left
	for i := digits - 1; i >= 0 && n > 0; i-- {
		result[i] = byte('0' + n%10)
		n /= 10
	}

	return string(result)
}

func float64ToString(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// ReadLengthEncodedInt reads a length-encoded integer
func ReadLengthEncodedInt(data []byte) (uint64, int, error) {
	if len(data) == 0 {
		return 0, 0, io.EOF
	}

	switch data[0] {
	case 0xfb:
		return 0, 1, nil
	case 0xfc:
		if len(data) < 3 {
			return 0, 0, io.EOF
		}
		return uint64(data[1]) | uint64(data[2])<<8, 3, nil
	case 0xfd:
		if len(data) < 4 {
			return 0, 0, io.EOF
		}
		return uint64(data[1]) | uint64(data[2])<<8 | uint64(data[3])<<16, 4, nil
	case 0xfe:
		if len(data) < 9 {
			return 0, 0, io.EOF
		}
		return binary.LittleEndian.Uint64(data[1:9]), 9, nil
	default:
		return uint64(data[0]), 1, nil
	}
}

// ReadLengthEncodedString reads a length-encoded string
func ReadLengthEncodedString(data []byte) (string, int, error) {
	length, n, err := ReadLengthEncodedInt(data)
	if err != nil {
		return "", 0, err
	}

	total := n + int(length)
	if len(data) < total {
		return "", 0, io.EOF
	}

	return string(data[n : n+int(length)]), total, nil
}

// NativePasswordAuth implements mysql_native_password authentication
func NativePasswordAuth(password, salt []byte) []byte {
	if len(password) == 0 {
		return nil
	}

	// SHA1(password) XOR SHA1(salt + SHA1(SHA1(password)))
	hash1 := sha1.Sum(password)
	hash2 := sha1.Sum(hash1[:])
	combined := append(salt, hash2[:]...)
	hash3 := sha1.Sum(combined)

	result := make([]byte, 20)
	for i := 0; i < 20; i++ {
		result[i] = hash1[i] ^ hash3[i]
	}

	return result
}
