package box

import (
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/lea/dal/internal/ast"
	"github.com/lea/dal/internal/diag"
	"github.com/lea/dal/internal/mathfont"
	"github.com/lea/dal/internal/symbols"
)

func (l *layouter) node(n ast.Node, st style) ([]atom, error) {
	switch n := n.(type) {
	case *ast.Symbol:
		return l.symbol(n, st)
	case *ast.Group:
		b, err := l.list(n.Body, st)
		if err != nil {
			return nil, err
		}
		return single(atom{class: n.Class, box: b}), nil
	case *ast.Scripts:
		return l.scripts(n, st)
	case *ast.Op:
		a, err := l.op(n, st)
		if err != nil {
			return nil, err
		}
		return single(a), nil
	case *ast.Frac:
		return l.frac(n, st)
	case *ast.Sqrt:
		return l.sqrt(n, st)
	case *ast.LeftRight:
		return l.leftRight(n, st)
	case *ast.Middle:
		return single(atom{class: ast.ClassOrd, middle: n.Delim, mid: true}), nil
	case *ast.SizedDelim:
		d, err := l.delim(n.Delim, float64(bigDelimEm[n.Size]*l.em*l.scale(st)), st)
		if err != nil {
			return nil, err
		}
		return single(atom{class: n.Class, box: d}), nil
	case *ast.Accent:
		return l.accent(n, st)
	case *ast.Overline:
		return l.overline(n, st)
	case *ast.Font:
		saved := l.font
		l.font = n.Style
		a, err := l.atoms(n.Body, st)
		l.font = saved
		return a, err
	case *ast.Text:
		b, err := l.text(n, st)
		if err != nil {
			return nil, err
		}
		return ord(b), nil
	case *ast.Space:
		return single(atom{space: true, box: kern(l.muToUnits(n.Mu, st))}), nil
	case *ast.Array:
		return l.array(n, st)
	case *ast.Color:
		a, err := l.atoms(n.Body, st)
		if err != nil {
			return nil, err
		}
		c := n.Color
		for i := range a {
			if a[i].box != nil {
				a[i].box = &Box{W: a[i].box.W, H: a[i].box.H, D: a[i].box.D, Kids: []Child{{0, 0, a[i].box}}, Color: &c}
			}
		}
		return a, nil
	case *ast.Style:
		return l.atoms(n.Body, styleOf(n.Style, st))
	case *ast.Phantom:
		b, err := l.list(n.Body, st)
		if err != nil {
			return nil, err
		}
		p := &Box{W: b.W, H: b.H, D: b.D}
		if n.NoWidth {
			p.W = 0
		}
		if n.NoHeight {
			p.H, p.D = 0, 0
		}
		return ord(p), nil
	case *ast.Smash:
		b, err := l.list(n.Body, st)
		if err != nil {
			return nil, err
		}
		out := &Box{W: b.W, H: b.H, D: b.D, Kids: []Child{{0, 0, b}}}
		if n.Top {
			out.H = 0
		}
		if n.Bottom {
			out.D = 0
		}
		return ord(out), nil
	case *ast.Lap:
		b, err := l.list(n.Body, st)
		if err != nil {
			return nil, err
		}
		x := 0.0
		switch n.Align {
		case ast.AlignRight:
			x = -b.W
		case ast.AlignCenter:
			x = -b.W / 2
		}
		return ord(&Box{H: b.H, D: b.D, Kids: []Child{{x, 0, b}}}), nil
	case *ast.Raise:
		b, err := l.list(n.Body, st)
		if err != nil {
			return nil, err
		}
		dy := l.muToUnits(n.Mu, st)
		if n.Center {
			dy = float64(l.c.AxisHeight*l.scale(st)) - (b.H-b.D)/2
		}
		return ord(shifted(b, dy)), nil
	case *ast.ColorBox:
		return l.colorBox(n, st)
	case *ast.Reflect:
		b, err := l.list(n.Body, st)
		if err != nil {
			return nil, err
		}
		return ord(&Box{W: b.W, H: b.H, D: b.D, Kids: []Child{{0, 0, b}}, Mirror: true}), nil
	case *ast.Rule:
		w, h, r := l.muToUnits(n.WMu, st), l.muToUnits(n.HMu, st), l.muToUnits(n.RaiseMu, st)
		if w <= 0 || h <= 0 {
			return ord(kern(max(w, 0))), nil
		}
		return ord(shifted(rule(w, h, 0), r)), nil
	case *ast.Size:
		saved := l.sizeFactor
		l.sizeFactor = n.Factor
		a, err := l.atoms(n.Body, st)
		l.sizeFactor = saved
		return a, err
	case *ast.Choice:
		return l.atoms(n.Bodies[st.level], st)
	case *ast.Logo:
		return l.atoms(logo(n), st)
	case *ast.Break:
		b := 1
		if !n.Allow {
			b = -1
		}
		return single(atom{space: true, brk: b, box: kern(0)}), nil
	case *ast.Enclose:
		return l.enclose(n, st)
	case *ast.Boxed:
		return l.boxed(n, st)
	case *ast.Cancel:
		return l.cancel(n, st)
	case *ast.Not:
		// NOTE(katex-compat): the slash spans the body's box diagonally
		// instead of using the font's \not glyph.
		return l.cancel(&ast.Cancel{Offset: n.Offset, Kind: "/", Body: n.Body}, st)
	case *ast.Brace:
		return l.brace(n, st)
	case *ast.OverUnder:
		return l.overUnder(n, st)
	case *ast.XArrow:
		return l.xarrow(n, st)
	}
	return nil, diag.Unsupported("node", n.Pos())
}

// glyphFor maps rune r through the current alphabet and returns its glyph.
func (l *layouter) glyphFor(r rune, letter bool) uint16 {
	fs := l.font
	if fs == ast.FontDefault && letter {
		fs = ast.FontItalic
	}
	if fs == ast.FontBold && letter && !(r < 0x80) {
		// NOTE(katex-compat): \mathbf keeps lowercase Greek italic like TeX.
		fs = ast.FontBoldItalic
	}
	if m := symbols.Alphabet(r, fs); m != r {
		if g := l.F.Glyph(m); g != 0 {
			return g
		}
	}
	return l.F.Glyph(r)
}

