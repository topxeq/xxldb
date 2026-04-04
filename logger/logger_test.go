package logger

import (
	"bytes"
	"strings"
	"testing"
)

func TestLevelString(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{DEBUG, "DEBUG"},
		{INFO, "INFO"},
		{WARN, "WARN"},
		{ERROR, "ERROR"},
		{Level(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		if got := tt.level.String(); got != tt.expected {
			t.Errorf("Level.String() = %s, want %s", got, tt.expected)
		}
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected Level
	}{
		{"DEBUG", DEBUG},
		{"debug", DEBUG},
		{"INFO", INFO},
		{"info", INFO},
		{"WARN", WARN},
		{"warn", WARN},
		{"WARNING", WARN},
		{"ERROR", ERROR},
		{"error", ERROR},
		{"unknown", INFO}, // default
	}

	for _, tt := range tests {
		if got := ParseLevel(tt.input); got != tt.expected {
			t.Errorf("ParseLevel(%s) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestNewLogger(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger(INFO, &buf)

	if log == nil {
		t.Fatal("NewLogger returned nil")
	}
	if log.level != INFO {
		t.Error("Default level should be INFO")
	}
}

func TestLoggerLevel(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger(WARN, &buf)

	// DEBUG and INFO should not be logged
	log.Debug("debug message")
	log.Info("info message")
	if buf.Len() > 0 {
		t.Error("DEBUG and INFO should not be logged at WARN level")
	}

	// WARN and ERROR should be logged
	log.Warn("warn message")
	if buf.Len() == 0 {
		t.Error("WARN should be logged")
	}
	buf.Reset()

	log.Error("error message")
	if buf.Len() == 0 {
		t.Error("ERROR should be logged")
	}
}

func TestLoggerSetLevel(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger(ERROR, &buf)

	// Start at ERROR level
	log.Warn("should not log")
	if buf.Len() > 0 {
		t.Error("WARN should not be logged at ERROR level")
	}

	// Change to DEBUG level
	log.SetLevel(DEBUG)
	log.Debug("debug message")
	if buf.Len() == 0 {
		t.Error("DEBUG should be logged after SetLevel")
	}

	// Test SetLevelFromString
	log.SetLevelFromString("ERROR")
	log.Info("should not log")
	buf.Reset()
	log.Error("error message")
	if buf.Len() == 0 {
		t.Error("ERROR should be logged")
	}
}

func TestLoggerPrefix(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger(INFO, &buf)
	log.SetPrefix("TEST")

	log.Info("message")

	output := buf.String()
	if !strings.Contains(output, "[TEST]") {
		t.Error("Output should contain prefix [TEST]")
	}
}

func TestLoggerWriter(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	log := NewLogger(INFO, &buf1)

	log.Info("message1")
	if buf1.Len() == 0 {
		t.Error("Should write to first buffer")
	}

	// Change writer
	log.SetWriter(&buf2)
	log.Info("message2")

	if buf2.Len() == 0 {
		t.Error("Should write to second buffer")
	}
}

func TestLoggerFormat(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger(INFO, &buf)

	log.Info("value: %d", 42)

	output := buf.String()
	if !strings.Contains(output, "value: 42") {
		t.Errorf("Output should contain formatted message, got: %s", output)
	}
	if !strings.Contains(output, "[INFO]") {
		t.Errorf("Output should contain level, got: %s", output)
	}
}

func TestDefaultLogger(t *testing.T) {
	// Just verify it exists and works
	SetLevel(INFO)
	Debug("should not crash")
	Info("should not crash")
	Warn("should not crash")
	Error("should not crash")
}

func TestGlobalFunctions(t *testing.T) {
	// Test global functions don't panic
	Debug("test")
	Info("test")
	Warn("test")
	Error("test")
}

func TestLoggerConcurrent(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger(INFO, &buf)

	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func(n int) {
			for j := 0; j < 100; j++ {
				log.Info("goroutine %d message %d", n, j)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// Should complete without deadlock or panic
}
