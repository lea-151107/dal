package dal

// Regression tests for defects found in reviews (formerly review_test.go,
// review2_test.go and review3_test.go).

import (
	"context"
	"errors"
	"fmt"
	"image"
	"math"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/lea/dal/internal/textgrid"
)

// TestAmplification feeds small inputs that used to blow up into huge
// allocations (macro bodies with many #1, ragged arrays, many rules) and
// checks that every output stops them early with the default limits.
func TestAmplification(t *testing.T) {
	ragged := `\begin{matrix}` + strings.Repeat("&", 2400) + strings.Repeat(`\\`, 2400) + `\end{matrix}`
	rules := `\begin{array}{` + strings.Repeat("|", 2000) + `c}` + strings.Repeat(`\hline `, 2000) + `a\end{array}`
	cases := map[string]string{
		"macro one level": `\def\f#1{` + strings.Repeat("#1", 4000) + `}\f{` + strings.Repeat("a", 7000) + `}`,
		"macro two levels": `\def\f#1{` + strings.Repeat("#1", 1000) + `}\def\w#1{\f{` +
			strings.Repeat("#1", 5000) + `}}\w{aaaaaaaaaa}`,
		"ragged array": ragged,
		"rules":        rules,
	}
	for name, src := range cases {
		if len(src) > 16<<10 {
			t.Fatalf("%s: input of %d bytes is over the default limit", name, len(src))
		}
		for out, run := range map[string]func() error{
			"render": func() error { _, err := Render(context.Background(), src, Options{}); return err },
			"text":   func() error { _, err := RenderText(context.Background(), src, TextOptions{}); return err },
			"line":   func() error { _, err := RenderLine(src, LineOptions{}); return err },
		} {
			var before, after runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&before)
			start := time.Now()
			err := run()
			d := time.Since(start)
			runtime.ReadMemStats(&after)
			// The rules case is within the node limit; image and line
			// output are small and may succeed, the text grid may not.
			mayPass := name == "rules" && out != "text"
			if !errors.Is(err, ErrTooLarge) && !(mayPass && err == nil) {
				t.Errorf("%s/%s: got %v, want ErrTooLarge", name, out, err)
			}
			if d > time.Second {
				t.Errorf("%s/%s: took %v", name, out, d)
			}
			if alloc := after.TotalAlloc - before.TotalAlloc; alloc > 64<<20 {
				t.Errorf("%s/%s: allocated %d MiB", name, out, alloc>>20)
			}
		}
	}
}

