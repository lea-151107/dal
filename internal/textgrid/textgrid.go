// Package textgrid sets a formula on a character grid for terminals:
// fractions over several lines, limits above and below, matrices with
// tall brackets made of box-drawing pieces. Scripts use Unicode
// superscript and subscript characters when every character has one.
package textgrid

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/lea/dal/internal/ast"
	"github.com/lea/dal/internal/diag"
	"github.com/lea/dal/internal/guard"
	"github.com/lea/dal/internal/symbols"
)

// RuneWidth returns the number of columns r occupies on its own: 0 for
// control, format and combining characters, 2 for East Asian Wide and
// Fullwidth characters, 1 otherwise. StringWidth also accounts for emoji
// presentation sequences.
//
//go:generate go run gen_width.go
func RuneWidth(r rune) int {
	switch {
	case r < 0x20 || (r >= 0x7f && r < 0xa0):
		return 0
	case r < 0x300:
		return 1
	case unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf, unicode.Zl, unicode.Zp):
		return 0
	case r < wideRanges[0][0]:
		return 1
	}
	lo, hi := 0, len(wideRanges)
	for lo < hi {
		m := int(uint(lo+hi) >> 1)
		switch {
		case r < wideRanges[m][0]:
			hi = m
		case r > wideRanges[m][1]:
			lo = m + 1
		default:
			return 2
		}
	}
	return 1
}

// emojiPresentation is VARIATION SELECTOR-16. After a character of width
// 1 (❤ U+2764, ☺, digits in keycaps) it asks for the emoji form, which
// UAX #11 treats as Wide and terminals draw two columns wide.
const emojiPresentation = 0xFE0F

// StringWidth returns the column width of s.
func StringWidth(s string) int {
	n, prev := 0, 0
	for _, r := range s {
		w := RuneWidth(r)
		if r == emojiPresentation && prev == 1 {
			w, prev = 1, 2 // the base becomes two columns wide
		} else if w > 0 {
			prev = w
		}
		n += w
	}
	return n
}

// Width returns the width of the widest line.
func Width(lines []string) int {
	w := 0
	for _, l := range lines {
		w = max(w, StringWidth(l))
	}
	return w
}

// grid is a rectangle of cells around a baseline row. Each cell holds one
// character; the cell after a wide character holds "" (continuation).
type grid struct {
	w, asc, desc int
	rows         [][]string
}

func (g *grid) h() int { return g.asc + 1 + g.desc }

// maxCells bounds the cells allocated by one call, summed over all grids,
// so that a small input cannot build a huge grid (for example a tall
// array next to a long row).
const maxCells = 1 << 22

// blank allocates an empty grid. When the cell budget is exhausted it
// records ErrTooLarge in t.err and returns an empty grid instead; callers
// see the error at the next check.
func (t *typesetter) blank(w, asc, desc int) *grid {
	if !t.chargeRect(w, asc+1+desc) {
		return &grid{rows: [][]string{nil}}
	}
	g := &grid{w: w, asc: asc, desc: desc, rows: make([][]string, asc+1+desc)}
	for i := range g.rows {
		g.rows[i] = make([]string, w)
		for j := range g.rows[i] {
			g.set(i, j, " ")
		}
	}
	return g
}

// makeText makes a one-row grid. Characters of width 0 (combining marks,
// zero width space, joiners) join the cell of the character before them
// (a + U+030B is one cell), or the next cell at the start, so that the grid
// holds the same text as RenderLine. VS16 after a character of width 1
// makes its cell two columns wide (see StringWidth). A run of only width-0
// characters has no cell to join and is dropped: a cell of its own would
// take a column that the terminal does not draw.
func makeText(s string) *grid {
	// Each cell is a substring of s: a character with the width-0
	// characters after it (or, for the first, also those before it), or
	// "" for the second column of a wide character. Cells are kept as
	// byte ranges while scanning, so that a long run of width-0
	// characters costs linear time.
	type span struct{ from, to int }
	var cells []span
	pending := -1 // start of the width-0 characters before the first cell
	for i, r := range s {
		end := i + utf8.RuneLen(r)
		w := RuneWidth(r)
		if w == 0 {
			last := len(cells) - 1
			for last >= 0 && cells[last].from == cells[last].to {
				last--
			}
			switch {
			case last < 0:
				if pending < 0 {
					pending = i
				}
			case r == emojiPresentation && last == len(cells)-1 && StringWidth(s[cells[last].from:cells[last].to]) == 1:
				cells[last].to = end
				cells = append(cells, span{end, end})
			default:
				cells[last].to = end
			}
			continue
		}
		from := i
		if pending >= 0 {
			from, pending = pending, -1
		}
		cells = append(cells, span{from, end})
		if w == 2 {
			cells = append(cells, span{end, end})
		}
	}
	out := make([]string, len(cells))
	for i, c := range cells {
		out[i] = s[c.from:c.to]
	}
	return &grid{w: len(out), rows: [][]string{out}}
}

// charge counts n cells against the budget of blank. When the budget is
// exhausted it records ErrTooLarge in t.err and reports false.
func (t *typesetter) charge(n int) bool {
	if t.err != nil || n > maxCells-t.cells {
		if t.err == nil {
			t.err = fmt.Errorf("%w: text layout needs more than %d cells", guard.ErrTooLarge, maxCells)
		}
		return false
	}
	t.cells += n
	return true
}

// chargeRect is charge(w*h) for w, h ≥ 0, compared without multiplying:
// with a 32-bit int the product of a wide and a tall part can wrap to a
// negative number that would pass the budget.
func (t *typesetter) chargeRect(w, h int) bool {
	if w > 0 && h > 0 && w > (maxCells-t.cells)/h {
		return t.charge(maxCells + 1)
	}
	return t.charge(max(w, 0) * max(h, 0))
}

// text is makeText with its cells counted against the budget of blank.
func (t *typesetter) text(s string) *grid {
	g := makeText(s)
	if !t.charge(g.w) {
		return &grid{rows: [][]string{nil}}
	}
	return g
}

// repeat makes a grid of s repeated n times. It checks the budget before
// building the string, so that a huge n (a long \rule) fails early.
func (t *typesetter) repeat(s string, n int) *grid {
	n = max(n, 0)
	if !t.chargeRect(n, max(StringWidth(s), 1)) {
		return &grid{rows: [][]string{nil}}
	}
	if utf8.RuneCountInString(s) == 1 && RuneWidth([]rune(s)[0]) == 1 {
		cells := make([]string, n)
		for i := range cells {
			cells[i] = s
		}
		return &grid{w: n, rows: [][]string{cells}}
	}
	return makeText(strings.Repeat(s, n))
}

