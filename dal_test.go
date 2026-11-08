package dal

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"

	"github.com/lea-151107/dal/internal/textgrid"
)

var update = flag.Bool("update", false, "rewrite golden files")

type fixture struct {
	name    string
	src     string
	display bool
}

func fixtures(t testing.TB) []fixture {
	t.Helper()
	paths, err := filepath.Glob("testdata/*.tex")
	if err != nil {
		t.Fatal(err)
	}
	var out []fixture
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		name := strings.TrimSuffix(filepath.Base(p), ".tex")
		out = append(out, fixture{name, strings.TrimSpace(string(b)), !strings.HasPrefix(name, "inline_")})
	}
	if len(out) == 0 {
		t.Fatal("no fixtures")
	}
	return out
}

func goldenOptions(f fixture) Options {
	opt := Options{Display: f.display, SizePx: 24, PaddingPx: 4, Background: color.White}
	if strings.HasSuffix(f.name, "_ja") {
		opt.TextFace = cjkFace
	}
	return opt
}

// cjkFont is the Japanese test font for the _ja golden cases.
var cjkFont = sync.OnceValues(func() (*opentype.Font, error) {
	b, err := os.ReadFile("testdata/fonts/NotoSansCJKjp-subset.otf")
	if err != nil {
		return nil, err
	}
	return opentype.Parse(b)
})

func cjkFace(size float64) (font.Face, error) {
	f, err := cjkFont()
	if err != nil {
		return nil, err
	}
	return opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingNone})
}

func checkGolden(t *testing.T, path string, got []byte, equal func(want []byte) bool) {
	t.Helper()
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run go test -update)", err)
	}
	if !equal(want) {
		t.Errorf("output differs from %s (run go test -update and check the diff)", path)
	}
}

func TestGolden(t *testing.T) {
	for _, f := range fixtures(t) {
		t.Run(f.name, func(t *testing.T) {
			res, err := Render(context.Background(), f.src, goldenOptions(f))
			if err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			if err := png.Encode(&buf, res.Image); err != nil {
				t.Fatal(err)
			}
			checkGolden(t, filepath.Join("testdata", "golden", f.name+".png"), buf.Bytes(), func(want []byte) bool {
				img, err := png.Decode(bytes.NewReader(want))
				return err == nil && pixelsEqual(img, res.Image)
			})
		})
	}
}

func textOutput(f fixture) string {
	res, err := RenderText(context.Background(), f.src, TextOptions{Display: f.display})
	if err != nil {
		return "error: " + err.Error() + "\n"
	}
	var b strings.Builder
	for i, l := range res.Lines {
		mark := "  "
		if i == res.Baseline {
			mark = "> "
		}
		b.WriteString(mark + l + "\n")
	}
	return b.String()
}

func TestGoldenText(t *testing.T) {
	for _, f := range fixtures(t) {
		t.Run(f.name, func(t *testing.T) {
			got := textOutput(f)
			checkGolden(t, filepath.Join("testdata", "golden", f.name+".txt"), []byte(got), func(want []byte) bool {
				return string(want) == got
			})
		})
	}
}

func TestGoldenLine(t *testing.T) {
	for _, f := range fixtures(t) {
		t.Run(f.name, func(t *testing.T) {
			var b strings.Builder
			for _, st := range []LineStyle{LineUnicode, LineASCII} {
				s, err := RenderLine(f.src, LineOptions{Style: st})
				if err != nil {
					s = "error: " + err.Error()
				}
				b.WriteString(s + "\n")
			}
			got := b.String()
			checkGolden(t, filepath.Join("testdata", "golden", f.name+"_line.txt"), []byte(got), func(want []byte) bool {
				return string(want) == got
			})
		})
	}
}

func pixelsEqual(a image.Image, b *image.RGBA) bool {
	if a.Bounds() != b.Bounds() {
		return false
	}
	r := a.Bounds()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if color.NRGBAModel.Convert(a.At(x, y)) != color.NRGBAModel.Convert(b.At(x, y)) {
				return false
			}
		}
	}
	return true
}