func (l *layouter) symbol(n *ast.Symbol, st style) ([]atom, error) {
	s := l.scale(st)
	if utf8.RuneCountInString(n.Text) == 1 {
		r, _ := utf8.DecodeRuneInString(n.Text)
		g := l.scriptForm(l.glyphFor(r, n.Letter), st)
		if g == 0 {
			b, err := l.missing(r, s, n.Offset)
			if err != nil {
				return nil, err
			}
			return single(atom{class: n.Class, box: b}), nil
		}
		return single(atom{class: n.Class, box: l.glyphBox(g, s), italic: float64(l.F.Italic(g) * s), char: true, glyph: g}), nil
	}
	b, err := l.wordStyle(n.Text, st, n.Offset)
	if err != nil {
		return nil, err
	}
	return single(atom{class: n.Class, box: b}), nil
}

// word sets upright text (operator names) with the math font.
// scriptForm returns the font's script style form of g (the ssty
// feature) in script and scriptscript style.
func (l *layouter) scriptForm(g uint16, st style) uint16 {
	if st.level >= 2 {
		return l.F.ScriptGlyph(g, int(st.level)-1)
	}
	return g
}

func (l *layouter) word(text string, s float64, off int) (*Box, error) {
	return l.wordGlyphs(text, s, off, func(g uint16) uint16 { return g })
}

// wordStyle is word in style st, with script forms in script styles.
func (l *layouter) wordStyle(text string, st style, off int) (*Box, error) {
	return l.wordGlyphs(text, l.scale(st), off, func(g uint16) uint16 { return l.scriptForm(g, st) })
}

func (l *layouter) wordGlyphs(text string, s float64, off int, form func(uint16) uint16) (*Box, error) {
	b := &Box{}
	x := 0.0
	for _, r := range text {
		if r == ' ' {
			x += float64(opNameSpaceEm * l.em * s)
			continue
		}
		saved := l.font
		if l.font == ast.FontDefault {
			l.font = ast.FontRoman
		}
		g := form(l.glyphFor(r, false))
		l.font = saved
		var gb *Box
		if g == 0 {
			var err error
			if gb, err = l.missing(r, s, off); err != nil {
				return nil, err
			}
		} else {
			gb = l.glyphBox(g, s)
		}
		b.add(x, 0, gb)
		x += gb.W
		b.H, b.D = max(b.H, gb.H), max(b.D, gb.D)
	}
	b.W = x
	return b, nil
}

// text sets \text{...}. Characters the math font has are set with it;
// runs of characters it lacks go to the caller's text face (Measure), so
// a Japanese face does not change how English words look. Spaces,
// combining marks and ZWJ stay with the run before them when its face has
// them, and otherwise use the math font.
func (l *layouter) text(n *ast.Text, st style) (*Box, error) {
	s := l.scale(st)
	fs := n.Style
	if fs == ast.FontRoman {
		fs = ast.FontDefault
	}
	b := &Box{}
	x := 0.0
	var pending strings.Builder // characters for the text face
	flush := func() error {
		if pending.Len() == 0 {
			return nil
		}
		run := pending.String()
		pending.Reset()
		adv, asc, desc, err := l.Measure(run, float64(l.SizePx*s))
		if err != nil {
			return err
		}
		k := l.em / l.SizePx // px to font units
		tb := &Box{W: float64(adv * k), H: float64(asc * k), D: float64(desc * k), Leaf: TextLeaf{Text: run, SizeEm: s}}
		b.add(x, 0, tb)
		x += tb.W
		b.H, b.D = max(b.H, tb.H), max(b.D, tb.D)
		return nil
	}
	for _, r := range n.Text {
		g := l.F.Glyph(symbols.Alphabet(r, fs))
		if g == 0 {
			g = l.F.Glyph(r)
		}
		g = l.scriptForm(g, st)
		if pending.Len() > 0 && attaches(r) {
			ok, err := l.faceHas(r, float64(l.SizePx*s))
			if err != nil {
				return nil, err
			}
			if ok {
				pending.WriteRune(r)
				continue
			}
			// Neither font has the mark: drawing it would show the text
			// face's replacement box (the path below fails for it too).
			if g == 0 && !isTextSpace(r) {
				return nil, diag.Unsupported(`\text with `+string(r)+` (not in the math font or the TextFace)`, n.Offset)
			}
		}
		if g == 0 && !isTextSpace(r) && pending.Len() == 0 {
			// A precomposed letter the font lacks (ǎ): its base letter and
			// combining mark, when the font has both.
			if base, mark, ok := symbols.Decompose(r); ok {
				bg, mg := l.F.Glyph(symbols.Alphabet(base, fs)), l.F.Glyph(mark)
				if bg == 0 {
					bg = l.F.Glyph(base)
				}
				if bg != 0 && mg != 0 && l.Measure == nil {
					for _, g := range []uint16{bg, mg} {
						gb := l.glyphBox(g, s)
						b.add(x, 0, gb)
						x += gb.W
						b.H, b.D = max(b.H, gb.H), max(b.D, gb.D)
					}
					continue
				}
			}
		}
		if g == 0 && !isTextSpace(r) {
			if l.Measure == nil {
				return nil, diag.Unsupported(`\text with `+string(r)+` (no TextFace given)`, n.Offset)
			}
			// Neither font has it: drawing it would show the text face's
			// replacement box, so fail and let the caller fall back.
			if ok, err := l.faceHas(r, float64(l.SizePx*s)); err != nil {
				return nil, err
			} else if !ok {
				return nil, diag.Unsupported(`\text with `+string(r)+` (not in the math font or the TextFace)`, n.Offset)
			}
			pending.WriteRune(r)
			continue
		}
		if err := flush(); err != nil {
			return nil, err
		}
		if g == 0 {
			x += float64(0.333 * l.em * s)
			continue
		}
		gb := l.glyphBox(g, s)
		b.add(x, 0, gb)
		x += gb.W
		b.H, b.D = max(b.H, gb.H), max(b.D, gb.D)
	}
	if err := flush(); err != nil {
		return nil, err
	}
	b.W = x
	return b, nil
}

