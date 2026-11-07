package mathfont

import (
	"errors"
	"fmt"
)

// ErrNoMath reports that a font has no usable MATH table.
var ErrNoMath = errors.New("font has no MATH table")

// data is a bounds-checked view of a byte slice. Reads past the end return 0
// instead of panicking: fonts may come from Options.Font, so every offset
// is untrusted. Callers treat 0 offsets as "absent", which is what the
// OpenType spec does too.
type data []byte

func (d data) u16(off int) uint16 {
	if off < 0 || off+2 > len(d) {
		return 0
	}
	return uint16(d[off])<<8 | uint16(d[off+1])
}

func (d data) i16(off int) int16 { return int16(d.u16(off)) }

func (d data) u32(off int) uint32 {
	if off < 0 || off+4 > len(d) {
		return 0
	}
	return uint32(d[off])<<24 | uint32(d[off+1])<<16 | uint32(d[off+2])<<8 | uint32(d[off+3])
}

// sub returns the view starting at off, or nil when off is 0 or out of range.
func (d data) sub(off int) data {
	if off <= 0 || off >= len(d) {
		return nil
	}
	return d[off:]
}

// findTable returns the bytes of the table with the given tag from an sfnt
// (not a collection) font file.
func findTable(file []byte, tag string) (data, error) {
	off, n, err := tableRange(file, tag)
	if err != nil {
		return nil, err
	}
	return data(file[off : off+n]), nil
}

func tableRange(file []byte, tag string) (off, length int, err error) {
	d := data(file)
	if len(d) < 12 {
		return 0, 0, errors.New("font file too short")
	}
	n := int(d.u16(4))
	for i := 0; i < n; i++ {
		rec := 12 + 16*i
		if rec+16 > len(d) {
			break
		}
		if string(d[rec:rec+4]) != tag {
			continue
		}
		o, l := int64(d.u32(rec+8)), int64(d.u32(rec+12))
		if o+l > int64(len(d)) || l == 0 {
			return 0, 0, fmt.Errorf("%s table out of range", tag)
		}
		return int(o), int(l), nil
	}
	return 0, 0, ErrNoMath
}

// coverage looks up glyph g in an OpenType Coverage table and returns its
// coverage index, or -1.
func coverage(c data, g uint16) int {
	switch c.u16(0) {
	case 1:
		n := int(c.u16(2))
		lo, hi := 0, n
		for lo < hi {
			m := (lo + hi) / 2
			v := c.u16(4 + 2*m)
			switch {
			case v == g:
				return m
			case v < g:
				lo = m + 1
			default:
				hi = m
			}
		}
	case 2:
		n := int(c.u16(2))
		lo, hi := 0, n
		for lo < hi {
			m := (lo + hi) / 2
			rec := 4 + 6*m
			start, end := c.u16(rec), c.u16(rec+2)
			switch {
			case g < start:
				hi = m
			case g > end:
				lo = m + 1
			default:
				return int(c.u16(rec+4)) + int(g-start)
			}
		}
	}
	return -1
}

// Constants holds the MathConstants table in font units (percentages are
// kept as given). The field order matches the table.
type Constants struct {
	ScriptPercentScaleDown                   float64
	ScriptScriptPercentScaleDown             float64
	DelimitedSubFormulaMinHeight             float64
	DisplayOperatorMinHeight                 float64
	MathLeading                              float64
	AxisHeight                               float64
	AccentBaseHeight                         float64
	FlattenedAccentBaseHeight                float64
	SubscriptShiftDown                       float64
	SubscriptTopMax                          float64
	SubscriptBaselineDropMin                 float64
	SuperscriptShiftUp                       float64
	SuperscriptShiftUpCramped                float64
	SuperscriptBottomMin                     float64
	SuperscriptBaselineDropMax               float64
	SubSuperscriptGapMin                     float64
	SuperscriptBottomMaxWithSubscript        float64
	SpaceAfterScript                         float64
	UpperLimitGapMin                         float64
	UpperLimitBaselineRiseMin                float64
	LowerLimitGapMin                         float64
	LowerLimitBaselineDropMin                float64
	StackTopShiftUp                          float64
	StackTopDisplayStyleShiftUp              float64
	StackBottomShiftDown                     float64
	StackBottomDisplayStyleShiftDown         float64
	StackGapMin                              float64
	StackDisplayStyleGapMin                  float64
	StretchStackTopShiftUp                   float64
	StretchStackBottomShiftDown              float64
	StretchStackGapAboveMin                  float64
	StretchStackGapBelowMin                  float64
	FractionNumeratorShiftUp                 float64
	FractionNumeratorDisplayStyleShiftUp     float64
	FractionDenominatorShiftDown             float64
	FractionDenominatorDisplayStyleShiftDown float64
	FractionNumeratorGapMin                  float64
	FractionNumDisplayStyleGapMin            float64
	FractionRuleThickness                    float64
	FractionDenominatorGapMin                float64
	FractionDenomDisplayStyleGapMin          float64
	SkewedFractionHorizontalGap              float64
	SkewedFractionVerticalGap                float64
	OverbarVerticalGap                       float64
	OverbarRuleThickness                     float64
	OverbarExtraAscender                     float64
	UnderbarVerticalGap                      float64
	UnderbarRuleThickness                    float64
	UnderbarExtraDescender                   float64
	RadicalVerticalGap                       float64
	RadicalDisplayStyleVerticalGap           float64
	RadicalRuleThickness                     float64
	RadicalExtraAscender                     float64
	RadicalKernBeforeDegree                  float64
	RadicalKernAfterDegree                   float64
	RadicalDegreeBottomRaisePercent          float64
}

