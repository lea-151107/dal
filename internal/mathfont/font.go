// Package mathfont loads an OpenType math font (the embedded Latin Modern
// Math by default) and exposes its glyph metrics, outlines and MATH table.
//
// A Font is created per render call and is not safe for concurrent use: it
// owns an sfnt.Buffer and a metrics cache.
package mathfont

import (
	_ "embed"
	"errors"
	"fmt"

	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// LatinModernMath is the embedded font (GUST Font License, see
// GUST-FONT-LICENSE.txt in this directory).
//
//go:embed latinmodern-math.otf
var latinModernMath []byte

// Metrics are the dimensions of a glyph in font units, y up.
type Metrics struct {
	Advance float64
	Height  float64 // top of the ink above the baseline
	Depth   float64 // bottom of the ink below the baseline
	MinX    float64 // left edge of the ink
	MaxX    float64 // right edge of the ink
}

// Font is a parsed math font.
type Font struct {
	sf         *sfnt.Font
	buf        sfnt.Buffer
	m          *mathTable
	unitsPerEm float64
	ppem       fixed.Int26_6
	metrics    map[uint16]Metrics
	gids       map[rune]uint16
	ssty       []sstySubtable
}

// Load parses a font. A nil or empty src selects the embedded font.
func Load(src []byte) (*Font, error) {
	if len(src) == 0 {
		src = latinModernMath
	}
	sf, err := sfnt.Parse(src)
	if err != nil {
		return nil, fmt.Errorf("parse font: %w", err)
	}
	m, err := parseMath(src)
	if err != nil {
		return nil, err
	}
	upem := float64(sf.UnitsPerEm())
	if upem <= 0 {
		return nil, errors.New("font has invalid unitsPerEm")
	}
	return &Font{
		sf: sf, m: m, unitsPerEm: upem,
		// Requesting ppem == unitsPerEm makes sfnt return font units (in 26.6).
		ppem:    fixed.Int26_6(sf.UnitsPerEm()) << 6,
		metrics: map[uint16]Metrics{},
		gids:    map[rune]uint16{},
		ssty:    parseSsty(src),
	}, nil
}

// ScriptGlyph returns the form of g designed for script style (level 1)
// or scriptscript style (level 2), from the font's ssty feature; g when
// there is none.
func (f *Font) ScriptGlyph(g uint16, level int) uint16 {
	if g == 0 || level < 1 {
		return g
	}
	if s := sstyGlyph(f.ssty, g, level); s != 0 {
		return s
	}
	return g
}

// UnitsPerEm returns the em size in font units.
func (f *Font) UnitsPerEm() float64 { return f.unitsPerEm }

// Constants returns the MathConstants table.
func (f *Font) Constants() *Constants { return &f.m.consts }

// MinConnectorOverlap is the minimal overlap of assembly parts.
func (f *Font) MinConnectorOverlap() float64 { return f.m.minConnector }

// Glyph returns the glyph for r, or 0 when the font has none.
func (f *Font) Glyph(r rune) uint16 {
	if g, ok := f.gids[r]; ok {
		return g
	}
	gi, err := f.sf.GlyphIndex(&f.buf, r)
	if err != nil {
		gi = 0
	}
	f.gids[r] = uint16(gi)
	return uint16(gi)
}

// Metrics returns the dimensions of glyph g.
func (f *Font) Metrics(g uint16) Metrics {
	if m, ok := f.metrics[g]; ok {
		return m
	}
	var m Metrics
	b, adv, err := f.sf.GlyphBounds(&f.buf, sfnt.GlyphIndex(g), f.ppem, font.HintingNone)
	if err == nil {
		m = Metrics{
			Advance: float64(adv) / 64,
			Height:  -float64(b.Min.Y) / 64,
			Depth:   float64(b.Max.Y) / 64,
			MinX:    float64(b.Min.X) / 64,
			MaxX:    float64(b.Max.X) / 64,
		}
		if b.Empty() {
			m.Height, m.Depth, m.MinX, m.MaxX = 0, 0, 0, 0
		}
	}
	f.metrics[g] = m
	return m
}

// Italic returns the italic correction of g.
func (f *Font) Italic(g uint16) float64 { return f.m.italic(g) }

// TopAccent returns the top accent attachment x of g.
func (f *Font) TopAccent(g uint16) (float64, bool) { return f.m.accent(g) }

// IsExtendedShape reports whether g is an extended shape (for scripts).
func (f *Font) IsExtendedShape(g uint16) bool { return f.m.isExtended(g) }

// Kern returns the MathKern value of g at the corner and height.
func (f *Font) Kern(g uint16, c Corner, height float64) float64 { return f.m.kern(g, c, height) }

// Variants returns the size variants of g (the first is usually g itself).
func (f *Font) Variants(g uint16, horiz bool) []Variant { return f.m.variantList(g, horiz) }

// Assembly returns the parts for building g at arbitrary size, bottom to top
// (vertical) or left to right (horizontal).
func (f *Font) Assembly(g uint16, horiz bool) ([]Part, float64) { return f.m.assembly(g, horiz) }

// Outline returns the outline of g in font units, y down. The returned slice
// is only valid until the next call.
func (f *Font) Outline(g uint16) (sfnt.Segments, error) {
	return f.sf.LoadGlyph(&f.buf, sfnt.GlyphIndex(g), f.ppem, nil)
}

// Embedded returns the embedded font file. The slice must not be modified.
func Embedded() []byte { return latinModernMath }

// MathTableRange returns the byte range of the MATH table in file, or 0, 0.
// It is used by tests that corrupt the table.
func MathTableRange(file []byte) (start, end int) {
	off, n, err := tableRange(file, "MATH")
	if err != nil {
		return 0, 0
	}
	return off, off + n
}