// missing sets a character the math font lacks: with the same-looking
// characters of symbols.ImageSubstitutes, else with the caller's text
// face, else it fails so that the caller can fall back to text output.
func (l *layouter) missing(r rune, s float64, off int) (*Box, error) {
	if sub, ok := symbols.ImageSubstitutes[r]; ok {
		b := &Box{}
		for _, c := range sub {
			g := l.F.Glyph(c)
			if g == 0 {
				b = nil
				break
			}
			gb := l.glyphBox(g, s)
			b.add(b.W, 0, gb)
			b.W += gb.W
			b.H, b.D = max(b.H, gb.H), max(b.D, gb.D)
		}
		if b != nil {
			return b, nil
		}
	}
	if l.Measure != nil {
		ok, err := l.faceHas(r, float64(l.SizePx*s))
		if err != nil {
			return nil, err
		}
		if ok {
			adv, asc, desc, err := l.Measure(string(r), float64(l.SizePx*s))
			if err != nil {
				return nil, err
			}
			k := l.em / l.SizePx // px to font units
			return &Box{W: float64(adv * k), H: float64(asc * k), D: float64(desc * k), Leaf: TextLeaf{Text: string(r), SizeEm: s}}, nil
		}
	}
	return nil, diag.Unsupported("character "+string(r)+" (not in the math font; a TextFace that has it can draw it)", off)
}

func (l *layouter) faceHas(r rune, sizePx float64) (bool, error) {
	if l.Has == nil {
		return true, nil
	}
	return l.Has(r, sizePx)
}

func isTextSpace(r rune) bool { return r == ' ' || r == '\u00a0' }

// attaches reports whether r belongs to the run before it rather than
// choosing a font of its own.
func attaches(r rune) bool {
	return isTextSpace(r) || r == '\u200d' || unicode.In(r, unicode.Mn, unicode.Me)
}

// nucleus typesets the base of a Scripts node as one atom.
func (l *layouter) nucleus(n ast.Node, st style) (atom, error) {
	if n == nil {
		return atom{class: ast.ClassOrd, box: &Box{}}, nil
	}
	if op, ok := n.(*ast.Op); ok {
		return l.op(op, st)
	}
	as, err := l.node(n, st)
	if err != nil {
		return atom{}, err
	}
	if len(as) == 1 && !as[0].mid {
		return as[0], nil
	}
	// hpack also sizes a \middle delimiter, which has no box yet.
	b, err := l.hpack(as, st)
	if err != nil {
		return atom{}, err
	}
	return atom{class: ast.ClassOrd, box: b}, nil
}

func (l *layouter) op(n *ast.Op, st style) (atom, error) {
	s := l.scale(st)
	a := atom{class: ast.ClassOp}
	switch n.Limits {
	case ast.LimitsOn:
		a.limits = true
	case ast.LimitsAuto:
		a.limits = n.DefaultLimits && st.display()
	}
	if n.Body != nil {
		b, err := l.list(n.Body, st)
		if err != nil {
			return atom{}, err
		}
		a.box = b
		return a, nil
	}
	if !n.Symbol {
		b, err := l.wordStyle(n.Text, st, n.Offset)
		if err != nil {
			return atom{}, err
		}
		a.box = b
		return a, nil
	}
	r, _ := utf8.DecodeRuneInString(n.Text)
	g := l.F.Glyph(r)
	if g == 0 {
		return atom{}, diag.Unsupported("operator "+n.Text, n.Offset)
	}
	if st.display() {
		minH := l.c.DisplayOperatorMinHeight
		for _, v := range l.F.Variants(g, false) {
			g = v.Glyph
			if v.Advance >= minH {
				break
			}
		}
	}
	b := l.glyphBox(g, s)
	a.italic = float64(l.F.Italic(g) * s)
	// Center on the math axis (rule 13).
	m := l.F.Metrics(g)
	dy := float64(l.c.AxisHeight*s) - float64((m.Height-m.Depth)*s)/2
	gb := &Box{W: b.W, Leaf: b.Leaf}
	a.box = &Box{W: b.W, H: max(float64(m.Height*s)+dy, 0), D: max(float64(m.Depth*s)-dy, 0)}
	a.box.add(0, dy, gb)
	a.glyph = g
	return a, nil
}

func (l *layouter) scripts(n *ast.Scripts, st style) ([]atom, error) {
	base, err := l.nucleus(n.Base, st)
	if err != nil {
		return nil, err
	}
	// Primes are ordinary superscripts (f' is f^{\prime}): the font's
	// script form of ′ is drawn for that position.
	sup, hasSup := n.Sup, n.HasSup
	var supBox, subBox *Box
	var supGlyph, subGlyph uint16
	if hasSup {
		if supBox, supGlyph, err = l.script(sup, st.sup()); err != nil {
			return nil, err
		}
	}
	if n.HasSub {
		if subBox, subGlyph, err = l.script(n.Sub, st.sub()); err != nil {
			return nil, err
		}
	}
	if base.limits {
		return single(l.limits(base, supBox, subBox, st)), nil
	}
	return single(l.attach(base, supBox, subBox, supGlyph, subGlyph, st)), nil
}

// script is l.list for a superscript or subscript. It also returns the
// glyph of a script that is a single character (0 otherwise), whose
// MathKern corners take part in the cut-ins.
func (l *layouter) script(nodes []ast.Node, st style) (*Box, uint16, error) {
	atoms, err := l.atoms(nodes, st)
	if err != nil {
		return nil, 0, err
	}
	var g uint16
	if len(atoms) == 1 && atoms[0].char {
		g = atoms[0].glyph
	}
	b, err := l.hpack(atoms, st)
	if err != nil {
		return nil, 0, err
	}
	return b, g, nil
}

