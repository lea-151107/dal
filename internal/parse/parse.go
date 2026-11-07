package parse

import (
	"fmt"
	"image/color"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/lea/dal/internal/ast"
	"github.com/lea/dal/internal/diag"
	"github.com/lea/dal/internal/guard"
	"github.com/lea/dal/internal/symbols"
)

// Limits bounds the work of one Parse call. All fields must be positive.
type Limits struct {
	MaxInputBytes int
	MaxNodes      int
	MaxDepth      int
	MaxExpansions int
}

// Parse parses src.
func Parse(src string, lim Limits) (*ast.Expr, error) {
	if len(src) > lim.MaxInputBytes {
		return nil, fmt.Errorf("%w: input of %d bytes exceeds limit %d", guard.ErrTooLarge, len(src), lim.MaxInputBytes)
	}
	toks, err := lex(src)
	if err != nil {
		return nil, err
	}
	p := &parser{
		e:      newExpander(toks, len(src), lim.MaxExpansions),
		lim:    lim,
		primes: map[*ast.Scripts]bool{},
	}
	expr := &ast.Expr{}
	p.tag = &expr.Tag
	body, end, err := p.list()
	if err != nil {
		return nil, err
	}
	if isRowEnd(end) {
		// \\ or \newline outside any environment breaks the formula into
		// lines, as in KaTeX.
		info := envs["lines"]
		p.tag = nil
		a, err := p.rows(end, newArray(0, "lines", info), info, body)
		if err != nil {
			return nil, err
		}
		body, end = []ast.Node{a}, token{kind: tEOF}
		p.tag = &expr.Tag
	}
	if end.kind != tEOF {
		return nil, p.unexpected(end)
	}
	for _, a := range p.tagged {
		if len(body) != 1 || body[0] != ast.Node(a) {
			return nil, diag.Syntaxf(a.Offset, "\\begin{%s} with \\tag must be the whole formula", a.Env)
		}
	}
	expr.Body = body
	if n := countNodes(reflect.ValueOf(expr)); n > lim.MaxNodes {
		return nil, fmt.Errorf("%w: more than %d nodes", guard.ErrTooLarge, lim.MaxNodes)
	}
	return expr, nil
}

type parser struct {
	e      *expander
	lim    Limits
	depth  int
	nodes  int
	left   int // nesting of \left ... \right
	array  int // nesting of array environments
	tag    **ast.Tag
	primes map[*ast.Scripts]bool // superscripts made only of primes so far
	// textMath > 0 while parsing $...$ inside \text, where $ ends the list.
	textMath int
	// textParen > 0 while parsing \( ... \) inside \text.
	textParen int
	// tagged lists the arrays with row tags (align, gather): they must be
	// the whole formula, or the tags have no place to go.
	tagged []*ast.Array
	// cdStop is the character that ends a label inside a CD arrow
	// (@>label>>), or "" outside one; cd > 0 inside a CD environment,
	// where @ ends an object.
	cdStop string
	cd     int
	// color is set while reading a color argument, where #1 is the start
	// of a hex color (#1a2b3c) rather than a macro parameter.
	color bool
}

func (p *parser) count() error { return p.countN(1) }

// countN counts n nodes at once (for example the empty cells an array will
// be padded with).
func (p *parser) countN(n int) error {
	p.nodes += n
	if p.nodes > p.lim.MaxNodes {
		return fmt.Errorf("%w: more than %d nodes", guard.ErrTooLarge, p.lim.MaxNodes)
	}
	return nil
}

func (p *parser) enter() error {
	p.depth++
	if p.depth > p.lim.MaxDepth {
		return fmt.Errorf("%w: nesting deeper than %d", guard.ErrTooLarge, p.lim.MaxDepth)
	}
	return nil
}

func (p *parser) leave() { p.depth-- }

func (p *parser) unexpected(t token) error {
	switch {
	case t.isChar("}"):
		return diag.Syntaxf(t.off, "unmatched }")
	case t.isChar("&"):
		return diag.Syntaxf(t.off, "misplaced & outside an environment")
	case isRowEnd(t):
		return diag.Syntaxf(t.off, "misplaced \\%s inside a group (a line break must be outside braces)", t.text)
	case t.isCmd("right"):
		return diag.Syntaxf(t.off, "\\right without \\left")
	case t.isCmd("end"):
		return diag.Syntaxf(t.off, "\\end without \\begin")
	case t.isChar("]"):
		return diag.Syntaxf(t.off, "unexpected ]")
	case t.kind == tEOF:
		return diag.Syntaxf(t.off, "unexpected end of input")
	}
	return diag.Syntaxf(t.off, "unexpected %s", t.text)
}

// isTerminator reports whether t ends a math list. The caller decides
// whether that terminator is valid in its context.
func (p *parser) isTerminator(t token, bracket bool) bool {
	switch {
	case t.kind == tEOF, t.isChar("}"), t.isChar("&"), isRowEnd(t),
		t.isCmd("right"), t.isCmd("end"):
		return true
	case bracket && t.isChar("]"):
		return true
	case p.textMath > 0 && t.isChar("$"):
		return true
	case p.textParen > 0 && t.isCmd(")"):
		return true
	case p.cd > 0 && p.cdStop == "" && t.isChar("@"):
		return true
	case p.cdStop != "" && t.isChar(p.cdStop):
		return true
	}
	return false
}

func (p *parser) list() ([]ast.Node, token, error) { return p.listUntil(false) }

