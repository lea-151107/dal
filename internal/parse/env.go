package parse

import (
	"github.com/lea-151107/dal/internal/ast"
	"github.com/lea-151107/dal/internal/diag"
)

// envInfo describes how an environment's array is set up.
type envInfo struct {
	left, right string
	style       ast.MathStyle
	cols        []ast.Column
	alignPairs  bool
	colSepEm    float64
	spec        bool // takes a column specification argument
	tags        bool // rows may carry \tag
	starAlign   bool // matrix*: takes an optional [l|c|r] for all columns
	count       bool // alignat: takes the number of column pairs
	noAmp       bool // & is not allowed
	noRows      bool // \\ is not allowed
}

var envs = map[string]envInfo{
	"matrix":      {style: ast.StyleText, colSepEm: 1},
	"pmatrix":     {left: "(", right: ")", style: ast.StyleText, colSepEm: 1},
	"bmatrix":     {left: "[", right: "]", style: ast.StyleText, colSepEm: 1},
	"Bmatrix":     {left: "{", right: "}", style: ast.StyleText, colSepEm: 1},
	"vmatrix":     {left: "|", right: "|", style: ast.StyleText, colSepEm: 1},
	"Vmatrix":     {left: "‖", right: "‖", style: ast.StyleText, colSepEm: 1},
	"smallmatrix": {style: ast.StyleScript, colSepEm: 0.556},
	"matrix*":     {style: ast.StyleText, colSepEm: 1, starAlign: true},
	"pmatrix*":    {left: "(", right: ")", style: ast.StyleText, colSepEm: 1, starAlign: true},
	"bmatrix*":    {left: "[", right: "]", style: ast.StyleText, colSepEm: 1, starAlign: true},
	"Bmatrix*":    {left: "{", right: "}", style: ast.StyleText, colSepEm: 1, starAlign: true},
	"vmatrix*":    {left: "|", right: "|", style: ast.StyleText, colSepEm: 1, starAlign: true},
	"Vmatrix*":    {left: "‖", right: "‖", style: ast.StyleText, colSepEm: 1, starAlign: true},
	"cases": {left: "{", style: ast.StyleText, colSepEm: 1,
		cols: []ast.Column{{Align: ast.AlignLeft}, {Align: ast.AlignLeft}}},
	"dcases": {left: "{", style: ast.StyleDisplay, colSepEm: 1,
		cols: []ast.Column{{Align: ast.AlignLeft}, {Align: ast.AlignLeft}}},
	"rcases": {right: "}", style: ast.StyleText, colSepEm: 1,
		cols: []ast.Column{{Align: ast.AlignLeft}, {Align: ast.AlignLeft}}},
	"drcases": {right: "}", style: ast.StyleDisplay, colSepEm: 1,
		cols: []ast.Column{{Align: ast.AlignLeft}, {Align: ast.AlignLeft}}},
	"alignat":   {style: ast.StyleDisplay, alignPairs: true, count: true, tags: true},
	"alignat*":  {style: ast.StyleDisplay, alignPairs: true, count: true},
	"alignedat": {style: ast.StyleDisplay, alignPairs: true, count: true},
	"aligned":   {style: ast.StyleDisplay, alignPairs: true, colSepEm: 2},
	"split":     {style: ast.StyleDisplay, alignPairs: true, colSepEm: 2},
	"align":     {style: ast.StyleDisplay, alignPairs: true, colSepEm: 2, tags: true},
	"align*":    {style: ast.StyleDisplay, alignPairs: true, colSepEm: 2},
	"gathered":  {style: ast.StyleDisplay, noAmp: true},
	"gather":    {style: ast.StyleDisplay, tags: true, noAmp: true},
	"gather*":   {style: ast.StyleDisplay, noAmp: true},
	"equation":  {style: ast.StyleDisplay, tags: true, noAmp: true, noRows: true},
	"equation*": {style: ast.StyleDisplay, noAmp: true, noRows: true},
	"array":     {style: ast.StyleText, colSepEm: 1, spec: true},
	"darray":    {style: ast.StyleDisplay, colSepEm: 1, spec: true},
	// substack is \substack{...}: closed by } instead of \end.
	"substack": {style: ast.StyleScript, cols: []ast.Column{{Align: ast.AlignCenter}}, noAmp: true},
	// lines holds a formula broken with \\ or \newline outside any
	// environment.
	"lines": {cols: []ast.Column{{Align: ast.AlignLeft}}, noAmp: true},
}

func (p *parser) envName(t token) (string, error) {
	n, err := p.argText(t)
	if err != nil {
		return "", err
	}
	return n, nil
}

