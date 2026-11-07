// Package scene is the drawing-command layer between typesetting and
// output: positioned glyphs, rules, text runs and lines in pixels. An
// SVG writer can be added on top of it without touching the typesetter.
package scene

import "image/color"

// Scene is a typeset formula ready to draw. Coordinates are in pixels with
// y growing downwards; the origin is the left end of the baseline.
type Scene struct {
	// Typographic box of the formula.
	Width, Height, Depth float64
	// Ink bounds relative to the origin; they may exceed the box (italic
	// overhang, accents).
	InkMinX, InkMinY, InkMaxX, InkMaxY float64
	Items                              []Item

	inked bool
}

// Item is one drawing command.
type Item interface{ isItem() }

// Glyph draws a glyph of the math font with its origin at (X, Y). Scale
// converts font units to pixels.
type Glyph struct {
	X, Y  float64
	Scale float64
	Glyph uint16
	Color color.NRGBA
	// Mirror draws the glyph flipped left to right about its origin
	// (\reflectbox): it extends to the left of X.
	Mirror bool
	// Clip, when not nil, limits the glyph to a band of the scene.
	Clip *Band
	// Group is nonzero for the parts of a glyph assembly. Consecutive
	// glyphs of one group are drawn together, so that coverage is added
	// up before compositing: their bands do not overlap, and the parts
	// join without seams.
	Group int
}

// Band is the part of the scene between Lo and Hi (pixels, Lo < Hi) along
// y when Vert, else along x. An open end is infinite.
type Band struct {
	Vert   bool
	Lo, Hi float64
}

// Rect fills a rectangle.
type Rect struct {
	X0, Y0, X1, Y1 float64
	Color          color.NRGBA
}

// Text draws a run of text with the caller's text face at its baseline.
type Text struct {
	X, Y   float64
	SizePx float64
	Text   string
	Color  color.NRGBA
}

// Line draws a straight line of the given width.
type Line struct {
	X0, Y0, X1, Y1 float64
	Width          float64
	Color          color.NRGBA
}

func (*Glyph) isItem() {}
func (*Rect) isItem()  {}
func (*Text) isItem()  {}
func (*Line) isItem()  {}

// AddInk extends the ink bounds by the rectangle.
func (s *Scene) AddInk(x0, y0, x1, y1 float64) {
	if !s.inked {
		s.InkMinX, s.InkMinY, s.InkMaxX, s.InkMaxY = x0, y0, x1, y1
		s.inked = true
		return
	}
	s.InkMinX = min(s.InkMinX, x0)
	s.InkMinY = min(s.InkMinY, y0)
	s.InkMaxX = max(s.InkMaxX, x1)
	s.InkMaxY = max(s.InkMaxY, y1)
}
