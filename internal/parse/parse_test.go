package parse

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/lea/dal/internal/ast"
	"github.com/lea/dal/internal/diag"
	"github.com/lea/dal/internal/guard"
)

var testLimits = Limits{MaxInputBytes: 16 << 10, MaxNodes: 5000, MaxDepth: 64, MaxExpansions: 1000}

// dump writes a compact form of the tree: symbols as their text, other
// nodes as name(children).
func dump(nodes []ast.Node) string {
	var parts []string
	for _, n := range nodes {
		parts = append(parts, dumpNode(n))
	}
	return strings.Join(parts, " ")
}

func dumpNode(n ast.Node) string {
	switch n := n.(type) {
	case *ast.Symbol:
		return n.Text
	case *ast.Group:
		if n.Class != ast.ClassOrd {
			return n.Class.String() + "{" + dump(n.Body) + "}"
		}
		return "{" + dump(n.Body) + "}"
	case *ast.Scripts:
		s := "s(" + dump([]ast.Node{n.Base})
		if n.HasSup {
			s += " ^" + dump(n.Sup)
		}
		if n.HasSub {
			s += " _" + dump(n.Sub)
		}
		return s + ")"
	case *ast.Frac:
		return fmt.Sprintf("frac%s%s(%s / %s)", n.Left, n.Right, dump(n.Num), dump(n.Den))
	case *ast.Sqrt:
		return "sqrt[" + dump(n.Index) + "](" + dump(n.Body) + ")"
	case *ast.Op:
		return "op:" + n.Text
	case *ast.Space:
		return fmt.Sprintf("sp%g", n.Mu)
	case *ast.Text:
		return "text:" + n.Text
	case *ast.Color:
		return fmt.Sprintf("color#%02x%02x%02x(%s)", n.Color.R, n.Color.G, n.Color.B, dump(n.Body))
	case *ast.Array:
		var rows []string
		for _, r := range n.Rows {
			var cells []string
			for _, c := range r.Cells {
				cells = append(cells, dump(c))
			}
			rows = append(rows, strings.Join(cells, "&"))
		}
		return n.Env + "[" + strings.Join(rows, `\\`) + "]"
	case *ast.LeftRight:
		return "lr" + n.Left + "(" + dump(n.Body) + ")" + n.Right
	case nil:
		return "_"
	}
	return fmt.Sprintf("%T", n)
}