// attach places scripts to the right of the nucleus (rules 17 and 18).
// supGlyph and subGlyph are the glyphs of single-character scripts, 0 for
// others.
func (l *layouter) attach(base atom, sup, sub *Box, supGlyph, subGlyph uint16, st style) atom {
	s := l.scale(st)
	c := l.c
	b := base.box
	var u, v float64
	if !base.char {
		u = b.H - float64(c.SuperscriptBaselineDropMax*s)
		v = b.D + float64(c.SubscriptBaselineDropMin*s)
	}
	if sup != nil {
		shift := c.SuperscriptShiftUp
		if st.cramped {
			shift = c.SuperscriptShiftUpCramped
		}
		u = max(u, float64(shift*s), sup.D+float64(c.SuperscriptBottomMin*s))
	}
	if sub != nil && sup == nil {
		v = max(v, float64(c.SubscriptShiftDown*s), sub.H-float64(c.SubscriptTopMax*s))
	}
	if sub != nil && sup != nil {
		v = max(v, float64(c.SubscriptShiftDown*s))
		gap := (u - sup.D) - (sub.H - v)
		if minGap := float64(c.SubSuperscriptGapMin * s); gap < minGap {
			v += minGap - gap
			if psi := float64(c.SuperscriptBottomMaxWithSubscript*s) - (u - sup.D); psi > 0 {
				u += psi
				v -= psi
			}
		}
	}
	// TeX adds the italic correction to the nucleus unless there is a
	// subscript; then only the superscript moves right by it.
	out := &Box{H: b.H, D: b.D}
	out.add(0, 0, b)
	w := b.W
	supX, subX := b.W+base.italic, b.W
	if base.char && base.glyph != 0 && base.class != ast.ClassOp {
		dSup, dSub := cutIns(l.glyphKern(base.glyph, s), b, u, v,
			sup, l.glyphKern(supGlyph, l.scale(st.sup())),
			sub, l.glyphKern(subGlyph, l.scale(st.sub())))
		supX += dSup
		subX += dSub
	}
	if base.class == ast.ClassOp && base.glyph != 0 {
		// For big operators the italic correction is the horizontal offset
		// between the top and the bottom of the glyph: the subscript moves
		// left by it (OpenType MATH convention).
		supX, subX = b.W, b.W-base.italic
	}
	if sup != nil {
		out.add(supX, u, sup)
		w = max(w, supX+sup.W)
		out.H = max(out.H, u+sup.H)
	}
	if sub != nil {
		out.add(subX, -v, sub)
		w = max(w, subX+sub.W)
		out.D = max(out.D, v+sub.D)
	}
	out.W = w + float64(c.SpaceAfterScript*s)
	return atom{class: base.class, box: out}
}

// glyphKern is the MathKern of a glyph drawn at scale s; the zero value
// (glyph 0) has no kern.
type glyphKern struct {
	kern func(c mathfont.Corner, height float64) float64 // font units
	s    float64
}

func (l *layouter) glyphKern(g uint16, s float64) glyphKern {
	if g == 0 {
		return glyphKern{}
	}
	return glyphKern{kern: func(c mathfont.Corner, h float64) float64 { return l.F.Kern(g, c, h) }, s: s}
}

// at returns the kern at corner c and height h above the glyph's baseline,
// both in layout units.
func (k glyphKern) at(c mathfont.Corner, h float64) float64 {
	if k.kern == nil || k.s <= 0 {
		return 0
	}
	return float64(k.kern(c, h/k.s) * k.s)
}

// cutIns returns the MathKern cut-ins (OpenType MATH, MathKernInfo) of a
// single-glyph base b with a superscript raised by u and a subscript
// lowered by v, so that V₁ tucks the subscript under the V. The base's
// top-right corner pairs with the superscript's bottom-left one, and the
// base's bottom-right corner with the subscript's top-left one. Each pair
// is summed at two heights: the near edge of the script and the near edge
// of the base, each measured from the glyph's own baseline. A script that
// is not a single glyph has no kern of its own. Fonts without
// MathKernInfo (Latin Modern Math) give 0.
//
// NOTE: the OpenType specification says to take the minimum of the two
// sums, but Microsoft's math handler and XeTeX take the maximum (the
// smaller cut-in for the usual negative kerns), which keeps the glyphs
// from clashing; see MicrosoftDocs/typography-issues#1147.
func cutIns(base glyphKern, b *Box, u, v float64, sup *Box, supK glyphKern, sub *Box, subK glyphKern) (dSup, dSub float64) {
	if sup != nil {
		atScriptBottom := base.at(mathfont.TopRight, u-sup.D) + supK.at(mathfont.BottomLeft, -sup.D)
		atBaseTop := base.at(mathfont.TopRight, b.H) + supK.at(mathfont.BottomLeft, b.H-u)
		dSup = max(atScriptBottom, atBaseTop)
	}
	if sub != nil {
		atScriptTop := base.at(mathfont.BottomRight, sub.H-v) + subK.at(mathfont.TopLeft, sub.H)
		atBaseBottom := base.at(mathfont.BottomRight, -b.D) + subK.at(mathfont.TopLeft, v-b.D)
		dSub = max(atScriptTop, atBaseBottom)
	}
	return dSup, dSub
}

// limits places scripts above and below the nucleus (rule 13a).
func (l *layouter) limits(base atom, sup, sub *Box, st style) atom {
	s := l.scale(st)
	c := l.c
	b := base.box
	w := b.W
	if sup != nil {
		w = max(w, sup.W)
	}
	if sub != nil {
		w = max(w, sub.W)
	}
	out := &Box{W: w, H: b.H, D: b.D}
	out.add((w-b.W)/2, 0, b)
	if sup != nil {
		gap := max(float64(c.UpperLimitGapMin*s), float64(c.UpperLimitBaselineRiseMin*s)-sup.D)
		y := b.H + gap + sup.D
		out.add((w-sup.W)/2+base.italic/2, y, sup)
		out.H = y + sup.H
	}
	if sub != nil {
		gap := max(float64(c.LowerLimitGapMin*s), float64(c.LowerLimitBaselineDropMin*s)-sub.H)
		y := b.D + gap + sub.H
		out.add((w-sub.W)/2-base.italic/2, -y, sub)
		out.D = y + sub.D
	}
	return atom{class: base.class, box: out}
}