// blit copies src into g with src's baseline at g's row baseRow (index
// from the top) and its left edge at column x. Spaces in src do not
// overwrite when overlay is true.
func (g *grid) blit(src *grid, x, baseRow int, overlay bool) {
	top := baseRow - src.asc
	for i, row := range src.rows {
		y := top + i
		if y < 0 || y >= len(g.rows) {
			continue
		}
		for j, c := range row {
			if overlay && c == " " {
				continue
			}
			if xx := x + j; xx >= 0 && xx < g.w {
				g.set(y, xx, c)
			}
		}
	}
}

// set writes cell (y, x), ignoring positions outside the grid (a grid
// that hit the cell budget is empty).
func (g *grid) set(y, x int, c string) {
	if y >= 0 && y < len(g.rows) && x >= 0 && x < len(g.rows[y]) {
		g.rows[y][x] = c
	}
}

func (g *grid) get(y, x int) string {
	if y >= 0 && y < len(g.rows) && x >= 0 && x < len(g.rows[y]) {
		return g.rows[y][x]
	}
	return ""
}

func (g *grid) flat() bool { return g.asc == 0 && g.desc == 0 }

func (g *grid) String() string {
	var b strings.Builder
	for _, c := range g.rows[g.asc] {
		b.WriteString(c)
	}
	return b.String()
}

// lines returns the grid as strings with trailing spaces removed.
func (g *grid) lines() []string {
	out := make([]string, len(g.rows))
	for i, row := range g.rows {
		out[i] = strings.TrimRight(strings.Join(row, ""), " ")
	}
	return out
}

// hcat joins grids on their baselines.
func (t *typesetter) hcat(gs ...*grid) *grid {
	w, asc, desc := 0, 0, 0
	for _, g := range gs {
		w += g.w
		asc, desc = max(asc, g.asc), max(desc, g.desc)
	}
	out := t.blank(w, asc, desc)
	x := 0
	for _, g := range gs {
		out.blit(g, x, asc, false)
		x += g.w
	}
	return out
}

// Render sets e on a grid and returns its lines and the baseline row.
// maxCols is the line width (0 when unknown): a display formula's equation
// tags are put at its right end.
//
// With wrap, a formula in text style that is wider than maxCols is broken
// into lines where TeX would break it (after relations and binary
// operators, at \allowbreak).
func Render(g *guard.Guard, e *ast.Expr, display bool, maxCols int, wrap bool) ([]string, int, error) {
	t := &typesetter{g: g}
	level := 1
	if display {
		level = 0
		// Row tags of a top-level array go to the line's end, unless the
		// formula has a tag of its own, which then needs that place.
		if maxCols > 0 && len(e.Body) == 1 && e.Tag == nil {
			if _, ok := e.Body[0].(*ast.Array); ok {
				t.tagRight = maxCols
			}
		}
	}
	out, err := t.list(e.Body, level)
	if err != nil {
		return nil, 0, err
	}
	var tg *grid
	if e.Tag != nil {
		if tg, err = t.tag(e.Tag); err != nil {
			return nil, 0, err
		}
	}
	// A wrapped formula keeps room for its tag (and the gap before it).
	width := maxCols
	if tg != nil && maxCols > 0 {
		width -= 2 + tg.w
	}
	if wrap && !display && width > 0 && out.w > width {
		if out, err = t.wrap(e.Body, level, width); err != nil {
			return nil, 0, err
		}
	}
	if tg != nil {
		gap := 2
		if display && maxCols > 0 {
			gap = max(gap, maxCols-out.w-tg.w)
		}
		out = t.hcat(out, t.repeat(" ", gap), tg)
	}
	if t.err != nil {
		return nil, 0, t.err
	}
	return out.lines(), out.asc, nil
}

type typesetter struct {
	g *guard.Guard
	// tagRight is the column where row tags of the top-level array end, 0
	// elsewhere.
	tagRight int
	font     ast.FontStyle
	cells    int   // cells allocated so far
	opName   bool  // inside an \operatorname body, where any space shows
	err      error // sticky error from blank
}

func (t *typesetter) tag(tg *ast.Tag) (*grid, error) {
	b, err := t.list(tg.Body, 1)
	if err != nil {
		return nil, err
	}
	if tg.Star {
		return b, nil
	}
	return t.hcat(t.text("("), b, t.text(")")), nil
}

// item is an atom on the grid.
type item struct {
	class ast.Class
	g     *grid
	space bool   // explicit space
	word  bool   // operator name: a space separates it from its operand
	mid   string // pending \middle delimiter
	isMid bool
	brk   int // +1 \allowbreak, -1 \nobreak
}

// level: 0 display, 1 text, 2 and more script.
func (t *typesetter) list(nodes []ast.Node, level int) (*grid, error) {
	items, err := t.items(nodes, level)
	if err != nil {
		return nil, err
	}
	return t.pack(items, level, 0, 0), nil
}

func (t *typesetter) items(nodes []ast.Node, level int) ([]item, error) {
	var out []item
	for _, n := range nodes {
		if t.err != nil {
			return nil, t.err
		}
		if err := t.g.Tick(); err != nil {
			return nil, err
		}
		if s, ok := n.(*ast.Symbol); ok && onlyMarks(s.Text) && len(out) > 0 && out[len(out)-1].g != nil {
			// A combining mark written after a symbol (e\u0301) joins the
			// last cell of the item before it.
			prev := out[len(out)-1].g
			row := prev.rows[prev.asc]
			for x := len(row) - 1; x >= 0; x-- {
				if row[x] != "" && row[x] != " " {
					row[x] += s.Text
					break
				}
			}
			continue
		}
		its, err := t.node(n, level)
		if err != nil {
			return nil, err
		}
		out = append(out, its...)
	}
	return out, nil
}

// onlyMarks reports whether s consists of combining marks.
func onlyMarks(s string) bool {
	for _, r := range s {
		if !unicode.In(r, unicode.Mn, unicode.Me) {
			return false
		}
	}
	return s != ""
}

func reclassifyBins(items []item) {
	prev := -1
	for i := range items {
		if items[i].space {
			continue
		}
		if items[i].class == ast.ClassBin {
			if prev < 0 {
				items[i].class = ast.ClassOrd
			} else {
				switch items[prev].class {
				case ast.ClassBin, ast.ClassOp, ast.ClassRel, ast.ClassOpen, ast.ClassPunct:
					items[i].class = ast.ClassOrd
				}
			}
		}
		if prev >= 0 && items[prev].class == ast.ClassBin {
			switch items[i].class {
			case ast.ClassRel, ast.ClassClose, ast.ClassPunct:
				items[prev].class = ast.ClassOrd
			}
		}
		prev = i
	}
	if prev >= 0 && items[prev].class == ast.ClassBin {
		items[prev].class = ast.ClassOrd
	}
}