// TestWorkBounds checks inputs that slipped past the limits: long rules
// in text output (cells outside the grid budget), long macro delimiters
// (quadratic matching), overlapping TextFace text (not counted as drawing
// work) and a huge image whose pixel count overflows int.
func TestWorkBounds(t *testing.T) {
	maxTime := time.Second
	measure := func(t *testing.T, name string, run func() error, want error, maxAlloc uint64) {
		t.Helper()
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		start := time.Now()
		err := run()
		d := time.Since(start)
		runtime.ReadMemStats(&after)
		if !errors.Is(err, want) {
			t.Errorf("%s: got %v, want %v", name, err, want)
		}
		if d > maxTime {
			t.Errorf("%s: took %v", name, d)
		}
		if alloc := after.TotalAlloc - before.TotalAlloc; alloc > maxAlloc {
			t.Errorf("%s: allocated %d MiB", name, alloc>>20)
		}
	}

	rules := `\def\r{` + strings.Repeat(`\rule{1000in}{1pt}`, 4) + `}\def\s{` + strings.Repeat(`\r`, 10) + `}` + strings.Repeat(`\s`, 80)
	// The text grid budget is 1<<22 cells of 16 bytes each.
	measure(t, "rules/text", func() error {
		_, err := RenderText(context.Background(), rules, TextOptions{})
		return err
	}, ErrTooLarge, 128<<20)

	a := strings.Repeat("a", 5120)
	delim := `\def\D#1{\def\x##1#1#1#1#1b{}}\D{` + a + `}\def\F#1{\x` + strings.Repeat("#1", 8) + `b}\F{` + a + `}`
	// Linear matching takes milliseconds; quadratic matching took about
	// a second.
	maxTime = 250 * time.Millisecond
	measure(t, "delimiter/line", func() error { _, err := RenderLine(delim, LineOptions{}); return err }, nil, 64<<20)
	measure(t, "delimiter/text", func() error {
		_, err := RenderText(context.Background(), delim, TextOptions{})
		return err
	}, nil, 64<<20)

	// \url{#1#1} doubles a verbatim URL, which is one token at any length.
	url := `\def\a#1.{\url{#1#1}}\def\b{\url{x}}` + strings.Repeat(`\edef\b{\expandafter\a\b.}`, 40) + `\b`
	measure(t, "url/line", func() error { _, err := RenderLine(url, LineOptions{}); return err }, ErrTooLarge, 64<<20)

	// \verb is one token too: copying it through macros multiplies its text.
	verb := `\def\a#1{#1#1#1#1#1#1#1#1#1#1}\def\b#1{#1#1#1#1}\b{\a{\a{\a{\verb|` + strings.Repeat("x", 12000) + `|}}}}`
	measure(t, "verb/line", func() error { _, err := RenderLine(verb, LineOptions{}); return err }, ErrTooLarge, 64<<20)
	measure(t, "verb/render", func() error { _, err := Render(context.Background(), verb, Options{}); return err }, ErrTooLarge, 64<<20)

	// \url{#1...} substitutes the argument's text for every #1: the
	// result is checked against the budget while it is built.
	urlSub := `\def\a#1{\url{` + strings.Repeat("#1", 1000) + `}}\def\b#1#2{\url{` + strings.Repeat("#2", 6000) + `}}\expandafter\b\a{` + strings.Repeat("x", 60) + `}`
	measure(t, "urlsub/line", func() error { _, err := RenderLine(urlSub, LineOptions{}); return err }, ErrTooLarge, 64<<20)

	// A long command name is one token: written back as source text for
	// \url{#1}, 32768 copies of it would be gigabytes.
	e8 := strings.Repeat("#1", 8)
	longName := `\def\u#1{\url{#1}}\def\a#1{\b{` + e8 + `}}\def\b#1{\c{` + e8 + `}}\def\c#1{\d{` + e8 + `}}\def\d#1{\e{` + e8 + `}}\def\e#1{\u{` + e8 + `}}` +
		`\a{\` + strings.Repeat("a", 15500) + `}`
	measure(t, "longname/text", func() error {
		_, err := RenderText(context.Background(), longName, TextOptions{})
		return err
	}, ErrTooLarge, 64<<20)

	// A long run of width-0 characters in \text: the text grid joins
	// them to one cell, which must not cost quadratic time.
	zw := `\def\a{` + strings.Repeat("\u200b", 100) + `}\def\b{` + strings.Repeat(`\a`, 60) + `}\text{` + strings.Repeat(`\b`, 10) + `}`
	measure(t, "zerowidth/text", func() error {
		_, err := RenderText(context.Background(), zw, TextOptions{})
		return err
	}, nil, 64<<20)

	// Nodes made inside \text (\textcircled, \sout, \verb) count
	// toward MaxNodes.
	circled := `\def\a{` + strings.Repeat(`\textcircled x`, 100) + `}\def\b{` + strings.Repeat(`\a`, 32) + `}\text{` + strings.Repeat(`\b`, 10) + `}`
	measure(t, "circled/render", func() error { _, err := Render(context.Background(), circled, Options{}); return err }, ErrTooLarge, 64<<20)

	// Lookups inside thousands of groups opened in an \edef body.
	scopes := `\def\a{` + strings.Repeat(`\alpha`, 60) + `}\def\b{` + strings.Repeat(`\a`, 30) + `}\def\c{` + strings.Repeat(`\b`, 30) + `}` +
		`\edef\x{` + strings.Repeat("{", 4000) + `\c` + strings.Repeat("}", 4000) + `}`
	maxTime = 100 * time.Millisecond
	measure(t, "scopes/line", func() error { _, err := RenderLine(scopes, LineOptions{}); return err }, nil, 64<<20)

	// Alternating colors far apart: each color change composites the
	// union of what was drawn since the last one.
	seg := `\rlap{\kern190em\raisebox{190em}{.}}\rlap{.}`
	colors := `\def\s{\textcolor{red}{` + seg + `}\textcolor{blue}{` + seg + `}}\def\t{` + strings.Repeat(`\s`, 10) + `}` + strings.Repeat(`\t`, 25)
	maxTime = time.Second
	measure(t, "colors/render", func() error { _, err := Render(context.Background(), colors, Options{}); return err }, ErrTooLarge, 256<<20)

	maxTime = time.Second
	overlap := `\Huge\def\a{` + strings.Repeat(`\rlap{½} `, 10) + `}` + strings.Repeat(`\a`, 100)
	measure(t, "textface/render", func() error {
		_, err := Render(context.Background(), overlap, Options{SizePx: 1024, TextFace: testFace})
		return err
	}, ErrTooLarge, 64<<20)

	huge := `\Huge\def\k{` + strings.Repeat(`\kern1000in`, 10) + `}\def\K{` + strings.Repeat(`\k`, 10) + `}` + strings.Repeat(`\K`, 30) +
		`\def\R{\vphantom{\rule{1pt}{1000in}}\\\\}\def\RR{` + strings.Repeat(`\R`, 10) + `}\begin{matrix}` + strings.Repeat(`\RR`, 20) + ` x\end{matrix}`
	for _, size := range []float64{20, 300, 1024} {
		measure(t, fmt.Sprintf("huge/%v", size), func() error {
			_, err := Render(context.Background(), huge, Options{SizePx: size})
			return err
		}, ErrTooLarge, 64<<20)
	}
}

// TestControlSymbol checks that a control character after a backslash is
// rejected like any other, so that it cannot reach text output through
// \url.
func TestControlSymbol(t *testing.T) {
	for _, src := range []string{"\\\x1b", "\\\u009b", "\\\u202e", "\\\x00",
		"\\newcommand\\u[1]{\\url{#1}}\\u{\\\x1b[2J}"} {
		var syn *SyntaxError
		if _, err := RenderLine(src, LineOptions{}); !errors.As(err, &syn) {
			t.Errorf("RenderLine(%q): got %v, want a SyntaxError", src, err)
		}
		if _, err := RenderText(context.Background(), src, TextOptions{}); !errors.As(err, &syn) {
			t.Errorf("RenderText(%q): got %v, want a SyntaxError", src, err)
		}
	}
}

// TestOperatorSpaceInScripts checks that the thin space after an operator
// is kept in scripts and inline fraction parts, as in TeX.
func TestOperatorSpaceInScripts(t *testing.T) {
	r, err := RenderText(context.Background(), `\frac{\sin x}{x}`, TextOptions{})
	if err != nil || r.Lines[0] != " sin x" {
		t.Errorf("RenderText: got %q, %v", r.Lines, err)
	}
	for style, want := range map[LineStyle]string{LineUnicode: "x^(sin x)", LineASCII: "x^(sin x)"} {
		if s, err := RenderLine(`x^{\sin x}`, LineOptions{Style: style}); s != want || err != nil {
			t.Errorf("RenderLine(style %v): got %q, %v, want %q", style, s, err, want)
		}
	}
	// Only the space before an ordinary or an operator: in scripts TeX
	// drops the one before a relation.
	for src, want := range map[string]string{`x^{\sin = 1}`: "xˢⁱⁿ⁼¹", `\sum_{\max\to 0}`: "∑_(max→0)",
		// and before an operator: a\sin x is not \operatorname{asin}x.
		`a\sin x`: "a sin x", `O(n\log n)`: "O(n log n)", `x)\sin y`: "x) sin y", `x^{a\sin x}`: "x^(a sin x)"} {
		if s, err := RenderLine(src, LineOptions{}); s != want || err != nil {
			t.Errorf("RenderLine(%q): got %q, %v, want %q", src, s, err, want)
		}
	}
}

// TestFractionBeforeOperator checks that a fraction followed by an
// operator keeps its parentheses in line output: 1/n ∑ xᵢ would read as
// 1/(n ∑ xᵢ). After an operator the space separates, as before.
func TestFractionBeforeOperator(t *testing.T) {
	for src, want := range map[string]string{
		`\frac{1}{n}\sum_{i=1}^n x_i`: "(1/n) ∑ᵢ₌₁ⁿ xᵢ",
		`\frac{1}{2\pi}\int f`:        "(1/(2π)) ∫ f",
		`\frac12\sin x`:               "(1/2) sin x",
		`\sin\frac{x}{2}`:             "sin x/2",
		`\sum_{i=1}^n\frac{1}{i^2}`:   "∑ᵢ₌₁ⁿ 1/i²",
	} {
		if s, err := RenderLine(src, LineOptions{}); s != want || err != nil {
			t.Errorf("RenderLine(%q): got %q, %v, want %q", src, s, err, want)
		}
	}
}

// TestVerbMultibyteDelimiter checks that \verb takes a whole character,
// not a byte, as its delimiter.
func TestVerbMultibyteDelimiter(t *testing.T) {
	if s, err := RenderLine("\\verbé|é", LineOptions{}); s != "|" || err != nil {
		t.Errorf("got %q, %v", s, err)
	}
	var syn *SyntaxError
	if _, err := RenderLine("\\verbéab", LineOptions{}); !errors.As(err, &syn) || !strings.Contains(err.Error(), "missing closing é") {
		t.Errorf("unclosed: got %v", err)
	}
}

// TestVerbInvalidUTF8 checks that the body of \verb is checked for
// invalid UTF-8 like the rest of the input, so raw bytes (0x9B is a C1
// CSI) cannot reach line output.
func TestVerbInvalidUTF8(t *testing.T) {
	for _, src := range []string{"\\verb|\x9b2J|", "\\verb|\xff|", "\\verbé\xc3é"} {
		var syn *SyntaxError
		if _, err := RenderLine(src, LineOptions{}); !errors.As(err, &syn) {
			t.Errorf("RenderLine(%q): got %v, want a SyntaxError", src, err)
		}
		if _, err := RenderText(context.Background(), src, TextOptions{}); !errors.As(err, &syn) {
			t.Errorf("RenderText(%q): got %v, want a SyntaxError", src, err)
		}
		if _, err := Parse(src, ParseOptions{}); !errors.As(err, &syn) {
			t.Errorf("Parse(%q): got %v, want a SyntaxError", src, err)
		}
	}
	// A \verb passed to \url{#1} is written back with a delimiter that
	// does not occur in it.
	if s, err := RenderLine(`\def\u#1{\url{#1}}\u{\verb!a|b!}`, LineOptions{}); s != `\verb!a|b!` || err != nil {
		t.Errorf("verb in url: got %q, %v", s, err)
	}
	all := `|!+/=:"@'()<>[]-.,;?^_` + "`~&$"
	if s, err := RenderLine(`\def\u#1{\url{#1}}\u{\verb¡`+all+`¡}`, LineOptions{}); s != `\verb¡`+all+`¡` || err != nil {
		t.Errorf("verb with every ASCII delimiter in url: got %q, %v", s, err)
	}
}

