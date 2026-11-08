// Package linear writes a formula as a single line of text: x², aᵢ,
// (a+b)/c, √(x+1). It never fails: constructs without a compact form are
// written in a functional notation.
package linear

import (
	"strings"
	"unicode/utf8"

	"github.com/lea-151107/dal/internal/ast"
	"github.com/lea-151107/dal/internal/symbols"
)

// Options controls the output.
type Options struct {
	// ASCII disables Unicode superscripts and subscripts (x^2 instead of
	// x²) and writes − and ∗ as - and *.
	ASCII bool
}

// Write returns the single-line form of e.
func Write(e *ast.Expr, opt Options) string {
	w := &writer{opt: opt}
	s := w.list(e.Body, 0)
	if e.Tag != nil {
		s += "  " + w.tag(e.Tag)
	}
	return strings.TrimSpace(s)
}

type writer struct {
	opt    Options
	font   ast.FontStyle
	opName bool // inside an \operatorname body, where any space shows
}

// item is one atom of a list with its class, used for spacing.
type item struct {
	text     string
	class    ast.Class
	space    bool // explicit space; no automatic spacing around it
	compound bool // needs parentheses when used as a script base or fraction part
	// scripted marks text that ends in a script or a radical sign's reach
	// (x², √x): another script on it needs parentheses, or x²³ and √x²
	// would read as x^23 and √(x²).
	scripted bool
	// frac marks a linearized fraction (a/b): next to another term it needs
	// parentheses, or a/bc/d would read as a/(bc)/d.
	frac bool
	// named marks a spelled-out command (cancel(x)): it is kept apart
	// from a neighbouring term, or cancel(x)y would run together.
	named bool
}

func (w *writer) tag(t *ast.Tag) string {
	s := w.list(t.Body, 0)
	if t.Star {
		return s
	}
	return "(" + s + ")"
}

// list writes a math list. level 0 is the main formula; inside scripts no
// spaces are added, except after an operator.
func (w *writer) list(nodes []ast.Node, level int) string {
	items := w.items(nodes, level)
	return join(items, level)
}

func (w *writer) items(nodes []ast.Node, level int) []item {
	var items []item
	for _, n := range nodes {
		for _, it := range w.node(n, level) {
			// Consecutive digits form one number.
			if k := len(items) - 1; k >= 0 && isNumber(it.text) && isNumber(items[k].text) &&
				!it.space && !items[k].space && it.class == ast.ClassOrd && items[k].class == ast.ClassOrd {
				items[k].text += it.text
				continue
			}
			items = append(items, it)
		}
	}
	reclassifyBins(items)
	return items
}

// reclassifyBins applies TeX's rules 5 and 6: a binary operator with no
// left operand, or followed by a relation, closing or punctuation, is
// ordinary.
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

func join(items []item, level int) string {
	// glued[i]: item i follows item i-1 with nothing in between.
	glued := make([]bool, len(items))
	prev := -1
	for i, it := range items {
		if prev >= 0 && prev == i-1 && !it.space {
			glued[i] = !(spacedAt(level, items[prev], it) && apart(items[prev], it))
		}
		if !it.space {
			prev = i
		} else {
			prev = -1
		}
	}
	var b strings.Builder
	for i, it := range items {
		if i > 0 && !glued[i] && !it.space && !items[i-1].space && spacedAt(level, items[i-1], it) && apart(items[i-1], it) {
			b.WriteByte(' ')
		}
		text := it.text
		if it.frac {
			// A sign before the fraction (−a/b) or a space at the boundary
			// (text like " where ") already separates it. Items with no
			// text ({}) are looked through, and the thin space around an
			// operator does not separate: 1/n ∑ would read as 1/(n ∑).
			// After an operator it does, as in ∑ 1/i² and sin x/2.
			touch := func(k int) bool {
				return k > 0 && !items[k].space && !items[k-1].space &&
					(glued[k] || items[k].class == ast.ClassOp && opSpace(items[k-1].class, items[k].class))
			}
			j := i - 1
			for j >= 0 && touch(j+1) && items[j].text == "" {
				j--
			}
			left := j >= 0 && touch(j+1) && items[j].class != ast.ClassOpen && !isSign(items[j].text) &&
				!strings.HasSuffix(items[j].text, " ")
			k := i + 1
			for k < len(items) && touch(k) && items[k].text == "" {
				k++
			}
			right := k < len(items) && touch(k) && items[k].class != ast.ClassClose &&
				items[k].class != ast.ClassPunct && !strings.HasPrefix(items[k].text, " ")
			if left || right {
				text = "(" + text + ")"
			}
		}
		b.WriteString(text)
	}
	return b.String()
}