// spacedAt reports whether spaces between a and b are set at level: in
// scripts (level 2 and up) TeX keeps only the thin spaces around an
// operator (x^{\sin x}, x^{a\sin x}).
func spacedAt(level int, a, b item) bool {
	return level < 2 || opSpace(a.class, b.class)
}

// opSpace reports whether a and b have TeX's thin space around an
// operator between them, which scripts keep: Op before Ord or Op, and
// Ord, Close or Inner before Op.
func opSpace(a, b ast.Class) bool {
	switch {
	case a == ast.ClassOp:
		return b == ast.ClassOrd || b == ast.ClassOp
	case b == ast.ClassOp:
		return a == ast.ClassOrd || a == ast.ClassClose || a == ast.ClassInner
	}
	return false
}

// spaced reports whether a space column goes between a and b (thick and
// medium TeX spaces become one column, thin spaces none).
func spaced(a, b item) bool {
	switch {
	case a.class == ast.ClassRel && b.class == ast.ClassRel:
		return false
	case a.class == ast.ClassRel || b.class == ast.ClassRel:
		return a.class != ast.ClassOpen && b.class != ast.ClassClose && b.class != ast.ClassPunct
	case a.class == ast.ClassBin || b.class == ast.ClassBin:
		return true
	case a.class == ast.ClassPunct:
		return true
	case a.class == ast.ClassOp:
		// TeX's thin space after an operator; one column keeps "∑ᵢ aᵢ"
		// and "sin x" readable.
		return b.class == ast.ClassOrd || b.class == ast.ClassOp || b.class == ast.ClassInner
	case b.class == ast.ClassOp:
		// The thin space before an operator: "n log n", not "nlog n".
		return a.class == ast.ClassOrd || a.class == ast.ClassClose || a.class == ast.ClassInner
	}
	return false
}

// pack concatenates items with spacing; pending \middle delimiters get
// the given height (0: use the items' own height).
func (t *typesetter) pack(items []item, level, midAsc, midDesc int) *grid {
	reclassifyBins(items)
	return t.packRaw(items, level, midAsc, midDesc)
}

// packRaw is pack for items whose bins were already reclassified.
func (t *typesetter) packRaw(items []item, level, midAsc, midDesc int) *grid {
	var parts []*grid
	prev := -1
	for i := range items {
		it := &items[i]
		if it.isMid {
			it.g = t.delimiter(it.mid, midAsc, midDesc)
		}
		if !it.space && prev >= 0 && spacedAt(level, items[prev], *it) && spaced(items[prev], *it) {
			parts = append(parts, t.text(" "))
		}
		parts = append(parts, it.g)
		if !it.space {
			prev = i
		}
	}
	if len(parts) == 0 {
		return t.text("")
	}
	return t.hcat(parts...)
}

// breakAfter reports the items after which a line may break (see the
// box package's breakAfter, which follows the same rules).
func breakAfter(items []item) []bool {
	out := make([]bool, len(items))
	next := -1
	for i := len(items) - 1; i >= 0; i-- {
		it := items[i]
		switch {
		case it.brk > 0:
			out[i] = true
		case it.space:
		case next < 0:
		case it.class == ast.ClassBin:
			out[i] = true
		case it.class == ast.ClassRel:
			n := items[next].class
			out[i] = n != ast.ClassRel && n != ast.ClassPunct
		}
		if i+1 < len(items) && items[i+1].brk < 0 {
			out[i] = false
		}
		if !it.space && it.brk == 0 {
			next = i
		}
	}
	return out
}

// wrap breaks the top-level list into lines of at most maxCols columns,
// greedily, and stacks them; the baseline is the first line's. Item
// positions are computed once (as packRaw would place them), so the work
// is linear in the length of the list.
func (t *typesetter) wrap(nodes []ast.Node, level, maxCols int) (*grid, error) {
	items, err := t.items(nodes, level)
	if err != nil {
		return nil, err
	}
	reclassifyBins(items)
	left := make([]int, len(items))
	right := make([]int, len(items))
	x, prev := 0, -1
	for i := range items {
		it := &items[i]
		if !it.space && prev >= 0 && spacedAt(level, items[prev], *it) && spaced(items[prev], *it) {
			x++
		}
		left[i] = x
		x += it.g.w
		right[i] = x
		if !it.space {
			prev = i
		}
	}
	brk := breakAfter(items)
	var lines []*grid
	start, last := 0, -1
	for i := range items {
		if !brk[i] && i != len(items)-1 {
			continue
		}
		if right[i]-left[start] > maxCols && last >= start {
			lines = append(lines, t.packRaw(items[start:last+1], level, 0, 0))
			start = last + 1
			for start < i && items[start].space {
				start++
			}
		}
		last = i
	}
	lines = append(lines, t.packRaw(items[start:], level, 0, 0))
	w, h := 0, 0
	for _, g := range lines {
		w, h = max(w, g.w), h+g.h()
	}
	first := lines[0]
	out := t.blank(w, first.asc, h-first.asc-1)
	y := 0
	for _, g := range lines {
		out.blit(g, 0, y+g.asc, false)
		y += g.h()
	}
	return out, nil
}

func one(class ast.Class, g *grid) []item { return []item{{class: class, g: g}} }

func scriptLevel(level int) int { return max(level, 1) + 1 }

func (t *typesetter) symbol(s string) string {
	if t.font == ast.FontDefault {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		b.WriteRune(symbols.TextAlphabet(r, t.font))
	}
	return b.String()
}