// listUntil parses a math list up to a terminator, which it consumes and
// returns.
func (p *parser) listUntil(bracket bool) ([]ast.Node, token, error) {
	if err := p.enter(); err != nil {
		return nil, token{}, err
	}
	defer p.leave()
	var out []ast.Node
	var infix *ast.Frac // \over and friends seen in this list
	for {
		t, err := p.e.next()
		if err != nil {
			return nil, t, err
		}
		if p.isTerminator(t, bracket) {
			if infix != nil {
				infix.Den = out
				out = []ast.Node{infix}
			}
			return out, t, nil
		}
		if f, err := p.infix(t); err != nil {
			return nil, t, err
		} else if f != nil {
			if infix != nil {
				return nil, t, diag.Syntaxf(t.off, "ambiguous: two generalized fractions (\\%s) in one group", t.text)
			}
			if err := p.count(); err != nil {
				return nil, t, err
			}
			f.Num, out, infix = out, nil, f
			continue
		}
		switch {
		case t.kind == tSpace:
			continue
		case t.isChar("^"), t.isChar("_"):
			if out, err = p.script(out, t); err != nil {
				return nil, t, err
			}
			continue
		case t.isChar("'"):
			if out, err = p.prime(out, t); err != nil {
				return nil, t, err
			}
			continue
		case t.isCmd("limits"), t.isCmd("nolimits"), t.isCmd("displaylimits"):
			if err := p.limits(out, t); err != nil {
				return nil, t, err
			}
			continue
		case t.isCmd("tag"):
			if err := p.parseTag(t); err != nil {
				return nil, t, err
			}
			continue
		}
		if wrap := p.switchCommand(t); wrap != nil {
			rest, end, err := p.listUntil(bracket)
			if err != nil {
				return nil, end, err
			}
			n, err := wrap(rest)
			if err != nil {
				return nil, end, err
			}
			if err := p.count(); err != nil {
				return nil, end, err
			}
			out = append(out, n)
			if infix != nil {
				infix.Den = out
				out = []ast.Node{infix}
			}
			return out, end, nil
		}
		n, err := p.atom(t)
		if err != nil {
			return nil, t, err
		}
		if n != nil {
			if err := p.count(); err != nil {
				return nil, t, err
			}
			out = append(out, n)
		}
	}
}

// infix returns the fraction for an infix command (\over, \atop, \choose,
// \brace, \brack, \above), with its numerator and denominator to be
// filled in by the caller, or nil for other tokens.
func (p *parser) infix(t token) (*ast.Frac, error) {
	if t.kind != tCmd {
		return nil, nil
	}
	f := &ast.Frac{Offset: t.off}
	switch t.text {
	case "over":
		f.Bar = true
	case "atop":
	case "choose":
		f.Left, f.Right = "(", ")"
	case "brace":
		f.Left, f.Right = "{", "}"
	case "brack":
		f.Left, f.Right = "[", "]"
	case "above":
		mu, err := p.dimension(t)
		if err != nil {
			return nil, err
		}
		f.Bar, f.BarEm = mu > 0, mu/18
	default:
		return nil, nil
	}
	return f, nil
}

// switchCommand handles commands that apply to the rest of the current
// list (\displaystyle, \color, \rm). It returns nil for other tokens.
func (p *parser) switchCommand(t token) func([]ast.Node) (ast.Node, error) {
	if t.kind != tCmd {
		return nil
	}
	off := t.off
	switch t.text {
	case "displaystyle", "textstyle", "scriptstyle", "scriptscriptstyle":
		st := map[string]ast.MathStyle{
			"displaystyle": ast.StyleDisplay, "textstyle": ast.StyleText,
			"scriptstyle": ast.StyleScript, "scriptscriptstyle": ast.StyleScriptScript,
		}[t.text]
		return func(body []ast.Node) (ast.Node, error) { return &ast.Style{Offset: off, Style: st, Body: body}, nil }
	case "color":
		c, err := p.colorArg(t)
		return func(body []ast.Node) (ast.Node, error) {
			if err != nil {
				return nil, err
			}
			return &ast.Color{Offset: off, Color: c, Body: body}, nil
		}
	}
	if fs, ok := symbols.FontSwitches[t.text]; ok {
		return func(body []ast.Node) (ast.Node, error) { return &ast.Font{Offset: off, Style: fs, Body: body}, nil }
	}
	if f, ok := symbols.Sizes[t.text]; ok {
		return func(body []ast.Node) (ast.Node, error) { return &ast.Size{Offset: off, Factor: f, Body: body}, nil }
	}
	return nil
}

func (p *parser) script(out []ast.Node, t token) ([]ast.Node, error) {
	sup := t.text == "^"
	arg, err := p.arg(t)
	if err != nil {
		return nil, err
	}
	s := p.scriptsFor(&out, t)
	switch {
	case sup && s.HasSup && p.primes[s]:
		s.Sup = append(s.Sup, arg...)
		delete(p.primes, s)
	case sup && s.HasSup:
		return nil, diag.Syntaxf(t.off, "double superscript")
	case !sup && s.HasSub:
		return nil, diag.Syntaxf(t.off, "double subscript")
	case sup:
		s.Sup, s.HasSup = arg, true
	default:
		s.Sub, s.HasSub = arg, true
	}
	return out, nil
}

func (p *parser) prime(out []ast.Node, t token) ([]ast.Node, error) {
	s := p.scriptsFor(&out, t)
	if s.HasSup && !p.primes[s] {
		return nil, diag.Syntaxf(t.off, "double superscript")
	}
	if err := p.count(); err != nil {
		return nil, err
	}
	s.Sup = append(s.Sup, &ast.Symbol{Offset: t.off, Text: "′", Class: ast.ClassOrd})
	s.HasSup = true
	p.primes[s] = true
	return out, nil
}

// scriptsFor returns the Scripts node that a script at t attaches to,
// creating it from the last node of out when needed.
func (p *parser) scriptsFor(out *[]ast.Node, t token) *ast.Scripts {
	l := *out
	if n := len(l); n > 0 {
		if s, ok := l[n-1].(*ast.Scripts); ok {
			return s
		}
		s := &ast.Scripts{Offset: l[n-1].Pos(), Base: l[n-1]}
		l[n-1] = s
		return s
	}
	s := &ast.Scripts{Offset: t.off}
	*out = append(l, s)
	return s
}

func (p *parser) limits(out []ast.Node, t token) error {
	var op *ast.Op
	if n := len(out); n > 0 {
		switch x := out[n-1].(type) {
		case *ast.Op:
			op = x
		case *ast.Scripts:
			op, _ = x.Base.(*ast.Op)
		}
	}
	if op == nil {
		return diag.Syntaxf(t.off, "\\%s must follow a math operator", t.text)
	}
	switch t.text {
	case "limits":
		op.Limits = ast.LimitsOn
	case "nolimits":
		op.Limits = ast.LimitsOff
	default:
		op.Limits = ast.LimitsAuto
	}
	return nil
}

