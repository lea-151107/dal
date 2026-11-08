package box

import (
	"fmt"
	"math"

	"github.com/lea-151107/dal/internal/guard"
	"github.com/lea-151107/dal/internal/mathfont"
)

// stretchVert returns glyph r at a total height (height + depth) of at
// least target: the smallest size variant that is large enough, else a
// glyph assembly, else the largest variant.
func (l *layouter) stretchVert(r rune, target, s float64, off int) (*Box, error) {
	return l.stretch(r, target, s, off, false)
}

// stretchHoriz is stretchVert for widths.
func (l *layouter) stretchHoriz(r rune, target, s float64, off int) (*Box, error) {
	return l.stretch(r, target, s, off, true)
}

func (l *layouter) size(g uint16, s float64, horiz bool) float64 {
	m := l.F.Metrics(g)
	if horiz {
		if m.Advance == 0 {
			return float64((m.MaxX - m.MinX) * s)
		}
		return float64(m.Advance * s)
	}
	return float64((m.Height + m.Depth) * s)
}

func (l *layouter) stretch(r rune, target, s float64, off int, horiz bool) (*Box, error) {
	g := l.F.Glyph(r)
	if g == 0 {
		// NOTE(katex-compat): a delimiter the math font lacks (\lmoustache)
		// is drawn at its natural size by l.missing and does not stretch.
		return l.missing(r, s, off)
	}
	if l.size(g, s, horiz) >= target {
		return l.glyphBox(g, s), nil
	}
	best := g
	for _, v := range l.F.Variants(g, horiz) {
		best = v.Glyph
		if float64(v.Advance*s) >= target {
			return l.glyphBox(v.Glyph, s), nil
		}
	}
	if parts, _ := l.F.Assembly(g, horiz); len(parts) > 0 {
		return l.assemble(parts, target, s, horiz)
	}
	return l.glyphBox(best, s), nil
}

// assemble builds a glyph assembly of at least target length (MathML Core,
// "shaping of the glyph assembly").
func (l *layouter) assemble(parts []mathfont.Part, target, s float64, horiz bool) (*Box, error) {
	minO := float64(l.F.MinConnectorOverlap() * s)
	hasExt := false
	for _, p := range parts {
		hasExt = hasExt || p.Extender
	}
	var seq []mathfont.Part
	for reps := 0; ; reps++ {
		seq = seq[:0]
		for _, p := range parts {
			if !p.Extender {
				seq = append(seq, p)
				continue
			}
			for i := 0; i < reps; i++ {
				seq = append(seq, p)
			}
		}
		if len(seq) > maxAssemblyParts {
			return nil, fmt.Errorf("%w: glyph assembly of more than %d parts", guard.ErrTooLarge, maxAssemblyParts)
		}
		total := 0.0
		for _, p := range seq {
			total += float64(p.FullAdvance * s)
		}
		// Stop once the parts, overlapped by the minimum, reach the target.
		// A single part is not enough by itself: the floor and ceiling
		// brackets have one fixed part and one extender, and with no
		// extender yet they are shorter than their largest size variant.
		// An extender that adds nothing beyond the overlap ends in the
		// maxAssemblyParts check above.
		if !hasExt || (len(seq) > 0 && total-float64(minO*float64(len(seq)-1)) >= target) {
			break
		}
		if err := l.G.Tick(); err != nil {
			return nil, err
		}
	}
	// A font can make every part tiny; bound the parts of all assemblies in
	// one call as well as of each one.
	l.parts += len(seq)
	if l.parts > maxTotalAssemblyParts {
		return nil, fmt.Errorf("%w: glyph assemblies of more than %d parts", guard.ErrTooLarge, maxTotalAssemblyParts)
	}
	total, maxO := 0.0, -1.0
	for i, p := range seq {
		total += float64(p.FullAdvance * s)
		if i > 0 {
			o := float64(min(seq[i-1].EndConn, p.StartConn) * s)
			if maxO < 0 || o < maxO {
				maxO = o
			}
		}
	}
	o := 0.0
	if n := len(seq); n > 1 {
		o = (total - target) / float64(n-1)
		o = max(min(o, max(maxO, minO)), minO)
	}
	b := &Box{Assembly: true}
	pos := 0.0
	// Each part is drawn up to the middle of its overlaps with the next
	// and the previous parts. Drawn whole, the overlaps would be painted
	// twice: their partly covered pixels would come out darker on the
	// edges of a stroke and lighter in its middle.
	lo := math.Inf(-1)
	for i, p := range seq {
		if i > 0 {
			pos -= o
		}
		adv := float64(p.FullAdvance * s)
		hi := math.Inf(1)
		if i < len(seq)-1 {
			hi = adv - o/2
		}
		gb := l.glyphBox(p.Glyph, s)
		var leaf Leaf = gb.Leaf
		if gl, ok := leaf.(GlyphLeaf); ok {
			gl.Clip = &Band{Horiz: horiz, Lo: lo, Hi: hi}
			leaf = gl
		}
		// The next part starts o before this one ends: the cut is o/2
		// above (or right of) its origin.
		lo = o / 2
		if horiz {
			b.add(pos, 0, &Box{W: gb.W, Leaf: leaf})
			b.H, b.D = max(b.H, gb.H), max(b.D, gb.D)
		} else {
			b.add(0, pos, &Box{W: gb.W, Leaf: leaf})
			b.W = max(b.W, gb.W)
		}
		pos += adv
	}
	if horiz {
		b.W = pos
	} else {
		b.H = pos
	}
	return b, nil
}