func (l *layouter) frac(n *ast.Frac, st style) ([]atom, error) {
	st = styleOf(n.Style, st)
	s := l.scale(st)
	c := l.c
	num, err := l.list(n.Num, st.num())
	if err != nil {
		return nil, err
	}
	den, err := l.list(n.Den, st.den())
	if err != nil {
		return nil, err
	}
	axis := float64(c.AxisHeight * s)
	var u, v float64
	if n.Bar {
		t := l.barThickness(n, s)
		numShift, denShift := c.FractionNumeratorShiftUp, c.FractionDenominatorShiftDown
		numGap, denGap := c.FractionNumeratorGapMin, c.FractionDenominatorGapMin
		if st.display() {
			numShift, denShift = c.FractionNumeratorDisplayStyleShiftUp, c.FractionDenominatorDisplayStyleShiftDown
			numGap, denGap = c.FractionNumDisplayStyleGapMin, c.FractionDenomDisplayStyleGapMin
		}
		u = max(float64(numShift*s), axis+t/2+float64(numGap*s)+num.D)
		v = max(float64(denShift*s), den.H+float64(denGap*s)-(axis-t/2))
	} else {
		u, v = float64(c.StackTopShiftUp*s), float64(c.StackBottomShiftDown*s)
		gapMin := float64(c.StackGapMin * s)
		if st.display() {
			u, v = float64(c.StackTopDisplayStyleShiftUp*s), float64(c.StackBottomDisplayStyleShiftDown*s)
			gapMin = float64(c.StackDisplayStyleGapMin * s)
		}
		if gap := (u - num.D) - (den.H - v); gap < gapMin {
			u += (gapMin - gap) / 2
			v += (gapMin - gap) / 2
		}
	}
	w := max(num.W, den.W)
	nds := float64(nullDelimiterSpaceEm * l.em * s)
	b := &Box{W: w + float64(2*nds), H: u + num.H, D: v + den.D}
	b.add(nds+(w-num.W)/2, u, num)
	b.add(nds+(w-den.W)/2, -v, den)
	if n.Bar {
		t := l.barThickness(n, s)
		b.add(nds, axis-t/2, rule(w, t, 0))
	}
	if n.Left == "" && n.Right == "" {
		return single(atom{class: ast.ClassInner, box: b}), nil
	}
	return l.fenced(b, n.Left, n.Right, st, true)
}

func (l *layouter) barThickness(n *ast.Frac, s float64) float64 {
	if n.BarEm > 0 {
		return float64(n.BarEm * l.em * s)
	}
	return float64(l.c.FractionRuleThickness * s)
}

// fenced puts delimiters around b and returns an Inner atom.
func (l *layouter) fenced(b *Box, left, right string, st style, frac bool) ([]atom, error) {
	s := l.scale(st)
	target := l.delimSize(b, st)
	if frac {
		// TeX uses \delim1 / \delim2 for generalized fractions.
		// NOTE(katex-compat): these are a minimum here; the delimiters
		// still grow with taller contents, where TeX keeps the fixed size.
		minEm := 1.01
		if st.display() {
			minEm = 2.39
		}
		target = max(target, float64(minEm*l.em*s))
	}
	lb, err := l.delim(left, target, st)
	if err != nil {
		return nil, err
	}
	rb, err := l.delim(right, target, st)
	if err != nil {
		return nil, err
	}
	out := &Box{W: lb.W + b.W + rb.W, H: max(lb.H, b.H, rb.H), D: max(lb.D, b.D, rb.D)}
	out.add(0, 0, lb)
	out.add(lb.W, 0, b)
	out.add(lb.W+b.W, 0, rb)
	return single(atom{class: ast.ClassInner, box: out}), nil
}

// delimSize returns the delimiter size needed to cover b (rule 19).
func (l *layouter) delimSize(b *Box, st style) float64 {
	s := l.scale(st)
	axis := float64(l.c.AxisHeight * s)
	delta := max(b.H-axis, b.D+axis)
	return max(float64(2*delta*delimiterFactor), float64(2*delta)-float64(delimiterShortfallEm*l.em*s))
}

func (l *layouter) leftRight(n *ast.LeftRight, st style) ([]atom, error) {
	atoms, err := l.atoms(n.Body, st)
	if err != nil {
		return nil, err
	}
	var h, d float64
	for _, a := range atoms {
		if a.box != nil {
			h, d = max(h, a.box.H), max(d, a.box.D)
		}
	}
	target := l.delimSize(&Box{H: h, D: d}, st)
	for i := range atoms {
		if atoms[i].mid {
			mb, err := l.delim(atoms[i].middle, target, st)
			if err != nil {
				return nil, err
			}
			atoms[i].box, atoms[i].mid = mb, false
		}
	}
	body, err := l.hpack(atoms, st)
	if err != nil {
		return nil, err
	}
	lb, err := l.delim(n.Left, target, st)
	if err != nil {
		return nil, err
	}
	rb, err := l.delim(n.Right, target, st)
	if err != nil {
		return nil, err
	}
	// The delimiters act as Open and Close atoms against the body.
	inner := []atom{{class: ast.ClassOpen, box: lb}}
	inner = append(inner, atom{class: ast.ClassOrd, box: body})
	inner = append(inner, atom{class: ast.ClassClose, box: rb})
	out := &Box{}
	x := 0.0
	for _, a := range inner {
		out.add(x, 0, a.box)
		x += a.box.W
		out.H, out.D = max(out.H, a.box.H), max(out.D, a.box.D)
	}
	out.W = x
	return single(atom{class: ast.ClassInner, box: out}), nil
}