func (p *parser) parseTag(t token) error {
	star := false
	if n, err := p.e.peek(); err != nil {
		return err
	} else if n.isChar("*") {
		_, _ = p.e.next() // the peeked "*"
		star = true
	}
	if p.tag == nil {
		return diag.Syntaxf(t.off, "\\tag is not allowed here")
	}
	if *p.tag != nil {
		return diag.Syntaxf(t.off, "multiple \\tag")
	}
	body, err := p.textArg(t, ast.FontRoman)
	if err != nil {
		return err
	}
	if err := p.count(); err != nil {
		return err
	}
	*p.tag = &ast.Tag{Offset: t.off, Star: star, Body: body}
	return nil
}

// arg parses a macro-style argument: a braced group or a single atom.
func (p *parser) arg(cmd token) ([]ast.Node, error) {
	t, err := p.nextNonSpace()
	if err != nil {
		return nil, err
	}
	switch {
	case t.isChar("{"):
		saved := p.tag
		p.tag = nil
		body, end, err := p.list()
		p.tag = saved
		if err != nil {
			return nil, err
		}
		if !end.isChar("}") {
			if end.kind == tEOF {
				return nil, diag.Syntaxf(t.off, "missing }")
			}
			return nil, p.unexpected(end)
		}
		return body, nil
	case p.isTerminator(t, false), t.isChar("^"), t.isChar("_"):
		return nil, diag.Syntaxf(cmdOff(cmd, t), "missing argument for %s", cmdName(cmd))
	}
	n, err := p.atom(t)
	if err != nil {
		return nil, err
	}
	if n == nil {
		return nil, nil
	}
	if err := p.count(); err != nil {
		return nil, err
	}
	return []ast.Node{n}, nil
}

func cmdName(t token) string {
	if t.kind == tCmd {
		return "\\" + t.text
	}
	return t.text
}

func cmdOff(cmd, at token) int {
	if at.kind == tEOF {
		return cmd.off
	}
	return at.off
}

func (p *parser) nextNonSpace() (token, error) {
	for {
		t, err := p.e.next()
		if err != nil || t.kind != tSpace {
			return t, err
		}
	}
}

// optArg parses an optional [ ... ] argument; it returns nil, false when
// there is none.
func (p *parser) optArg() ([]ast.Node, bool, error) {
	t, err := p.nextNonSpace()
	if err != nil {
		return nil, false, err
	}
	if !t.isChar("[") {
		p.e.unread(t)
		return nil, false, nil
	}
	body, end, err := p.listUntil(true)
	if err != nil {
		return nil, false, err
	}
	if !end.isChar("]") {
		if end.kind == tEOF {
			return nil, false, diag.Syntaxf(t.off, "missing ]")
		}
		return nil, false, p.unexpected(end)
	}
	return body, true, nil
}

// argText reads a braced argument as plain text (for \text, \operatorname,
// environment names and colors). Spaces are kept, runs collapsed.
func (p *parser) argText(cmd token) (string, error) {
	t, err := p.nextNonSpace()
	if err != nil {
		return "", err
	}
	if !t.isChar("{") {
		if t.kind == tChar && !p.isTerminator(t, false) {
			return t.text, nil
		}
		return "", diag.Syntaxf(cmdOff(cmd, t), "missing argument for %s", cmdName(cmd))
	}
	var b strings.Builder
	depth := 0
	for {
		x, err := p.e.next()
		if err != nil {
			return "", err
		}
		switch {
		case x.kind == tEOF:
			return "", diag.Syntaxf(t.off, "missing }")
		case x.isChar("{"):
			depth++
			b.WriteByte('{') // kept for column specifications like p{2cm}
		case x.isChar("}"):
			if depth == 0 {
				return b.String(), nil
			}
			depth--
			b.WriteByte('}')
		case x.kind == tSpace, x.isChar("~"):
			b.WriteByte(' ')
		case x.kind == tChar:
			if x.text == "$" {
				return "", diag.Unsupported("math inside "+cmdName(cmd), x.off)
			}
			b.WriteString(x.text)
		case x.kind == tVerb:
			b.WriteString(x.text)
		case x.kind == tParam && p.color:
			// NOTE(katex-compat): inside a macro body #1 is still the
			// parameter, as in TeX; write the color outside the body.
			b.WriteString("#" + x.text)
		case x.kind == tParam:
			return "", diag.Syntaxf(x.off, "unexpected #")
		case x.kind == tCmd:
			switch x.text {
			case " ", "space", "quad", "qquad", ",", ";", ":", "nobreakspace":
				b.WriteByte(' ')
			case "{", "}", "$", "%", "&", "_", "#":
				b.WriteString(x.text)
			case "textbackslash":
				b.WriteByte('\\')
			case "relax":
			default:
				// argText reads plain arguments (colors, environment names,
				// URLs, \fbox); \text and its relatives, with nested
				// commands and math, go through textArg.
				return "", diag.Unsupported("\\"+x.text+" inside "+cmdName(cmd), x.off)
			}
		}
	}
}

// delim reads a delimiter after \left, \right, \middle or \big.
func (p *parser) delim(cmd token) (string, error) {
	t, err := p.nextNonSpace()
	if err != nil {
		return "", err
	}
	key := t.text
	if t.kind == tCmd {
		key = "\\" + t.text
		if a, ok := symbols.Aliases[t.text]; ok {
			key = "\\" + a
			if !isLetter(a[0]) {
				key = a
			}
		}
	}
	if t.kind == tChar || t.kind == tCmd {
		if d, ok := symbols.Delims[key]; ok {
			return d, nil
		}
	}
	if t.kind == tEOF {
		return "", diag.Syntaxf(cmd.off, "missing delimiter after \\%s", cmd.text)
	}
	return "", diag.Syntaxf(t.off, "invalid delimiter %s after \\%s", key, cmd.text)
}

func (p *parser) colorArg(cmd token) (color.NRGBA, error) {
	p.color = true
	s, err := p.argText(cmd)
	p.color = false
	if err != nil {
		return color.NRGBA{}, err
	}
	s = strings.TrimSpace(s)
	if rgb, ok := symbols.Colors[strings.ToLower(s)]; ok {
		return color.NRGBA{rgb[0], rgb[1], rgb[2], 255}, nil
	}
	h := strings.TrimPrefix(s, "#")
	if len(h) == 3 {
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	}
	if len(h) == 6 {
		if v, err := strconv.ParseUint(h, 16, 32); err == nil {
			return color.NRGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}, nil
		}
	}
	return color.NRGBA{}, diag.Syntaxf(cmd.off, "invalid color %q", s)
}