// TestVerbFallbackDelimiter checks that when every ASCII candidate occurs
// in a \verb written back for \url{#1}, the delimiter chosen instead is a
// visible character: not a format, bidi or combining character.
func TestVerbFallbackDelimiter(t *testing.T) {
	var b strings.Builder
	b.WriteString(`|!+/=:"@'()<>[]-.,;?^_` + "`~&$")
	for r := rune(0xA1); r < 0x061C; r++ {
		b.WriteRune(r)
	}
	src := `\def\u#1{\url{#1}}\u{\verb#` + b.String() + `#}`
	s, err := RenderLine(src, LineOptions{})
	if err != nil {
		t.Fatal(err)
	}
	d, _ := utf8.DecodeRuneInString(strings.TrimPrefix(s, `\verb`))
	if !unicode.IsPrint(d) || unicode.Is(unicode.M, d) || unicode.Is(unicode.Cf, d) {
		t.Errorf("delimiter U+%04X", d)
	}
	if strings.ContainsRune(s, 0x061C) {
		t.Errorf("output contains U+061C")
	}
}

// TestLongColorNameError checks that an invalid color name made long by
// macros is quoted only in part.
func TestLongColorNameError(t *testing.T) {
	src := `\def\a{` + strings.Repeat("z", 100) + `}\def\b{` + strings.Repeat(`\a`, 100) + `}\color{\b}x`
	_, err := RenderLine(src, LineOptions{})
	var syn *SyntaxError
	if !errors.As(err, &syn) || len(err.Error()) > 300 {
		t.Errorf("got %d bytes: %.100v", len(err.Error()), err)
	}
}

// TestMaxNodesBoundsTree checks that MaxNodes bounds the nodes of the
// returned tree, including constructs that make several nodes at once.
func TestMaxNodesBoundsTree(t *testing.T) {
	for _, src := range []string{strings.Repeat(`\varliminf`, 1000), strings.Repeat(`\pmod a`, 1000), strings.Repeat(`{}^{}`, 3000)} {
		if _, err := Parse(src, ParseOptions{}); !errors.Is(err, ErrTooLarge) {
			t.Errorf("Parse(%.30q...): got %v, want ErrTooLarge", src, err)
		}
	}
}

// TestLongNameErrors checks that names made long by macros are cut in
// errors: environment names, \end names and options.
func TestLongNameErrors(t *testing.T) {
	pre := `\def\a{` + strings.Repeat("x", 2000) + `}\def\b{\a\a\a\a\a\a\a\a}`
	dim := `\def\a{` + strings.Repeat("0", 7900) + `}\kern\a\a\a\a\a\a\a\a1001pt`
	for _, src := range []string{pre + `\begin{\b\b\b\b}`, pre + `\begin{matrix}x\end{\b\b\b\b}`, pre + `\smash[\b\b\b\b]{x}`, dim} {
		if _, err := RenderLine(src, LineOptions{}); err == nil || len(err.Error()) > 400 {
			t.Errorf("got %v bytes", len(fmt.Sprint(err)))
		}
	}
}

// TestExpandafterChain checks chains of \expandafter: expanding an
// \expandafter once carries out its own step, as in TeX.
func TestExpandafterChain(t *testing.T) {
	for src, want := range map[string]string{
		`\def\a{x}\def\b#1{(#1)}\expandafter\b\expandafter{\a}`:                         "(x)",
		`\def\a{x}\def\b#1{(#1)}\expandafter\expandafter\expandafter\b\expandafter\a\a`: "(x)x",
		`\def\u#1{\url{#1}}\def\v{\verb|ab|}\expandafter\u\expandafter{\v}`:             `\verb|ab|`,
		`\def\a{x}\def\b#1{(#1)}\expandafter\b\a`:                                       "(x)",
	} {
		if s, err := RenderLine(src, LineOptions{}); s != want || err != nil {
			t.Errorf("RenderLine(%q): got %q, %v, want %q", src, s, err, want)
		}
	}
}

// TestLongExpandafterChain checks that a chain of \expandafter as long as
// a raised MaxInputBytes allows does not overflow the stack.
func TestLongExpandafterChain(t *testing.T) {
	src := strings.Repeat(`\expandafter x`, 1200000)
	lim := Limits{MaxInputBytes: 32 << 20, MaxExpansions: 1 << 30, MaxNodes: 1 << 30}
	if _, err := Parse(src, ParseOptions{Limits: lim}); err != nil {
		t.Error(err)
	}
}

// TestWideTallText checks that a wide part next to a tall one fails the
// text cell budget; with a 32-bit int (GOARCH=386) their product wrapped
// past it and the process ran out of memory.
func TestWideTallText(t *testing.T) {
	src := `\begin{array}{c}` + strings.Repeat(`a\\`, 1100) + `\end{array}` + strings.Repeat(`\rule{1000in}{1pt}`, 150)
	if _, err := RenderText(context.Background(), src, TextOptions{}); !errors.Is(err, ErrTooLarge) {
		t.Errorf("got %v, want ErrTooLarge", err)
	}
}

// TestImageSide checks that an image wider than float32 coordinates can
// address fails instead of coming out blank, when MaxPixels allows it.
func TestImageSide(t *testing.T) {
	src := `\def\b{` + strings.Repeat(`\kern1000in`, 10) + `}\def\c{` + strings.Repeat(`\b`, 10) + `}` + strings.Repeat(`\c`, 24) + `\rule{1em}{1em}x`
	_, err := Render(context.Background(), src, Options{SizePx: 1, Limits: Limits{MaxPixels: 200 << 20}})
	if !errors.Is(err, ErrTooLarge) {
		t.Errorf("got %v, want ErrTooLarge", err)
	}
}

// TestFarGlyph checks that a glyph far to the right (400000 px) is drawn
// with the same pixels as near the origin: float32 holds multiples of
// 1/64 exactly only up to 2^18, so device points are kept in float64.
func TestFarGlyph(t *testing.T) {
	near, err := Render(context.Background(), `\kern1em x`, Options{SizePx: 20})
	if err != nil {
		t.Fatal(err)
	}
	far, err := Render(context.Background(), strings.Repeat(`\kern1000em`, 20)+`\kern1em x`, Options{SizePx: 20})
	if err != nil {
		t.Fatal(err)
	}
	nw, fw := near.Image.Bounds().Dx(), far.Image.Bounds().Dx()
	for y := 0; y < near.Image.Bounds().Dy(); y++ {
		for x := 0; x < nw; x++ {
			if a, b := near.Image.RGBAAt(x, y), far.Image.RGBAAt(fw-nw+x, y); a != b {
				t.Fatalf("pixel (%d,%d): near %v, far %v", x, y, a, b)
			}
		}
	}
}