func (p *parser) environment(begin token) (ast.Node, error) {
	name, err := p.envName(begin)
	if err != nil {
		return nil, err
	}
	if name == "CD" {
		return p.cdEnv(begin)
	}
	info, ok := envs[name]
	if !ok || name == "substack" || name == "lines" {
		return nil, diag.Unsupported("\\begin{"+name+"}", begin.off)
	}
	a := newArray(begin.off, name, info)
	if info.starAlign {
		if err := p.starAlign(begin, a); err != nil {
			return nil, err
		}
	}
	if info.count {
		// The number of column pairs only matters to LaTeX's checks.
		if _, err := p.argText(begin); err != nil {
			return nil, err
		}
	}
	if info.spec {
		if err := p.columnSpec(begin, a); err != nil {
			return nil, err
		}
	}
	return p.rows(begin, a, info, nil)
}

func newArray(off int, name string, info envInfo) *ast.Array {
	return &ast.Array{
		Offset: off, Env: name, Left: info.left, Right: info.right, Style: info.style,
		Cols: info.cols, AlignPairs: info.alignPairs, ColSepEm: info.colSepEm,
	}
}

// rows parses the body of an array up to its end: \end{name}, or the }
// that closes \substack when closing is the opening brace, or the end of
// the formula for "lines" (whose first row is first).
func (p *parser) rows(begin token, a *ast.Array, info envInfo, first []ast.Node) (ast.Node, error) {
	name := a.Env
	p.array++
	defer func() { p.array-- }()
	savedTag := p.tag
	defer func() { p.tag = savedTag }()

	// Each cell is a group for local definitions, as in TeX. Lines broken
	// with \\ outside any environment are not (as in KaTeX): a definition
	// on one line holds on the next.
	cells := name != "lines"
	if cells {
		p.e.pushScope()
		defer p.e.popScope()
	}
	row := ast.Row{}
	if first != nil {
		row.Cells = [][]ast.Node{first}
		a.Rows = append(a.Rows, row)
		row = ast.Row{}
	}
	var err error
	if row.HLineBefore, row.HLineDashed, err = p.countHLines(); err != nil {
		return nil, err
	}
	for {
		p.tag = nil
		if info.tags {
			p.tag = &row.Tag
		}
		cell, end, err := p.list()
		if err != nil {
			return nil, err
		}
		if info.alignPairs && len(row.Cells)%2 == 1 {
			// The right-hand side of an alignment point starts with an
			// empty Ord so that "&= b" gets relation spacing.
			cell = append([]ast.Node{&ast.Group{Offset: begin.off}}, cell...)
		}
		row.Cells = append(row.Cells, cell)
		if err := p.count(); err != nil {
			return nil, err
		}
		if cells && (end.isChar("&") || isRowEnd(end)) {
			p.e.popScope()
			p.e.pushScope()
		}
		switch {
		case end.isChar("&"):
			if info.noAmp {
				return nil, diag.Syntaxf(end.off, "misplaced & in %s", name)
			}
			continue
		case isRowEnd(end):
			if info.noRows {
				return nil, diag.Syntaxf(end.off, "misplaced \\\\ in %s", name)
			}
			if _, _, err := p.optArg(); err != nil { // row spacing like \\[2pt] is ignored
				return nil, err
			}
			a.Rows = append(a.Rows, row)
			row = ast.Row{}
			if row.HLineBefore, row.HLineDashed, err = p.countHLines(); err != nil {
				return nil, err
			}
			continue
		case end.isCmd("end") && name != "substack" && name != "lines":
			endName, err := p.envName(end)
			if err != nil {
				return nil, err
			}
			if endName != name {
				return nil, diag.Syntaxf(end.off, "\\begin{%s} ended by \\end{%s}", name, endName)
			}
		case end.isChar("}") && name == "substack":
		case end.kind == tEOF && name == "lines":
		case end.kind == tEOF && name == "substack":
			return nil, diag.Syntaxf(begin.off, "missing }")
		case end.kind == tEOF:
			return nil, diag.Syntaxf(begin.off, "missing \\end{%s}", name)
		default:
			return nil, p.unexpected(end)
		}
		// A trailing \\ leaves an empty last row, which LaTeX drops.
		if len(row.Cells) == 1 && len(row.Cells[0]) == 0 && row.Tag == nil && len(a.Rows) > 0 {
			a.HLineAfter, a.HLineAfterDashed = row.HLineBefore, row.HLineDashed
		} else {
			a.Rows = append(a.Rows, row)
		}
		// Layout pads short rows to the widest one, so the padding
		// cells count as nodes too.
		if err := p.countPadding(a); err != nil {
			return nil, err
		}
		for _, r := range a.Rows {
			if r.Tag != nil {
				p.tagged = append(p.tagged, a)
				break
			}
		}
		return a, nil
	}
}

func isRowEnd(t token) bool { return t.isCmd("\\") || t.isCmd("cr") || t.isCmd("newline") }

