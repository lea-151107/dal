package dal

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/lea/dal/internal/linear"
	"github.com/lea/dal/internal/mathfont"
	"github.com/lea/dal/internal/parse"
)

const fuzzTimeLimit = 2 * time.Second

func fuzzSeeds(f *testing.F) {
	paths, _ := filepath.Glob("testdata/*.tex")
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(string(b))
	}
	for _, s := range []string{"", "{", "}", `\`, `\left(`, `x^`, `\begin{matrix}`, `\frac{}{}`, `\sqrt[]{}`,
		`\def\a#1{#1#1}\a{\a{\a{x}}}`, `\begin{array}{|c|}\hline a\\\hline\end{array}`, `\middle|`, "\x00", "%",
		`\left(\middle|^2\right)`, `\mathrm{\middle|}^2`, `{x^2}^3`, `\sqrt{x}^2`, `\sqrt[\frac12]{x}`, `\not a`,
		`\begin{matrix}&&&\\\\\\\end{matrix}`, "x\u009b", "a\u202eb",
		`a \over b \over c`, `\begin{CD}@`, `\begin{CD}A@>x`, `\def\f#1.{#1}\f`, `\verb`, `\verb!x`,
		`\char"110000`, `\char`, `{\def\a{}`, `\expandafter`, `\futurelet`, `\let\a=`, `\genfrac`,
		`\genfrac{(}{}{-1pt}{3}{}{}`, `\substack{`, `\newline`, `a\\`, `\\`, `\rule{1000pt}{1000pt}`,
		`\begin{array}{:::::}\hdashline\end{array}`, `\raisebox{}{}`, `\Huge\tiny x`, `\mathchoice{}{}{}`,
		`\Braket{|||}`, `\set{|}`, `\text{\(\)}`, `\text{\(}`, `\textcircled{}`, `\reflectbox{\reflectbox{x}}`} {
		f.Add(s)
	}
	// Every input of the KaTeX coverage list.
	if b, err := os.ReadFile("testdata/katex_coverage.txt"); err == nil {
		for _, l := range strings.Split(string(b), "\n") {
			if l != "" && !strings.HasPrefix(l, "#") {
				f.Add(l)
			}
		}
	}
}

// checkErr fails unless err is nil or one of the documented errors.
func checkErr(t *testing.T, src string, err error) {
	t.Helper()
	var se *SyntaxError
	var ue *UnsupportedError
	switch {
	case err == nil, errors.Is(err, ErrTooLarge), errors.Is(err, context.DeadlineExceeded):
	case errors.As(err, &se):
		if se.Offset < 0 || se.Offset > len(src) || se.Line < 1 || se.Col < 1 {
			t.Fatalf("bad position %+v for %q", *se, src)
		}
	case errors.As(err, &ue):
		if ue.Offset < 0 || ue.Offset > len(src) || !errors.Is(err, ErrUnsupported) {
			t.Fatalf("bad unsupported error %+v for %q", *ue, src)
		}
	default:
		t.Fatalf("unexpected error %v for %q", err, src)
	}
}

// checkNoControl fails when out contains a control or bidirectional
// formatting character.
func checkNoControl(t *testing.T, src, out string) {
	t.Helper()
	for _, r := range out {
		if unicode.IsControl(r) || unicode.Is(unicode.Bidi_Control, r) {
			t.Fatalf("output %q for %q contains U+%04X", out, src, r)
		}
	}
}

func timed(t *testing.T, src string, what string, fn func()) {
	t.Helper()
	start := time.Now()
	fn()
	if d := time.Since(start); d > fuzzTimeLimit {
		t.Fatalf("%s took %v for %q", what, d, src)
	}
}

var fuzzLimits = Limits{MaxNodes: 300, MaxInputBytes: 2000}

func FuzzParse(f *testing.F) {
	fuzzSeeds(f)
	f.Fuzz(func(t *testing.T, src string) {
		timed(t, src, "Parse", func() {
			// Call the internal parser so that a panic is not hidden by the
			// recover in Parse.
			_, err := parse.Parse(src, parseLimits(fuzzLimits))
			checkErr(t, src, convertErr(src, err))
		})
	})
}

func FuzzRender(f *testing.F) {
	fuzzSeeds(f)
	f.Fuzz(func(t *testing.T, src string) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		timed(t, src, "Render", func() {
			// Call the unexported function so that a panic is not hidden by
			// the recover in Render.
			res, err := render(ctx, src, Options{SizePx: 12, MaxWidth: 2000, Limits: fuzzLimits})
			checkErr(t, src, convertErr(src, err))
			if err == nil && res.Image.Bounds().Dx() > 2000 {
				t.Fatalf("width %d", res.Image.Bounds().Dx())
			}
		})
	})
}

func FuzzRenderText(f *testing.F) {
	fuzzSeeds(f)
	f.Fuzz(func(t *testing.T, src string) {
		timed(t, src, "RenderText", func() {
			res, err := renderText(context.Background(), src, TextOptions{MaxCols: 200, Limits: fuzzLimits})
			checkErr(t, src, convertErr(src, err))
			if err == nil {
				for _, l := range res.Lines {
					if stringWidth(l) > 200 {
						t.Fatalf("line too wide: %q", l)
					}
					checkNoControl(t, src, l)
				}
			}
		})
	})
}

func FuzzRenderLine(f *testing.F) {
	fuzzSeeds(f)
	f.Fuzz(func(t *testing.T, src string) {
		timed(t, src, "RenderLine", func() {
			// Call the internal packages so that a panic is not hidden by the
			// recover in RenderLine.
			e, err := parse.Parse(src, parseLimits(fuzzLimits))
			checkErr(t, src, convertErr(src, err))
			if err != nil {
				return
			}
			for _, ascii := range []bool{false, true} {
				checkNoControl(t, src, linear.Write(e, linear.Options{ASCII: ascii}))
			}
		})
	})
}

// FuzzMathTable feeds corrupted fonts through the MATH table parser and the
// typesetter.
func FuzzMathTable(f *testing.F) {
	orig := mathfont.Embedded()
	f.Add(uint32(0), byte(0))
	f.Add(uint32(len(orig)/2), byte(0xff))
	f.Fuzz(func(t *testing.T, pos uint32, val byte) {
		data := append([]byte(nil), orig...)
		// Corrupt a window of bytes inside the MATH table.
		start, end := mathfont.MathTableRange(data)
		if end <= start {
			t.Skip()
		}
		p := start + int(pos)%(end-start)
		for i := 0; i < 16 && p+i < end; i++ {
			data[p+i] ^= val + byte(i)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		timed(t, "", "Render", func() {
			_, err := render(ctx, `\left(\sum_{i}^{n}\frac{\sqrt[3]{x}}{\widehat{y}}\right) \overbrace{a}^{b} \xrightarrow{f}`,
				Options{Font: data, Limits: fuzzLimits})
			if err != nil && !errors.Is(err, ErrTooLarge) && !errors.Is(err, context.DeadlineExceeded) {
				var ue *UnsupportedError
				if !errors.As(convertErr("", err), &ue) {
					t.Logf("error (allowed for a broken font): %v", err)
				}
			}
		})
	})
}
