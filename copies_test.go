package dal

import (
	"bytes"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"github.com/lea/dal/internal/symbols"
)

// damDir is the mermaid module, which dal's copies come from. Both modules
// live side by side in the vim_markdown_viewer repository.
const damDir = "../dam"

// TestCopiesFromDam checks that the files dal copied from dam (to stay an
// independent module without a shared dependency) have not drifted. guard.go
// may differ in comments only; the width table and its generator must be
// identical. Fix a drift by changing both copies in the same commit.
func TestCopiesFromDam(t *testing.T) {
	if _, err := os.Stat(damDir); err != nil {
		t.Skip("dam module not checked out next to dal")
	}
	for _, c := range []struct {
		path           string
		ignoreComments bool
	}{
		{"internal/guard/guard.go", true},
		{"internal/textgrid/widthtable.go", false},
		{"internal/textgrid/gen_width.go", false},
		{"internal/curve/curve.go", false},
		{"internal/curve/curve_test.go", false},
		{"testdata/fonts/NotoSansCJKjp-subset.otf", false},
		{"testdata/fonts/OFL.txt", false},
	} {
		t.Run(c.path, func(t *testing.T) {
			ours := readCopy(t, c.path, c.ignoreComments)
			theirs := readCopy(t, filepath.Join(damDir, c.path), c.ignoreComments)
			if !bytes.Equal(ours, theirs) {
				t.Errorf("%s differs from %s", c.path, filepath.Join(damDir, c.path))
			}
		})
	}
}

// readCopy returns the file's contents, or with ignoreComments its code
// printed without comments.
func readCopy(t *testing.T, path string, ignoreComments bool) []byte {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !ignoreComments {
		return src
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, 0) // no ParseComments
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, f); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestColorsMatchDam checks that the CSS color table equals dam's, so that
// the two renderers agree on \color{teal} and style:fill:teal.
func TestColorsMatchDam(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(damDir, "internal/theme/theme.go"))
	if err != nil {
		t.Skip("dam module not checked out next to dal")
	}
	re := regexp.MustCompile(`"(\w+)": \{0x([0-9a-f]{2}), 0x([0-9a-f]{2}), 0x([0-9a-f]{2}), 255\}`)
	n := 0
	for _, m := range re.FindAllStringSubmatch(string(src), -1) {
		n++
		var want [3]uint8
		for i := range want {
			v, _ := strconv.ParseUint(m[i+2], 16, 8)
			want[i] = uint8(v)
		}
		if got, ok := symbols.Colors[m[1]]; !ok || got != want {
			t.Errorf("%s: %v, dam has %v", m[1], got, want)
		}
	}
	if n != len(symbols.Colors) {
		t.Errorf("dam has %d colors, dal %d", n, len(symbols.Colors))
	}
}
