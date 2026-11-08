package textgrid_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lea-151107/dal/internal/diag"
	"github.com/lea-151107/dal/internal/guard"
	"github.com/lea-151107/dal/internal/parse"
	"github.com/lea-151107/dal/internal/textgrid"
)

var lim = parse.Limits{MaxInputBytes: 16 << 10, MaxNodes: 5000, MaxDepth: 64, MaxExpansions: 1000}

func render(t *testing.T, src string, display bool) ([]string, int, error) {
	t.Helper()
	e, err := parse.Parse(src, lim)
	if err != nil {
		t.Fatalf("%s: %v", src, err)
	}
	return textgrid.Render(guard.New(context.Background(), lim.MaxNodes), e, display, 0, false)
}

func TestRender(t *testing.T) {
	for _, c := range []struct {
		src     string
		display bool
		want    string // lines joined by "|"
		base    int
	}{
		{`x^2`, false, `x²`, 0},
		{`\frac{a}{b}`, true, ` a|───| b`, 1},
		{`\frac{a+b}{c}`, false, ` a+b|─────|  c`, 1},
		{`\sum_{i=1}^n i`, true, ` n| ∑  i|i=1`, 1},
		{`\sqrt{x}`, false, `√x`, 0},
		{`x_{\frac12}`, false, `x|  1| ───|  2`, 0},
	} {
		lines, base, err := render(t, c.src, c.display)
		if err != nil {
			t.Errorf("%s: %v", c.src, err)
			continue
		}
		if got := strings.Join(lines, "|"); got != c.want || base != c.base {
			t.Errorf("%s:\n got %q base %d\nwant %q base %d", c.src, got, base, c.want, c.base)
		}
	}
}

func TestUnsupported(t *testing.T) {
	_, _, err := render(t, `\not a`, false)
	var u *diag.UnsupportedError
	if !errors.As(err, &u) {
		t.Errorf("\\not a: %v, want UnsupportedError", err)
	}
	if lines, _, err := render(t, `\sout{ab}`, false); err != nil || lines[0] != "sout(ab)" {
		t.Errorf("\\sout: %q, %v", lines, err)
	}
}

func TestWidth(t *testing.T) {
	for _, c := range []struct {
		s string
		w int
	}{{"abc", 3}, {"日本", 4}, {"é", 1}, {"e\u0301", 1}, {"\u200d", 0}, {"─│√∑", 4}} {
		if got := textgrid.StringWidth(c.s); got != c.w {
			t.Errorf("StringWidth(%q) = %d, want %d", c.s, got, c.w)
		}
	}
}