// TestEmptyGroupSpacing checks that {} is an ordinary atom in line output,
// as in TeX and in text output: a ={} b has spaces on both sides of =.
func TestEmptyGroupSpacing(t *testing.T) {
	for src, want := range map[string]string{
		`x{} = y`:         "x = y",
		`f(x){}={}g`:      "f(x) = g",
		`\sin {} x`:       "sin x",
		`-{}1`:            "−1",
		`a{}b`:            "ab",
		`\frac{d}{dx}{}`:  "d/(dx)",
		`\frac12{}b`:      "(1/2)b",
		`a{}\frac12`:      "a(1/2)",
		`a\text{}\frac12`: "a(1/2)",
		`\begin{aligned}a &={} b+c\\ &={}d\end{aligned}`: "a = b + c; = d",
	} {
		if s, err := RenderLine(src, LineOptions{}); s != want || err != nil {
			t.Errorf("RenderLine(%q): got %q, %v, want %q", src, s, err, want)
		}
	}
}

// TestEdefUnbalancedScope checks that an unbalanced } produced while
// expanding an \edef body does not drop the definitions of the group
// around the \edef.
func TestEdefUnbalancedScope(t *testing.T) {
	src := `\def\url#1{#1}{\def\x{A}\edef\y{\url{\}}}\x}`
	if s, err := RenderLine(src, LineOptions{}); s != "A" || err != nil {
		t.Errorf("got %q, %v, want \"A\"", s, err)
	}
}

// TestHugeMaxPixels checks that a MaxPixels meant as no limit does not
// overflow the compositing budget.
func TestHugeMaxPixels(t *testing.T) {
	for _, n := range []int{math.MaxInt, math.MaxInt/4 + 1} {
		if _, err := Render(context.Background(), `x+1`, Options{Limits: Limits{MaxPixels: n}}); err != nil {
			t.Errorf("MaxPixels %d: %v", n, err)
		}
	}
}

func TestScriptedMiddle(t *testing.T) {
	for _, src := range []string{`\left(\middle|^2\right)`, `\left(a\mathrm{\middle|}^2 b\right)`} {
		if _, err := Render(context.Background(), src, Options{}); err != nil {
			t.Errorf("Render(%q): %v", src, err)
		}
	}
}

// TestNestedScripts checks that a script on something that already ends in
// a script or a radical is parenthesized, so it cannot be misread.
func TestNestedScripts(t *testing.T) {
	cases := []struct{ src, line, text string }{
		{`{x^2}^3`, "(x²)³", "(x²)³"},
		{`{x_1}_2`, "(x₁)₂", "(x₁)₂"},
		{`\sqrt{x}^2`, "(√x)²", "(√x)²"},
		{`\sqrt[3]{x}^2`, "(∛x)²", "(∛x)²"},
		{`x^2`, "x²", "x²"},
		{`(\sqrt{x})^2`, "(√x)²", "(√x)²"},
	}
	for _, c := range cases {
		if got, err := RenderLine(c.src, LineOptions{}); err != nil || got != c.line {
			t.Errorf("RenderLine(%q) = %q, %v; want %q", c.src, got, err, c.line)
		}
		res, err := RenderText(context.Background(), c.src, TextOptions{})
		if err != nil || len(res.Lines) != 1 || res.Lines[0] != c.text {
			t.Errorf("RenderText(%q) = %+v, %v; want %q", c.src, res, err, c.text)
		}
	}
}

func TestNot(t *testing.T) {
	if got, err := RenderLine(`\not a`, LineOptions{}); err != nil || got != "a̸" {
		t.Errorf("RenderLine(\\not a) = %q, %v", got, err)
	}
	if got, err := RenderLine(`\not=`, LineOptions{}); err != nil || got != "≠" {
		t.Errorf("RenderLine(\\not=) = %q, %v", got, err)
	}
	if _, err := Render(context.Background(), `\not\infty`, Options{}); err != nil {
		t.Errorf("Render(\\not\\infty): %v", err)
	}
	var ue *UnsupportedError
	if _, err := RenderText(context.Background(), `\not a`, TextOptions{}); !errors.As(err, &ue) || ue.Command != `\not` {
		t.Errorf("RenderText(\\not a): %v", err)
	}
}

func TestControlCharacters(t *testing.T) {
	for _, src := range []string{"x\u009b31m", `\text{a` + "\u202e" + `b}`, "a\u2066b", "a\u200fb"} {
		var se *SyntaxError
		if _, err := RenderLine(src, LineOptions{}); !errors.As(err, &se) {
			t.Errorf("RenderLine(%q): got %v, want a SyntaxError", src, err)
		}
	}
}

func TestSqrtIndexText(t *testing.T) {
	res, err := RenderText(context.Background(), `\sqrt[\frac{1}{2}]{x+y}`, TextOptions{})
	if err != nil {
		t.Fatal(err)
	}
	all := strings.Join(res.Lines, "\n")
	for _, want := range []string{"1", "2", "√x + y"} {
		if !strings.Contains(all, want) {
			t.Errorf("missing %q in\n%s", want, all)
		}
	}
}

func TestOriginX(t *testing.T) {
	res, err := Render(context.Background(), `x`, Options{PaddingPx: 3})
	if err != nil {
		t.Fatal(err)
	}
	if res.OriginX != 3 {
		t.Errorf("OriginX = %d, want the padding 3", res.OriginX)
	}
	res, err = Render(context.Background(), `\kern-2em x`, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.OriginX < 30 || res.OriginX >= res.Image.Bounds().Dx()+40 {
		t.Errorf("OriginX = %d for ink 2em left of the origin", res.OriginX)
	}
}

func TestHexColor(t *testing.T) {
	for _, src := range []string{`\color{#ff0000}x`, `\textcolor{#F00}{x}`} {
		res, err := Render(context.Background(), src, Options{})
		if err != nil {
			t.Fatalf("%q: %v", src, err)
		}
		red := false
		b := res.Image.Bounds()
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				if c := res.Image.RGBAAt(x, y); c.A == 255 && c.R == 255 && c.G == 0 {
					red = true
				}
			}
		}
		if !red {
			t.Errorf("%q: no red pixel", src)
		}
	}
	var se *SyntaxError
	if _, err := Parse(`a#b`, ParseOptions{}); !errors.As(err, &se) || se.Offset != 1 {
		t.Errorf("a#b: %v", err)
	}
}