// starAlign reads the optional [l], [c] or [r] of matrix* environments.
func (p *parser) starAlign(begin token, a *ast.Array) error {
	t, err := p.nextNonSpace()
	if err != nil {
		return err
	}
	al := ast.AlignCenter
	if t.isChar("[") {
		c, err := p.nextNonSpace()
		if err != nil {
			return err
		}
		switch {
		case c.isChar("l"):
			al = ast.AlignLeft
		case c.isChar("c"):
		case c.isChar("r"):
			al = ast.AlignRight
		default:
			return diag.Syntaxf(c.off, "invalid alignment in \\begin{%s}: want l, c or r", a.Env)
		}
		if e, err := p.nextNonSpace(); err != nil {
			return err
		} else if !e.isChar("]") {
			return diag.Syntaxf(e.off, "missing ] in \\begin{%s}", a.Env)
		}
	} else {
		p.e.unread(t)
	}
	a.Cols = []ast.Column{{Align: al}}
	return nil
}

// substack parses \substack{a \\ b}.
func (p *parser) substack(t token) (ast.Node, error) {
	open, err := p.nextNonSpace()
	if err != nil {
		return nil, err
	}
	if !open.isChar("{") {
		return nil, diag.Syntaxf(cmdOff(t, open), "missing argument for \\substack")
	}
	info := envs["substack"]
	return p.rows(open, newArray(t.off, "substack", info), info, nil)
}

// countHLines reads \hline and \hdashline commands; dashed is true when
// any of them is \hdashline.
// NOTE(katex-compat): a mix of \hline and \hdashline before one row is
// drawn all dashed.
func (p *parser) countHLines() (n int, dashed bool, err error) {
	for {
		t, err := p.nextNonSpace()
		if err != nil {
			return 0, false, err
		}
		if !t.isCmd("hline") && !t.isCmd("hdashline") {
			p.e.unread(t)
			return n, dashed, nil
		}
		if err := p.count(); err != nil {
			return 0, false, err
		}
		n++
		dashed = dashed || t.isCmd("hdashline")
	}
}

func (p *parser) columnSpec(begin token, a *ast.Array) error {
	spec, err := p.argText(begin)
	if err != nil {
		return err
	}
	rules := 0
	dashed := false
	for i := 0; i < len(spec); i++ {
		switch c := spec[i]; c {
		case 'l', 'c', 'r':
			al := map[byte]ast.Align{'l': ast.AlignLeft, 'c': ast.AlignCenter, 'r': ast.AlignRight}[c]
			a.Cols = append(a.Cols, ast.Column{Align: al, RuleBefore: rules, Dashed: dashed})
			rules, dashed = 0, false
		case '|', ':':
			if err := p.count(); err != nil {
				return err
			}
			rules++
			dashed = dashed || c == ':'
		case ' ':
		case 'p', 'm', 'b', '@', '!':
			// NOTE(katex-compat): paragraph columns (p{2cm}, m{...}, b{...})
			// become left-aligned columns of natural width, and @{...} /
			// !{...} inter-column material is dropped.
			end, ok := skipBraced(spec, i+1)
			if !ok {
				return diag.Syntaxf(begin.off, "missing {...} after %c in column specification", c)
			}
			i = end
			if c != '@' && c != '!' {
				a.Cols = append(a.Cols, ast.Column{Align: ast.AlignLeft, RuleBefore: rules, Dashed: dashed})
				rules, dashed = 0, false
			}
		default:
			return diag.Unsupported("array column type "+string(c), begin.off)
		}
	}
	if len(a.Cols) == 0 {
		return diag.Syntaxf(begin.off, "empty array column specification")
	}
	a.RuleAfter, a.RuleAfterDashed = rules, dashed
	return nil
}

func (p *parser) countPadding(a *ast.Array) error {
	cols, cells := 0, 0
	for _, r := range a.Rows {
		cols = max(cols, len(r.Cells))
		cells += len(r.Cells)
	}
	return p.countN(len(a.Rows)*cols - cells)
}

// skipBraced returns the index of the } that closes the group starting at
// spec[i] (after optional spaces).
func skipBraced(spec string, i int) (int, bool) {
	for i < len(spec) && spec[i] == ' ' {
		i++
	}
	if i >= len(spec) || spec[i] != '{' {
		return 0, false
	}
	depth := 0
	for ; i < len(spec); i++ {
		switch spec[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i, true
			}
		}
	}
	return 0, false
}