// spacedAt reports whether a space between a and b is written at level:
// only at the top level, except the thin spaces around an operator,
// which TeX keeps in scripts too (x^{\sin x} is not x^(sinx)).
func spacedAt(level int, a, b item) bool {
	return level == 0 || opSpace(a.class, b.class)
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

// apart reports whether a space separates a and b at the top level.
func apart(a, b item) bool {
	if a.named && b.class != ast.ClassClose && b.class != ast.ClassPunct ||
		b.named && a.class != ast.ClassOpen {
		return true
	}
	return spaced(a.class, b.class)
}

func spaced(a, b ast.Class) bool {
	switch {
	case a == ast.ClassRel && b == ast.ClassRel:
		return false
	case a == ast.ClassRel || b == ast.ClassRel:
		return a != ast.ClassOpen && b != ast.ClassClose && b != ast.ClassPunct
	case a == ast.ClassBin || b == ast.ClassBin:
		return true
	case a == ast.ClassPunct:
		return true
	case a == ast.ClassOp:
		return b == ast.ClassOrd || b == ast.ClassOp || b == ast.ClassInner
	case b == ast.ClassOp:
		// The thin space before an operator: "n log n", not "nlog n".
		return a == ast.ClassOrd || a == ast.ClassClose || a == ast.ClassInner
	}
	return false
}

func (w *writer) symbol(s string) string {
	if w.opt.ASCII {
		s = strings.NewReplacer("−", "-", "∗", "*", "′", "'").Replace(s)
	}
	if w.font == ast.FontDefault {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		b.WriteRune(symbols.TextAlphabet(r, w.font))
	}
	return b.String()
}

// wrap parenthesizes s when it is made of several atoms.
func wrap(s string, compound bool) string {
	if !compound {
		return s
	}
	return "(" + s + ")"
}

func isNumber(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r == '.') {
			return false
		}
	}
	return true
}

// unicodeScript converts s to Unicode superscripts or subscripts; ok is
// false when a character has no such form.
func unicodeScript(s string, sup bool) (string, bool) {
	table := symbols.Subscripts
	if sup {
		table = symbols.Superscripts
	}
	var b strings.Builder
	for _, r := range s {
		m, found := table[r]
		if !found {
			return "", false
		}
		b.WriteRune(m)
	}
	return b.String(), s != ""
}

func linearScript(s string, sup bool) string {
	mark := "_"
	if sup {
		mark = "^"
	}
	if utf8.RuneCountInString(s) == 1 {
		return mark + s
	}
	return mark + "(" + s + ")"
}

func (w *writer) scripts(base string, compound bool, n *ast.Scripts, level int) string {
	if _, op := n.Base.(*ast.Op); n.Base != nil && !op {
		base = wrap(base, compound)
	}
	var b strings.Builder
	b.WriteString(base)
	var sub, sup string
	if n.HasSub {
		sub = w.list(n.Sub, level+1)
	}
	if n.HasSup {
		sup = w.list(n.Sup, level+1)
		// Primes are written as they are, before the rest of the script.
		rest := strings.TrimLeft(sup, "′'")
		b.WriteString(sup[:len(sup)-len(rest)])
		sup = rest
	}
	if !w.opt.ASCII {
		// Use Unicode only when every script converts, so that a reader
		// never has to tell raised characters from linearized ones.
		us, okSub := unicodeScript(sub, false)
		up, okSup := unicodeScript(sup, true)
		if (okSub || sub == "") && (okSup || sup == "") {
			b.WriteString(us)
			b.WriteString(up)
			return b.String()
		}
	}
	if sub != "" || n.HasSub && sup == "" {
		b.WriteString(linearScript(sub, false))
	}
	if sup != "" {
		b.WriteString(linearScript(sup, true))
	}
	return b.String()
}