// TestDeterministic renders every fixture repeatedly, concurrently, and
// requires identical pixels and text.
func TestDeterministic(t *testing.T) {
	for _, f := range fixtures(t) {
		t.Run(f.name, func(t *testing.T) {
			const n = 4
			var wg sync.WaitGroup
			sums := make([][32]byte, n)
			texts := make([]string, n)
			lines := make([]string, n)
			errs := make([]error, n)
			for i := 0; i < n; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					res, err := Render(context.Background(), f.src, goldenOptions(f))
					if err != nil {
						errs[i] = err
						return
					}
					sums[i] = sha256.Sum256(res.Image.Pix)
					texts[i] = textOutput(f)
					lines[i], _ = RenderLine(f.src, LineOptions{})
				}(i)
			}
			wg.Wait()
			for i := 0; i < n; i++ {
				if errs[i] != nil {
					t.Fatal(errs[i])
				}
				if sums[i] != sums[0] || texts[i] != texts[0] || lines[i] != lines[0] {
					t.Fatalf("run %d differs", i)
				}
			}
		})
	}
}

func TestMaxWidth(t *testing.T) {
	for _, f := range fixtures(t) {
		for _, w := range []int{60, 200, 400} {
			opt := goldenOptions(f)
			opt.MaxWidth = w
			res, err := Render(context.Background(), f.src, opt)
			switch {
			case errors.Is(err, ErrTooLarge):
			case err != nil:
				t.Errorf("%s/%d: %v", f.name, w, err)
			case res.Image.Bounds().Dx() > w:
				t.Errorf("%s/%d: width %d", f.name, w, res.Image.Bounds().Dx())
			}
		}
	}
}

func TestMaxCols(t *testing.T) {
	for _, f := range fixtures(t) {
		for _, c := range []int{10, 30, 80} {
			res, err := RenderText(context.Background(), f.src, TextOptions{Display: f.display, MaxCols: c})
			switch {
			case errors.Is(err, ErrTooLarge), errors.Is(err, ErrUnsupported):
			case err != nil:
				t.Errorf("%s/%d: %v", f.name, c, err)
			default:
				for _, l := range res.Lines {
					if w := stringWidth(l); w > c {
						t.Errorf("%s/%d: line %q has width %d", f.name, c, l, w)
					}
					for _, r := range l {
						if r < 0x20 || r == 0x7f || r >= 0x80 && r < 0xa0 {
							t.Errorf("%s: control character %U", f.name, r)
						}
					}
				}
				if res.Baseline < 0 || res.Baseline >= len(res.Lines) {
					t.Errorf("%s: baseline %d of %d lines", f.name, res.Baseline, len(res.Lines))
				}
			}
		}
	}
}

func stringWidth(s string) int { return textgrid.StringWidth(s) }

func testFace(size float64) (font.Face, error) {
	f, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return nil, err
	}
	return opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingNone})
}

func TestResultMetrics(t *testing.T) {
	res, err := Render(context.Background(), `\frac{a}{b}`, Options{SizePx: 20, PaddingPx: 3})
	if err != nil {
		t.Fatal(err)
	}
	if res.Height <= 0 || res.Depth <= 0 || res.Width <= 0 {
		t.Fatalf("metrics %+v", res)
	}
	b := res.Image.Bounds()
	if res.Baseline < 3+int(res.Height) || res.Baseline > b.Dy()-3 {
		t.Errorf("baseline %d outside the box (height %.1f, image %d)", res.Baseline, res.Height, b.Dy())
	}
	if b.Dx() < int(res.Width)+6 {
		t.Errorf("image width %d smaller than box %.1f + padding", b.Dx(), res.Width)
	}
	// Transparent background by default; the ink is black.
	if _, _, _, a := res.Image.At(0, 0).RGBA(); a != 0 {
		t.Errorf("corner not transparent")
	}
}

func TestForeground(t *testing.T) {
	red := color.NRGBA{255, 0, 0, 255}
	res, err := Render(context.Background(), `I`, Options{SizePx: 40, Foreground: red})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := 0; i < len(res.Image.Pix); i += 4 {
		p := res.Image.Pix[i : i+4]
		if p[3] == 255 {
			found = true
			if p[0] != 255 || p[1] != 0 || p[2] != 0 {
				t.Fatalf("opaque pixel %v is not red", p)
			}
		}
	}
	if !found {
		t.Fatal("no opaque pixel")
	}
}