func (t *typesetter) node(n ast.Node, level int) ([]item, error) {
	switch n := n.(type) {
	case *ast.Symbol:
		return one(n.Class, t.text(t.symbol(n.Text))), nil
	case *ast.Group:
		g, err := t.list(n.Body, level)
		if err != nil {
			return nil, err
		}
		return one(n.Class, g), nil
	case *ast.Scripts:
		return t.scripts(n, level)
	case *ast.Op:
		if n.Body != nil {
			saved := t.opName
			t.opName = true
			g, err := t.list(n.Body, level)
			t.opName = saved
			if err != nil {
				return nil, err
			}
			return []item{{class: ast.ClassOp, g: g, word: true}}, nil
		}
		return []item{{class: ast.ClassOp, g: t.text(n.Text), word: !n.Symbol}}, nil
	case *ast.Frac:
		return t.frac(n, level)
	case *ast.Sqrt:
		return t.sqrt(n, level)
	case *ast.LeftRight:
		return t.leftRight(n, level)
	case *ast.Middle:
		return []item{{class: ast.ClassOrd, mid: n.Delim, isMid: true}}, nil
	case *ast.SizedDelim:
		h := n.Size // rows
		asc := h / 2
		return one(n.Class, t.delimiter(n.Delim, asc, h-1-asc)), nil
	case *ast.Accent:
		return t.accent(n, level)
	case *ast.Overline:
		b, err := t.list(n.Body, level)
		if err != nil {
			return nil, err
		}
		if n.Under {
			out := t.blank(b.w, b.asc, b.desc+1)
			out.blit(b, 0, b.asc, false)
			out.blit(t.repeat("‾", b.w), 0, b.asc+b.desc+1, false)
			return one(ast.ClassOrd, out), nil
		}
		out := t.blank(b.w, b.asc+1, b.desc)
		out.blit(b, 0, b.asc+1, false)
		out.blit(t.repeat("_", b.w), 0, 0, false)
		return one(ast.ClassOrd, out), nil
	case *ast.Font:
		saved := t.font
		t.font = n.Style
		its, err := t.items(n.Body, level)
		t.font = saved
		return its, err
	case *ast.Text:
		return one(ast.ClassOrd, t.text(n.Text)), nil
	case *ast.Space:
		cols := 0
		switch {
		case n.Mu >= 36:
			cols = 4
		case n.Mu >= 18:
			cols = 2
		case n.Mu >= 5, t.opName && n.Mu > 0:
			cols = 1
		}
		return []item{{space: true, g: t.repeat(" ", cols)}}, nil
	case *ast.Array:
		return t.array(n, level)
	case *ast.Color:
		// NOTE(katex-compat): colors are dropped in text output.
		return t.items(n.Body, level)
	case *ast.Style:
		l := level
		switch n.Style {
		case ast.StyleDisplay:
			l = 0
		case ast.StyleText:
			l = 1
		case ast.StyleScript, ast.StyleScriptScript:
			l = max(level, 2)
		}
		return t.items(n.Body, l)
	case *ast.Not:
		// A slash over a cell would need a combining character, which
		// terminals render unreliably.
		return nil, diag.Unsupported("\\not", n.Offset)
	case *ast.Phantom:
		b, err := t.list(n.Body, level)
		if err != nil {
			return nil, err
		}
		switch {
		case n.NoWidth:
			return one(ast.ClassOrd, t.blank(0, b.asc, b.desc)), nil
		case n.NoHeight:
			return one(ast.ClassOrd, t.blank(b.w, 0, 0)), nil
		}
		return one(ast.ClassOrd, t.blank(b.w, b.asc, b.desc)), nil
	case *ast.Smash, *ast.Lap, *ast.Raise, *ast.Reflect, *ast.Size:
		// NOTE(katex-compat): on a character grid these only show their
		// contents: overlaps, shifts, mirroring and sizes need pixels.
		return t.items(layoutBody(n), level)
	case *ast.ColorBox:
		if n.Frame == nil {
			return t.items(n.Body, level)
		}
		b, err := t.list(n.Body, level)
		if err != nil {
			return nil, err
		}
		return one(ast.ClassOrd, t.frame(b)), nil
	case *ast.Rule:
		// About two columns per em, as for spaces.
		cols := int(n.WMu/9 + 0.5)
		if n.HMu <= 0 || cols <= 0 {
			return []item{{space: true, g: t.repeat(" ", max(cols, 0))}}, nil
		}
		return one(ast.ClassOrd, t.repeat("█", cols)), nil
	case *ast.Choice:
		return t.items(n.Bodies[min(level, 3)], level)
	case *ast.Logo:
		return one(ast.ClassOrd, t.text(n.Name)), nil
	case *ast.Break:
		b := 1
		if !n.Allow {
			b = -1
		}
		return []item{{space: true, brk: b, g: t.text("")}}, nil
	case *ast.Enclose:
		b, err := t.list(n.Body, level)
		if err != nil {
			return nil, err
		}
		if n.Kind == "angl" {
			out := t.blank(b.w+1, b.asc+1, b.desc)
			out.blit(b, 0, b.asc+1, false)
			out.blit(t.text(strings.Repeat("─", b.w)+"┐"), 0, 0, false)
			for y := 1; y < out.h(); y++ {
				out.set(y, b.w, "│")
			}
			return one(ast.ClassOrd, out), nil
		}
		return one(ast.ClassOrd, t.hcat(t.text("∠"), b)), nil
	case *ast.Boxed:
		if n.Circle {
			return one(ast.ClassOrd, t.text(circledText(n.Body, func(b []ast.Node) string {
				g, err := t.list(b, level)
				if err != nil || !g.flat() {
					return "?"
				}
				return g.String()
			}))), nil
		}
		b, err := t.list(n.Body, level)
		if err != nil {
			return nil, err
		}
		return one(ast.ClassOrd, t.frame(b)), nil
	case *ast.Cancel:
		// A slash over cells would need combining characters that
		// terminals draw unreliably, so the command is spelled out, as in
		// the single-line output: cancel(x).
		b, err := t.list(n.Body, level)
		if err != nil {
			return nil, err
		}
		name := map[string]string{"/": "cancel", "\\": "bcancel", "X": "xcancel", "-": "sout"}[n.Kind]
		return one(ast.ClassOrd, t.hcat(t.text(name), t.fence(b, "(", ")"))), nil
	case *ast.Brace:
		return t.brace(n, level)
	case *ast.OverUnder:
		base, err := t.list(n.Base, level)
		if err != nil {
			return nil, err
		}
		var over, under *grid
		if n.Over != nil {
			if over, err = t.list(n.Over, scriptLevel(level)); err != nil {
				return nil, err
			}
		}
		if n.Under != nil {
			if under, err = t.list(n.Under, scriptLevel(level)); err != nil {
				return nil, err
			}
		}
		return one(n.Class, t.limits(base, over, under)), nil
	case *ast.XArrow:
		return t.xarrow(n, level)
	}
	return nil, diag.Unsupported("construct in text output", n.Pos())
}