// dimension reads a TeX dimension such as "1em", "-3mu", "2.5pt", optionally
// braced, and returns it in mu.
func (p *parser) dimension(cmd token) (float64, error) {
	t, err := p.nextNonSpace()
	if err != nil {
		return 0, err
	}
	braced := t.isChar("{")
	if braced {
		if t, err = p.nextNonSpace(); err != nil {
			return 0, err
		}
	}
	var num, unit strings.Builder
	for t.kind == tChar && (strings.Contains("+-.,0123456789", t.text)) {
		if t.text == "," {
			num.WriteByte('.')
		} else {
			num.WriteString(t.text)
		}
		if t, err = p.e.next(); err != nil {
			return 0, err
		}
	}
	for t.kind == tSpace {
		if t, err = p.e.next(); err != nil {
			return 0, err
		}
	}
	for i := 0; i < 2 && t.kind == tChar && len(t.text) == 1 && isLetter(t.text[0]); i++ {
		unit.WriteString(t.text)
		if t, err = p.e.next(); err != nil {
			return 0, err
		}
	}
	if braced {
		for t.kind == tSpace {
			if t, err = p.e.next(); err != nil {
				return 0, err
			}
		}
		if !t.isChar("}") {
			return 0, diag.Syntaxf(cmd.off, "invalid dimension for \\%s", cmd.text)
		}
	} else {
		p.e.unread(t)
	}
	return toMu(cmd, num.String(), unit.String())
}

// toMu converts a dimension's number and unit to mu.
func toMu(cmd token, num, unit string) (float64, error) {
	v, err := strconv.ParseFloat(num, 64)
	if err != nil || v != v {
		return 0, diag.Syntaxf(cmd.off, "invalid dimension for \\%s", cmd.text)
	}
	if v > 1000 || v < -1000 {
		return 0, fmt.Errorf("%w: dimension %s for \\%s exceeds 1000 units", guard.ErrTooLarge, num, cmd.text)
	}
	// Conversions assume a 10pt font as TeX does: 1em = 10pt = 18mu.
	const pt = 1.8
	perUnit := map[string]float64{"mu": 1, "em": 18, "ex": 18 * 0.431, "pt": pt, "px": pt, "bp": pt * 1.00375,
		"mm": pt * 2.84528, "cm": pt * 28.4528, "in": pt * 72.27, "pc": pt * 12,
		"dd": pt * 1238 / 1157, "cc": pt * 12 * 1238 / 1157, "nd": pt * 685 / 642, "nc": pt * 12 * 685 / 642,
		"sp": pt / 65536}
	f, ok := perUnit[unit]
	if !ok {
		return 0, diag.Syntaxf(cmd.off, "invalid unit %q for \\%s", unit, cmd.text)
	}
	return v * f, nil
}

// dimensionText converts a dimension written as text ("2pt", "0.5em") to
// mu. cmd is the command for error messages.
func dimensionText(cmd token, s string) (float64, error) {
	s = strings.TrimSpace(s)
	i := 0
	for i < len(s) && strings.IndexByte("+-.,0123456789", s[i]) >= 0 {
		i++
	}
	return toMu(cmd, strings.ReplaceAll(s[:i], ",", "."), strings.TrimSpace(s[i:]))
}

func sym(off int, s symbols.Sym) *ast.Symbol {
	return &ast.Symbol{Offset: off, Text: s.Text, Class: s.Class}
}

// atom parses one atom starting at token t. It may return nil for no-op
// commands.
func (p *parser) atom(t token) (ast.Node, error) {
	if err := p.enter(); err != nil {
		return nil, err
	}
	defer p.leave()
	switch t.kind {
	case tParam:
		return nil, diag.Syntaxf(t.off, "unexpected #")
	case tVerb:
		return &ast.Text{Offset: t.off, Text: strings.ReplaceAll(t.text, "\n", " "), Style: ast.FontMono}, nil
	case tChar:
		return p.char(t)
	case tCmd:
		return p.command(t)
	}
	return nil, p.unexpected(t)
}

func (p *parser) char(t token) (ast.Node, error) {
	switch t.text {
	case "#":
		return nil, diag.Syntaxf(t.off, "unexpected #")
	case "{":
		saved := p.tag
		p.tag = nil
		body, end, err := p.list()
		p.tag = saved
		if err != nil {
			return nil, err
		}
		if !end.isChar("}") {
			if end.kind == tEOF {
				return nil, diag.Syntaxf(t.off, "missing }")
			}
			return nil, p.unexpected(end)
		}
		return &ast.Group{Offset: t.off, Body: body}, nil
	case "~":
		return &ast.Space{Offset: t.off, Mu: 6}, nil
	case "$":
		return nil, diag.Syntaxf(t.off, "unexpected $ in math mode")
	case "]":
		return &ast.Symbol{Offset: t.off, Text: "]", Class: ast.ClassClose}, nil
	}
	r, _ := utf8.DecodeRuneInString(t.text)
	if s, ok := symbols.Chars[r]; ok {
		return sym(t.off, s), nil
	}
	return &ast.Symbol{Offset: t.off, Text: t.text, Class: ast.ClassOrd, Letter: symbols.IsLetter(r)}, nil
}

var classCommands = map[string]ast.Class{
	"mathord": ast.ClassOrd, "mathop": ast.ClassOp, "mathbin": ast.ClassBin, "mathrel": ast.ClassRel, "mathopen": ast.ClassOpen,
	"mathclose": ast.ClassClose, "mathpunct": ast.ClassPunct, "mathinner": ast.ClassInner,
}