// TestScriptedBaseText checks the 2D text against the line output for
// bases that end in a script.
func TestScriptedBaseText(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`{yx^2}^3`, "(yx²)³"},
		{`\mathrm{x^2}^3`, "(x²)³"},
		{`{\color{red}x^2}^3`, "(x²)³"},
		{`{x^2y}^3`, "x²y³"},
	} {
		res, err := RenderText(context.Background(), c.src, TextOptions{})
		if err != nil || len(res.Lines) != 1 || res.Lines[0] != c.want {
			t.Errorf("RenderText(%q) = %+v, %v; want %q", c.src, res, err, c.want)
		}
	}
	res, err := RenderText(context.Background(), `\frac{a}{b}^2`, TextOptions{})
	if err != nil || len(res.Lines) != 3 || res.Lines[0][:len("⎛")] != "⎛" {
		t.Errorf(`RenderText(\frac{a}{b}^2) = %q, %v; want tall parentheses`, res.Lines, err)
	}
}

func TestLineFractions(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`\frac{a}{b}\frac{c}{d}`, "(a/b)(c/d)"},
		{`\frac{1}{2}x`, "(1/2)x"},
		{`a/\frac{b}{c}`, "a/(b/c)"},
		{`\frac{\frac{a}{b}}{c}`, "(a/b)/c"},
		{`\frac{1}{\text{total count}}`, "1/(total count)"},
		{`\frac{a}{b} + c`, "a/b + c"},
		{`-\frac{a}{b}`, "−a/b"},
		{`(\frac{a}{b})`, "(a/b)"},
		{`v = \frac{d}{t} \text{ where }`, "v = d/t where"},
	} {
		if got, err := RenderLine(c.src, LineOptions{}); err != nil || got != c.want {
			t.Errorf("RenderLine(%q) = %q, %v; want %q", c.src, got, err, c.want)
		}
	}
}

func TestTextContents(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`\text{for all $x$}`, "for all x"},
		{`\text{a \textbf{b} c}`, "a b c"},
		{`\text{caf\'e na\"{\i}ve}`, "café naïve"},
		{`\text{\ldots}`, "…"},
		{`\begin{array}{p{2cm}c} a & b \end{array}`, "a, b"},
	} {
		if got, err := RenderLine(c.src, LineOptions{}); err != nil || got != c.want {
			t.Errorf("RenderLine(%q) = %q, %v; want %q", c.src, got, err, c.want)
		}
		if _, err := Render(context.Background(), c.src, Options{}); err != nil {
			t.Errorf("Render(%q): %v", c.src, err)
		}
	}
	var se *SyntaxError
	if _, err := RenderLine(`\text{$x}`, LineOptions{}); !errors.As(err, &se) {
		t.Errorf(`\text{$x}: %v`, err)
	}
}

func TestNotNeedsSymbol(t *testing.T) {
	for _, src := range []string{`\begin{matrix}\not&b\end{matrix}`, `\not_i x`, `{\not}`, `\not\right)`, `\not\kern1em`, `\not`} {
		var se *SyntaxError
		if _, err := RenderLine(src, LineOptions{}); !errors.As(err, &se) {
			t.Errorf("RenderLine(%q): %v, want a SyntaxError", src, err)
		}
	}
}

// TestCombiningText checks that combining marks are not dropped: they share
// the cell of the character before them (formerly ErrUnsupported).
func TestCombiningText(t *testing.T) {
	for src, want := range map[string]string{"\\text{か\u3099}": "か\u3099", "e\u0301": "e\u0301", `\text{\H{a}}`: "a\u030b"} {
		res, err := RenderText(context.Background(), src, TextOptions{MaxCols: textgrid.StringWidth(want)})
		if err != nil || len(res.Lines) != 1 || res.Lines[0] != want {
			t.Errorf("RenderText(%q): %+v, %v, want %q", src, res, err, want)
		}
		if _, err := RenderLine(src, LineOptions{}); err != nil {
			t.Errorf("RenderLine(%q): %v", src, err)
		}
	}
}

func TestInvalidOptions(t *testing.T) {
	for _, o := range []Options{
		{SizePx: math.NaN()}, {SizePx: math.Inf(1)}, {SizePx: -1},
		{PaddingPx: -1}, {MaxWidth: -1},
	} {
		if _, err := Render(context.Background(), "x", o); err == nil || errors.Is(err, ErrTooLarge) {
			t.Errorf("%+v: %v, want an invalid option error", o, err)
		}
	}
	if _, err := Render(context.Background(), "x", Options{PaddingPx: 5000}); !errors.Is(err, ErrTooLarge) {
		t.Errorf("PaddingPx 5000: %v", err)
	}
}

func TestOperatorname(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`\operatorname{arg\,max}_x f`, "arg maxₓ f"},
		{`\operatorname{f_1}(x)`, "f₁(x)"},
		{`\operatorname{Var}(X)`, "Var(X)"},
	} {
		if got, err := RenderLine(c.src, LineOptions{}); err != nil || got != c.want {
			t.Errorf("RenderLine(%q) = %q, %v; want %q", c.src, got, err, c.want)
		}
	}
}

// translucent is a color.Color that is not one of the image/color types.
type translucent struct{}

func (translucent) RGBA() (r, g, b, a uint32) { return 0, 0, 0, 0x8000 }

// TestTranslucentOverlap checks that overlapping parts of a stretched
// delimiter do not get darker than a single glyph.
func TestTranslucentOverlap(t *testing.T) {
	maxA := func(img *image.RGBA) uint8 {
		m := uint8(0)
		for i := 3; i < len(img.Pix); i += 4 {
			m = max(m, img.Pix[i])
		}
		return m
	}
	one, err := Render(context.Background(), `x`, Options{Foreground: translucent{}})
	if err != nil {
		t.Fatal(err)
	}
	tall, err := Render(context.Background(), `\left(\begin{matrix}a\\b\\c\\d\\e\\f\\g\\h\end{matrix}\right)`, Options{Foreground: translucent{}})
	if err != nil {
		t.Fatal(err)
	}
	if a, b := maxA(one.Image), maxA(tall.Image); b > a+1 {
		t.Errorf("max alpha %d for an assembly, %d for one glyph", b, a)
	}
}

func TestHugeDimension(t *testing.T) {
	for _, src := range []string{`a\kern1000000pt b`, `a\hspace{100000em}b`, `a\kern-1001mu b`} {
		if _, err := RenderLine(src, LineOptions{}); !errors.Is(err, ErrTooLarge) {
			t.Errorf("RenderLine(%q): got %v, want ErrTooLarge", src, err)
		}
	}
	var se *SyntaxError
	if _, err := RenderLine(`a\kern..pt b`, LineOptions{}); !errors.As(err, &se) {
		t.Errorf("malformed dimension: got %v, want a SyntaxError", err)
	}
}

func TestBreakInsideLeftRight(t *testing.T) {
	cases := []struct {
		src, msg string
		off      int
	}{
		{`\left( a \\ b \right)`, `\\ cannot appear between \left and \right`, 9},
		{`\begin{matrix}\left( a & b \right)\end{matrix}`, `& cannot appear between \left and \right`, 23},
		{`\left( a`, `missing \right`, 0},
	}
	for _, c := range cases {
		_, err := RenderLine(c.src, LineOptions{})
		var se *SyntaxError
		if !errors.As(err, &se) || !strings.Contains(se.Error(), c.msg) || se.Offset != c.off {
			t.Errorf("RenderLine(%q): got %v, want %q at %d", c.src, err, c.msg, c.off)
		}
	}
}