// cdEnv parses the CD environment (amscd): rows of objects joined by
// horizontal arrows (@>a>b>, @<<<, @=, @.) and rows of vertical arrows
// (@VaVbV, @AAA, @|, @.) under the objects. It builds an Array whose even
// columns hold objects and vertical arrows and whose odd columns hold
// horizontal arrows.
func (p *parser) cdEnv(begin token) (ast.Node, error) {
	a := &ast.Array{Offset: begin.off, Env: "CD", Style: ast.StyleDisplay, ColSepEm: 0.5,
		Cols: []ast.Column{{Align: ast.AlignCenter}}}
	p.cd++
	defer func() { p.cd-- }()
	savedTag := p.tag
	p.tag = nil
	defer func() { p.tag = savedTag }()
	for {
		var cells [][]ast.Node
		vertical := false
		for {
			obj, end, err := p.list()
			if err != nil {
				return nil, err
			}
			if !end.isChar("@") {
				if len(obj) > 0 || len(cells) > 0 {
					cells = append(cells, obj)
				}
				switch {
				case isRowEnd(end):
				case end.isCmd("end"):
					endName, err := p.envName(end)
					if err != nil {
						return nil, err
					}
					if endName != "CD" {
						return nil, diag.Syntaxf(end.off, "\\begin{CD} ended by \\end{%s}", endName)
					}
				case end.kind == tEOF:
					return nil, diag.Syntaxf(begin.off, "missing \\end{CD}")
				default:
					return nil, p.unexpected(end)
				}
				for len(cells) > 0 && len(cells[len(cells)-1]) == 0 {
					cells = cells[:len(cells)-1] // trailing empty cells
				}
				if len(cells) > 0 {
					a.Rows = append(a.Rows, ast.Row{Cells: cells})
					if err := p.countN(len(cells)); err != nil {
						return nil, err
					}
				}
				if end.isCmd("end") {
					if err := p.countPadding(a); err != nil {
						return nil, err
					}
					return a, nil
				}
				break
			}
			arrow, isVert, err := p.cdArrow(end)
			if err != nil {
				return nil, err
			}
			if isVert {
				// Vertical arrows stand under the objects: even columns.
				if !vertical && len(obj) > 0 {
					return nil, diag.Syntaxf(end.off, "an object in a row of vertical arrows")
				}
				vertical = true
				if len(cells)%2 == 1 {
					cells = append(cells, nil)
				}
				cells = append(cells, arrow)
				continue
			}
			cells = append(cells, obj, arrow)
		}
	}
}

// cdArrow reads the arrow after @ in a CD environment.
func (p *parser) cdArrow(at token) (cell []ast.Node, vertical bool, err error) {
	t, err := p.e.next()
	if err != nil {
		return nil, false, err
	}
	if t.kind != tChar {
		return nil, false, diag.Syntaxf(at.off, "invalid arrow after @ in CD")
	}
	off := at.off
	switch c := t.text; c {
	case ".":
		return []ast.Node{}, false, nil
	case "=":
		return []ast.Node{&ast.XArrow{Offset: off, Arrow: '='}}, false, nil
	case "|":
		return []ast.Node{&ast.SizedDelim{Offset: off, Delim: "‖", Size: 3, Class: ast.ClassOrd}}, true, nil
	case ">", "<", "V", "A":
		first, err := p.cdLabel(at, c)
		if err != nil {
			return nil, false, err
		}
		second, err := p.cdLabel(at, c)
		if err != nil {
			return nil, false, err
		}
		if c == ">" || c == "<" {
			r := '→'
			if c == "<" {
				r = '←'
			}
			// The label is required by xarrow; an empty one is fine.
			return []ast.Node{&ast.XArrow{Offset: off, Arrow: r, Over: first, Under: second}}, false, nil
		}
		d := "↓"
		if c == "A" {
			d = "↑"
		}
		body := []ast.Node{}
		if len(first) > 0 {
			body = append(body, &ast.Style{Offset: off, Style: ast.StyleScript, Body: first}, &ast.Space{Offset: off, Mu: 3})
		}
		body = append(body, &ast.SizedDelim{Offset: off, Delim: d, Size: 3, Class: ast.ClassOrd})
		if len(second) > 0 {
			body = append(body, &ast.Space{Offset: off, Mu: 3}, &ast.Style{Offset: off, Style: ast.StyleScript, Body: second})
		}
		return []ast.Node{&ast.Group{Offset: off, Body: body}}, true, nil
	}
	return nil, false, diag.Syntaxf(t.off, "invalid arrow @%s in CD (want @>>>, @<<<, @VVV, @AAA, @=, @| or @.)", t.text)
}

// cdLabel parses a label of a CD arrow up to the next stop character.
func (p *parser) cdLabel(at token, stop string) ([]ast.Node, error) {
	saved := p.cdStop
	p.cdStop = stop
	body, end, err := p.list()
	p.cdStop = saved
	if err != nil {
		return nil, err
	}
	if !end.isChar(stop) {
		return nil, diag.Syntaxf(at.off, "missing %s to end the arrow label", stop)
	}
	return body, nil
}