func (p *parser) command(t token) (ast.Node, error) {
	off, name := t.off, t.text
	if a, ok := symbols.Aliases[name]; ok {
		if !isLetter(a[0]) {
			return p.char(token{kind: tChar, text: a, off: off})
		}
		name = a
	}
	if s, ok := symbols.Commands[name]; ok {
		return &ast.Symbol{Offset: off, Text: s.Text, Class: s.Class,
			Letter: symbols.IsLetter([]rune(s.Text)[0])}, nil
	}
	if d, ok := symbols.Delims["\\"+name]; ok {
		return &ast.Symbol{Offset: off, Text: d, Class: symbols.DelimClass(d)}, nil
	}
	if o, ok := symbols.Ops[name]; ok {
		return &ast.Op{Offset: off, Text: o.Text, Symbol: o.Symbol, DefaultLimits: o.DefaultLimits}, nil
	}
	if mu, ok := symbols.Spaces[name]; ok {
		return &ast.Space{Offset: off, Mu: mu}, nil
	}
	if a, ok := symbols.Accents[name]; ok {
		body, err := p.arg(t)
		if err != nil {
			return nil, err
		}
		return &ast.Accent{Offset: off, Char: a.Char, Name: name, Wide: a.Wide, Under: a.Under, Body: body}, nil
	}
	if fs, ok := symbols.Fonts[name]; ok {
		body, err := p.arg(t)
		if err != nil {
			return nil, err
		}
		return &ast.Font{Offset: off, Style: fs, Body: body}, nil
	}
	if fs, ok := symbols.TextFonts[name]; ok {
		body, err := p.textArg(t, fs)
		if err != nil {
			return nil, err
		}
		if len(body) == 1 {
			return body[0], nil
		}
		return &ast.Group{Offset: off, Body: body}, nil
	}
	if a, ok := symbols.XArrows[name]; ok {
		under, _, err := p.optArg()
		if err != nil {
			return nil, err
		}
		over, err := p.arg(t)
		if err != nil {
			return nil, err
		}
		return &ast.XArrow{Offset: off, Arrow: a, Over: over, Under: under}, nil
	}
	if c, ok := classCommands[name]; ok {
		body, err := p.arg(t)
		if err != nil {
			return nil, err
		}
		return &ast.Group{Offset: off, Class: c, Body: body}, nil
	}
	switch name {
	case "frac", "dfrac", "tfrac", "cfrac", "binom", "dbinom", "tbinom":
		num, err := p.arg(t)
		if err != nil {
			return nil, err
		}
		den, err := p.arg(t)
		if err != nil {
			return nil, err
		}
		f := &ast.Frac{Offset: off, Num: num, Den: den, Bar: !strings.HasSuffix(name, "binom")}
		if !f.Bar {
			f.Left, f.Right = "(", ")"
		}
		switch name[0] {
		case 'd', 'c':
			f.Style = ast.StyleDisplay
		case 't':
			f.Style = ast.StyleText
		}
		return f, nil
	case "genfrac":
		return p.genfrac(t)
	case "substack":
		return p.substack(t)
	case "sqrt":
		idx, _, err := p.optArg()
		if err != nil {
			return nil, err
		}
		body, err := p.arg(t)
		if err != nil {
			return nil, err
		}
		return &ast.Sqrt{Offset: off, Index: idx, Body: body}, nil
	case "left":
		return p.leftRight(t)
	case "middle":
		if p.left == 0 {
			return nil, diag.Syntaxf(off, "\\middle without \\left")
		}
		d, err := p.delim(t)
		if err != nil {
			return nil, err
		}
		return &ast.Middle{Offset: off, Delim: d}, nil
	case "big", "Big", "bigg", "Bigg", "bigl", "Bigl", "biggl", "Biggl",
		"bigr", "Bigr", "biggr", "Biggr", "bigm", "Bigm", "biggm", "Biggm":
		d, err := p.delim(t)
		if err != nil {
			return nil, err
		}
		base := strings.TrimRight(name, "lrm")
		size := map[string]int{"big": 1, "Big": 2, "bigg": 3, "Bigg": 4}[base]
		cls := symbols.DelimClass(d)
		switch name[len(name)-1] {
		case 'l':
			cls = ast.ClassOpen
		case 'r':
			cls = ast.ClassClose
		case 'm':
			cls = ast.ClassRel
		}
		return &ast.SizedDelim{Offset: off, Delim: d, Size: size, Class: cls}, nil
	case "overline", "underline":
		body, err := p.arg(t)
		if err != nil {
			return nil, err
		}
		return &ast.Overline{Offset: off, Under: name == "underline", Body: body}, nil
	case "operatorname", "operatornamewithlimits":
		lim := name == "operatornamewithlimits"
		if n, err := p.e.peek(); err != nil {
			return nil, err
		} else if n.isChar("*") {
			_, _ = p.e.next() // the peeked "*"
			lim = true
		}
		body, err := p.arg(t)
		if err != nil {
			return nil, err
		}
		if len(body) == 0 {
			return nil, diag.Syntaxf(off, "empty \\operatorname")
		}
		// Plain letters keep the simple form; anything else (spaces,
		// scripts, digits) is typeset as an upright math list.
		var name strings.Builder
		for _, n := range body {
			s, ok := n.(*ast.Symbol)
			if !ok || !(len(s.Text) == 1 && (s.Text[0]|0x20) >= 'a' && (s.Text[0]|0x20) <= 'z') {
				name.Reset()
				break
			}
			name.WriteString(s.Text)
		}
		if name.Len() > 0 {
			return &ast.Op{Offset: off, Text: name.String(), DefaultLimits: lim}, nil
		}
		return &ast.Op{Offset: off, Body: []ast.Node{&ast.Font{Offset: off, Style: ast.FontRoman, Body: body}}, DefaultLimits: lim}, nil
	case "bmod":
		return &ast.Symbol{Offset: off, Text: "mod", Class: ast.ClassBin}, nil
	case "pmod", "mod", "pod":
		arg, err := p.arg(t)
		if err != nil {
			return nil, err
		}
		body := []ast.Node{&ast.Space{Offset: off, Mu: 18}}
		if name != "mod" {
			body = append(body, &ast.Symbol{Offset: off, Text: "(", Class: ast.ClassOpen})
		}
		if name != "pod" {
			body = append(body, &ast.Symbol{Offset: off, Text: "mod", Class: ast.ClassOrd}, &ast.Space{Offset: off, Mu: 6})
		}
		body = append(body, arg...)
		if name != "mod" {
			body = append(body, &ast.Symbol{Offset: off, Text: ")", Class: ast.ClassClose})
		}
		return &ast.Group{Offset: off, Body: body}, nil
	case "not":
		n, err := p.nextNonSpace()
		if err != nil {
			return nil, err
		}
		if p.isTerminator(n, false) || n.isChar("^") || n.isChar("_") || n.isChar("'") || n.isCmd("middle") {
			return nil, diag.Syntaxf(off, "\\not needs a symbol")
		}
		a, err := p.atom(n)
		if err != nil {
			return nil, err
		}
		// Commands that produce nothing (\relax, \notag) or only a hint
		// leave \not without a symbol.
		switch a.(type) {
		case nil, *ast.Space, *ast.Break:
			return nil, diag.Syntaxf(off, "\\not needs a symbol")
		}
		if s, ok := a.(*ast.Symbol); ok {
			if neg, ok := symbols.Negations[s.Text]; ok {
				return &ast.Symbol{Offset: off, Text: neg, Class: s.Class}, nil
			}
		}
		return &ast.Not{Offset: off, Body: []ast.Node{a}}, nil
	case "hspace", "kern", "mkern", "mskip", "hskip":
		if name == "hspace" {
			// \hspace* only differs at line breaks, which a formula has not.
			if n, err := p.e.peek(); err != nil {
				return nil, err
			} else if n.isChar("*") {
				_, _ = p.e.next() // the peeked "*"
			}
		}
		mu, err := p.dimension(t)
		if err != nil {
			return nil, err
		}
		return &ast.Space{Offset: off, Mu: mu}, nil
	case "phantom", "hphantom", "vphantom":
		body, err := p.arg(t)
		if err != nil {
			return nil, err
		}
		return &ast.Phantom{Offset: off, Body: body, NoWidth: name == "vphantom", NoHeight: name == "hphantom"}, nil
	case "mathstrut":
		return &ast.Phantom{Offset: off, NoWidth: true, Body: []ast.Node{&ast.Symbol{Offset: off, Text: "(", Class: ast.ClassOpen}}}, nil
	case "smash":
		opt, _, err := p.bracketText()
		if err != nil {
			return nil, err
		}
		sm := &ast.Smash{Offset: off}
		switch strings.TrimSpace(opt) {
		case "":
			sm.Top, sm.Bottom = true, true
		case "t":
			sm.Top = true
		case "b":
			sm.Bottom = true
		default:
			return nil, diag.Syntaxf(off, "invalid option [%s] for \\smash (want t or b)", opt)
		}
		if sm.Body, err = p.arg(t); err != nil {
			return nil, err
		}
		return sm, nil
	case "llap", "rlap", "clap", "mathllap", "mathrlap", "mathclap":
		body, err := p.arg(t)
		if err != nil {
			return nil, err
		}
		al := map[byte]ast.Align{'l': ast.AlignRight, 'r': ast.AlignLeft, 'c': ast.AlignCenter}[strings.TrimPrefix(name, "math")[0]]
		return &ast.Lap{Offset: off, Align: al, Body: body}, nil
	case "raisebox":
		dim, err := p.argText(t)
		if err != nil {
			return nil, err
		}
		mu, err := dimensionText(t, dim)
		if err != nil {
			return nil, err
		}
		body, err := p.textArg(t, ast.FontRoman)
		if err != nil {
			return nil, err
		}
		return &ast.Raise{Offset: off, Mu: mu, Body: body}, nil
	case "vcenter":
		body, err := p.arg(t)
		if err != nil {
			return nil, err
		}
		return &ast.Raise{Offset: off, Center: true, Body: body}, nil
	case "colorbox", "fcolorbox":
		cb := &ast.ColorBox{Offset: off}
		if name == "fcolorbox" {
			fr, err := p.colorArg(t)
			if err != nil {
				return nil, err
			}
			cb.Frame = &fr
		}
		bg, err := p.colorArg(t)
		if err != nil {
			return nil, err
		}
		cb.Bg = bg
		if cb.Body, err = p.textArg(t, ast.FontRoman); err != nil {
			return nil, err
		}
		return cb, nil
	case "reflectbox", "mathreflectbox":
		var body []ast.Node
		var err error
		if name == "reflectbox" {
			body, err = p.textArg(t, ast.FontRoman)
		} else {
			body, err = p.arg(t)
		}
		if err != nil {
			return nil, err
		}
		return &ast.Reflect{Offset: off, Body: body}, nil
	case "rule":
		r := &ast.Rule{Offset: off}
		raise, ok, err := p.bracketText()
		if err != nil {
			return nil, err
		}
		if ok {
			if r.RaiseMu, err = dimensionText(t, raise); err != nil {
				return nil, err
			}
		}
		for _, d := range []*float64{&r.WMu, &r.HMu} {
			txt, err := p.argText(t)
			if err != nil {
				return nil, err
			}
			if *d, err = dimensionText(t, txt); err != nil {
				return nil, err
			}
		}
		return r, nil
	case "sout":
		body, err := p.arg(t)
		if err != nil {
			return nil, err
		}
		return &ast.Cancel{Offset: off, Kind: "-", Body: body}, nil
	case "char":
		return p.charCode(t)
	case "href", "url":
		var url string
		if n, err := p.e.next(); err != nil {
			return nil, err
		} else if n.kind == tURL {
			url = n.text
		} else {
			p.e.unread(n)
			if url, err = p.argText(t); err != nil {
				return nil, err
			}
		}
		// NOTE(katex-compat): images and text have no links; the link
		// text (or the URL) is drawn and the target dropped.
		if name == "url" {
			return &ast.Text{Offset: off, Text: url, Style: ast.FontMono}, nil
		}
		body, err := p.arg(t)
		if err != nil {
			return nil, err
		}
		return &ast.Group{Offset: off, Body: body}, nil
	case "htmlClass", "htmlId", "htmlStyle", "htmlData":
		// The HTML attribute does not affect an image; the body is drawn.
		if _, err := p.argText(t); err != nil {
			return nil, err
		}
		body, err := p.arg(t)
		if err != nil {
			return nil, err
		}
		return &ast.Group{Offset: off, Body: body}, nil
	case "TeX", "LaTeX", "KaTeX":
		return &ast.Logo{Offset: off, Name: name}, nil
	case "pmb":
		// NOTE(katex-compat): \pmb ("poor man's bold") uses the bold
		// alphabet instead of overprinting.
		body, err := p.arg(t)
		if err != nil {
			return nil, err
		}
		return &ast.Font{Offset: off, Style: ast.FontBoldItalic, Body: body}, nil
	case "angln":
		return &ast.Enclose{Offset: off, Kind: "angl", Body: []ast.Node{&ast.Symbol{Offset: off, Text: "n", Class: ast.ClassOrd, Letter: true}}}, nil
	case "angl", "phase":
		body, err := p.arg(t)
		if err != nil {
			return nil, err
		}
		return &ast.Enclose{Offset: off, Kind: name, Body: body}, nil
	case "bra", "ket", "braket", "Bra", "Ket", "Braket", "set", "Set":
		body, err := p.arg(t)
		if err != nil {
			return nil, err
		}
		return braket(off, name, body), nil
	case "plim":
		return &ast.Op{Offset: off, Text: "plim", DefaultLimits: true}, nil
	case "varliminf", "varlimsup", "varinjlim", "varprojlim":
		lim := []ast.Node{&ast.Font{Offset: off, Style: ast.FontRoman, Body: []ast.Node{
			&ast.Symbol{Offset: off, Text: "l", Class: ast.ClassOrd}, &ast.Symbol{Offset: off, Text: "i", Class: ast.ClassOrd},
			&ast.Symbol{Offset: off, Text: "m", Class: ast.ClassOrd}}}}
		var deco ast.Node
		switch name {
		case "varliminf":
			deco = &ast.Overline{Offset: off, Under: true, Body: lim}
		case "varlimsup":
			deco = &ast.Overline{Offset: off, Body: lim}
		case "varinjlim":
			deco = &ast.Accent{Offset: off, Char: 0x20EF, Name: "underrightarrow", Wide: true, Under: true, Body: lim}
		default:
			deco = &ast.Accent{Offset: off, Char: 0x20EE, Name: "underleftarrow", Wide: true, Under: true, Body: lim}
		}
		return &ast.Op{Offset: off, Body: []ast.Node{deco}, DefaultLimits: true}, nil
	case "mathchoice":
		c := &ast.Choice{Offset: off}
		for i := range c.Bodies {
			b, err := p.arg(t)
			if err != nil {
				return nil, err
			}
			c.Bodies[i] = b
		}
		return c, nil
	case "boxed":
		body, err := p.arg(t)
		if err != nil {
			return nil, err
		}
		return &ast.Boxed{Offset: off, Body: body}, nil
	case "fbox":
		s, err := p.argText(t)
		if err != nil {
			return nil, err
		}
		return &ast.Boxed{Offset: off, Body: []ast.Node{&ast.Text{Offset: off, Text: s, Style: ast.FontRoman}}}, nil
	case "cancel", "bcancel", "xcancel":
		body, err := p.arg(t)
		if err != nil {
			return nil, err
		}
		kind := map[string]string{"cancel": "/", "bcancel": "\\", "xcancel": "X"}[name]
		return &ast.Cancel{Offset: off, Kind: kind, Body: body}, nil
	case "overbrace", "underbrace", "overbracket", "underbracket":
		body, err := p.arg(t)
		if err != nil {
			return nil, err
		}
		return &ast.Brace{Offset: off, Under: strings.HasPrefix(name, "under"), Square: strings.HasSuffix(name, "bracket"), Body: body}, nil
	case "underbar":
		body, err := p.arg(t)
		if err != nil {
			return nil, err
		}
		return &ast.Overline{Offset: off, Under: true, Body: body}, nil
	case "underlinesegment":
		// NOTE(katex-compat): drawn as \underline; the font has no bridge
		// below that stretches.
		body, err := p.arg(t)
		if err != nil {
			return nil, err
		}
		return &ast.Overline{Offset: off, Under: true, Body: body}, nil
	case "overset", "underset", "stackrel":
		a, err := p.arg(t)
		if err != nil {
			return nil, err
		}
		base, err := p.arg(t)
		if err != nil {
			return nil, err
		}
		ou := &ast.OverUnder{Offset: off, Base: base, Class: classOf(base)}
		if name == "underset" {
			ou.Under = a
		} else {
			ou.Over = a
		}
		if name == "stackrel" {
			ou.Class = ast.ClassRel
		}
		return ou, nil
	case "textcolor":
		c, err := p.colorArg(t)
		if err != nil {
			return nil, err
		}
		body, err := p.arg(t)
		if err != nil {
			return nil, err
		}
		return &ast.Color{Offset: off, Color: c, Body: body}, nil
	case "begin":
		return p.environment(t)
	case "allowbreak", "nobreak":
		return &ast.Break{Offset: off, Allow: name == "allowbreak"}, nil
	case "relax", "nonumber", "notag", "displaylimits":
		return nil, nil
	case "hline":
		return nil, diag.Syntaxf(off, "misplaced \\hline")
	case "displaystyle", "textstyle", "scriptstyle", "scriptscriptstyle", "color":
		return nil, diag.Syntaxf(off, "\\%s needs a group here", name)
	}
	return nil, diag.Unsupported("\\"+name, off)
}

