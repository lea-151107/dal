package box

import (
	"math"
	"testing"

	"github.com/lea-151107/dal/internal/mathfont"
)

type kernCall struct {
	c mathfont.Corner
	h float64
}

// stepKern is a MathKern with value lo below height at and hi from there
// up, recording the lookups.
func stepKern(calls *[]kernCall, at, lo, hi float64) func(mathfont.Corner, float64) float64 {
	return func(c mathfont.Corner, h float64) float64 {
		*calls = append(*calls, kernCall{c, h})
		if h < at {
			return lo
		}
		return hi
	}
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestCutIns(t *testing.T) {
	b := &Box{H: 500, D: 100}
	sup := &Box{H: 300, D: 40}
	sub := &Box{H: 280, D: 60}
	const u, v = 400.0, 150.0

	t.Run("base only, constant", func(t *testing.T) {
		var calls []kernCall
		k := func(c mathfont.Corner, h float64) float64 {
			calls = append(calls, kernCall{c, h})
			if c == mathfont.BottomRight {
				return -50
			}
			return 20
		}
		// Scale 0.5: heights go to font units and kerns back to layout
		// units.
		dSup, dSub := cutIns(glyphKern{k, 0.5}, b, u, v, sup, glyphKern{}, sub, glyphKern{})
		if !near(dSup, 10) || !near(dSub, -25) {
			t.Errorf("cut-ins %v %v, want 10 -25", dSup, dSub)
		}
		want := []kernCall{
			{mathfont.TopRight, (u - sup.D) / 0.5}, {mathfont.TopRight, b.H / 0.5},
			{mathfont.BottomRight, (sub.H - v) / 0.5}, {mathfont.BottomRight, -b.D / 0.5},
		}
		if len(calls) != len(want) {
			t.Fatalf("lookups %v, want %v", calls, want)
		}
		for i := range want {
			if calls[i].c != want[i].c || !near(calls[i].h, want[i].h) {
				t.Errorf("lookup %d: %v, want %v", i, calls[i], want[i])
			}
		}
	})

	t.Run("script corners at both heights", func(t *testing.T) {
		var bc, sc []kernCall
		base := glyphKern{stepKern(&bc, 450, -30, -80), 1}
		// The script is drawn at 0.7; its kern is -100 below its baseline
		// height 0 and -10 above it.
		script := glyphKern{stepKern(&sc, 0, -100, -10), 0.7}
		dSup, dSub := cutIns(base, b, u, v, sup, script, sub, script)
		// Superscript: at its bottom (base 360, script -40) -30 + -70 = -100;
		// at the base top (base 500, script 100) -80 + -7 = -87. The
		// maximum is -87.
		if !near(dSup, -87) {
			t.Errorf("superscript cut-in %v, want -87", dSup)
		}
		// Subscript: at its top (base 130, script 280) -30 + -7 = -37; at
		// the base bottom (base -100, script 50) -30 + -7 = -37.
		if !near(dSub, -37) {
			t.Errorf("subscript cut-in %v, want -37", dSub)
		}
		wantScript := []kernCall{
			{mathfont.BottomLeft, -sup.D / 0.7}, {mathfont.BottomLeft, (b.H - u) / 0.7},
			{mathfont.TopLeft, sub.H / 0.7}, {mathfont.TopLeft, (v - b.D) / 0.7},
		}
		for i := range wantScript {
			if i >= len(sc) || sc[i].c != wantScript[i].c || !near(sc[i].h, wantScript[i].h) {
				t.Errorf("script lookups %v, want %v", sc, wantScript)
				break
			}
		}
	})

	t.Run("no scripts or no kerns", func(t *testing.T) {
		if a, c := cutIns(glyphKern{}, b, 0, 0, nil, glyphKern{}, nil, glyphKern{}); a != 0 || c != 0 {
			t.Errorf("no scripts: %v %v", a, c)
		}
		if a, c := cutIns(glyphKern{}, b, u, v, sup, glyphKern{}, sub, glyphKern{}); a != 0 || c != 0 {
			t.Errorf("no kerns: %v %v", a, c)
		}
	})
}
