// Package diag defines the internal error types that the root package
// converts into its public SyntaxError and UnsupportedError.
package diag

import "fmt"

// SyntaxError is a malformed input at a byte offset.
type SyntaxError struct {
	Offset int
	Msg    string
}

func (e *SyntaxError) Error() string { return fmt.Sprintf("offset %d: %s", e.Offset, e.Msg) }

// Syntaxf returns a *SyntaxError.
func Syntaxf(off int, format string, args ...any) error {
	return &SyntaxError{Offset: off, Msg: fmt.Sprintf(format, args...)}
}

// UnsupportedError is a well-formed construct that the library (or one of its
// outputs) cannot render.
type UnsupportedError struct {
	Command string
	Offset  int
}

func (e *UnsupportedError) Error() string {
	return fmt.Sprintf("offset %d: unsupported %s", e.Offset, e.Command)
}

// Unsupported returns an *UnsupportedError.
func Unsupported(cmd string, off int) error { return &UnsupportedError{Command: cmd, Offset: off} }