func (l *layouter) sqrt(n *ast.Sqrt, st style) ([]atom, error) {
	s := l.scale(st)
	c := l.c
	body, err := l.list(n.Body, st.cramp())
	if err != nil {
		return nil, err
	}
	gap := float64(c.RadicalVerticalGap * s)
	if st.display() {
		gap = float64(c.RadicalDisplayStyleVerticalGap * s)
	}
	t := float64(c.RadicalRuleThickness * s)
	target := body.H + body.D + gap + t
	rad, err := l.stretchVert('√', target, s, n.Offset)
	if err != nil {
		return nil, err
	}
	if extra := rad.H + rad.D - target; extra > 0 {
		gap += extra / 2
	}
	dy := body.H + gap + t - rad.H
	out := &Box{H: body.H + gap + t + float64(c.RadicalExtraAscender*s), D: max(body.D, rad.D-dy)}
	x := 0.0
	if n.Index != nil {
		deg, err := l.list(n.Index, style{level: 3, cramped: true})
		if err != nil {
			return nil, err
		}
		before, after := float64(c.RadicalKernBeforeDegree*s), float64(c.RadicalKernAfterDegree*s)
		bottom := dy - rad.D
		raise := bottom + float64(c.RadicalDegreeBottomRaisePercent/100*(rad.H+rad.D))
		out.add(before, raise, deg)
		out.H = max(out.H, raise+deg.H)
		x = max(before+deg.W+after, 0)
	}
	out.add(x, dy, rad)
	x += rad.W
	out.add(x, body.H+gap, rule(body.W, t, 0))
	out.add(x, 0, body)
	out.W = x + body.W
	return ord(out), nil
}

// accentBase reports the single glyph of an accented nucleus, if any.
func singleGlyph(b *Box) (uint16, float64, bool) {
	for b != nil {
		if g, ok := b.Leaf.(GlyphLeaf); ok {
			return g.Glyph, g.Scale, true
		}
		if len(b.Kids) != 1 {
			return 0, 0, false
		}
		b = b.Kids[0].B
	}
	return 0, 0, false
}

func (l *layouter) accent(n *ast.Accent, st style) ([]atom, error) {
	s := l.scale(st)
	c := l.c
	base, err := l.list(n.Body, st.cramp())
	if err != nil {
		return nil, err
	}
	attach := base.W / 2
	if g, gs, ok := singleGlyph(base); ok {
		if a, ok := l.F.TopAccent(g); ok {
			attach = float64(a * gs)
		}
	}
	ag := l.F.Glyph(n.Char)
	if ag == 0 {
		return nil, diag.Unsupported("\\"+n.Name, n.Offset)
	}
	if n.Under || l.F.Metrics(ag).Advance > 0 {
		return l.overUnderAccent(n, base, s)
	}
	var acc *Box
	if n.Wide {
		acc, err = l.stretchHoriz(n.Char, base.W, s, n.Offset)
		if err != nil {
			return nil, err
		}
	} else {
		acc = l.glyphBox(ag, s)
	}
	accAttach := acc.W / 2
	if g, gs, ok := singleGlyph(acc); ok {
		if a, ok := l.F.TopAccent(g); ok {
			accAttach = float64(a * gs)
		} else if m := l.F.Metrics(g); m.Advance == 0 {
			accAttach = float64((m.MinX + m.MaxX) / 2 * gs)
		}
	}
	dy := max(base.H-float64(c.AccentBaseHeight*s), 0)
	m := l.inkHeight(acc)
	out := &Box{W: base.W, H: max(base.H, dy+m), D: base.D}
	out.add(0, 0, base)
	out.add(attach-accAttach, dy, acc)
	return ord(out), nil
}

// overUnderAccent sets accents that are not combining marks above the
// base (\overgroup, \Overrightarrow) and accents below it (\utilde,
// \underrightarrow), stretched to the base's width with a small gap, the
// way \overbrace is placed.
func (l *layouter) overUnderAccent(n *ast.Accent, base *Box, s float64) ([]atom, error) {
	acc := l.glyphBox(l.F.Glyph(n.Char), s)
	if n.Wide {
		var err error
		if acc, err = l.stretchHoriz(n.Char, base.W, s, n.Offset); err != nil {
			return nil, err
		}
	}
	gap := float64(braceGapEm * l.em * s)
	top, bottom := l.inkExtent(acc)
	out := &Box{W: max(base.W, acc.W)}
	out.add((out.W-base.W)/2, 0, base)
	// Combining marks are drawn around x = 0 (no advance); others start
	// there.
	ax := (out.W - acc.W) / 2
	if m := l.F.Metrics(l.F.Glyph(n.Char)); m.Advance == 0 {
		if g, gs, ok := singleGlyph(acc); ok {
			gm := l.F.Metrics(g)
			ax = out.W/2 - float64((gm.MinX+gm.MaxX)/2*gs)
		}
	}
	if n.Under {
		y := -(base.D + gap) - top
		out.add(ax, y, acc)
		out.H, out.D = base.H, max(base.D, -(y+bottom))
	} else {
		y := base.H + gap - bottom
		out.add(ax, y, acc)
		out.H, out.D = y+top, base.D
	}
	return ord(out), nil
}

// inkHeight is the height of a box including negative-depth glyphs (accent
// glyphs sit entirely above the baseline).
func (l *layouter) inkHeight(b *Box) float64 {
	if g, gs, ok := singleGlyph(b); ok {
		return float64(l.F.Metrics(g).Height * gs)
	}
	return b.H
}

func (l *layouter) overline(n *ast.Overline, st style) ([]atom, error) {
	s := l.scale(st)
	c := l.c
	sub := st.cramp()
	if n.Under {
		sub = st
	}
	base, err := l.list(n.Body, sub)
	if err != nil {
		return nil, err
	}
	out := &Box{W: base.W, H: base.H, D: base.D}
	out.add(0, 0, base)
	if n.Under {
		t, gap := float64(c.UnderbarRuleThickness*s), float64(c.UnderbarVerticalGap*s)
		out.add(0, -(base.D + gap + t), rule(base.W, t, 0))
		out.D = base.D + gap + t + float64(c.UnderbarExtraDescender*s)
	} else {
		t, gap := float64(c.OverbarRuleThickness*s), float64(c.OverbarVerticalGap*s)
		out.add(0, base.H+gap, rule(base.W, t, 0))
		out.H = base.H + gap + t + float64(c.OverbarExtraAscender*s)
	}
	return ord(out), nil
}