// circledText writes \textcircled{x} as the circled character when there
// is one (ⓐ, ①), else as (x).
func circledText(body []ast.Node, plain func([]ast.Node) string) string {
	if len(body) == 1 {
		if tx, ok := body[0].(*ast.Text); ok && utf8.RuneCountInString(tx.Text) == 1 {
			r, _ := utf8.DecodeRuneInString(tx.Text)
			if c, ok := symbols.Circled(r); ok {
				return string(c)
			}
		}
	}
	return "(" + plain(body) + ")"
}

// layoutBody returns the body of a node that only moves or scales it.
func layoutBody(n ast.Node) []ast.Node {
	switch n := n.(type) {
	case *ast.Smash:
		return n.Body
	case *ast.Lap:
		return n.Body
	case *ast.Raise:
		return n.Body
	case *ast.Reflect:
		return n.Body
	case *ast.Size:
		return n.Body
	}
	return nil
}

// frame draws a box around b with one space of padding.
func (t *typesetter) frame(b *grid) *grid {
	out := t.blank(b.w+4, b.asc+1, b.desc+1)
	out.blit(b, 2, b.asc+1, false)
	last := out.h() - 1
	for x := 1; x < out.w-1; x++ {
		out.set(0, x, "─")
		out.set(last, x, "─")
	}
	for y := 1; y < last; y++ {
		out.set(y, 0, "│")
		out.set(y, out.w-1, "│")
	}
	out.set(0, 0, "┌")
	out.set(0, out.w-1, "┐")
	out.set(last, 0, "└")
	out.set(last, out.w-1, "┘")
	return out
}

// limits centers over and under above and below base.
func (t *typesetter) limits(base, over, under *grid) *grid {
	w := base.w
	if over != nil {
		w = max(w, over.w)
	}
	if under != nil {
		w = max(w, under.w)
	}
	asc, desc := base.asc, base.desc
	if over != nil {
		asc += over.h()
	}
	if under != nil {
		desc += under.h()
	}
	out := t.blank(w, asc, desc)
	out.blit(base, (w-base.w)/2, asc, false)
	if over != nil {
		out.blit(over, (w-over.w)/2, asc-base.asc-1-over.desc, false)
	}
	if under != nil {
		out.blit(under, (w-under.w)/2, asc+base.desc+1+under.asc, false)
	}
	return out
}

// unicodeScript converts a flat grid to superscript or subscript
// characters.
func unicodeScript(g *grid, sup bool) (string, bool) {
	if g == nil || !g.flat() {
		return "", false
	}
	table := symbols.Subscripts
	if sup {
		table = symbols.Superscripts
	}
	var b strings.Builder
	for _, r := range g.String() {
		m, ok := table[r]
		if !ok {
			return "", false
		}
		b.WriteRune(m)
	}
	return b.String(), b.Len() > 0
}

func leadingPrimes(sup []ast.Node) (int, []ast.Node) {
	for i, n := range sup {
		if s, ok := n.(*ast.Symbol); !ok || s.Text != "′" {
			return i, sup[i:]
		}
	}
	return len(sup), nil
}

func (t *typesetter) scripts(n *ast.Scripts, level int) ([]item, error) {
	var base item
	if n.Base != nil {
		its, err := t.node(n.Base, level)
		if err != nil {
			return nil, err
		}
		if len(its) == 1 {
			base = its[0]
		} else {
			base = item{class: ast.ClassOrd, g: t.pack(its, level, 0, 0)}
		}
	} else {
		base = item{class: ast.ClassOrd, g: t.text("")}
	}
	if base.isMid {
		base.g = t.delimiter(base.mid, 0, 0)
		base.isMid = false
	}
	sup, hasSup := n.Sup, n.HasSup
	if hasSup {
		if np, rest := leadingPrimes(sup); np > 0 {
			base.g = t.hcat(base.g, t.repeat("′", np))
			sup, hasSup = rest, len(rest) > 0
		}
	}
	sl := scriptLevel(level)
	var supG, subG *grid
	var err error
	if hasSup {
		if supG, err = t.list(sup, sl); err != nil {
			return nil, err
		}
	}
	if n.HasSub {
		if subG, err = t.list(n.Sub, sl); err != nil {
			return nil, err
		}
	}
	lim := false
	if op, ok := n.Base.(*ast.Op); ok {
		lim = op.Limits == ast.LimitsOn || op.Limits == ast.LimitsAuto && op.DefaultLimits && level == 0
	}
	if _, ok := n.Base.(*ast.Brace); ok {
		lim = true
	}
	if lim {
		return []item{{class: base.class, g: t.limits(base.g, supG, subG), word: base.word}}, nil
	}
	b := base.g
	us, okSub := unicodeScript(subG, false)
	up, okSup := unicodeScript(supG, true)
	if (okSub || subG == nil) && (okSup || supG == nil) {
		// Everything converts: x²ᵢ on the base rows.
		if scriptedBase(n.Base) {
			// (x²)³ and (√x)², not x²³ and √x².
			if b.flat() {
				b = t.hcat(t.text("("), b, t.text(")"))
			} else {
				b = t.fence(b, "(", ")")
			}
		}
		var sg, pg *grid
		w := 0
		if subG != nil {
			sg = t.text(us)
			w = max(w, sg.w)
		}
		if supG != nil {
			pg = t.text(up)
			w = max(w, pg.w)
		}
		if b.flat() && sg != nil && pg != nil {
			return []item{{class: base.class, g: t.hcat(b, sg, pg)}}, nil
		}
		out := t.blank(b.w+w, b.asc, b.desc)
		out.blit(b, 0, b.asc, false)
		if pg != nil {
			out.blit(pg, b.w, 0, false)
		}
		if sg != nil {
			out.blit(sg, b.w, b.h()-1, false)
		}
		return []item{{class: base.class, g: out}}, nil
	}
	// Stack the scripts above and below the base's right edge.
	w, asc, desc := 0, b.asc, b.desc
	if supG != nil {
		w = max(w, supG.w)
		asc = b.asc + supG.h()
	}
	if subG != nil {
		w = max(w, subG.w)
		desc = b.desc + subG.h()
	}
	out := t.blank(b.w+w, asc, desc)
	out.blit(b, 0, asc, false)
	if supG != nil {
		out.blit(supG, b.w, asc-b.asc-1-supG.desc, false)
	}
	if subG != nil {
		out.blit(subG, b.w, asc+b.desc+1+subG.asc, false)
	}
	return []item{{class: base.class, g: out}}, nil
}