// charCode parses \char followed by a decimal, "hex or 'octal number.
func (p *parser) charCode(cmd token) (ast.Node, error) {
	t, err := p.nextNonSpace()
	if err != nil {
		return nil, err
	}
	base := 10
	switch {
	case t.isChar("\""):
		base = 16
	case t.isChar("'"):
		base = 8
	default:
		p.e.unread(t)
	}
	var digits strings.Builder
	for {
		d, err := p.e.next()
		if err != nil {
			return nil, err
		}
		if d.kind != tChar || len(d.text) != 1 || !strings.ContainsRune("0123456789abcdefABCDEF"[:map[int]int{8: 8, 10: 10, 16: 22}[base]], rune(d.text[0])) {
			p.e.unread(d)
			break
		}
		digits.WriteString(d.text)
		if digits.Len() > 8 {
			return nil, diag.Syntaxf(cmd.off, "character code too long for \\char")
		}
	}
	v, err := strconv.ParseUint(digits.String(), base, 32)
	if err != nil || v > 0x10FFFF || (v >= 0xD800 && v <= 0xDFFF) {
		return nil, diag.Syntaxf(cmd.off, "invalid character code for \\char")
	}
	r := rune(v)
	if isControl(r) {
		return nil, diag.Syntaxf(cmd.off, "control character U+%04X", r)
	}
	// The character itself, even one with a meaning in the source ({, $,
	// ~, #): \char"7B is a brace symbol, not the start of a group.
	if s, ok := symbols.Chars[r]; ok {
		return sym(cmd.off, s), nil
	}
	return &ast.Symbol{Offset: cmd.off, Text: string(r), Class: ast.ClassOrd, Letter: symbols.IsLetter(r)}, nil
}