func TestSyntaxErrors(t *testing.T) {
	cases := []struct {
		src            string
		off, line, col int
		msg            string
	}{
		{`\frac{a}`, 0, 1, 1, "missing argument"},
		{`{x`, 0, 1, 1, "missing }"},
		{`x}`, 1, 1, 2, "unmatched }"},
		{`x^2^3`, 3, 1, 4, "double superscript"},
		{`a_1_2`, 3, 1, 4, "double subscript"},
		{`\left( x`, 0, 1, 1, "missing \\right"},
		{`x \right)`, 2, 1, 3, "\\right without \\left"},
		{`a & b`, 2, 1, 3, "misplaced &"},
		{"a +\n\\frac{1}", 4, 2, 1, "missing argument"},
		{"\\begin{matrix}\na \\end{pmatrix}", 17, 2, 3, "ended by"},
		{`\begin{matrix} a`, 0, 1, 1, "missing \\end"},
		{`\sqrt[3{x}`, 5, 1, 6, "missing ]"},
		{`\left\foo x \right)`, 5, 1, 6, "invalid delimiter"},
		{`\color{nocolor} x`, 0, 1, 1, "invalid color"},
		{`x^`, 1, 1, 2, "missing argument"},
		{"\xff", 0, 1, 1, "invalid UTF-8"},
		{`\newcommand{\f}[2]{#3}`, 19, 1, 20, "parameter #3"},
		{`\middle|`, 0, 1, 1, "\\middle without"},
	}
	for _, c := range cases {
		_, err := Parse(c.src, ParseOptions{})
		var se *SyntaxError
		if !errors.As(err, &se) {
			t.Errorf("%q: got %v, want SyntaxError", c.src, err)
			continue
		}
		if se.Offset != c.off || se.Line != c.line || se.Col != c.col || !strings.Contains(se.Msg, c.msg) {
			t.Errorf("%q: got %+v, want offset %d %d:%d %q", c.src, *se, c.off, c.line, c.col, c.msg)
		}
		if errors.Is(err, ErrUnsupported) || errors.Is(err, ErrTooLarge) {
			t.Errorf("%q: syntax error matches another sentinel", c.src)
		}
		// Every output reports the same error.
		if _, err := RenderLine(c.src, LineOptions{}); !errors.As(err, &se) {
			t.Errorf("%q: RenderLine got %v", c.src, err)
		}
	}
}

func TestUnsupported(t *testing.T) {
	cases := []struct {
		src, cmd string
		off      int
	}{
		{`x + \foo`, `\foo`, 4},
		{`\begin{tabular}{c} a \end{tabular}`, `\begin{tabular}`, 0},
		{`\begin{array}{*{2}{c}} a \end{array}`, `array column type *`, 0},
		{`\mathbbm{x}`, `\mathbbm`, 0},
	}
	for _, c := range cases {
		for name, run := range map[string]func() error{
			"parse":  func() error { _, err := Parse(c.src, ParseOptions{}); return err },
			"render": func() error { _, err := Render(context.Background(), c.src, Options{}); return err },
			"text":   func() error { _, err := RenderText(context.Background(), c.src, TextOptions{}); return err },
			"line":   func() error { _, err := RenderLine(c.src, LineOptions{}); return err },
		} {
			err := run()
			var ue *UnsupportedError
			if !errors.As(err, &ue) || !errors.Is(err, ErrUnsupported) {
				t.Errorf("%s %q: got %v", name, c.src, err)
				continue
			}
			if ue.Command != c.cmd || ue.Offset != c.off {
				t.Errorf("%s %q: got %+v, want %q at %d", name, c.src, *ue, c.cmd, c.off)
			}
		}
	}
	// Output-specific: \not before a symbol without a negated form has no
	// text form, CJK needs a TextFace.
	if _, err := RenderText(context.Background(), `\not a`, TextOptions{}); !errors.Is(err, ErrUnsupported) {
		t.Errorf("RenderText(\\not a): %v", err)
	}
	if res, err := RenderText(context.Background(), `\cancel{x}`, TextOptions{}); err != nil || res.Lines[0] != "cancel(x)" {
		t.Errorf("RenderText(\\cancel) = %+v, %v", res, err)
	}
	if s, err := RenderLine(`\cancel{x}`, LineOptions{}); err != nil || s != "cancel(x)" {
		t.Errorf("RenderLine(\\cancel) = %q, %v", s, err)
	}
	if _, err := Render(context.Background(), `\text{速度}`, Options{}); !errors.Is(err, ErrUnsupported) {
		t.Errorf("Render(\\text{速度}) without TextFace: %v", err)
	}
}