func (w *writer) node(n ast.Node, level int) []item {
	switch n := n.(type) {
	case *ast.Symbol:
		return []item{{text: w.symbol(n.Text), class: n.Class}}
	case *ast.Group:
		inner := w.items(n.Body, level)
		s := join(inner, level)
		if len(inner) == 0 {
			return []item{{text: "", class: n.Class}}
		}
		one := len(inner) == 1
		return []item{{text: s, class: n.Class, compound: !one, scripted: one && inner[0].scripted, frac: one && inner[0].frac}}
	case *ast.Scripts:
		var base item
		if n.Base != nil {
			base = merge(w.node(n.Base, level), level)
		}
		return []item{{text: w.scripts(base.text, base.compound || base.scripted, n, level), class: base.class, scripted: true}}
	case *ast.Op:
		if n.Body != nil {
			saved := w.opName
			w.opName = true
			s := w.list(n.Body, level)
			w.opName = saved
			return []item{{text: s, class: ast.ClassOp}}
		}
		return []item{{text: w.symbol(n.Text), class: ast.ClassOp}}
	case *ast.Frac:
		num := merge(w.items(n.Num, level), level)
		den := merge(w.items(n.Den, level), level)
		if !n.Bar {
			if n.Left == "(" && n.Right == ")" {
				return []item{{text: "(" + num.text + " choose " + den.text + ")", class: ast.ClassOrd}}
			}
			l, r := n.Left, n.Right
			if l == "" && r == "" {
				l, r = "(", ")"
			}
			return []item{{text: l + num.text + " atop " + den.text + r, class: ast.ClassOrd}}
		}
		part := func(it item) string {
			return wrap(it.text, it.compound || it.frac || strings.ContainsRune(it.text, ' '))
		}
		if n.Left != "" || n.Right != "" {
			return []item{{text: n.Left + part(num) + "/" + part(den) + n.Right, class: ast.ClassOrd}}
		}
		return []item{{text: part(num) + "/" + part(den), class: ast.ClassOrd, compound: true, frac: true}}
	case *ast.Sqrt:
		body := merge(w.items(n.Body, level), level)
		s := "√" + wrap(body.text, body.compound)
		if n.Index != nil {
			idx := w.list(n.Index, level+1)
			switch idx {
			case "3":
				s = "∛" + wrap(body.text, body.compound)
			case "4":
				s = "∜" + wrap(body.text, body.compound)
			default:
				if sup, ok := unicodeScript(idx, true); ok && !w.opt.ASCII {
					s = sup + s
				} else {
					s = "root(" + idx + ", " + body.text + ")"
				}
			}
		}
		return []item{{text: s, class: ast.ClassOrd, scripted: true}}
	case *ast.LeftRight:
		var b strings.Builder
		b.WriteString(n.Left)
		b.WriteString(w.list(n.Body, level))
		b.WriteString(n.Right)
		return []item{{text: b.String(), class: ast.ClassInner}}
	case *ast.Middle:
		return []item{{text: n.Delim, class: ast.ClassRel}}
	case *ast.SizedDelim:
		return []item{{text: n.Delim, class: n.Class}}
	case *ast.Accent:
		return []item{{text: n.Name + "(" + w.list(n.Body, level) + ")", class: ast.ClassOrd}}
	case *ast.Overline:
		name := "overline"
		if n.Under {
			name = "underline"
		}
		return []item{{text: name + "(" + w.list(n.Body, level) + ")", class: ast.ClassOrd}}
	case *ast.Font:
		saved := w.font
		w.font = n.Style
		s := w.items(n.Body, level)
		w.font = saved
		return s
	case *ast.Text:
		return []item{{text: n.Text, class: ast.ClassOrd}}
	case *ast.Space:
		switch {
		case n.Mu >= 36:
			return []item{{text: "  ", space: true}}
		case n.Mu >= 5, w.opName && n.Mu > 0:
			return []item{{text: " ", space: true}}
		}
		return nil
	case *ast.Array:
		return []item{{text: w.array(n), class: ast.ClassInner}}
	case *ast.Color:
		return w.items(n.Body, level)
	case *ast.Style:
		return w.items(n.Body, level)
	case *ast.Not:
		it := merge(w.items(n.Body, level), level)
		if utf8.RuneCountInString(it.text) == 1 {
			// NOTE(katex-compat): U+0338 COMBINING LONG SOLIDUS OVERLAY; one
			// line of text has no other way to strike a character through.
			it.text += "\u0338"
		} else {
			it.text = "not(" + it.text + ")"
			it.compound = false
		}
		return []item{it}
	case *ast.Phantom:
		return nil
	case *ast.Smash:
		return w.items(n.Body, level)
	case *ast.Lap:
		return w.items(n.Body, level)
	case *ast.Raise:
		return w.items(n.Body, level)
	case *ast.ColorBox:
		return w.items(n.Body, level)
	case *ast.Reflect:
		return w.items(n.Body, level)
	case *ast.Size:
		return w.items(n.Body, level)
	case *ast.Rule:
		return nil
	case *ast.Break:
		return nil
	case *ast.Logo:
		return []item{{text: n.Name, class: ast.ClassOrd}}
	case *ast.Enclose:
		body := merge(w.items(n.Body, level), level)
		if n.Kind == "phase" {
			return []item{{text: "∠" + wrap(body.text, body.compound), class: ast.ClassOrd}}
		}
		return []item{{text: "angl(" + body.text + ")", class: ast.ClassOrd, named: true}}
	case *ast.Choice:
		// The single line is set in text style; scripts one level down.
		return w.items(n.Bodies[min(level+1, 3)], level)
	case *ast.Boxed:
		if n.Circle {
			if len(n.Body) == 1 {
				if tx, ok := n.Body[0].(*ast.Text); ok && utf8.RuneCountInString(tx.Text) == 1 {
					r, _ := utf8.DecodeRuneInString(tx.Text)
					if c, ok := symbols.Circled(r); ok {
						return []item{{text: string(c), class: ast.ClassOrd}}
					}
				}
			}
			return []item{{text: "(" + w.list(n.Body, level) + ")", class: ast.ClassOrd}}
		}
		return w.items(n.Body, level)
	case *ast.Cancel:
		// The strike-through carries meaning, so it is spelled out like
		// the command; a combining overlay is easy to miss in a terminal.
		it := merge(w.items(n.Body, level), level)
		name := map[string]string{"/": "cancel", "\\": "bcancel", "X": "xcancel", "-": "sout"}[n.Kind]
		return []item{{text: name + "(" + it.text + ")", class: ast.ClassOrd, named: true}}
	case *ast.Brace:
		it := merge(w.items(n.Body, level), level)
		it.class = ast.ClassOrd
		return []item{it}
	case *ast.OverUnder:
		base := merge(w.items(n.Base, level), level)
		s := &ast.Scripts{Sup: n.Over, HasSup: n.Over != nil, Sub: n.Under, HasSub: n.Under != nil, Base: n}
		return []item{{text: w.scripts(base.text, base.compound || base.scripted, s, level), class: n.Class, scripted: true}}
	case *ast.XArrow:
		s := &ast.Scripts{Sup: n.Over, HasSup: len(n.Over) > 0, Sub: n.Under, HasSub: len(n.Under) > 0}
		return []item{{text: string(n.Arrow) + w.scripts("", false, s, level), class: ast.ClassRel}}
	}
	return nil
}

// merge joins items into one, remembering whether it was compound.
func merge(items []item, level int) item {
	if len(items) == 1 {
		return items[0]
	}
	cls := ast.ClassOrd
	return item{text: join(items, level), class: cls, compound: len(items) > 1}
}

func (w *writer) array(a *ast.Array) string {
	var rows []string
	for _, r := range a.Rows {
		var cells []string
		for _, c := range r.Cells {
			if c := strings.TrimSpace(w.list(c, 0)); c != "" || !a.AlignPairs {
				cells = append(cells, c)
			}
		}
		sep := ", "
		if a.AlignPairs {
			sep = " "
		}
		row := strings.Join(cells, sep)
		if r.Tag != nil {
			row += "  " + w.tag(r.Tag)
		}
		rows = append(rows, row)
	}
	return a.Left + strings.Join(rows, "; ") + a.Right
}

func isSign(s string) bool {
	switch s {
	case "−", "-", "+", "±", "∓":
		return true
	}
	return false
}
