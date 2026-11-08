// Package box is the TeX-style math typesetter: it turns the syntax tree
// into nested boxes following the rules of TeXbook appendix G, with the
// parameters taken from the OpenType MATH table.
//
// All lengths are in font units of the math font at text size (1 em =
// UnitsPerEm), y up. Pixels appear only when converting to a scene.
package box

import (
	"image/color"

	"github.com/lea-151107/dal/internal/ast"
	"github.com/lea-151107/dal/internal/guard"
	"github.com/lea-151107/dal/internal/mathfont"
)

// Tuning values in em (TeX's values at 10pt where they exist).
const (
	nullDelimiterSpaceEm  = 0.12  // \nulldelimiterspace = 1.2pt
	delimiterShortfallEm  = 0.5   // \delimitershortfall = 5pt
	delimiterFactor       = 0.901 // \delimiterfactor = 901
	fboxSepEm             = 0.3   // \fboxsep = 3pt
	fboxRuleEm            = 0.04  // \fboxrule = 0.4pt
	cancelLineEm          = 0.046 // KaTeX's \cancel stroke
	xArrowPadEm           = 0.4   // label padding of \xrightarrow
	xArrowGapEm           = 0.111 // label gap of \xrightarrow (KaTeX)
	braceGapEm            = 0.1   // gap between \overbrace and its base
	arrayStrutHeightEm    = 0.84  // KaTeX: 0.7 * baselineskip (1.2em)
	arrayStrutDepthEm     = 0.36  // KaTeX: 0.3 * baselineskip
	jotEm                 = 0.3   // \jot = 3pt, extra row gap in aligned
	arrayRuleEm           = 0.04  // \arrayrulewidth = 0.4pt
	dashEm                = 0.4   // \dashlinedash = 4pt
	dashGapEm             = 0.4   // \dashlinegap = 4pt
	tagGapEm              = 1.0   // space before an equation tag
	lineSkipEm            = 1.2   // \baselineskip of broken lines
	lineSkipLimitEm       = 0.1   // \lineskip when lines would touch
	opNameSpaceEm         = 1.0 / 6
	maxAssemblyParts      = 2000
	maxTotalAssemblyParts = 20000
	maxTotalDashes        = 20000 // dash segments of \hdashline and ":" per call
)

// bigDelimEm are the total heights of \big, \Big, \bigg and \Bigg (KaTeX).
var bigDelimEm = [5]float64{0, 1.2, 1.8, 2.4, 3.0}

// Box is a typeset box. Children are positioned relative to the box origin
// (left end of the baseline), y up.
type Box struct {
	W, H, D float64
	Kids    []Child
	Leaf    Leaf
	Color   *color.NRGBA
	// Mirror flips the box and its contents left to right (\reflectbox).
	Mirror bool
	// Assembly marks a glyph assembly: its parts are drawn together, each
	// within its band (GlyphLeaf.Clip), so that the overlaps are not
	// painted twice.
	Assembly bool
}

// Child is a positioned child box.
type Child struct {
	X, Y float64
	B    *Box
}

// Leaf is the drawable content of a box: GlyphLeaf, RuleLeaf, TextLeaf or
// LineLeaf.
type Leaf interface{ isLeaf() }

// GlyphLeaf draws glyph Glyph scaled by Scale at the box origin.
type GlyphLeaf struct {
	Glyph uint16
	Scale float64
	// Clip, when set, limits the glyph to a band of a glyph assembly.
	Clip *Band
}

// Band is the part of a glyph between Lo and Hi along an axis (y up when
// vertical, else x), relative to the glyph origin. An open end is
// infinite.
type Band struct {
	Horiz  bool
	Lo, Hi float64
}

// RuleLeaf fills the whole box.
type RuleLeaf struct{}

// TextLeaf is a run drawn with the caller's text face.
type TextLeaf struct {
	Text   string
	SizeEm float64 // font size relative to the text size
}

// LineLeaf is a straight line from (X0, Y0) to (X1, Y1) relative to the
// box origin.
type LineLeaf struct {
	X0, Y0, X1, Y1, Width float64
}

func (GlyphLeaf) isLeaf() {}
func (RuleLeaf) isLeaf()  {}
func (TextLeaf) isLeaf()  {}
func (LineLeaf) isLeaf()  {}

func (b *Box) add(x, y float64, c *Box) { b.Kids = append(b.Kids, Child{x, y, c}) }

func rule(w, h, d float64) *Box { return &Box{W: w, H: h, D: d, Leaf: RuleLeaf{}} }