// braket builds the braket package's notations: \bra{x} ⟨x|, \ket{x} |x⟩,
// \braket{x} ⟨x⟩ and \set{x} {x}, with fixed-size delimiters; the
// capitalized forms use \left ... \right and turn a | in the body into
// \middle| (\Braket: every |, \Set: the first, \set: the first as \mid).
func braket(off int, name string, body []ast.Node) ast.Node {
	open, closing := map[string][2]string{
		"bra": {"⟨", "|"}, "ket": {"|", "⟩"}, "braket": {"⟨", "⟩"}, "set": {"{", "}"},
	}[strings.ToLower(name)][0], map[string][2]string{
		"bra": {"⟨", "|"}, "ket": {"|", "⟩"}, "braket": {"⟨", "⟩"}, "set": {"{", "}"},
	}[strings.ToLower(name)][1]
	bars := 0 // how many | to replace: -1 all
	switch name {
	case "Braket":
		bars = -1
	case "Set", "set":
		bars = 1
	}
	out := make([]ast.Node, 0, len(body))
	for _, n := range body {
		if s, ok := n.(*ast.Symbol); ok && (s.Text == "|" || s.Text == "‖") && bars != 0 {
			bars--
			if name == "set" {
				out = append(out, &ast.Symbol{Offset: s.Offset, Text: "∣", Class: ast.ClassRel})
			} else {
				out = append(out, &ast.Middle{Offset: s.Offset, Delim: s.Text})
			}
			continue
		}
		out = append(out, n)
	}
	if name[0] >= 'A' && name[0] <= 'Z' {
		return &ast.LeftRight{Offset: off, Left: open, Right: closing, Body: out}
	}
	return &ast.Group{Offset: off, Class: ast.ClassInner, Body: append(append([]ast.Node{
		&ast.Symbol{Offset: off, Text: open, Class: ast.ClassOpen}}, out...),
		&ast.Symbol{Offset: off, Text: closing, Class: ast.ClassClose})}
}