func (l *layouter) boxed(n *ast.Boxed, st style) ([]atom, error) {
	s := l.scale(st)
	b, err := l.list(n.Body, st)
	if err != nil {
		return nil, err
	}
	if n.Circle {
		return ord(l.circled(b, s)), nil
	}
	pad, t := float64(fboxSepEm*l.em*s), float64(fboxRuleEm*l.em*s)
	w := b.W + float64(2*(pad+t))
	out := &Box{W: w, H: b.H + pad + t, D: b.D + pad + t}
	out.add(t+pad, 0, b)
	out.add(0, out.H-t, rule(w, t, 0))
	out.add(0, -out.D, rule(w, t, 0))
	out.add(0, -out.D, rule(t, out.H+out.D, 0))
	out.add(w-t, -out.D, rule(t, out.H+out.D, 0))
	return ord(out), nil
}

// logo spells \TeX, \LaTeX and \KaTeX with the kerns and shifts of
// KaTeX's definitions.
func logo(n *ast.Logo) []ast.Node {
	off := n.Offset
	letter := func(c string) ast.Node {
		return &ast.Font{Offset: off, Style: ast.FontRoman, Body: []ast.Node{&ast.Symbol{Offset: off, Text: c, Class: ast.ClassOrd}}}
	}
	tex := []ast.Node{letter("T"), &ast.Space{Offset: off, Mu: -0.1667 * 18},
		&ast.Raise{Offset: off, Mu: -0.2155 * 18, Body: []ast.Node{letter("E")}},
		&ast.Space{Offset: off, Mu: -0.125 * 18}, letter("X")}
	if n.Name == "TeX" {
		return tex
	}
	first, kern := "L", -0.36
	if n.Name == "KaTeX" {
		first, kern = "K", -0.17
	}
	a := &ast.Raise{Offset: off, Mu: 0.2 * 18, Body: []ast.Node{
		&ast.Style{Offset: off, Style: ast.StyleScript, Body: []ast.Node{letter("A")}}}}
	return append([]ast.Node{letter(first), &ast.Space{Offset: off, Mu: float64(kern * 18)}, a,
		&ast.Space{Offset: off, Mu: -0.15 * 18}}, tex...)
}

// enclose draws \angl (a bar above and a rule on the right) and \phase (a
// rule below and a slanted line rising on the left).
func (l *layouter) enclose(n *ast.Enclose, st style) ([]atom, error) {
	s := l.scale(st)
	b, err := l.list(n.Body, st)
	if err != nil {
		return nil, err
	}
	t := float64(fboxRuleEm * l.em * s)
	gap := float64(0.1 * l.em * s)
	if n.Kind == "angl" {
		h := b.H + gap + t
		out := &Box{W: b.W + gap + t, H: h, D: b.D}
		out.add(0, 0, b)
		out.add(0, h-t, rule(out.W, t, 0))
		out.add(b.W+gap, -b.D, rule(t, h+b.D, 0))
		return ord(out), nil
	}
	d := b.D + gap + t
	slant := float64((b.H + d) * 0.5)
	out := &Box{W: slant + gap + b.W, H: b.H, D: d}
	out.add(slant+gap, 0, b)
	out.add(0, -d, rule(out.W, t, 0))
	out.add(0, 0, &Box{Leaf: LineLeaf{t / 2, -d + t/2, slant, b.H, t}})
	return ord(out), nil
}

// circled draws a circle around b (\textcircled), as 24 straight
// segments.
func (l *layouter) circled(b *Box, s float64) *Box {
	t := float64(fboxRuleEm * l.em * s)
	r := max(b.W, b.H+b.D)/2 + float64(0.1*l.em*s)
	cx, cy := r+t, (b.H-b.D)/2
	out := &Box{W: float64(2 * (r + t)), H: cy + r + t, D: max(r+t-cy, 0)}
	out.add(cx-b.W/2, 0, b)
	const n = 24
	for i := 0; i < n; i++ {
		a0, a1 := float64(2*math.Pi*float64(i))/n, float64(2*math.Pi*float64(i+1))/n
		out.add(0, 0, &Box{Leaf: LineLeaf{cx + float64(r*math.Cos(a0)), cy + float64(r*math.Sin(a0)), cx + float64(r*math.Cos(a1)), cy + float64(r*math.Sin(a1)), t}})
	}
	return out
}

// colorBox fills the box of n.Body (padded by \fboxsep) with its
// background and, for \fcolorbox, frames it.
func (l *layouter) colorBox(n *ast.ColorBox, st style) ([]atom, error) {
	s := l.scale(st)
	b, err := l.list(n.Body, st)
	if err != nil {
		return nil, err
	}
	pad, t := float64(fboxSepEm*l.em*s), 0.0
	if n.Frame != nil {
		t = float64(fboxRuleEm * l.em * s)
	}
	w := b.W + float64(2*(pad+t))
	out := &Box{W: w, H: b.H + pad + t, D: b.D + pad + t}
	bg := n.Bg
	out.add(0, -out.D, &Box{W: w, H: out.H + out.D, Leaf: RuleLeaf{}, Color: &bg})
	out.add(t+pad, 0, b)
	if n.Frame != nil {
		fc := *n.Frame
		for _, r := range []struct{ x, y, w, h float64 }{
			{0, out.H - t, w, t}, {0, -out.D, w, t}, {0, -out.D, t, out.H + out.D}, {w - t, -out.D, t, out.H + out.D},
		} {
			rb := rule(r.w, r.h, 0)
			rb.Color = &fc
			out.add(r.x, r.y, rb)
		}
	}
	return ord(out), nil
}