func TestCancelLine(t *testing.T) {
	cases := map[string]string{
		`\cancel{x} + y`:                 "cancel(x) + y",
		`2\cancel{x}y`:                   "2 cancel(x) y",
		`\bcancel{a+b}\xcancel{z}`:       "bcancel(a + b) xcancel(z)",
		`\frac{\cancel{2}x}{\cancel{2}}`: "(cancel(2) x)/cancel(2)",
		`(\cancel{x})`:                   "(cancel(x))",
	}
	for src, want := range cases {
		if got, err := RenderLine(src, LineOptions{}); err != nil || got != want {
			t.Errorf("RenderLine(%q) = %q, %v; want %q", src, got, err, want)
		}
	}
}

// TestTextFaceMissingMark checks that a combining mark the text face lacks
// is drawn with the math font instead of becoming a missing-glyph box.
func TestTextFaceMissingMark(t *testing.T) {
	// The CJK test font has no combining tilde: the mark after the run
	// is drawn with the math font, not as a replacement box.
	opt := Options{TextFace: cjkFace}
	if _, err := Render(context.Background(), `\text{定義 `+"̃"+`é}`, opt); err != nil {
		t.Fatal(err)
	}
}

// TestPhase0Fixes covers the defects found when listing what dal lacks
// (docs/plans/math-completion.md, phase 0).
func TestPhase0Fixes(t *testing.T) {
	ctx := context.Background()
	// A hex color whose first digit is 1-9 used to lex as a macro
	// parameter (#1).
	for _, src := range []string{`\color{#1a2b3c}x`, `\textcolor{#228B22}{x}`, `\def\c#1{\color{#1}x}\c{red}`, `\hspace*{1em}x`} {
		if _, err := Render(ctx, src, Options{}); err != nil {
			t.Errorf("%s: %v", src, err)
		}
	}
	// Characters the math font lacks: substitutes first, then the text
	// face, else an UnsupportedError that names the way out.
	if _, err := Render(ctx, `a \triangleleft b \Coloneqq c`, Options{}); err != nil {
		t.Errorf("substitutes: %v", err)
	}
	_, err := Render(ctx, `\digamma`, Options{})
	var u *UnsupportedError
	if !errors.As(err, &u) || !strings.Contains(err.Error(), "TextFace") {
		t.Errorf("\\digamma without a face: %v", err)
	}
	// ½ is in the Go font but not in the math font: drawn by the face.
	res, err := Render(ctx, `a + ½`, Options{TextFace: testFace})
	if err != nil || res.Width <= 0 {
		t.Errorf("½ with a face: %v", err)
	}
	if _, err := Render(ctx, `a + ½`, Options{}); !errors.As(err, &u) {
		t.Errorf("½ without a face: %v", err)
	}
}

// TestPhase1Fixes covers the defects found in the review of phase 1.
func TestPhase1Fixes(t *testing.T) {
	for src, want := range map[string]string{
		`\text{a\verb!x!b}`:         "axb",
		`\char"7B x`:                "{x",
		`\char"24 + \char"23`:       "$ + #",
		`\char"7E`:                  "~",
		`\edef\a{\def\b{x}}\a\b`:    "x",
		`\textcolor{red}{\verb!q!}`: "q",
	} {
		got, err := RenderLine(src, LineOptions{})
		if err != nil || got != want {
			t.Errorf("RenderLine(%q) = %q, %v; want %q", src, got, err, want)
		}
	}
	// \edef keeps \def in its body: \b is defined only when \a is used.
	if _, err := Parse(`\edef\a{\def\b{x}}\b`, ParseOptions{}); !errors.Is(err, ErrUnsupported) {
		t.Errorf("\\b defined by \\edef itself: %v", err)
	}
}