func (c *Constants) fields() []*float64 {
	return []*float64{
		&c.ScriptPercentScaleDown, &c.ScriptScriptPercentScaleDown,
		&c.DelimitedSubFormulaMinHeight, &c.DisplayOperatorMinHeight,
		&c.MathLeading, &c.AxisHeight, &c.AccentBaseHeight, &c.FlattenedAccentBaseHeight,
		&c.SubscriptShiftDown, &c.SubscriptTopMax, &c.SubscriptBaselineDropMin,
		&c.SuperscriptShiftUp, &c.SuperscriptShiftUpCramped, &c.SuperscriptBottomMin,
		&c.SuperscriptBaselineDropMax, &c.SubSuperscriptGapMin, &c.SuperscriptBottomMaxWithSubscript,
		&c.SpaceAfterScript, &c.UpperLimitGapMin, &c.UpperLimitBaselineRiseMin,
		&c.LowerLimitGapMin, &c.LowerLimitBaselineDropMin,
		&c.StackTopShiftUp, &c.StackTopDisplayStyleShiftUp, &c.StackBottomShiftDown,
		&c.StackBottomDisplayStyleShiftDown, &c.StackGapMin, &c.StackDisplayStyleGapMin,
		&c.StretchStackTopShiftUp, &c.StretchStackBottomShiftDown,
		&c.StretchStackGapAboveMin, &c.StretchStackGapBelowMin,
		&c.FractionNumeratorShiftUp, &c.FractionNumeratorDisplayStyleShiftUp,
		&c.FractionDenominatorShiftDown, &c.FractionDenominatorDisplayStyleShiftDown,
		&c.FractionNumeratorGapMin, &c.FractionNumDisplayStyleGapMin, &c.FractionRuleThickness,
		&c.FractionDenominatorGapMin, &c.FractionDenomDisplayStyleGapMin,
		&c.SkewedFractionHorizontalGap, &c.SkewedFractionVerticalGap,
		&c.OverbarVerticalGap, &c.OverbarRuleThickness, &c.OverbarExtraAscender,
		&c.UnderbarVerticalGap, &c.UnderbarRuleThickness, &c.UnderbarExtraDescender,
		&c.RadicalVerticalGap, &c.RadicalDisplayStyleVerticalGap, &c.RadicalRuleThickness,
		&c.RadicalExtraAscender, &c.RadicalKernBeforeDegree, &c.RadicalKernAfterDegree,
		&c.RadicalDegreeBottomRaisePercent,
	}
}

// Size of the MathConstants table: 4 int16/uint16 values, 51 MathValueRecords
// and a trailing int16.
const constantsSize = 4*2 + 51*4 + 2

func parseConstants(d data) (Constants, error) {
	var c Constants
	if len(d) < constantsSize {
		return c, errors.New("MathConstants table too short")
	}
	f := c.fields()
	for i := 0; i < 4; i++ {
		*f[i] = float64(d.i16(2 * i))
	}
	for i := 0; i < 51; i++ {
		*f[4+i] = float64(d.i16(8 + 4*i)) // device table offsets are ignored
	}
	*f[55] = float64(d.i16(8 + 51*4))
	if c.ScriptPercentScaleDown <= 0 || c.ScriptPercentScaleDown > 100 {
		c.ScriptPercentScaleDown = 70
	}
	if c.ScriptScriptPercentScaleDown <= 0 || c.ScriptScriptPercentScaleDown > 100 {
		c.ScriptScriptPercentScaleDown = 50
	}
	return c, nil
}

// Variant is one size variant of a stretchy glyph.
type Variant struct {
	Glyph   uint16
	Advance float64 // height for vertical, width for horizontal variants
}

// Part is one part of a glyph assembly.
type Part struct {
	Glyph       uint16
	StartConn   float64
	EndConn     float64
	FullAdvance float64
	Extender    bool
}

// Corner selects one of the four MathKern tables of a glyph.
type Corner int

// Corners of a glyph for MathKern lookups.
const (
	TopRight Corner = iota
	TopLeft
	BottomRight
	BottomLeft
)

