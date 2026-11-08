package box

import (
	"context"
	"testing"

	"github.com/lea-151107/dal/internal/guard"
	"github.com/lea-151107/dal/internal/mathfont"
)

func newTestLayouter(t *testing.T, f *mathfont.Font) *layouter {
	t.Helper()
	env := &Env{F: f, G: guard.New(context.Background(), 1<<10), SizePx: 20}
	return &layouter{Env: env, c: f.Constants(), em: f.UnitsPerEm(), sizeFactor: 1}
}

// TestStretchVertReachesTarget checks the invariant that a delimiter with a
// glyph assembly is stretched to at least the requested total height. The
// floor and ceiling brackets of Latin Modern Math have one fixed part and
// one extender, so an assembly with no extender is a single part shorter
// than the largest size variant.
func TestStretchVertReachesTarget(t *testing.T) {
	f, err := mathfont.Load(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range "⌈⌉⌊⌋([|{‖⟨" {
		for _, target := range []float64{2000, 5000, 12000} {
			for _, s := range []float64{1, 0.7} {
				l := newTestLayouter(t, f)
				if g := f.Glyph(r); g == 0 {
					t.Fatalf("%q: no glyph", r)
				} else if parts, _ := f.Assembly(g, false); len(parts) == 0 {
					continue // no assembly: the largest variant is the best there is
				}
				b, err := l.stretchVert(r, target*s, s, 0)
				if err != nil {
					t.Fatalf("%q target %v scale %v: %v", r, target, s, err)
				}
				if got := b.H + b.D; got < target*s-1e-6 {
					t.Errorf("%q target %v scale %v: total height %v", r, target*s, s, got)
				}
			}
		}
	}
}