// TestWrap covers Options.Wrap and TextOptions.Wrap (phase 2).
func TestWrap(t *testing.T) {
	ctx := context.Background()
	src := `a + b + c + d = e + f + \frac{g}{h} \le i + j + k + l = m`
	one, err := Render(ctx, src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := Render(ctx, src, Options{MaxWidth: 200, Wrap: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Image.Bounds().Dx() > 200 || res.Image.Bounds().Dy() < 2*one.Image.Bounds().Dy() {
		t.Errorf("wrapped image %v, single line %v", res.Image.Bounds(), one.Image.Bounds())
	}
	// The baseline is the first line's, which has no fraction: as high
	// as that of a formula of plain letters.
	plain, err := Render(ctx, `a + b`, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Baseline != plain.Baseline {
		t.Errorf("baseline %d, want the first line's %d", res.Baseline, plain.Baseline)
	}
	if _, err := Render(ctx, src, Options{MaxWidth: 200}); !errors.Is(err, ErrTooLarge) {
		t.Errorf("without Wrap: %v", err)
	}
	// Display formulas are not broken.
	if _, err := Render(ctx, src, Options{MaxWidth: 200, Wrap: true, Display: true}); !errors.Is(err, ErrTooLarge) {
		t.Errorf("display with Wrap: %v", err)
	}
	tr, err := RenderText(ctx, `a + b + c = d \allowbreak e \nobreak = f + g`, TextOptions{MaxCols: 10, Wrap: true})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a + b +", "c = de =", "f + g"}
	if strings.Join(tr.Lines, "|") != strings.Join(want, "|") || tr.Baseline != 0 {
		t.Errorf("text lines %q baseline %d, want %q", tr.Lines, tr.Baseline, want)
	}
	// A piece wider than the line on its own still fails.
	if _, err := RenderText(ctx, `abcdefghijkl + b`, TextOptions{MaxCols: 10, Wrap: true}); !errors.Is(err, ErrTooLarge) {
		t.Errorf("unbreakable piece: %v", err)
	}
}

// TestTagAtLineEnd checks that display tags go to the right end of the
// known width (phase 2).
func TestTagAtLineEnd(t *testing.T) {
	ctx := context.Background()
	tr, err := RenderText(ctx, `E = mc^2 \tag{1}`, TextOptions{Display: true, MaxCols: 30})
	if err != nil {
		t.Fatal(err)
	}
	if l := tr.Lines[tr.Baseline]; textgrid.StringWidth(l) != 30 || !strings.HasSuffix(l, "(1)") {
		t.Errorf("tag line %q", l)
	}
	short, err := Render(ctx, `E = mc^2 \tag{1}`, Options{Display: true})
	if err != nil {
		t.Fatal(err)
	}
	wide, err := Render(ctx, `E = mc^2 \tag{1}`, Options{Display: true, MaxWidth: 500})
	if err != nil {
		t.Fatal(err)
	}
	if w := wide.Image.Bounds().Dx(); w > 500 || w < 490 || short.Image.Bounds().Dx() >= w {
		t.Errorf("image width %d with MaxWidth 500 (%d without)", w, short.Image.Bounds().Dx())
	}
}

// TestWrapLinear checks that breaking a long formula takes linear time:
// it used to repack the line at every break point.
func TestWrapLinear(t *testing.T) {
	src := strings.Repeat("a+", 2400) + "a"
	full, err := RenderText(context.Background(), src, TextOptions{})
	if err != nil {
		t.Fatal(err)
	}
	w := textgrid.StringWidth(full.Lines[0])
	start := time.Now()
	if _, err := RenderText(context.Background(), src, TextOptions{MaxCols: w - 3, Wrap: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := Render(context.Background(), src, Options{MaxWidth: 1 << 14, Wrap: true}); err != nil && !errors.Is(err, ErrTooLarge) {
		t.Fatal(err)
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("wrapping took %v", d)
	}
}

// TestFinalReviewFixes covers the serious findings of the final review.
func TestFinalReviewFixes(t *testing.T) {
	ctx := context.Background()
	// Row tags and an outer tag fit a line that holds them all.
	src := `\begin{align}a&=b\tag{1}\\c&=d\tag{2}\end{align}\tag{3}`
	if _, err := RenderText(ctx, src, TextOptions{Display: true, MaxCols: 70}); err != nil {
		t.Errorf("text with row and outer tags: %v", err)
	}
	if _, err := Render(ctx, src, Options{Display: true, MaxWidth: 600}); err != nil {
		t.Errorf("image with row and outer tags: %v", err)
	}
	// Wrapping keeps room for the tag.
	tagged := `a+b+c+d+e+f+g+h+i+j+k+l+m+n+o+p\tag{1}`
	if res, err := RenderText(ctx, tagged, TextOptions{MaxCols: 30, Wrap: true}); err != nil || len(res.Lines) < 2 {
		t.Errorf("text wrap with a tag: %+v, %v", res, err)
	}
	if _, err := Render(ctx, tagged, Options{MaxWidth: 200, Wrap: true}); err != nil {
		t.Errorf("image wrap with a tag: %v", err)
	}
	// A character neither font has fails instead of drawing a
	// replacement box (the CJK test font has no emoji).
	for _, s := range []string{`\text{😀}`, `\text{定😀}`} {
		var u *UnsupportedError
		if _, err := Render(ctx, s, Options{TextFace: cjkFace}); !errors.As(err, &u) {
			t.Errorf("%s: %v, want UnsupportedError", s, err)
		}
	}
	if _, err := Render(ctx, `\text{定義}`, Options{TextFace: cjkFace}); err != nil {
		t.Errorf("\\text{定義}: %v", err)
	}
}

// TestMinorReviewFixes covers the minor findings of the final review.
func TestMinorReviewFixes(t *testing.T) {
	ctx := context.Background()
	for src, want := range map[string]string{
		// Cells, environments and \left ... \right are groups.
		`\def\x{b}\begin{matrix}\def\x{a}\end{matrix}\x`: "b",
		`\def\x{b}\left(\def\x{a}\right)\x`:              "()b",
		`\begin{matrix}\gdef\x{a}\end{matrix}\x`:         "a",
		// \providecommand leaves built-in commands alone.
		`\providecommand\frac{F}\frac12`: "1/2",
		// ## is the parameter of a definition inside a macro.
		`\def\a{\def\b##1{[##1]}}\a\b{x}`: "[x]",
		// URLs keep % and #.
		`\url{http://a/%20b#x}`: "http://a/%20b#x",
	} {
		got, err := RenderLine(src, LineOptions{})
		if err != nil || got != want {
			t.Errorf("RenderLine(%q) = %q, %v; want %q", src, got, err, want)
		}
	}
	if _, err := RenderLine(`\begin{matrix}\def\x{a}&\x\end{matrix}`, LineOptions{}); !errors.Is(err, ErrUnsupported) {
		t.Errorf("definition leaking into the next cell: %v", err)
	}
	for _, src := range []string{`\newcommand\frac{F}`, `\newcommand\x{X}\newcommand\x{Y}`} {
		var se *SyntaxError
		if _, err := Parse(src, ParseOptions{}); !errors.As(err, &se) {
			t.Errorf("%s: %v, want SyntaxError", src, err)
		}
	}
	// Tiny sizes are invalid instead of giving NaN dimensions.
	for _, size := range []float64{5e-324, 0.5} {
		if _, err := Render(ctx, `E=mc^2\tag{1}`, Options{SizePx: size, Display: true, MaxWidth: 100}); err == nil || errors.Is(err, ErrTooLarge) {
			t.Errorf("SizePx %v: %v, want an invalid option error", size, err)
		}
	}
}

// TestReReviewFixes covers the findings of the second final review.
func TestReReviewFixes(t *testing.T) {
	ctx := context.Background()
	for src, want := range map[string]string{
		`\newcommand\u[1]{\url{#1}}\u{abc}`:    "abc",
		`\def\u#1{\url{http://x/#1}}\u{a\%b}`:  "http://x/a%b",
		`\renewcommand\url[1]{[#1]}\url{ab}`:   "[ab]",
		`a\def\x{1}\\ \x \def\y{2}\\ \y`:       "a; 1; 2",
		`\newcommand{\R}{\mathbb{R}}\R`:        "ℝ",
		`\def\a#1{\def\b##1{#1##1}}\a{p}\b{q}`: "pq",
	} {
		got, err := RenderLine(src, LineOptions{})
		if err != nil || got != want {
			t.Errorf("RenderLine(%q) = %q, %v; want %q", src, got, err, want)
		}
	}
	var se *SyntaxError
	for _, src := range []string{
		`\providecommand\\{+}\begin{matrix}a\\b\end{matrix}x`, // \\ is built in: kept
		`\newcommand\hdashline{Q}`,
		`\def\bgroup{Q}`,
		`\begin{align}a&=b\tag{1}\end{align}+1`,
	} {
		_, err := Parse(src, ParseOptions{})
		if src[1] == 'p' {
			if err != nil {
				t.Errorf("%s: %v", src, err)
			}
			continue
		}
		if !errors.As(err, &se) {
			t.Errorf("%s: %v, want SyntaxError", src, err)
		}
	}
	if s, _ := RenderLine(`\providecommand\\{+}\begin{matrix}a\\b\end{matrix}`, LineOptions{}); s != "a; b" {
		t.Errorf("\\providecommand\\\\ changed the matrix: %q", s)
	}
	// A mark that neither font has fails after a CJK run too.
	var u *UnsupportedError
	if _, err := Render(ctx, "\\text{定\u20E3}", Options{TextFace: cjkFace}); !errors.As(err, &u) {
		t.Errorf("\\text{定⃣}: %v, want UnsupportedError", err)
	}
}

// TestAdversarialReviewFixes covers the findings of the adversarial review
// (2026-11): the context during rasterization, Limits.MaxPixels,
// \csname via \expandafter, separators and invisible characters, emoji
// presentation width and TextResult.Width.
func TestAdversarialReviewFixes(t *testing.T) {
	huge := `\rule{39em}{830em}`
	t.Run("deadline during raster", func(t *testing.T) {
		// Fewer than 16 scene items, so the per-item Tick never looks at
		// the context; each rule alone takes tens of milliseconds.
		src := huge + `\kern-39em` + huge + `\kern-39em` + huge
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
		defer cancel()
		start := time.Now()
		res, err := Render(ctx, src, Options{MaxWidth: 800})
		if !errors.Is(err, context.DeadlineExceeded) || res != nil {
			t.Fatalf("got %v, %v; want DeadlineExceeded", res, err)
		}
		if d := time.Since(start); d > 300*time.Millisecond {
			t.Errorf("took %v after a 5ms deadline", d)
		}
	})
	t.Run("MaxPixels", func(t *testing.T) {
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		_, err := Render(context.Background(), huge, Options{MaxWidth: 800, Limits: Limits{MaxPixels: 1 << 20}})
		runtime.ReadMemStats(&after)
		if !errors.Is(err, ErrTooLarge) {
			t.Fatalf("got %v, want ErrTooLarge", err)
		}
		if alloc := after.TotalAlloc - before.TotalAlloc; alloc > 4<<20 {
			t.Errorf("allocated %d MiB before failing", alloc>>20)
		}
		// The limit is on W*H and also bounds the rasterizer's work.
		res, err := Render(context.Background(), `\rule{2em}{2em}`, Options{Limits: Limits{MaxPixels: 1 << 20}})
		if err != nil || res.Image.Bounds().Dx()*res.Image.Bounds().Dy() > 1<<20 {
			t.Fatalf("small formula: %v", err)
		}
		if DefaultMaxPixels*9 > 160<<20 {
			t.Errorf("DefaultMaxPixels no longer matches the documented ~150 MB")
		}
	})
	t.Run("csname", func(t *testing.T) {
		for _, src := range []string{
			`\expandafter\def\csname a\endcsname{x}\a`,
			`\def\csname{x}`,
			`\let\endcsname=x`,
		} {
			_, err := RenderLine(src, LineOptions{})
			var ue *UnsupportedError
			if !errors.As(err, &ue) || ue.Command != `\csname` {
				t.Errorf("%q: got %v, want unsupported \\csname", src, err)
			}
		}
	})
	t.Run("separators", func(t *testing.T) {
		for _, src := range []string{"a\u2028b", `\text{a` + "\u2029" + `b}`} {
			if _, err := RenderLine(src, LineOptions{}); !isSyntax(err) {
				t.Errorf("line %q: got %v, want SyntaxError", src, err)
			}
			if _, err := RenderText(context.Background(), src, TextOptions{}); !isSyntax(err) {
				t.Errorf("text %q: got %v, want SyntaxError", src, err)
			}
		}
	})
	t.Run("invisible characters kept", func(t *testing.T) {
		src := "\\text{a\u200Bb\uFEFFc}"
		line, err := RenderLine(src, LineOptions{})
		if err != nil {
			t.Fatal(err)
		}
		res, err := RenderText(context.Background(), src, TextOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Join(res.Lines, ""); got != line || got != "a\u200Bb\uFEFFc" {
			t.Errorf("text %q, line %q; want both %q", got, line, "a\u200Bb\uFEFFc")
		}
		if res.Width != 3 {
			t.Errorf("Width = %d, want 3", res.Width)
		}
	})
	t.Run("emoji presentation", func(t *testing.T) {
		res, err := RenderText(context.Background(), `\text{❤`+"\uFE0F"+`x} \text{❤x}`, TextOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if got := textgrid.StringWidth(res.Lines[res.Baseline]); got != res.Width || got != 5 {
			t.Errorf("width %d, Width %d, want 5 (❤️ is two columns, ❤ one): %q", got, res.Width, res.Lines)
		}
	})
	t.Run("TextResult.Width", func(t *testing.T) {
		res, err := RenderText(context.Background(), `\frac{a+b}{c}`, TextOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if res.Width != textgrid.Width(res.Lines) || res.Width == 0 {
			t.Errorf("Width = %d, lines %q", res.Width, res.Lines)
		}
	})
}

func isSyntax(err error) bool {
	var se *SyntaxError
	return errors.As(err, &se)
}

// TestTallFloorCeil checks that floor and ceiling brackets grow to the
// height of what they enclose. Latin Modern Math builds them from one
// fixed part and an extender; the assembly used to stop at the fixed part
// alone (1.5em), shorter than the largest size variant (3em).
func TestTallFloorCeil(t *testing.T) {
	const sizePx = 20.0
	for _, d := range []string{`\lceil`, `\rceil`, `\lfloor`, `\rfloor`} {
		for _, em := range []float64{5, 12} {
			src := fmt.Sprintf(`\left%s \vphantom{\rule{1em}{%gem}} \right.`, d, em)
			res, err := Render(context.Background(), src, Options{SizePx: sizePx})
			if err != nil {
				t.Fatalf("%s: %v", src, err)
			}
			// Only the delimiter has ink; TeX makes it at least 90% of the
			// enclosed height (\delimiterfactor 901).
			b := res.Image.Bounds()
			top, bottom := b.Max.Y, b.Min.Y
			for y := b.Min.Y; y < b.Max.Y; y++ {
				for x := b.Min.X; x < b.Max.X; x++ {
					if res.Image.RGBAAt(x, y).A != 0 {
						top, bottom = min(top, y), max(bottom, y+1)
						break
					}
				}
			}
			if ink, want := float64(bottom-top), 0.9*em*sizePx; ink < want {
				t.Errorf("%s: delimiter ink is %v px high, want at least %v", src, ink, want)
			}
		}
	}
}

// TestAssemblySeams checks that a stretched delimiter has no seams: its
// parts overlap, and compositing each part on its own made the partly
// covered pixels of the overlap darker on the stroke's edges and lighter
// in its middle. Along a straight stroke every column must keep nearly
// one alpha value from top to bottom.
func TestAssemblySeams(t *testing.T) {
	for _, d := range []string{`|`, `\|`, `\lceil`, `\rfloor`, `\uparrow`} {
		// 30em at 20 px is taller than a rasterizer tile (512 px): the
		// tiles must join without seams too.
		for _, c := range []struct{ em, sizePx float64 }{{12, 20}, {12, 37}, {30, 20}} {
			sizePx := c.sizePx
			src := fmt.Sprintf(`\left%s \vphantom{\rule{1em}{%gem}} \right.`, d, c.em)
			res, err := Render(context.Background(), src, Options{SizePx: sizePx})
			if err != nil {
				t.Fatalf("%s: %v", src, err)
			}
			img := res.Image
			b := img.Bounds()
			top, bottom := b.Max.Y, b.Min.Y
			for y := b.Min.Y; y < b.Max.Y; y++ {
				for x := b.Min.X; x < b.Max.X; x++ {
					if img.RGBAAt(x, y).A != 0 {
						top, bottom = min(top, y), max(bottom, y+1)
					}
				}
			}
			// Skip the end parts (the bar of ⌈, the arrow head; up to 1.5em
			// in Latin Modern Math), whose strokes may differ slightly
			// from the extender's. Between them lie many seams.
			margin := int(math.Ceil(2 * sizePx))
			for x := b.Min.X; x < b.Max.X; x++ {
				lo, hi := 255, 0
				for y := top + margin; y < bottom-margin; y++ {
					a := int(img.RGBAAt(x, y).A)
					lo, hi = min(lo, a), max(hi, a)
				}
				// The extender of | in Latin Modern Math leans by about
				// 0.03 px over its length, which ramps an edge column by
				// a few levels per part; the seams were 25 to 65 levels.
				if hi > lo+10 {
					t.Errorf("%s at %v px: column %d varies from %d to %d", src, sizePx, x, lo, hi)
				}
			}
		}
	}
}
