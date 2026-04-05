// Package types provides error definitions for xxldb
package types

import "fmt"

// Error codes
const (
	ErrUnknown ErrorCode = iota
	ErrNotFound
	ErrDuplicateKey
	ErrInvalidType
	ErrInvalidValue
	ErrConstraintViolation
	ErrSyntaxError
	ErrPermissionDenied
)

// ErrorCode represents an error code
type ErrorCode int

// Error represents a database error
type Error struct {
	Code    ErrorCode
	Message string
}

// Error implements the error interface
func (e *Error) Error() string {
	return e.Message
}

// NewError creates a new error
func NewError(code ErrorCode, format string, args ...interface{}) *Error {
	return &Error{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	}
}
