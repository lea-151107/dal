package box

import "github.com/lea/dal/internal/ast"

func (l *layouter) array(n *ast.Array, outer style) ([]atom, error) {
	// Only the top-level array puts its row tags at the line's end; the
	// cells must not see tagRight.
	tagRight := l.tagRight
	l.tagRight = 0
	st := styleOf(n.Style, style{level: 1})
	if n.Style == ast.StyleNone {
		st = outer
	}
	s := l.scale(st)
	em := float64(l.em * s)
	ncols := 0
	for _, r := range n.Rows {
		ncols = max(ncols, len(r.Cells))
	}
	cells := make([][]*Box, len(n.Rows))
	colW := make([]float64, ncols)
	rowH := make([]float64, len(n.Rows))
	rowD := make([]float64, len(n.Rows))
	strutH, strutD := float64(arrayStrutHeightEm*em), float64(arrayStrutDepthEm*em)
	switch n.Env {
	case "smallmatrix":
		strutH, strutD = float64(strutH*0.6), float64(strutD*0.6)
	case "substack":
		// Rows of \substack sit as close as lines of script text.
		strutH, strutD = float64(strutH*0.7), float64(strutD*0.7)
	}
	for i, r := range n.Rows {
		cells[i] = make([]*Box, ncols)
		rowH[i], rowD[i] = strutH, strutD
		for j := 0; j < ncols; j++ {
			if err := l.G.Tick(); err != nil {
				return nil, err
			}
			var cell []ast.Node
			if j < len(r.Cells) {
				cell = r.Cells[j]
			}
			b, err := l.list(cell, st)
			if err != nil {
				return nil, err
			}
			cells[i][j] = b
			colW[j] = max(colW[j], b.W)
			rowH[i], rowD[i] = max(rowH[i], b.H), max(rowD[i], b.D)
		}
	}
	align := func(j int) ast.Align {
		if n.AlignPairs {
			if j%2 == 0 {
				return ast.AlignRight
			}
			return ast.AlignLeft
		}
		if len(n.Cols) == 0 {
			return ast.AlignCenter
		}
		if j < len(n.Cols) {
			return n.Cols[j].Align
		}
		return n.Cols[len(n.Cols)-1].Align
	}
	rulesBefore := func(j int) int {
		if j < len(n.Cols) {
			return n.Cols[j].RuleBefore
		}
		return 0
	}
	dashedBefore := func(j int) bool { return j < len(n.Cols) && n.Cols[j].Dashed }
	// Horizontal positions.
	sep := float64(n.ColSepEm * em)
	ruleW := float64(arrayRuleEm * em)
	colX := make([]float64, ncols)
	x := 0.0
	if n.Env == "array" {
		x = sep / 2
	}
	type vrule struct {
		x      float64
		dashed bool
	}
	var vrules []vrule
	for j := 0; j < ncols; j++ {
		if j > 0 {
			gap := sep
			if n.AlignPairs && j%2 == 1 {
				gap = 0
			}
			k := rulesBefore(j)
			if k > 0 {
				x += gap / 2
				for r := 0; r < k; r++ {
					vrules = append(vrules, vrule{x, dashedBefore(j)})
					x += float64(ruleW * 2)
				}
				x += gap / 2
			} else {
				x += gap
			}
		} else {
			for r := 0; r < rulesBefore(0); r++ {
				vrules = append(vrules, vrule{x - sep/2, dashedBefore(0)})
				x += float64(ruleW * 2)
			}
		}
		colX[j] = x
		x += colW[j]
	}
	if n.Env == "array" {
		x += sep / 2
	}
	for r := 0; r < n.RuleAfter; r++ {
		vrules = append(vrules, vrule{x, n.RuleAfterDashed})
		x += float64(ruleW * 2)
	}
	width := x
	// Vertical positions, from the top.
	gapY := 0.0
	if n.AlignPairs || n.Env == "gathered" || n.Env == "gather" || n.Env == "gather*" || n.Env == "lines" {
		gapY = float64(jotEm * em)
	}
	if n.Env == "cases" || n.Env == "dcases" || n.Env == "rcases" {
		gapY = float64(0.2 * em)
	}
	total := 0.0
	baseY := make([]float64, len(n.Rows)) // baseline offset from the top
	type hrule struct {
		y      float64
		dashed bool
	}
	var hrules []hrule
	for i := range n.Rows {
		if i > 0 {
			total += gapY
		}
		for k := 0; k < n.Rows[i].HLineBefore; k++ {
			hrules = append(hrules, hrule{total, n.Rows[i].HLineDashed})
			total += float64(ruleW * 2)
		}
		baseY[i] = total + rowH[i]
		total += rowH[i] + rowD[i]
	}
	for k := 0; k < n.HLineAfter; k++ {
		hrules = append(hrules, hrule{total, n.HLineAfterDashed})
		total += float64(ruleW * 2)
	}
	axis := float64(l.c.AxisHeight * s)
	top := total/2 + axis
	b := &Box{W: width, H: top, D: total - top}
	for i := range n.Rows {
		if err := l.G.Tick(); err != nil {
			return nil, err
		}
		y := top - baseY[i]
		for j := 0; j < ncols; j++ {
			c := cells[i][j]
			cx := colX[j]
			switch align(j) {
			case ast.AlignCenter:
				cx += (colW[j] - c.W) / 2
			case ast.AlignRight:
				cx += colW[j] - c.W
			}
			b.add(cx, y, c)
		}
	}
	dash, dashGap := float64(dashEm*em), float64(dashGapEm*em)
	// dashed reports whether a rule of the given length can be drawn
	// dashed within the per-call budget; past it rules are drawn solid, so
	// that many long dashed rules cannot make millions of boxes.
	dashed := func(d bool, length float64) bool {
		if !d {
			return false
		}
		n := int(length/(dash+dashGap)) + 1
		if l.dashes+n > maxTotalDashes {
			return false
		}
		l.dashes += n
		return true
	}
	for _, r := range vrules {
		if !dashed(r.dashed, total) {
			b.add(r.x, -b.D, rule(ruleW, total, 0))
			continue
		}
		for y := 0.0; y < total; y += dash + dashGap {
			b.add(r.x, top-y-min(dash, total-y), rule(ruleW, min(dash, total-y), 0))
		}
	}
	for _, r := range hrules {
		if !dashed(r.dashed, width) {
			b.add(0, top-r.y-ruleW, rule(width, ruleW, 0))
			continue
		}
		for x := 0.0; x < width; x += dash + dashGap {
			b.add(x, top-r.y-ruleW, rule(min(dash, width-x), ruleW, 0))
		}
	}
	// Row tags go to the right of the array.
	hasTags := false
	for _, r := range n.Rows {
		hasTags = hasTags || r.Tag != nil
	}
	if hasTags {
		tagX := width + float64(tagGapEm*l.em)
		for i, r := range n.Rows {
			if r.Tag == nil {
				continue
			}
			tb, err := l.tag(r.Tag, st)
			if err != nil {
				return nil, err
			}
			x := tagX
			if tagRight-tb.W > tagX {
				x = tagRight - tb.W
			}
			b.add(x, top-baseY[i], tb)
			b.W = max(b.W, x+tb.W)
		}
	}
	if n.Left == "" && n.Right == "" {
		return ord(b), nil
	}
	return l.fenced(b, n.Left, n.Right, outer, false)
}