func TestLimits(t *testing.T) {
	cases := []struct {
		name string
		src  string
		lim  Limits
	}{
		{"input", strings.Repeat("x", 100), Limits{MaxInputBytes: 50}},
		{"nodes", strings.Repeat("x+", 100), Limits{MaxNodes: 50}},
		{"depth", strings.Repeat("{", 100) + strings.Repeat("}", 100), Limits{MaxDepth: 20}},
		{"expansions", `\def\a{\a}\a`, Limits{MaxExpansions: 100}},
		{"exponential", `\def\a{xx}\def\b{\a\a\a\a}\def\c{\b\b\b\b}\def\d{\c\c\c\c}\def\e{\d\d\d\d}\def\f{\e\e\e\e}\def\g{\f\f\f\f}\def\h{\g\g\g\g}\h`, Limits{}},
		{"default input", strings.Repeat("x", DefaultMaxInputBytes+1), Limits{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Render(context.Background(), c.src, Options{Limits: c.lim})
			if !errors.Is(err, ErrTooLarge) {
				t.Errorf("Render: %v", err)
			}
			if _, err := RenderText(context.Background(), c.src, TextOptions{Limits: c.lim}); !errors.Is(err, ErrTooLarge) {
				t.Errorf("RenderText: %v", err)
			}
			if _, err := RenderLine(c.src, LineOptions{Limits: c.lim}); !errors.Is(err, ErrTooLarge) {
				t.Errorf("RenderLine: %v", err)
			}
			if err != nil && strings.Count(err.Error(), "too large") != 1 {
				t.Errorf("message %q", err)
			}
		})
	}
	if _, err := Render(context.Background(), "x", Options{SizePx: 1e6}); !errors.Is(err, ErrTooLarge) {
		t.Errorf("huge SizePx: %v", err)
	}
}

func TestContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Render(ctx, "x", Options{}); !errors.Is(err, context.Canceled) {
		t.Errorf("Render: %v", err)
	}
	if _, err := RenderText(ctx, "x", TextOptions{}); !errors.Is(err, context.Canceled) {
		t.Errorf("RenderText: %v", err)
	}
	// A large matrix with a deadline that has passed during rendering.
	var b strings.Builder
	b.WriteString(`\begin{pmatrix}`)
	for i := 0; i < 60; i++ {
		b.WriteString(`\frac{a}{b} & \sqrt{x} & y \\`)
	}
	b.WriteString(`\end{pmatrix}`)
	ctx, cancel = context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond)
	if _, err := Render(ctx, b.String(), Options{}); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("deadline: %v", err)
	}
}

func TestTextFace(t *testing.T) {
	calls := 0
	opt := Options{TextFace: func(size float64) (font.Face, error) {
		calls++
		return cjkFace(size)
	}}
	// Characters the math font has never reach the text face.
	if _, err := Render(context.Background(), `\text{otherwise} + x^{\text{ab}}`, opt); err != nil || calls != 0 {
		t.Errorf("Latin \\text: %v, %d TextFace calls (want 0)", err, calls)
	}
	res, err := Render(context.Background(), `\text{定義 abc} + x^{\text{定}}`, opt)
	if err != nil {
		t.Fatal(err)
	}
	if res.Width <= 0 || calls != 2 {
		t.Errorf("width %.1f, %d TextFace calls (want 2 sizes)", res.Width, calls)
	}
	opt.TextFace = func(float64) (font.Face, error) { return nil, fmt.Errorf("boom") }
	if _, err := Render(context.Background(), `\text{定}`, opt); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("TextFace error not reported: %v", err)
	}
}

func TestFontOption(t *testing.T) {
	if _, err := Render(context.Background(), "x", Options{Font: []byte("not a font")}); err == nil {
		t.Error("invalid font accepted")
	}
}

func TestVersion(t *testing.T) {
	if Version == "" {
		t.Fatal("empty Version")
	}
}