func (t *typesetter) frac(n *ast.Frac, level int) ([]item, error) {
	l := level
	switch n.Style {
	case ast.StyleDisplay:
		l = 0
	case ast.StyleText:
		l = 1
	}
	inner := l + 1 // display → text, text → script, ...
	num, err := t.list(n.Num, inner)
	if err != nil {
		return nil, err
	}
	den, err := t.list(n.Den, inner)
	if err != nil {
		return nil, err
	}
	if !n.Bar {
		w := max(num.w, den.w)
		out := t.blank(w, num.h()-1, den.h())
		out.blit(num, (w-num.w)/2, num.asc, false)
		out.blit(den, (w-den.w)/2, num.h()+den.asc, false)
		return one(ast.ClassInner, t.fence(out, n.Left, n.Right)), nil
	}
	w := max(num.w, den.w) + 2
	out := t.blank(w, num.h(), den.h())
	out.blit(num, (w-num.w)/2, num.asc, false)
	out.blit(t.repeat("─", w), 0, num.h(), false)
	out.blit(den, (w-den.w)/2, num.h()+1+den.asc, false)
	if n.Left != "" || n.Right != "" {
		out = t.fence(out, n.Left, n.Right)
	}
	return one(ast.ClassInner, out), nil
}

func (t *typesetter) sqrt(n *ast.Sqrt, level int) ([]item, error) {
	body, err := t.list(n.Body, level)
	if err != nil {
		return nil, err
	}
	var sign *grid
	if n.Index == nil {
		sign = t.text("√")
	} else {
		idx, err := t.list(n.Index, scriptLevel(level))
		if err != nil {
			return nil, err
		}
		switch s := idx.String(); {
		case idx.flat() && s == "3":
			sign = t.text("∛")
		case idx.flat() && s == "4":
			sign = t.text("∜")
		default:
			if u, ok := unicodeScript(idx, true); ok {
				sign = t.text(u + "√")
			} else {
				// The index sits above and to the left of the sign, all of
				// its rows above the sign's row.
				sign = t.blank(idx.w+1, idx.h(), 0)
				sign.blit(idx, 0, idx.asc, false)
				sign.set(idx.h(), idx.w, "√")
			}
		}
	}
	// A single character or number needs no overline: √2, √x.
	if body.flat() && simpleRadicand(n.Body) {
		return one(ast.ClassOrd, t.hcat(sign, body)), nil
	}
	// Overline row above the body; for tall bodies a vertical stroke
	// joins it to the sign at the bottom.
	asc := max(body.asc+1, sign.asc-body.desc)
	top := asc - body.asc - 1 // overline row
	out := t.blank(sign.w+body.w, asc, body.desc)
	out.blit(body, sign.w, asc, false)
	out.blit(t.repeat("─", body.w), sign.w, top, false)
	for y := top + 1; y < out.h()-1; y++ {
		out.set(y, sign.w-1, "│")
	}
	out.blit(sign, 0, out.h()-1-sign.desc, true)
	return one(ast.ClassOrd, out), nil
}

// simpleRadicand reports whether the body is one symbol or a number.
func simpleRadicand(nodes []ast.Node) bool {
	digits := true
	for _, n := range nodes {
		s, ok := n.(*ast.Symbol)
		if !ok {
			return false
		}
		if !(len(s.Text) == 1 && (s.Text[0] >= '0' && s.Text[0] <= '9' || s.Text[0] == '.')) {
			digits = false
		}
	}
	return len(nodes) == 1 || digits
}

func (t *typesetter) leftRight(n *ast.LeftRight, level int) ([]item, error) {
	items, err := t.items(n.Body, level)
	if err != nil {
		return nil, err
	}
	asc, desc := 0, 0
	for _, it := range items {
		if !it.isMid {
			asc, desc = max(asc, it.g.asc), max(desc, it.g.desc)
		}
	}
	body := t.pack(items, level, asc, desc)
	return one(ast.ClassInner, t.fence(body, n.Left, n.Right)), nil
}

// fence puts delimiters as tall as b around it.
func (t *typesetter) fence(b *grid, left, right string) *grid {
	var parts []*grid
	if left != "" {
		parts = append(parts, t.delimiter(left, b.asc, b.desc))
	}
	parts = append(parts, b)
	if right != "" {
		parts = append(parts, t.delimiter(right, b.asc, b.desc))
	}
	return t.hcat(parts...)
}

// Pieces of tall delimiters: top, extension, middle, bottom.
var delimPieces = map[string][4]string{
	"(": {"⎛", "⎜", "⎜", "⎝"}, ")": {"⎞", "⎟", "⎟", "⎠"},
	"[": {"⎡", "⎢", "⎢", "⎣"}, "]": {"⎤", "⎥", "⎥", "⎦"},
	"{": {"⎧", "⎪", "⎨", "⎩"}, "}": {"⎫", "⎪", "⎬", "⎭"},
	"|": {"│", "│", "│", "│"}, "‖": {"‖", "‖", "‖", "‖"},
	"⌊": {"⎢", "⎢", "⎢", "⎣"}, "⌋": {"⎥", "⎥", "⎥", "⎦"},
	"⌈": {"⎡", "⎢", "⎢", "⎢"}, "⌉": {"⎤", "⎥", "⎥", "⎥"},
	"/": {"╱", "╱", "╱", "╱"}, "∖": {"╲", "╲", "╲", "╲"},
	"↑": {"↑", "│", "│", "│"}, "↓": {"│", "│", "│", "↓"}, "↕": {"↑", "│", "│", "↓"},
	"⇑": {"⇑", "║", "║", "║"}, "⇓": {"║", "║", "║", "⇓"}, "⇕": {"⇑", "║", "║", "⇓"},
}

// delimiter draws d spanning asc rows above and desc rows below the
// baseline.
func (t *typesetter) delimiter(d string, asc, desc int) *grid {
	h := asc + 1 + desc
	if d == "" {
		return t.blank(0, asc, desc)
	}
	if h == 1 {
		return t.text(d)
	}
	out := t.blank(1, asc, desc)
	switch d {
	case "⟨", "⟩":
		// Two diagonals meeting at the middle.
		up, down := "╱", "╲"
		if d == "⟩" {
			up, down = down, up
		}
		for y := 0; y < h; y++ {
			if y < h/2 || (h%2 == 1 && y == h/2) {
				out.set(y, 0, up)
			} else {
				out.set(y, 0, down)
			}
		}
		if h%2 == 1 {
			out.set(h/2, 0, map[string]string{"⟨": "<", "⟩": ">"}[d])
		}
		return out
	}
	p, ok := delimPieces[d]
	if !ok {
		// Unknown tall delimiter: repeat it.
		for y := 0; y < h; y++ {
			out.set(y, 0, d)
		}
		return out
	}
	for y := 0; y < h; y++ {
		out.set(y, 0, p[1])
	}
	out.set(0, 0, p[0])
	out.set(h-1, 0, p[3])
	if d == "{" || d == "}" {
		if h > 2 {
			out.set(h/2, 0, p[2])
		}
	}
	return out
}

