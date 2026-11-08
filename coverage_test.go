package dal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/lea-151107/dal/internal/symbols"
)

// coverageStatus summarizes one output's result for the coverage table.
func coverageStatus(err error) string {
	var u *UnsupportedError
	switch {
	case err == nil:
		return "ok"
	case errors.As(err, &u):
		return "unsupported"
	case errors.Is(err, ErrTooLarge):
		return "too-large"
	}
	var s *SyntaxError
	if errors.As(err, &s) {
		return "syntax"
	}
	return "error"
}

// TestKaTeXCoverage runs every input of testdata/katex_coverage.txt (taken
// from KaTeX's list of supported functions) through the three outputs and
// compares the results with a golden table, so that both new support and
// lost support show up in the diff. The table is kept outside
// testdata/golden: it is not rendering output, so changing the input list
// must not require a new Version. The summary line counts the inputs
// that work in all three outputs. "Work" means no error: it does not
// compare the drawing with KaTeX or TeX, so it measures coverage, not
// typesetting quality.
func TestKaTeXCoverage(t *testing.T) {
	src, err := os.ReadFile("testdata/katex_coverage.txt")
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	total, full := 0, 0
	section := ""
	for _, line := range strings.Split(string(src), "\n") {
		switch {
		case line == "", strings.HasPrefix(line, "# "):
			continue
		case strings.HasPrefix(line, "## "):
			section = strings.TrimPrefix(line, "## ")
			b.WriteString(line + "\n")
			continue
		}
		ctx := context.Background()
		_, e1 := Render(ctx, line, Options{Display: true})
		_, e2 := RenderText(ctx, line, TextOptions{Display: true})
		_, e3 := RenderLine(line, LineOptions{})
		st := coverageStatus(e1) + " " + coverageStatus(e2) + " " + coverageStatus(e3)
		if section != "out of scope" && section != "regressions" {
			total++
			if e1 == nil && e2 == nil && e3 == nil {
				full++
			}
		}
		b.WriteString(st + "\t" + line + "\n")
	}
	got := "# image text line\n# all three outputs: " + strconv.Itoa(full) + " of " + strconv.Itoa(total) + " (out of scope and regressions excluded)\n" + b.String()
	checkGolden(t, filepath.Join("testdata", "katex_coverage.golden"), []byte(got), func(want []byte) bool {
		return string(want) == got
	})
}

// needsTextFace lists the characters of the command tables that the
// embedded math font lacks and that have no substitute: images draw them
// only with a TextFace that has them.
var needsTextFace = func() map[string]bool {
	m := map[string]bool{}
	for _, r := range "ϝ⅁Ⅎ⋔⎰⎱★⧫⦵Ⓢ⪷⪸⪹⪺⪵⪶⫅⫆⫋⫌⩞⇠⇢⦃⦄" {
		m[string(r)] = true
	}
	return m
}()

// TestSymbolTablesRender renders every entry of the command tables, so that
// a character missing from the math font cannot slip in unnoticed.
func TestSymbolTablesRender(t *testing.T) {
	var srcs []string
	want := map[string]string{} // source -> character expected to need a TextFace
	add := func(src, char string) {
		srcs = append(srcs, src)
		if needsTextFace[char] {
			want[src] = char
		}
	}
	for name, s := range symbols.Commands {
		add(`\`+name, s.Text)
	}
	for r, s := range symbols.Chars {
		add(string(r), s.Text)
	}
	for key, d := range symbols.Delims {
		if d != "" {
			add(`\left`+key+` x \right.`, d)
		}
	}
	for name := range symbols.Ops {
		add(`\`+name+`_a^b`, "")
	}
	for name := range symbols.Accents {
		add(`\`+name+`{ab}`, "")
	}
	for name := range symbols.XArrows {
		add(`\`+name+`{ab}`, "")
	}
	sort.Strings(srcs)
	for _, src := range srcs {
		_, err := Render(context.Background(), src, Options{})
		var u *UnsupportedError
		switch {
		case want[src] != "":
			if !errors.As(err, &u) {
				t.Errorf("%s: %v, want an UnsupportedError (no glyph for %s)", src, err, want[src])
			}
		case err != nil:
			t.Errorf("%s: %v", src, err)
		}
	}
}

// TestGoldenOutputs records, for every input of testdata/katex_coverage.txt,
// the single-line output and hashes of the text and image outputs in
// testdata/golden/katex_outputs.txt. The file is under the Version check,
// so a change of any of these outputs needs a new Version even when no
// fixture shows it (a macro or URL fix, say).
func TestGoldenOutputs(t *testing.T) {
	src, err := os.ReadFile("testdata/katex_coverage.txt")
	if err != nil {
		t.Fatal(err)
	}
	short := func(b []byte) string {
		h := sha256.Sum256(b)
		return hex.EncodeToString(h[:6])
	}
	var b strings.Builder
	ctx := context.Background()
	for _, line := range strings.Split(string(src), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out := line + "\n"
		if s, err := RenderLine(line, LineOptions{}); err != nil {
			out += "  line: error " + coverageStatus(err) + "\n"
		} else {
			out += "  line: " + s + "\n"
		}
		if res, err := RenderText(ctx, line, TextOptions{Display: true}); err != nil {
			out += "  text: error " + coverageStatus(err) + "\n"
		} else {
			out += "  text: " + short([]byte(strings.Join(res.Lines, "\n")+"\n"+strconv.Itoa(res.Baseline))) + "\n"
		}
		if res, err := Render(ctx, line, Options{Display: true}); err != nil {
			out += "  image: error " + coverageStatus(err) + "\n"
		} else {
			out += "  image: " + short(res.Image.Pix) + fmt.Sprintf(" %dx%d %d", res.Image.Bounds().Dx(), res.Image.Bounds().Dy(), res.Baseline) + "\n"
		}
		b.WriteString(out)
	}
	got := b.String()
	checkGolden(t, filepath.Join("testdata", "golden", "katex_outputs.txt"), []byte(got), func(want []byte) bool {
		return string(want) == got
	})
}