func kern(w float64) *Box { return &Box{W: w} }

// style is one of TeX's eight styles (four sizes, cramped or not).
type style struct {
	level   uint8 // 0 display, 1 text, 2 script, 3 scriptscript
	cramped bool
}

func (s style) display() bool { return s.level == 0 }
func (s style) script() bool  { return s.level >= 2 }

func (s style) sup() style {
	switch s.level {
	case 0, 1:
		return style{2, s.cramped}
	}
	return style{3, s.cramped}
}

func (s style) sub() style { return style{s.sup().level, true} }

func (s style) num() style {
	switch s.level {
	case 0:
		return style{1, s.cramped}
	case 1:
		return style{2, s.cramped}
	}
	return style{3, s.cramped}
}

func (s style) den() style   { return style{s.num().level, true} }
func (s style) cramp() style { return style{s.level, true} }

func styleOf(m ast.MathStyle, cur style) style {
	switch m {
	case ast.StyleDisplay:
		return style{0, cur.cramped}
	case ast.StyleText:
		return style{1, cur.cramped}
	case ast.StyleScript:
		return style{2, cur.cramped}
	case ast.StyleScriptScript:
		return style{3, cur.cramped}
	}
	return cur
}

// TextMeasure measures a run of \text for the caller's text face. It
// returns the advance, ascent and descent in pixels.
type TextMeasure func(s string, sizePx float64) (advance, ascent, descent float64, err error)

// Env is the per-call typesetting environment.
type Env struct {
	F       *mathfont.Font
	G       *guard.Guard
	SizePx  float64
	Measure TextMeasure // nil: \text uses the math font
	// Has reports whether the text face has a glyph for r. Nil means it
	// has every character.
	Has func(r rune, sizePx float64) (bool, error)
	// LineWidth is the width available to a display formula in font
	// units, 0 when unknown. Equation tags go to its right end.
	LineWidth float64
	// Wrap breaks a formula in text style that is wider than LineWidth
	// into lines.
	Wrap bool
}

type layouter struct {
	*Env
	c      *mathfont.Constants
	em     float64
	font   ast.FontStyle
	parts  int // glyph assembly parts placed so far
	dashes int // dash segments placed so far
	// size is the factor of \tiny ... \Huge (1 for \normalsize).
	sizeFactor float64
	// tagRight is where row tags of the top-level array end (the line
	// width), 0 elsewhere.
	tagRight float64
}

func (l *layouter) scale(s style) float64 {
	switch s.level {
	case 2:
		return float64(l.sizeFactor*l.c.ScriptPercentScaleDown) / 100
	case 3:
		return float64(l.sizeFactor*l.c.ScriptScriptPercentScaleDown) / 100
	}
	return l.sizeFactor
}

// Layout typesets e. Display selects display style.
func Layout(env *Env, e *ast.Expr, display bool) (*Box, error) {
	l := &layouter{Env: env, c: env.F.Constants(), em: env.F.UnitsPerEm(), sizeFactor: 1}
	st := style{level: 1}
	if display {
		st.level = 0
	}
	// Row tags of a top-level array go to the line's end, unless the
	// formula has a tag of its own, which then needs that place.
	if display && env.LineWidth > 0 && len(e.Body) == 1 && e.Tag == nil {
		if _, ok := e.Body[0].(*ast.Array); ok {
			l.tagRight = env.LineWidth
		}
	}
	b, err := l.list(e.Body, st)
	if err != nil {
		return nil, err
	}
	// A wrapped formula keeps room for its tag on the line.
	width := env.LineWidth
	if e.Tag != nil && !display && env.Wrap && width > 0 {
		tb, err := l.tag(e.Tag, st)
		if err != nil {
			return nil, err
		}
		width -= float64(tagGapEm*l.em) + tb.W
	}
	if !display && env.Wrap && width > 0 && b.W > width {
		if b, err = l.wrap(e.Body, st, width); err != nil {
			return nil, err
		}
	}
	if e.Tag != nil {
		return l.withTag(b, e.Tag, st, display)
	}
	return b, nil
}