var accentText = map[string]string{
	"hat": "^", "widehat": "^", "check": "ˇ", "widecheck": "ˇ", "tilde": "~", "widetilde": "~",
	"acute": "´", "grave": "`", "dot": "˙", "ddot": "¨", "dddot": "⋯", "breve": "˘", "bar": "_",
	"vec": "→", "mathring": "°", "overrightarrow": "→", "overleftarrow": "←", "overleftrightarrow": "↔",
}

func (t *typesetter) accent(n *ast.Accent, level int) ([]item, error) {
	b, err := t.list(n.Body, level)
	if err != nil {
		return nil, err
	}
	mark := accentText[n.Name]
	if mark == "" {
		mark = symbols.AccentText[n.Char]
	}
	var row *grid
	switch {
	case n.Name == "bar":
		row = t.repeat("_", b.w)
	case n.Wide && b.w > 1:
		row = t.text(stretchRow(mark, b.w))
	default:
		row = t.text(mark)
	}
	w := max(b.w, row.w)
	if n.Under {
		out := t.blank(w, b.asc, b.desc+1)
		out.blit(b, (w-b.w)/2, b.asc, false)
		out.blit(row, (w-row.w)/2, out.h()-1, false)
		return one(ast.ClassOrd, out), nil
	}
	out := t.blank(w, b.asc+1, b.desc)
	out.blit(b, (w-b.w)/2, b.asc+1, false)
	out.blit(row, (w-row.w)/2, 0, false)
	return one(ast.ClassOrd, out), nil
}

// stretchRow draws a wide accent mark w columns wide: arrows and harpoons
// get a shaft, arcs get corners, other marks repeat.
func stretchRow(mark string, w int) string {
	shaft := strings.Repeat("─", max(w-1, 0))
	switch mark {
	case "→", "⇀":
		return shaft + mark
	case "←", "↼":
		return mark + shaft
	case "⇒":
		return strings.Repeat("═", w-1) + "⇒"
	case "↔":
		return "←" + strings.Repeat("─", max(w-2, 0)) + "→"
	case "⏠":
		return "╭" + strings.Repeat("─", max(w-2, 0)) + "╮"
	case "⏡":
		return "╰" + strings.Repeat("─", max(w-2, 0)) + "╯"
	case "‾":
		return "├" + strings.Repeat("─", max(w-2, 0)) + "┤"
	case "^", "~", "ˇ":
		return strings.Repeat(mark, w)
	}
	return mark
}

func (t *typesetter) brace(n *ast.Brace, level int) ([]item, error) {
	b, err := t.list(n.Body, level)
	if err != nil {
		return nil, err
	}
	var row *grid
	switch {
	case b.w < 3 && n.Square:
		row = t.text(map[bool]string{false: "⎴", true: "⎵"}[n.Under])
	case b.w < 3:
		row = t.text(map[bool]string{false: "⏞", true: "⏟"}[n.Under])
	default:
		l, m, r := "╭", "┴", "╮"
		if n.Under {
			l, m, r = "╰", "┬", "╯"
		}
		if n.Square {
			l, m, r = "┌", "─", "┐"
			if n.Under {
				l, r = "└", "┘"
			}
		}
		left := (b.w - 3) / 2
		row = t.text(l + strings.Repeat("─", left) + m + strings.Repeat("─", b.w-3-left) + r)
	}
	var out *grid
	if n.Under {
		out = t.blank(b.w, b.asc, b.desc+1)
		out.blit(b, 0, b.asc, false)
		out.blit(row, (b.w-row.w)/2, out.h()-1, false)
	} else {
		out = t.blank(b.w, b.asc+1, b.desc)
		out.blit(b, 0, b.asc+1, false)
		out.blit(row, (b.w-row.w)/2, 0, false)
	}
	return one(ast.ClassOrd, out), nil
}

func (t *typesetter) xarrow(n *ast.XArrow, level int) ([]item, error) {
	sl := scriptLevel(level)
	over, err := t.list(n.Over, sl)
	if err != nil {
		return nil, err
	}
	var under *grid
	if n.Under != nil {
		if under, err = t.list(n.Under, sl); err != nil {
			return nil, err
		}
	}
	w := over.w + 2
	if under != nil {
		w = max(w, under.w+2)
	}
	w = max(w, 2)
	shaft := "─"
	switch n.Arrow {
	case '⇒', '⇐', '⇔', '=':
		shaft = "═"
	}
	var s string
	switch n.Arrow {
	case '←', '⇐', '↩', '↞', '↼', '↽':
		s = string(n.Arrow) + strings.Repeat(shaft, w-1)
	case '↔', '⇔':
		l, r := "←", "→"
		if n.Arrow == '⇔' {
			l, r = "⇐", "⇒"
		}
		s = l + strings.Repeat(shaft, w-2) + r
	case '=':
		s = strings.Repeat(shaft, w)
	case '↦':
		s = "├" + strings.Repeat(shaft, w-2) + "→"
	default:
		s = strings.Repeat(shaft, w-1) + string(n.Arrow)
	}
	arrow := t.text(s)
	if over.w == 0 {
		over = nil
	}
	return one(ast.ClassRel, t.limits(arrow, over, under)), nil
}

