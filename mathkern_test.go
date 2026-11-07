package dal

import (
	"context"
	"encoding/binary"
	"maps"
	"math"
	"slices"
	"testing"

	"github.com/lea/dal/internal/mathfont"
)

// corners holds a constant kern for each corner of a glyph, indexed by
// mathfont.Corner; 0 leaves that corner without a MathKern table.
type corners [4]int16

// withMathKern returns a copy of the embedded font whose MATH table has a
// MathKernInfo (Latin Modern Math has none) giving each glyph in kerns the
// corner kerns, each constant over all heights.
func withMathKern(t *testing.T, kerns map[uint16]corners) []byte {
	t.Helper()
	file := append([]byte(nil), mathfont.Embedded()...)
	be := binary.BigEndian
	n := int(be.Uint16(file[4:]))
	rec := -1
	for i := 0; i < n; i++ {
		if string(file[12+16*i:12+16*i+4]) == "MATH" {
			rec = 12 + 16*i
		}
	}
	if rec < 0 {
		t.Fatal("no MATH table")
	}
	off, length := int(be.Uint32(file[rec+8:])), int(be.Uint32(file[rec+12:]))
	mt := append([]byte(nil), file[off:off+length]...)
	for len(mt)%4 != 0 {
		mt = append(mt, 0)
	}
	glyphInfo := int(be.Uint16(mt[6:]))
	if be.Uint16(mt[glyphInfo+6:]) != 0 {
		t.Fatal("font already has a MathKernInfo")
	}
	kernInfo := len(mt)
	gids := slices.Sorted(maps.Keys(kerns)) // coverage format 1 is sorted
	// MathKernInfo: coverage offset, count, one MathKernInfoRecord (offsets
	// of the top-right, top-left, bottom-right and bottom-left MathKern
	// tables) per glyph, the coverage (format 1), then MathKern tables
	// with no heights and one value each.
	recs := 4
	cov := recs + 8*len(gids)
	tables := cov + 4 + 2*len(gids)
	blob := make([]byte, tables)
	be.PutUint16(blob[0:], uint16(cov))
	be.PutUint16(blob[2:], uint16(len(gids)))
	be.PutUint16(blob[cov:], 1)
	be.PutUint16(blob[cov+2:], uint16(len(gids)))
	for i, g := range gids {
		be.PutUint16(blob[cov+4+2*i:], g)
		for c, k := range kerns[g] {
			if k == 0 {
				continue
			}
			be.PutUint16(blob[recs+8*i+2*c:], uint16(len(blob)))
			table := make([]byte, 6) // heightCount 0, value, device offset 0
			be.PutUint16(table[2:], uint16(k))
			blob = append(blob, table...)
		}
	}
	mt = append(mt, blob...)
	be.PutUint16(mt[glyphInfo+6:], uint16(kernInfo-glyphInfo))
	for len(file)%4 != 0 {
		file = append(file, 0)
	}
	be.PutUint32(file[rec+8:], uint32(len(file)))
	be.PutUint32(file[rec+12:], uint32(len(mt)))
	return append(file, mt...)
}

// TestMathKern renders with fonts that have MathKernInfo and checks that
// the scripts move by the kerns of both the base and the script: the
// subscript of V tucks under it and the superscript moves out.
func TestMathKern(t *testing.T) {
	f, err := mathfont.Load(nil)
	if err != nil {
		t.Fatal(err)
	}
	v := f.Glyph('𝑉') // the italic V that \(V\) is drawn with
	// The digits of scripts are drawn with their ssty forms, if any.
	scriptGlyphs := func(r rune) []uint16 {
		g := f.Glyph(r)
		return []uint16{g, f.ScriptGlyph(g, 1)}
	}
	baseKerned := withMathKern(t, map[uint16]corners{v: {mathfont.TopRight: 50, mathfont.BottomRight: -100}})
	scriptKern := map[uint16]corners{}
	for _, g := range scriptGlyphs('1') {
		scriptKern[g] = corners{mathfont.TopLeft: -100}
	}
	for _, g := range scriptGlyphs('2') {
		scriptKern[g] = corners{mathfont.BottomLeft: -100}
	}
	scriptKerned := withMathKern(t, scriptKern)
	for _, font := range [][]byte{baseKerned, scriptKerned} {
		if _, err := mathfont.Load(font); err != nil {
			t.Fatalf("patched font: %v", err)
		}
	}
	// Scripts are drawn at ScriptPercentScaleDown of the text size.
	scriptScale := f.Constants().ScriptPercentScaleDown / 100
	const size = 100.0 // px; 1000 font units per em, so 1 unit = 0.1 px
	for _, c := range []struct {
		name string
		src  string
		font []byte
		want float64 // change of the width in px
	}{
		{"base", `V_1`, baseKerned, -10},
		{"base", `V^2`, baseKerned, 5},
		{"base", `x_1`, baseKerned, 0}, // other glyphs have no kern
		{"script", `x_1`, scriptKerned, -10 * scriptScale},
		{"script", `x^2`, scriptKerned, -10 * scriptScale},
		{"script", `x_{11}`, scriptKerned, 0}, // only a single-glyph script has a kern
		{"script", `x_a`, scriptKerned, 0},
	} {
		t.Run(c.name+" "+c.src, func(t *testing.T) {
			plain, err := Render(context.Background(), c.src, Options{SizePx: size, Font: mathfont.Embedded()})
			if err != nil {
				t.Fatal(err)
			}
			withKern, err := Render(context.Background(), c.src, Options{SizePx: size, Font: c.font})
			if err != nil {
				t.Fatal(err)
			}
			if d := withKern.Width - plain.Width; math.Abs(d-c.want) > 0.01 {
				t.Errorf("width changed by %.3f px, want %.3f", d, c.want)
			}
		})
	}
}