// breakAfter reports the atoms after which a line may break, as TeX does:
// after a relation (unless another relation or punctuation follows), after
// a binary operator, and at \allowbreak; never before \nobreak or after
// the last atom.
func breakAfter(atoms []atom) []bool {
	out := make([]bool, len(atoms))
	next := -1 // next non-space atom after i
	for i := len(atoms) - 1; i >= 0; i-- {
		a := atoms[i]
		switch {
		case a.brk > 0:
			out[i] = true
		case a.space:
		case next < 0:
		case a.class == ast.ClassBin:
			out[i] = true
		case a.class == ast.ClassRel:
			n := atoms[next].class
			out[i] = n != ast.ClassRel && n != ast.ClassPunct
		}
		if i+1 < len(atoms) && atoms[i+1].brk < 0 {
			out[i] = false
		}
		if !a.space && a.brk == 0 {
			next = i
		}
	}
	return out
}

// wrap breaks the top-level list into lines no wider than LineWidth,
// greedily, and stacks them. A piece between two break points that is
// wider than the line on its own stays whole (the caller's width check
// then fails). The list is packed once to find each atom's position, so
// the width of any run of atoms is known without repacking; the work is
// linear in the length of the list.
func (l *layouter) wrap(nodes []ast.Node, st style, width float64) (*Box, error) {
	atoms, err := l.atoms(nodes, st)
	if err != nil {
		return nil, err
	}
	reclassifyBins(atoms)
	full, err := l.hpackRaw(atoms, st)
	if err != nil {
		return nil, err
	}
	// hpackRaw adds one child per atom, in order.
	left := func(i int) float64 { return full.Kids[i].X }
	right := func(i int) float64 { return full.Kids[i].X + full.Kids[i].B.W }
	brk := breakAfter(atoms)
	var lines []*Box
	emit := func(from, to int) error {
		b, err := l.hpackRaw(atoms[from:to+1], st)
		if err != nil {
			return err
		}
		lines = append(lines, b)
		return nil
	}
	start, last := 0, -1
	for i := range atoms {
		if !brk[i] && i != len(atoms)-1 {
			continue
		}
		if right(i)-left(start) > width && last >= start {
			if err := emit(start, last); err != nil {
				return nil, err
			}
			start = last + 1
			for start < i && atoms[start].space {
				start++ // glue at a break disappears
			}
		}
		last = i
	}
	if err := emit(start, len(atoms)-1); err != nil {
		return nil, err
	}
	return l.stack(lines), nil
}

// stack puts lines below each other with TeX's baselineskip (1.2em) and
// lineskip; the result's baseline is the first line's.
func (l *layouter) stack(lines []*Box) *Box {
	out := &Box{}
	y := 0.0
	for i, b := range lines {
		if i == 0 {
			out.H = b.H
		} else {
			y -= max(float64(lineSkipEm*l.em), lines[i-1].D+b.H+float64(lineSkipLimitEm*l.em))
		}
		out.add(0, y, b)
		out.W = max(out.W, b.W)
		out.D = max(out.D, -y+b.D)
	}
	return out
}

func (l *layouter) withTag(b *Box, t *ast.Tag, st style, display bool) (*Box, error) {
	tb, err := l.tag(t, st)
	if err != nil {
		return nil, err
	}
	out := &Box{H: max(b.H, tb.H), D: max(b.D, tb.D)}
	out.add(0, 0, b)
	// NOTE(katex-compat): with a known line width (Options.MaxWidth) a
	// display formula's tag sits at its right end; otherwise it follows
	// the formula after a gap. The formula itself stays at the left.
	gap := float64(tagGapEm * l.em)
	x := b.W + gap
	if display && l.LineWidth > 0 && l.LineWidth-tb.W > x {
		x = l.LineWidth - tb.W
	}
	out.add(x, 0, tb)
	out.W = x + tb.W
	return out, nil
}

func (l *layouter) tag(t *ast.Tag, st style) (*Box, error) {
	body := t.Body
	if !t.Star {
		body = append(append([]ast.Node{&ast.Text{Offset: t.Offset, Text: "(", Style: ast.FontRoman}}, body...),
			&ast.Text{Offset: t.Offset, Text: ")", Style: ast.FontRoman})
	}
	return l.list(body, style{level: 1, cramped: st.cramped})
}

// atom is one element of a math list being typeset.
type atom struct {
	class  ast.Class
	box    *Box
	italic float64 // italic correction of a single-character nucleus
	char   bool    // the nucleus is a single character (TeX rule 17/18)
	glyph  uint16  // for char atoms
	space  bool    // explicit space: not an atom for spacing purposes
	// brk is +1 after \allowbreak, -1 after \nobreak (line breaking).
	brk    int
	limits bool   // op with limits (scripts above and below)
	middle string // \middle placeholder, sized by the enclosing \left
	// mid marks a \middle atom that still needs its delimiter.
	mid bool
}

