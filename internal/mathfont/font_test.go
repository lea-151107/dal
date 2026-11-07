package mathfont

import "testing"

func TestEmbedded(t *testing.T) {
	f, err := Load(nil)
	if err != nil {
		t.Fatal(err)
	}
	if f.UnitsPerEm() != 1000 {
		t.Errorf("unitsPerEm %v", f.UnitsPerEm())
	}
	c := f.Constants()
	// Values published in Latin Modern Math 1.959.
	for name, got := range map[string]float64{
		"AxisHeight":                      c.AxisHeight,
		"FractionRuleThickness":           c.FractionRuleThickness,
		"ScriptPercentScaleDown":          c.ScriptPercentScaleDown,
		"ScriptScriptPercentScaleDown":    c.ScriptScriptPercentScaleDown,
		"DisplayOperatorMinHeight":        c.DisplayOperatorMinHeight,
		"RadicalKernAfterDegree":          c.RadicalKernAfterDegree,
		"RadicalDegreeBottomRaisePercent": c.RadicalDegreeBottomRaisePercent,
	} {
		want := map[string]float64{
			"AxisHeight": 250, "FractionRuleThickness": 40, "ScriptPercentScaleDown": 70,
			"ScriptScriptPercentScaleDown": 50, "DisplayOperatorMinHeight": 1300,
			"RadicalKernAfterDegree": -556, "RadicalDegreeBottomRaisePercent": 60,
		}[name]
		if got != want {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
	if f.MinConnectorOverlap() != 20 {
		t.Errorf("MinConnectorOverlap %v", f.MinConnectorOverlap())
	}
	paren := f.Glyph('(')
	if v := f.Variants(paren, false); len(v) != 8 || v[0].Glyph != paren {
		t.Errorf("paren variants %v", v)
	}
	if parts, _ := f.Assembly(paren, false); len(parts) != 3 || !parts[1].Extender {
		t.Errorf("paren assembly %v", parts)
	}
	if parts, _ := f.Assembly(f.Glyph('→'), true); len(parts) != 3 {
		t.Errorf("arrow assembly %v", parts)
	}
	if it := f.Italic(f.Glyph('∫')); it <= 0 {
		t.Errorf("integral italic correction %v", it)
	}
	if _, ok := f.TopAccent(f.Glyph(0x1D453)); !ok {
		t.Error("no top accent attachment for italic f")
	}
	if f.Glyph(0x4E00) != 0 {
		t.Error("unexpected CJK glyph")
	}
}

func TestCoverage(t *testing.T) {
	f1 := data{0, 1, 0, 3, 0, 5, 0, 9, 0, 20}
	for g, want := range map[uint16]int{5: 0, 9: 1, 20: 2, 6: -1, 0: -1, 21: -1} {
		if got := coverage(f1, g); got != want {
			t.Errorf("format 1 glyph %d: %d, want %d", g, got, want)
		}
	}
	f2 := data{0, 2, 0, 2, 0, 10, 0, 12, 0, 0, 0, 20, 0, 21, 0, 3}
	for g, want := range map[uint16]int{10: 0, 12: 2, 20: 3, 21: 4, 13: -1, 9: -1} {
		if got := coverage(f2, g); got != want {
			t.Errorf("format 2 glyph %d: %d, want %d", g, got, want)
		}
	}
	if coverage(data{0, 1, 0xff, 0xff}, 3) != -1 {
		t.Error("truncated coverage")
	}
}

func TestBrokenFonts(t *testing.T) {
	for name, b := range map[string][]byte{
		"empty":     nil,
		"short":     []byte("OTTO"),
		"garbage":   []byte("this is not a font at all, really not"),
		"truncated": Embedded()[:5000],
	} {
		if name == "empty" {
			continue // nil selects the embedded font
		}
		if _, err := Load(b); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	start, end := MathTableRange(Embedded())
	if start <= 0 || end <= start {
		t.Fatalf("MATH range %d-%d", start, end)
	}
}

func TestScriptGlyph(t *testing.T) {
	f, err := Load(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range []rune{'x', '2', '′', 'α'} {
		g := f.Glyph(r)
		s1, s2 := f.ScriptGlyph(g, 1), f.ScriptGlyph(g, 2)
		if s1 == g || s2 == g || s1 == s2 || s1 == 0 || s2 == 0 {
			t.Errorf("%c: glyph %d, script forms %d and %d", r, g, s1, s2)
		}
	}
	// Glyphs without a script form, and level 0, keep the glyph.
	if g := f.Glyph('+'); f.ScriptGlyph(g, 1) != g || f.ScriptGlyph(f.Glyph('x'), 0) != f.Glyph('x') {
		t.Error("script form for + or at level 0")
	}
	// A font whose GSUB is garbage still loads, without script forms.
	b := append([]byte(nil), latinModernMath...)
	if off, n, err := tableRange(b, "GSUB"); err == nil {
		for i := off; i < off+n; i++ {
			b[i] = 0xff
		}
		if f2, err := Load(b); err != nil {
			t.Errorf("font with a broken GSUB: %v", err)
		} else if g := f2.Glyph('x'); f2.ScriptGlyph(g, 1) != g {
			t.Error("script form from a broken GSUB")
		}
	} else {
		t.Error("embedded font has no GSUB")
	}
}
