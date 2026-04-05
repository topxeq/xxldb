// Package mysql provides MySQL protocol server tests
package mysql

import (
	"testing"
)

func TestLengthEncodedInt(t *testing.T) {
	tests := []struct {
		n        uint64
		expected []byte
	}{
		{0, []byte{0x00}},
		{1, []byte{0x01}},
		{250, []byte{0xfa}},
		{251, []byte{0xfc, 0xfb, 0x00}},
		{65535, []byte{0xfc, 0xff, 0xff}},
		{65536, []byte{0xfd, 0x00, 0x00, 0x01}},
		{16777215, []byte{0xfd, 0xff, 0xff, 0xff}},
		{16777216, []byte{0xfe, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00}},
	}

	for _, tt := range tests {
		result := writeLengthEncodedInt(tt.n)
		if !equalBytes(result, tt.expected) {
			t.Errorf("writeLengthEncodedInt(%d) = %v, want %v", tt.n, result, tt.expected)
		}
	}
}

func TestReadLengthEncodedInt(t *testing.T) {
	tests := []struct {
		data     []byte
		expected uint64
	}{
		{[]byte{0x00}, 0},
		{[]byte{0x01}, 1},
		{[]byte{0xfa}, 250},
		{[]byte{0xfc, 0xfb, 0x00}, 251},
		{[]byte{0xfc, 0xff, 0xff}, 65535},
		{[]byte{0xfd, 0x00, 0x00, 0x01}, 65536},
		{[]byte{0xfd, 0xff, 0xff, 0xff}, 16777215},
	}

	for _, tt := range tests {
		result, _, err := ReadLengthEncodedInt(tt.data)
		if err != nil {
			t.Errorf("ReadLengthEncodedInt(%v) error: %v", tt.data, err)
			continue
		}
		if result != tt.expected {
			t.Errorf("ReadLengthEncodedInt(%v) = %d, want %d", tt.data, result, tt.expected)
		}
	}
}

func TestLengthEncodedString(t *testing.T) {
	tests := []string{
		"",
		"hello",
		"hello world",
		"你好世界",
	}

	for _, tt := range tests {
		encoded := writeLengthEncodedString(tt)
		decoded, _, err := ReadLengthEncodedString(encoded)
		if err != nil {
			t.Errorf("ReadLengthEncodedString error: %v", err)
			continue
		}
		if decoded != tt {
			t.Errorf("String encode/decode mismatch: got %q, want %q", decoded, tt)
		}
	}
}

func TestNativePasswordAuth(t *testing.T) {
	// Test empty password
	result := NativePasswordAuth([]byte{}, []byte{1, 2, 3, 4})
	if result != nil {
		t.Errorf("Empty password should return nil")
	}

	// Test with password and salt
	password := []byte("password")
	salt := make([]byte, 20)
	for i := range salt {
		salt[i] = byte(i)
	}

	result = NativePasswordAuth(password, salt)
	if len(result) != 20 {
		t.Errorf("Auth result should be 20 bytes, got %d", len(result))
	}

	// Same password and salt should produce same result
	result2 := NativePasswordAuth(password, salt)
	if !equalBytes(result, result2) {
		t.Errorf("Same password and salt should produce same result")
	}
}

func TestInt64ToString(t *testing.T) {
	tests := []struct {
		n        int64
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{-1, "-1"},
		{12345, "12345"},
		{-12345, "-12345"},
		{9223372036854775807, "9223372036854775807"},
		{-9223372036854775808, "-9223372036854775808"},
	}

	for _, tt := range tests {
		result := int64ToString(tt.n)
		if result != tt.expected {
			t.Errorf("int64ToString(%d) = %q, want %q", tt.n, result, tt.expected)
		}
	}
}

func TestUint64ToString(t *testing.T) {
	tests := []struct {
		n        uint64
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{12345, "12345"},
		{18446744073709551615, "18446744073709551615"},
	}

	for _, tt := range tests {
		result := uint64ToString(tt.n)
		if result != tt.expected {
			t.Errorf("uint64ToString(%d) = %q, want %q", tt.n, result, tt.expected)
		}
	}
}

func TestFloat64ToString(t *testing.T) {
	tests := []struct {
		n        float64
		expected string
	}{
		{0.0, "0"},
		{1.0, "1"},
		{1.5, "1.5"},
		{3.14159, "3.14159"},
		{-1.5, "-1.5"},
	}

	for _, tt := range tests {
		result := float64ToString(tt.n)
		if result != tt.expected {
			t.Errorf("float64ToString(%f) = %q, want %q", tt.n, result, tt.expected)
		}
	}
}

func TestWriteUint16(t *testing.T) {
	tests := []uint16{0, 1, 255, 256, 65535}
	for _, tt := range tests {
		result := writeUint16(tt)
		if len(result) != 2 {
			t.Errorf("writeUint16 should return 2 bytes")
		}
		n := uint16(result[0]) | uint16(result[1])<<8
		if n != tt {
			t.Errorf("writeUint16(%d) roundtrip = %d", tt, n)
		}
	}
}

func TestWriteUint32(t *testing.T) {
	tests := []uint32{0, 1, 255, 256, 65535, 65536, 4294967295}
	for _, tt := range tests {
		result := writeUint32(tt)
		if len(result) != 4 {
			t.Errorf("writeUint32 should return 4 bytes")
		}
	}
}
