// Package parse turns LaTeX math source into the syntax tree of package ast.
// It tokenizes, expands user macros (\newcommand, \def) and parses.
package parse

import (
	"strings"
	"unicode/utf8"

	"github.com/lea-151107/dal/internal/diag"
)

type tokKind uint8

const (
	tEOF   tokKind = iota
	tChar          // a single character; text is the character
	tCmd           // a control sequence; text is the name without "\"
	tSpace         // one or more white space characters
	tParam         // #1..#9 inside macro definitions; text is the digit
	tVerb          // \verb!...!; text is the verbatim contents
	tURL           // the {...} argument of \url and \href, taken verbatim
)

type token struct {
	kind tokKind
	text string
	off  int
	// seen marks a token the expander has already delivered once (and
	// that was pushed back): it is not expanded or counted as a group
	// boundary again.
	seen bool
}

func (t token) isChar(c string) bool { return t.kind == tChar && t.text == c }
func (t token) isCmd(c string) bool  { return t.kind == tCmd && t.text == c }

func isLetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' }

// lex splits src into tokens. Comments are dropped and runs of white space
// become one tSpace token; spaces after a control word are skipped as in TeX.
func lex(src string) ([]token, error) {
	var out []token
	i := 0
	for i < len(src) {
		c := src[i]
		switch {
		case isSpace(c):
			start := i
			for i < len(src) && isSpace(src[i]) {
				i++
			}
			out = append(out, token{kind: tSpace, text: " ", off: start})
		case c == '%':
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case c == '\\':
			start := i
			i++
			if i >= len(src) {
				return nil, diag.Syntaxf(start, "unexpected end of input after \\")
			}
			if isLetter(src[i]) {
				j := i
				for j < len(src) && isLetter(src[j]) {
					j++
				}
				if src[i:j] == "verb" {
					// \verb takes the next character (after an optional *)
					// as its delimiter and keeps everything up to the next
					// one as it is.
					if j < len(src) && src[j] == '*' {
						j++
					}
					if j >= len(src) || isSpace(src[j]) || isLetter(src[j]) {
						return nil, diag.Syntaxf(start, "\\verb needs a delimiter")
					}
					d, size := utf8.DecodeRuneInString(src[j:])
					if d == utf8.RuneError && size <= 1 {
						return nil, diag.Syntaxf(j, "invalid UTF-8")
					}
					if isControl(d) {
						return nil, diag.Syntaxf(j, "control character U+%04X", d)
					}
					end := strings.Index(src[j+size:], string(d))
					if end < 0 {
						return nil, diag.Syntaxf(start, "missing closing %c for \\verb", d)
					}
					body := src[j+size : j+size+end]
					for k, r := range body {
						if r == utf8.RuneError {
							if _, n := utf8.DecodeRuneInString(body[k:]); n <= 1 {
								return nil, diag.Syntaxf(j+size+k, "invalid UTF-8")
							}
						}
						if isControl(r) {
							return nil, diag.Syntaxf(start, "control character U+%04X", r)
						}
					}
					out = append(out, token{kind: tVerb, text: body, off: start})
					i = j + size + end + size
					continue
				}
				out = append(out, token{kind: tCmd, text: src[i:j], off: start})
				i = j
				if name := src[start+1 : j]; name == "url" || name == "href" {
					// The URL is taken as it is: % and # are characters
					// in it, not a comment and a parameter.
					k := i
					for k < len(src) && isSpace(src[k]) {
						k++
					}
					if k < len(src) && src[k] == '{' {
						url, n, err := lexURL(src, k)
						if err != nil {
							return nil, err
						}
						out = append(out, token{kind: tURL, text: url, off: k})
						i = n
						continue
					}
				}
				for i < len(src) && isSpace(src[i]) {
					i++
				}
				continue
			}
			r, size := utf8.DecodeRuneInString(src[i:])
			if r == utf8.RuneError && size <= 1 {
				return nil, diag.Syntaxf(i, "invalid UTF-8")
			}
			if isControl(r) {
				return nil, diag.Syntaxf(i, "control character U+%04X", r)
			}
			out = append(out, token{kind: tCmd, text: src[i : i+size], off: start})
			i += size
			if r == ' ' {
				// "\ " consumed its space; skip the rest of the run.
				for i < len(src) && isSpace(src[i]) {
					i++
				}
			}
		case c == '#':
			if i+1 < len(src) && src[i+1] >= '1' && src[i+1] <= '9' {
				out = append(out, token{kind: tParam, text: src[i+1 : i+2], off: i})
				i += 2
				continue
			}
			// A lone # is only meaningful in a color (\color{#c03}); the
			// parser rejects it everywhere else.
			out = append(out, token{kind: tChar, text: "#", off: i})
			i++
			continue
		default:
			r, size := utf8.DecodeRuneInString(src[i:])
			if r == utf8.RuneError && size <= 1 {
				return nil, diag.Syntaxf(i, "invalid UTF-8")
			}
			if isControl(r) {
				return nil, diag.Syntaxf(i, "control character U+%04X", r)
			}
			out = append(out, token{kind: tChar, text: src[i : i+size], off: i})
			i += size
		}
	}
	return out, nil
}

// isControl reports characters that must not reach the output: C0 and C1
// controls (a C1 CSI drives some terminals), the line and paragraph
// separators (which break the single line of RenderLine and the lines of
// RenderText like a newline would) and the bidirectional formatting
// characters, which can make the output display differently from what it
// contains.
func isControl(r rune) bool {
	switch {
	case r < 0x20, r >= 0x7F && r <= 0x9F, r == 0x2028, r == 0x2029:
		return true
	case r == 0x061C, r == 0x200E, r == 0x200F,
		r >= 0x202A && r <= 0x202E, r >= 0x2066 && r <= 0x2069:
		return true
	}
	return false
}

// lexURL reads the braced URL that starts at src[open] ('{') and returns
// it, unescaped (\% is %), and the index after the closing brace.
func lexURL(src string, open int) (string, int, error) {
	var b strings.Builder
	depth := 0
	for i := open + 1; i < len(src); i++ {
		c := src[i]
		switch {
		case c == '\\' && i+1 < len(src) && strings.IndexByte("#$%&~_^{}\\", src[i+1]) >= 0:
			i++
			b.WriteByte(src[i])
			continue
		case c == '{':
			depth++
		case c == '}':
			if depth == 0 {
				return b.String(), i + 1, nil
			}
			depth--
		}
		r, size := utf8.DecodeRuneInString(src[i:])
		if r == utf8.RuneError && size <= 1 {
			return "", 0, diag.Syntaxf(i, "invalid UTF-8")
		}
		if isControl(r) {
			return "", 0, diag.Syntaxf(i, "control character U+%04X", r)
		}
		b.WriteString(src[i : i+size])
		i += size - 1
	}
	return "", 0, diag.Syntaxf(open, "missing } after the URL")
}
