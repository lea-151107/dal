package dal

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/lea-151107/dal/internal/diag"
	"github.com/lea-151107/dal/internal/guard"
)

// ErrTooLarge reports that the input exceeded a limit (Limits, the pixel
// limit, SizePx or PaddingPx over 1024, a dimension over 1000 units, or an
// internal bound: tokens after macro expansion, text grid cells, drawing
// work, parts of a stretched symbol) or that the output does not fit in
// MaxWidth or MaxCols.
var ErrTooLarge = errors.New("dal: formula too large")

// ErrUnsupported reports a command or environment that is not supported,
// either at all or by the requested output. Every *UnsupportedError matches
// it with errors.Is.
var ErrUnsupported = errors.New("dal: unsupported")

// UnsupportedError reports an unsupported construct.
type UnsupportedError struct {
	Command string // the construct, e.g. `\begin{align}` or `\cancel`
	Offset  int    // byte offset in the source
}

func (e *UnsupportedError) Error() string {
	return fmt.Sprintf("dal: offset %d: unsupported %s", e.Offset, e.Command)
}

// Is makes errors.Is(err, ErrUnsupported) true.
func (e *UnsupportedError) Is(target error) bool { return target == ErrUnsupported }

// SyntaxError reports malformed input such as unbalanced braces or a
// missing argument.
type SyntaxError struct {
	Offset int // byte offset in the source
	Line   int // 1-based line
	Col    int // 1-based column, in bytes
	Msg    string
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("dal: %d:%d: %s", e.Line, e.Col, e.Msg)
}

// lineCol converts a byte offset to a 1-based line and column.
func lineCol(src string, off int) (line, col int) {
	if off > len(src) {
		off = len(src)
	}
	if off < 0 {
		off = 0
	}
	line = 1 + strings.Count(src[:off], "\n")
	col = off - strings.LastIndexByte(src[:off], '\n')
	return line, col
}

// shorten cuts s to at most n bytes at a character boundary, marking the
// cut with "...". Names quoted in errors come from the input and macros
// can make them up to the expansion limit long.
func shorten(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n] + "..."
}

// convertErr maps internal errors to the public ones.
func convertErr(src string, err error) error {
	if err == nil {
		return nil
	}
	var se *diag.SyntaxError
	if errors.As(err, &se) {
		l, c := lineCol(src, se.Offset)
		return &SyntaxError{Offset: se.Offset, Line: l, Col: c, Msg: shorten(se.Msg, 256)}
	}
	var ue *diag.UnsupportedError
	if errors.As(err, &ue) {
		return &UnsupportedError{Command: shorten(ue.Command, 128), Offset: ue.Offset}
	}
	if errors.Is(err, guard.ErrTooLarge) {
		msg := strings.TrimPrefix(err.Error(), guard.ErrTooLarge.Error()+": ")
		return fmt.Errorf("%w: %s", ErrTooLarge, shorten(msg, 256))
	}
	return fmt.Errorf("dal: %w", err)
}