func (t *typesetter) array(n *ast.Array, outer int) ([]item, error) {
	tagRight := t.tagRight
	t.tagRight = 0
	level := 1
	switch n.Style {
	case ast.StyleNone:
		level = outer
	case ast.StyleDisplay:
		level = 0
	case ast.StyleScript, ast.StyleScriptScript:
		level = max(outer, 2)
	}
	ncols := 0
	for _, r := range n.Rows {
		ncols = max(ncols, len(r.Cells))
	}
	cells := make([][]*grid, len(n.Rows))
	colW := make([]int, ncols)
	rowAsc := make([]int, len(n.Rows))
	rowDesc := make([]int, len(n.Rows))
	tall := false
	for i, r := range n.Rows {
		cells[i] = make([]*grid, ncols)
		for j := 0; j < ncols; j++ {
			if err := t.g.Tick(); err != nil {
				return nil, err
			}
			var cell []ast.Node
			if j < len(r.Cells) {
				cell = r.Cells[j]
			}
			c, err := t.list(cell, level)
			if err != nil {
				return nil, err
			}
			if n.AlignPairs && j%2 == 1 {
				// The leading empty group makes "= b" start with a space.
				c = t.trimLeft(c)
			}
			cells[i][j] = c
			colW[j] = max(colW[j], c.w)
			rowAsc[i], rowDesc[i] = max(rowAsc[i], c.asc), max(rowDesc[i], c.desc)
			tall = tall || !c.flat()
		}
	}
	gap := 0
	if tall && !n.AlignPairs && n.Env != "lines" {
		gap = 1
	}
	sep := 2
	if n.ColSepEm < 0.9 {
		sep = 1
	}
	// Column positions.
	colX := make([]int, ncols)
	x := 0
	rulesBefore := func(j int) int {
		if j < len(n.Cols) {
			return n.Cols[j].RuleBefore
		}
		return 0
	}
	type vrule struct {
		x      int
		dashed bool
	}
	var vrules []vrule
	dashedBefore := func(j int) bool { return j < len(n.Cols) && n.Cols[j].Dashed }
	for j := 0; j < ncols; j++ {
		if j > 0 {
			s := sep
			if n.AlignPairs {
				s = map[bool]int{true: 1, false: 4}[j%2 == 1]
				if n.ColSepEm == 0 && j%2 == 0 {
					s = 1 // alignat: pairs follow each other
				}
			}
			if k := rulesBefore(j); k > 0 {
				x += s / 2
				for r := 0; r < k; r++ {
					vrules = append(vrules, vrule{x, dashedBefore(j)})
					x++
				}
				x += s - s/2
			} else {
				x += s
			}
		} else {
			for r := 0; r < rulesBefore(0); r++ {
				vrules = append(vrules, vrule{x, dashedBefore(0)})
				x += 2
			}
		}
		colX[j] = x
		x += colW[j]
	}
	for r := 0; r < n.RuleAfter; r++ {
		vrules = append(vrules, vrule{x + 1, n.RuleAfterDashed})
		x += 2
	}
	width := x
	// Rows.
	rowsTotal := 0
	baseRow := make([]int, len(n.Rows))
	type hrule struct {
		y      int
		dashed bool
	}
	var hrules []hrule
	for i := range n.Rows {
		if i > 0 {
			rowsTotal += gap
		}
		for k := 0; k < n.Rows[i].HLineBefore; k++ {
			hrules = append(hrules, hrule{rowsTotal, n.Rows[i].HLineDashed})
			rowsTotal++
		}
		baseRow[i] = rowsTotal + rowAsc[i]
		rowsTotal += rowAsc[i] + 1 + rowDesc[i]
	}
	for k := 0; k < n.HLineAfter; k++ {
		hrules = append(hrules, hrule{rowsTotal, n.HLineAfterDashed})
		rowsTotal++
	}
	if rowsTotal == 0 {
		rowsTotal = 1
	}
	asc := (rowsTotal - 1) / 2
	out := t.blank(width, asc, rowsTotal-1-asc)
	align := func(j int) ast.Align {
		if n.AlignPairs {
			return map[bool]ast.Align{true: ast.AlignRight, false: ast.AlignLeft}[j%2 == 0]
		}
		if len(n.Cols) == 0 {
			return ast.AlignCenter
		}
		if j < len(n.Cols) {
			return n.Cols[j].Align
		}
		return n.Cols[len(n.Cols)-1].Align
	}
	for i := range n.Rows {
		for j := 0; j < ncols; j++ {
			c := cells[i][j]
			cx := colX[j]
			switch align(j) {
			case ast.AlignCenter:
				cx += (colW[j] - c.w) / 2
			case ast.AlignRight:
				cx += colW[j] - c.w
			}
			out.blit(c, cx, baseRow[i], false)
		}
	}
	for _, r := range hrules {
		c := "─"
		if r.dashed {
			c = "┄"
		}
		for x := 0; x < width; x++ {
			out.set(r.y, x, c)
		}
	}
	for _, r := range vrules {
		if r.x >= width {
			continue
		}
		c := "│"
		if r.dashed {
			c = "┆"
		}
		for y := 0; y < rowsTotal; y++ {
			if h := out.get(y, r.x); h == "─" || h == "┄" {
				out.set(y, r.x, "┼")
			} else {
				out.set(y, r.x, c)
			}
		}
	}
	var tags []*grid
	tagW := 0
	for _, r := range n.Rows {
		var tg *grid
		if r.Tag != nil {
			var err error
			if tg, err = t.tag(r.Tag); err != nil {
				return nil, err
			}
			tagW = max(tagW, tg.w)
		}
		tags = append(tags, tg)
	}
	if tagW > 0 {
		gap := 2
		if tagRight > 0 {
			gap = max(gap, tagRight-width-tagW)
		}
		wide := t.blank(width+gap+tagW, out.asc, out.desc)
		wide.blit(out, 0, out.asc, false)
		for i, tg := range tags {
			if tg != nil {
				// Right-aligned, so tags of different widths end together.
				wide.blit(tg, width+gap+tagW-tg.w, baseRow[i], false)
			}
		}
		out = wide
	}
	cls := ast.ClassOrd
	if n.Left != "" || n.Right != "" {
		cls = ast.ClassInner
		out = t.fence(out, n.Left, n.Right)
	}
	return one(cls, out), nil
}

// trimLeft removes leading blank columns.
func (t *typesetter) trimLeft(g *grid) *grid {
	k := 0
	for k < g.w {
		empty := true
		for _, row := range g.rows {
			if row[k] != " " {
				empty = false
				break
			}
		}
		if !empty {
			break
		}
		k++
	}
	if k == 0 {
		return g
	}
	out := &grid{w: g.w - k, asc: g.asc, desc: g.desc, rows: make([][]string, len(g.rows))}
	for i, row := range g.rows {
		out.rows[i] = row[k:]
	}
	return out
}

// scriptedBase reports whether a Unicode script attached to n would be
// misread: n ends in a script or a radical (x², √x), or is a bare fraction
// or matrix whose top row the script would land next to.
func scriptedBase(n ast.Node) bool {
	last := func(body []ast.Node) bool { return len(body) > 0 && scriptedBase(body[len(body)-1]) }
	switch n := n.(type) {
	case *ast.Scripts, *ast.Sqrt:
		return true
	case *ast.Frac:
		return n.Left == "" && n.Right == ""
	case *ast.Array:
		return n.Left == "" && n.Right == ""
	case *ast.Group:
		return last(n.Body)
	case *ast.Font:
		return last(n.Body)
	case *ast.Color:
		return last(n.Body)
	case *ast.Style:
		return last(n.Body)
	}
	return false
}