func (l *layouter) cancel(n *ast.Cancel, st style) ([]atom, error) {
	s := l.scale(st)
	atoms, err := l.atoms(n.Body, st)
	if err != nil {
		return nil, err
	}
	b, err := l.hpack(atoms, st)
	if err != nil {
		return nil, err
	}
	w := float64(cancelLineEm * l.em * s)
	out := &Box{W: b.W, H: b.H, D: b.D}
	out.add(0, 0, b)
	if n.Kind == "/" || n.Kind == "X" {
		out.add(0, 0, &Box{Leaf: LineLeaf{0, -b.D, b.W, b.H, w}})
	}
	if n.Kind == "\\" || n.Kind == "X" {
		out.add(0, 0, &Box{Leaf: LineLeaf{0, b.H, b.W, -b.D, w}})
	}
	if n.Kind == "-" {
		y := float64(l.c.AxisHeight * s)
		out.add(0, 0, &Box{Leaf: LineLeaf{0, y, b.W, y, w}})
	}
	cls := ast.ClassOrd
	if len(atoms) == 1 {
		cls = atoms[0].class
	}
	return single(atom{class: cls, box: out}), nil
}

func (l *layouter) brace(n *ast.Brace, st style) ([]atom, error) {
	s := l.scale(st)
	base, err := l.list(n.Body, st)
	if err != nil {
		return nil, err
	}
	r := '⏞'
	switch {
	case n.Under && n.Square:
		r = '⎵'
	case n.Square:
		r = '⎴'
	case n.Under:
		r = '⏟'
	}
	br, err := l.stretchHoriz(r, base.W, s, n.Offset)
	if err != nil {
		return nil, err
	}
	gap := float64(braceGapEm * l.em * s)
	top, bottom := l.inkExtent(br)
	out := &Box{W: max(base.W, br.W)}
	out.add((out.W-base.W)/2, 0, base)
	bx := (out.W - br.W) / 2
	if n.Under {
		y := -(base.D + gap) - top
		out.add(bx, y, br)
		out.H, out.D = base.H, max(base.D, -(y+bottom))
	} else {
		y := base.H + gap - bottom
		out.add(bx, y, br)
		out.H, out.D = y+top, base.D
	}
	return single(atom{class: ast.ClassOp, box: out, limits: true}), nil
}

// inkExtent returns the top and bottom (y up) of the ink of a stretched
// glyph or assembly at its origin.
func (l *layouter) inkExtent(b *Box) (top, bottom float64) {
	first := true
	var walk func(b *Box, y float64)
	walk = func(b *Box, y float64) {
		if g, ok := b.Leaf.(GlyphLeaf); ok {
			m := l.F.Metrics(g.Glyph)
			t, bt := y+float64(m.Height*g.Scale), y-float64(m.Depth*g.Scale)
			if first {
				top, bottom, first = t, bt, false
			} else {
				top, bottom = max(top, t), min(bottom, bt)
			}
		}
		for _, k := range b.Kids {
			walk(k.B, y+k.Y)
		}
	}
	walk(b, 0)
	return top, bottom
}

func (l *layouter) overUnder(n *ast.OverUnder, st style) ([]atom, error) {
	base, err := l.list(n.Base, st)
	if err != nil {
		return nil, err
	}
	var over, under *Box
	if n.Over != nil {
		if over, err = l.list(n.Over, st.sup()); err != nil {
			return nil, err
		}
	}
	if n.Under != nil {
		if under, err = l.list(n.Under, st.sub()); err != nil {
			return nil, err
		}
	}
	a := l.limits(atom{class: n.Class, box: base}, over, under, st)
	return single(a), nil
}

func (l *layouter) xarrow(n *ast.XArrow, st style) ([]atom, error) {
	s := l.scale(st)
	var over, under *Box
	var err error
	if over, err = l.list(n.Over, st.sup()); err != nil {
		return nil, err
	}
	if n.Under != nil {
		if under, err = l.list(n.Under, st.sub()); err != nil {
			return nil, err
		}
	}
	pad := float64(xArrowPadEm * l.em * s)
	w := over.W + float64(2*pad)
	if under != nil {
		w = max(w, under.W+float64(2*pad))
	}
	var arrow *Box
	if n.Arrow == '=' {
		t := float64(l.c.FractionRuleThickness * s)
		axis := float64(l.c.AxisHeight * s)
		sep := float64(0.1 * l.em * s)
		w = max(w, float64(0.8*l.em*s))
		arrow = &Box{W: w, H: axis + sep + t, D: 0}
		arrow.add(0, axis+sep, rule(w, t, 0))
		arrow.add(0, axis-sep-t, rule(w, t, 0))
	} else if arrow, err = l.stretchHoriz(n.Arrow, w, s, n.Offset); err != nil {
		return nil, err
	}
	w = max(w, arrow.W)
	gap := float64(xArrowGapEm * l.em * s)
	out := &Box{W: w, H: arrow.H, D: arrow.D}
	out.add((w-arrow.W)/2, 0, arrow)
	top, bottom := l.inkExtent(arrow)
	if n.Arrow == '=' {
		top, bottom = arrow.H, float64(l.c.AxisHeight*s)-float64(0.1*l.em*s)-float64(l.c.FractionRuleThickness*s)
	}
	if over.W > 0 || over.H > 0 {
		y := top + gap + over.D
		out.add((w-over.W)/2, y, over)
		out.H = max(out.H, y+over.H)
	}
	if under != nil {
		y := bottom - gap - under.H
		out.add((w-under.W)/2, y, under)
		out.D = max(out.D, -y+under.D)
	}
	return single(atom{class: ast.ClassRel, box: out}), nil
}

// delim returns a delimiter covering target (total height), centered on
// the axis. target 0 gives the normal size.
func (l *layouter) delim(d string, target float64, st style) (*Box, error) {
	s := l.scale(st)
	if d == "" {
		return kern(float64(nullDelimiterSpaceEm * l.em * s)), nil
	}
	r, _ := utf8.DecodeRuneInString(d)
	b, err := l.stretchVert(r, target, s, 0)
	if err != nil {
		return nil, err
	}
	dy := float64(l.c.AxisHeight*s) - (b.H-b.D)/2
	if target == 0 {
		dy = 0
	}
	return shifted(b, dy), nil
}