func TestParseTree(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`a+b`, `a + b`},
		{`x^2_i`, `s(x ^2 _i)`},
		{`f''`, `s(f ^′ ′)`},
		{`\frac{a}{b}`, `frac(a / b)`},
		{`\binom nk`, `frac()(n / k)`},
		{`\sqrt[3]{x}`, `sqrt[3](x)`},
		{`\left( x \right]`, `lr((x)]`},
		{`\sum_{i}`, `s(op:∑ _i)`},
		{`\text{a b}`, `text:a b`},
		{`\color{#1a2b3c}x`, `color#1a2b3c(x)`},
		{`\textcolor{#123}{y}`, `color#112233(y)`},
		{`\hspace*{1em}x`, `sp18 x`},
		{`\begin{matrix}a&b\\c&d\end{matrix}`, `matrix[a&b\\c&d]`},
		{`\def\f#1{#1^2}\f{x}`, `s(x ^2)`},
		{`\newcommand\g[2]{#2#1}\g ab`, `b a`},
		{`\def\c#1{\color{#1}z}\c{red}`, `color#ff0000(z)`},
		{`\mathrel{x}`, `rel{x}`},
	} {
		e, err := Parse(c.src, testLimits)
		if err != nil {
			t.Errorf("%s: %v", c.src, err)
			continue
		}
		if got := dump(e.Body); got != c.want {
			t.Errorf("%s:\n got %s\nwant %s", c.src, got, c.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	for _, c := range []struct {
		src  string
		off  int
		msg  string
		kind string // "syntax", "unsupported" or "large"
	}{
		{`{a`, 0, "missing }", "syntax"},
		{`a}`, 1, "unmatched }", "syntax"},
		{`x^1^2`, 3, "double superscript", "syntax"},
		{`\frac{a}`, 0, "missing argument", "syntax"},
		{`\color{#12345g}x`, 0, "invalid color", "syntax"},
		{`a & b`, 2, "misplaced &", "syntax"},
		{`\left( x`, 0, `missing \right`, "syntax"},
		{`\foo`, 0, `\foo`, "unsupported"},
		{`\begin{foo}x\end{foo}`, 0, `\begin{foo}`, "unsupported"},
		{`\kern2000em`, 0, "exceeds 1000", "large"},
	} {
		_, err := Parse(c.src, testLimits)
		var se *diag.SyntaxError
		var ue *diag.UnsupportedError
		switch {
		case err == nil:
			t.Errorf("%s: no error", c.src)
			continue
		case c.kind == "syntax" && errors.As(err, &se):
			if se.Offset != c.off {
				t.Errorf("%s: offset %d, want %d", c.src, se.Offset, c.off)
			}
		case c.kind == "unsupported" && errors.As(err, &ue):
			if ue.Offset != c.off {
				t.Errorf("%s: offset %d, want %d", c.src, ue.Offset, c.off)
			}
		case c.kind == "large" && errors.Is(err, guard.ErrTooLarge):
		default:
			t.Errorf("%s: %v (%T), want a %s error", c.src, err, err, c.kind)
			continue
		}
		if !strings.Contains(err.Error(), c.msg) {
			t.Errorf("%s: %q does not mention %q", c.src, err, c.msg)
		}
	}
}

func TestLimits(t *testing.T) {
	lim := testLimits
	lim.MaxExpansions = 3
	if _, err := Parse(`\def\a{x}\a\a\a\a`, lim); !errors.Is(err, guard.ErrTooLarge) {
		t.Errorf("expansions: %v", err)
	}
	lim = testLimits
	lim.MaxDepth = 5
	if _, err := Parse(strings.Repeat("{", 10)+strings.Repeat("}", 10), lim); !errors.Is(err, guard.ErrTooLarge) {
		t.Errorf("depth: %v", err)
	}
	lim = testLimits
	lim.MaxNodes = 10
	if _, err := Parse(strings.Repeat("a", 20), lim); !errors.Is(err, guard.ErrTooLarge) {
		t.Errorf("nodes: %v", err)
	}
}

// TestParserCommandsListed checks that every command the parser handles in
// command and switchCommand is in parserCommands, so that \newcommand
// cannot redefine it.
func TestParserCommandsListed(t *testing.T) {
	src, err := os.ReadFile("parse.go")
	if err != nil {
		t.Fatal(err)
	}
	// Commands the parser recognizes elsewhere, by name.
	for _, file := range []string{"env.go", "text.go", "parse.go"} {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range regexp.MustCompile(`isCmd\("([^"]+)"\)|x\.text == "([^"$()]+)"`).FindAllStringSubmatch(string(b), -1) {
			name := m[1] + m[2]
			if name == "\\\\" {
				name = "\\"
			}
			if !isBuiltin(name) {
				t.Errorf(`\%s is recognized in %s but not built in`, name, file)
			}
		}
	}
	s := string(src)
	for _, fn := range []string{"func (p *parser) command(", "func (p *parser) switchCommand("} {
		i := strings.Index(s, fn)
		j := strings.Index(s[i:], "\n}\n")
		for _, m := range regexp.MustCompile(`(?m)^\tcase ((?:"[^"]+",?\s*)+):`).FindAllStringSubmatch(s[i:i+j], -1) {
			for _, n := range regexp.MustCompile(`"([^"]+)"`).FindAllStringSubmatch(m[1], -1) {
				if !parserCommands[n[1]] {
					t.Errorf(`\%s is handled by the parser but missing from parserCommands`, n[1])
				}
			}
		}
	}
}

func TestNewcommandBuiltins(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`\providecommand\frac{F}\frac12`, `frac(1 / 2)`},
		{`\providecommand\foo{F}\foo`, `F`},
		{`\renewcommand\alpha{A}\alpha`, `A`},
	} {
		e, err := Parse(c.src, testLimits)
		if err != nil {
			t.Errorf("%s: %v", c.src, err)
			continue
		}
		if got := dump(e.Body); got != c.want {
			t.Errorf("%s: %s, want %s", c.src, got, c.want)
		}
	}
	for _, src := range []string{`\newcommand\frac{F}`, `\newcommand\x{X}\newcommand\x{Y}`, `\newcommand\alpha{A}`} {
		if _, err := Parse(src, testLimits); err == nil {
			t.Errorf("%s: no error", src)
		}
	}
}

func TestNestedParameters(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`\def\a{\def\b##1{[##1]}}\a\b{x}`, `[ x ]`},
		{`\def\a#1{\def\b##1{#1##1}}\a{p}\b{q}`, `p q`},
	} {
		e, err := Parse(c.src, testLimits)
		if err != nil {
			t.Errorf("%s: %v", c.src, err)
			continue
		}
		if got := dump(e.Body); got != c.want {
			t.Errorf("%s: %s, want %s", c.src, got, c.want)
		}
	}
}

// TestHashRuns checks TeX's ## rule for runs of # before a parameter.
func TestHashRuns(t *testing.T) {
	arg := [][]token{{{kind: tChar, text: "A"}}}
	for _, c := range []struct{ body, want string }{
		{"#1", "A"},
		{"##1", "#1"},    // the parameter of an inner definition
		{"###1", "#A"},   // ## is #, then the argument
		{"####1", "##1"}, // two levels down
		{"x##y", "x#y"},
	} {
		toks, err := lex(c.body)
		if err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		_ = expandBody(toks, arg, func(ts ...token) error {
			for _, tk := range ts {
				if tk.kind == tParam {
					b.WriteString("#")
				}
				b.WriteString(tk.text)
			}
			return nil
		})
		if b.String() != c.want {
			t.Errorf("%s: %s, want %s", c.body, b.String(), c.want)
		}
	}
}
