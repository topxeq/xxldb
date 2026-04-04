// Package logger provides logging functionality for xxldb
package logger

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Level represents log level
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

// String returns the string representation of log level
func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel parses a string to Level
func ParseLevel(s string) Level {
	switch s {
	case "DEBUG", "debug":
		return DEBUG
	case "INFO", "info":
		return INFO
	case "WARN", "warn", "WARNING", "warning":
		return WARN
	case "ERROR", "error":
		return ERROR
	default:
		return INFO
	}
}

// Logger is the main logger structure
type Logger struct {
	mu       sync.Mutex
	level    Level
	writer   io.Writer
	prefix   string
	showTime bool
}

// DefaultLogger is the global default logger
var DefaultLogger = NewLogger(INFO, os.Stderr)

// NewLogger creates a new logger
func NewLogger(level Level, writer io.Writer) *Logger {
	if writer == nil {
		writer = os.Stderr
	}
	return &Logger{
		level:    level,
		writer:   writer,
		showTime: true,
	}
}

// SetLevel sets the log level
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// SetLevelFromString sets the log level from string
func (l *Logger) SetLevelFromString(level string) {
	l.SetLevel(ParseLevel(level))
}

// SetWriter sets the output writer
func (l *Logger) SetWriter(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.writer = w
}

// SetPrefix sets the log prefix
func (l *Logger) SetPrefix(prefix string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prefix = prefix
}

// log writes a log message
func (l *Logger) log(level Level, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := ""
	if l.showTime {
		timestamp = time.Now().Format("2006-01-02 15:04:05.000") + " "
	}

	msg := fmt.Sprintf(format, args...)
	levelStr := fmt.Sprintf("[%s] ", level)
	prefix := l.prefix
	if prefix != "" {
		prefix = "[" + prefix + "] "
	}

	fmt.Fprintf(l.writer, "%s%s%s%s\n", timestamp, prefix, levelStr, msg)
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(DEBUG, format, args...)
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(INFO, format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(WARN, format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ERROR, format, args...)
}

// Fatal logs an error message and exits
func (l *Logger) Fatal(format string, args ...interface{}) {
	l.log(ERROR, format, args...)
	os.Exit(1)
}

// Global functions using DefaultLogger

// SetLevel sets the global log level
func SetLevel(level Level) {
	DefaultLogger.SetLevel(level)
}

// SetLevelFromString sets the global log level from string
func SetLevelFromString(level string) {
	DefaultLogger.SetLevelFromString(level)
}

// Debug logs a debug message using DefaultLogger
func Debug(format string, args ...interface{}) {
	DefaultLogger.Debug(format, args...)
}

// Info logs an info message using DefaultLogger
func Info(format string, args ...interface{}) {
	DefaultLogger.Info(format, args...)
}

// Warn logs a warning message using DefaultLogger
func Warn(format string, args ...interface{}) {
	DefaultLogger.Warn(format, args...)
}

// Error logs an error message using DefaultLogger
func Error(format string, args ...interface{}) {
	DefaultLogger.Error(format, args...)
}

// Fatal logs an error message and exits using DefaultLogger
func Fatal(format string, args ...interface{}) {
	DefaultLogger.Fatal(format, args...)
}