// mathTable is a validated view of a MATH table. Lookups read the bytes
// directly; nothing is decoded ahead of time.
type mathTable struct {
	consts       Constants
	italics      data // MathItalicsCorrectionInfo
	topAccent    data // MathTopAccentAttachment
	extended     data // ExtendedShapeCoverage
	kernInfo     data // MathKernInfo
	variants     data // MathVariants
	minConnector float64
}

func parseMath(file []byte) (*mathTable, error) {
	d, err := findTable(file, "MATH")
	if err != nil {
		return nil, err
	}
	if d.u16(0) != 1 {
		return nil, fmt.Errorf("unsupported MATH table version %d", d.u16(0))
	}
	m := &mathTable{}
	if m.consts, err = parseConstants(d.sub(int(d.u16(4)))); err != nil {
		return nil, err
	}
	if gi := d.sub(int(d.u16(6))); gi != nil {
		m.italics = gi.sub(int(gi.u16(0)))
		m.topAccent = gi.sub(int(gi.u16(2)))
		m.extended = gi.sub(int(gi.u16(4)))
		m.kernInfo = gi.sub(int(gi.u16(6)))
	}
	m.variants = d.sub(int(d.u16(8)))
	m.minConnector = float64(m.variants.u16(0))
	return m, nil
}

// valueRecordLookup reads a table of the form {coverageOffset, count,
// MathValueRecord[count]}.
func valueRecordLookup(t data, g uint16) (float64, bool) {
	if t == nil {
		return 0, false
	}
	i := coverage(t.sub(int(t.u16(0))), g)
	if i < 0 || i >= int(t.u16(2)) {
		return 0, false
	}
	return float64(t.i16(4 + 4*i)), true
}

func (m *mathTable) italic(g uint16) float64 {
	v, _ := valueRecordLookup(m.italics, g)
	return v
}

func (m *mathTable) accent(g uint16) (float64, bool) { return valueRecordLookup(m.topAccent, g) }

func (m *mathTable) isExtended(g uint16) bool {
	return m.extended != nil && coverage(m.extended, g) >= 0
}

// kern returns the MathKern value for glyph g at the given corner and height
// (font units).
func (m *mathTable) kern(g uint16, c Corner, height float64) float64 {
	t := m.kernInfo
	if t == nil {
		return 0
	}
	i := coverage(t.sub(int(t.u16(0))), g)
	if i < 0 || i >= int(t.u16(2)) {
		return 0
	}
	k := t.sub(int(t.u16(4 + 8*i + 2*int(c))))
	if k == nil {
		return 0
	}
	n := int(k.u16(0))
	// Heights are sorted; the kern value at index j applies below height j.
	j := 0
	for j < n && height >= float64(k.i16(2+4*j)) {
		j++
	}
	return float64(k.i16(2 + 4*n + 4*j))
}

// construction returns the MathGlyphConstruction table of g.
func (m *mathTable) construction(g uint16, horiz bool) data {
	v := m.variants
	if v == nil {
		return nil
	}
	covOff, count, base := int(v.u16(2)), int(v.u16(6)), 10
	if horiz {
		covOff, count, base = int(v.u16(4)), int(v.u16(8)), 10+2*int(v.u16(6))
	}
	i := coverage(v.sub(covOff), g)
	if i < 0 || i >= count {
		return nil
	}
	return v.sub(int(v.u16(base + 2*i)))
}

// Real fonts have a few dozen variants and a handful of assembly parts at
// most; these bounds keep a hostile font from making each lookup costly.
const (
	maxVariants = 256
	maxParts    = 64
)

func (m *mathTable) variantList(g uint16, horiz bool) []Variant {
	c := m.construction(g, horiz)
	if c == nil {
		return nil
	}
	n := min(int(c.u16(2)), maxVariants)
	if 4+4*n > len(c) {
		return nil
	}
	out := make([]Variant, n)
	for i := range out {
		out[i] = Variant{Glyph: c.u16(4 + 4*i), Advance: float64(c.u16(6 + 4*i))}
	}
	return out
}

func (m *mathTable) assembly(g uint16, horiz bool) (parts []Part, italic float64) {
	c := m.construction(g, horiz)
	if c == nil {
		return nil, 0
	}
	a := c.sub(int(c.u16(0)))
	if a == nil {
		return nil, 0
	}
	n := int(a.u16(4))
	if 6+10*n > len(a) || n > maxParts {
		return nil, 0
	}
	parts = make([]Part, n)
	for i := range parts {
		p := 6 + 10*i
		parts[i] = Part{
			Glyph:       a.u16(p),
			StartConn:   float64(a.u16(p + 2)),
			EndConn:     float64(a.u16(p + 4)),
			FullAdvance: float64(a.u16(p + 6)),
			Extender:    a.u16(p+8)&1 != 0,
		}
	}
	return parts, float64(a.i16(0))
}