// Spacing between atom classes in mu (TeXbook chapter 18). Negative values
// are only used in display and text style.
var spacing = [8][8]int8{
	//          Ord Op Bin Rel Open Close Punct Inner
	/* Ord   */ {0, 3, -4, -5, 0, 0, 0, -3},
	/* Op    */ {3, 3, 0, -5, 0, 0, 0, -3},
	/* Bin   */ {-4, -4, 0, 0, -4, 0, 0, -4},
	/* Rel   */ {-5, -5, 0, 0, -5, 0, 0, -5},
	/* Open  */ {0, 0, 0, 0, 0, 0, 0, 0},
	/* Close */ {0, 3, -4, -5, 0, 0, 0, -3},
	/* Punct */ {-3, -3, 0, -3, -3, -3, -3, -3},
	/* Inner */ {-3, 3, -4, -5, -3, 0, -3, -3},
}

func (l *layouter) muToUnits(mu float64, st style) float64 {
	return float64(mu / 18 * l.em * l.scale(st))
}

// list typesets a math list into one horizontal box.
func (l *layouter) list(nodes []ast.Node, st style) (*Box, error) {
	atoms, err := l.atoms(nodes, st)
	if err != nil {
		return nil, err
	}
	return l.hpack(atoms, st)
}

func (l *layouter) atoms(nodes []ast.Node, st style) ([]atom, error) {
	var out []atom
	for _, n := range nodes {
		if err := l.G.Tick(); err != nil {
			return nil, err
		}
		a, err := l.node(n, st)
		if err != nil {
			return nil, err
		}
		out = append(out, a...)
	}
	return out, nil
}

// hpack applies the bin rules and inter-atom spacing and concatenates.
func (l *layouter) hpack(atoms []atom, st style) (*Box, error) {
	reclassifyBins(atoms)
	return l.hpackRaw(atoms, st)
}

// hpackRaw is hpack for atoms whose bins were already reclassified (the
// lines of a broken formula).
func (l *layouter) hpackRaw(atoms []atom, st style) (*Box, error) {
	b := &Box{}
	x := 0.0
	prev := -1
	for i := range atoms {
		a := &atoms[i]
		if a.mid {
			d, err := l.delim(a.middle, 0, st)
			if err != nil {
				return nil, err
			}
			a.box, a.mid = d, false
		}
		if !a.space && prev >= 0 {
			sp := spacing[atoms[prev].class][a.class]
			if sp < 0 && !st.script() {
				sp = -sp
			}
			if sp > 0 {
				x += l.muToUnits(float64(sp), st)
			}
		}
		b.add(x, 0, a.box)
		x += a.box.W
		b.H = max(b.H, a.box.H)
		b.D = max(b.D, a.box.D)
		if !a.space {
			prev = i
		}
	}
	b.W = x
	return b, nil
}

func reclassifyBins(atoms []atom) {
	prev := -1
	for i := range atoms {
		a := &atoms[i]
		if a.space {
			continue
		}
		if a.class == ast.ClassBin {
			if prev < 0 {
				a.class = ast.ClassOrd
			} else {
				switch atoms[prev].class {
				case ast.ClassBin, ast.ClassOp, ast.ClassRel, ast.ClassOpen, ast.ClassPunct:
					a.class = ast.ClassOrd
				}
			}
		}
		if prev >= 0 && atoms[prev].class == ast.ClassBin {
			switch a.class {
			case ast.ClassRel, ast.ClassClose, ast.ClassPunct:
				atoms[prev].class = ast.ClassOrd
			}
		}
		prev = i
	}
	if prev >= 0 && atoms[prev].class == ast.ClassBin {
		atoms[prev].class = ast.ClassOrd
	}
}

func single(a atom) []atom { return []atom{a} }

func ord(b *Box) []atom { return single(atom{class: ast.ClassOrd, box: b}) }

// glyphBox returns the box of glyph g at scale s.
func (l *layouter) glyphBox(g uint16, s float64) *Box {
	m := l.F.Metrics(g)
	return &Box{W: float64(m.Advance * s), H: max(float64(m.Height*s), 0), D: max(float64(m.Depth*s), 0), Leaf: GlyphLeaf{Glyph: g, Scale: s}}
}

// shifted wraps b moved up by dy.
func shifted(b *Box, dy float64) *Box {
	out := &Box{W: b.W, H: max(b.H+dy, 0), D: max(b.D-dy, 0)}
	out.add(0, dy, b)
	return out
}