// genfrac parses \genfrac{left}{right}{thickness}{style}{num}{den}.
func (p *parser) genfrac(t token) (ast.Node, error) {
	f := &ast.Frac{Offset: t.off, Bar: true}
	for _, d := range []*string{&f.Left, &f.Right} {
		v, err := p.optionalDelim(t)
		if err != nil {
			return nil, err
		}
		*d = v
	}
	th, err := p.argText(t)
	if err != nil {
		return nil, err
	}
	if th = strings.TrimSpace(th); th != "" {
		mu, err := dimensionText(t, th)
		if err != nil {
			return nil, err
		}
		f.Bar, f.BarEm = mu > 0, mu/18
	}
	st, err := p.argText(t)
	if err != nil {
		return nil, err
	}
	switch strings.TrimSpace(st) {
	case "":
	case "0":
		f.Style = ast.StyleDisplay
	case "1":
		f.Style = ast.StyleText
	case "2":
		f.Style = ast.StyleScript
	case "3":
		f.Style = ast.StyleScriptScript
	default:
		return nil, diag.Syntaxf(t.off, "invalid style %q in \\genfrac (want 0 to 3)", st)
	}
	if f.Num, err = p.arg(t); err != nil {
		return nil, err
	}
	if f.Den, err = p.arg(t); err != nil {
		return nil, err
	}
	return f, nil
}

// bracketText reads an optional [ ... ] argument as plain text; ok is
// false when there is none.
func (p *parser) bracketText() (s string, ok bool, err error) {
	t, err := p.nextNonSpace()
	if err != nil {
		return "", false, err
	}
	if !t.isChar("[") {
		p.e.unread(t)
		return "", false, nil
	}
	var b strings.Builder
	for {
		x, err := p.e.next()
		if err != nil {
			return "", false, err
		}
		switch {
		case x.kind == tEOF:
			return "", false, diag.Syntaxf(t.off, "missing ]")
		case x.isChar("]"):
			return b.String(), true, nil
		case x.kind == tChar:
			b.WriteString(x.text)
		case x.kind == tSpace:
			b.WriteByte(' ')
		default:
			return "", false, diag.Syntaxf(x.off, "unexpected %s in [ ... ]", cmdName(x))
		}
	}
}

// optionalDelim reads a braced delimiter that may be empty ({}, {(},
// {\\lbrace}) and returns the delimiter character, "" for none.
func (p *parser) optionalDelim(cmd token) (string, error) {
	open, err := p.nextNonSpace()
	if err != nil {
		return "", err
	}
	if !open.isChar("{") {
		p.e.unread(open)
		return p.delim(cmd)
	}
	t, err := p.nextNonSpace()
	if err != nil {
		return "", err
	}
	if t.isChar("}") {
		return "", nil
	}
	p.e.unread(t)
	d, err := p.delim(cmd)
	if err != nil {
		return "", err
	}
	if c, err := p.nextNonSpace(); err != nil {
		return "", err
	} else if !c.isChar("}") {
		return "", diag.Syntaxf(c.off, "expected } after the delimiter of \\%s", cmd.text)
	}
	return d, nil
}

// classOf returns the class of a single-symbol list, Ord otherwise.
func classOf(l []ast.Node) ast.Class {
	if len(l) == 1 {
		if s, ok := l[0].(*ast.Symbol); ok {
			return s.Class
		}
	}
	return ast.ClassOrd
}

func (p *parser) leftRight(t token) (ast.Node, error) {
	l, err := p.delim(t)
	if err != nil {
		return nil, err
	}
	p.left++
	p.e.pushScope() // \left ... \right is a group
	body, end, err := p.list()
	p.e.popScope()
	p.left--
	if err != nil {
		return nil, err
	}
	if !end.isCmd("right") {
		if end.isChar("&") || isRowEnd(end) {
			what := end.text
			if end.kind != tChar {
				what = `\` + what
			}
			return nil, diag.Syntaxf(end.off, "%s cannot appear between \\left and \\right", what)
		}
		if end.kind == tEOF || end.isChar("}") || end.isCmd("end") {
			return nil, diag.Syntaxf(t.off, "missing \\right")
		}
		return nil, p.unexpected(end)
	}
	r, err := p.delim(end)
	if err != nil {
		return nil, err
	}
	return &ast.LeftRight{Offset: t.off, Left: l, Right: r, Body: body}, nil
}
