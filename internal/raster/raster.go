// Package raster draws a scene into an RGBA image with anti-aliasing
// (golang.org/x/image/vector). Coordinates are rounded to 1/64 px first so
// that floating-point differences between CPUs do not change the pixels.
package raster

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"unicode/utf8"

	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"

	"github.com/lea/dal/internal/curve"
	"github.com/lea/dal/internal/guard"
	"github.com/lea/dal/internal/mathfont"
	"github.com/lea/dal/internal/scene"
)

// DefaultMaxPixels is the default bound on the size of an image. One pixel
// costs about 9 bytes while drawing (4 for the RGBA image, 1 for the
// coverage mask and 4 for the rasterizer's accumulation buffer), so the
// default allows about 150 MB per call.
const DefaultMaxPixels = 16 << 20

// BytesPerPixel is the peak memory used per image pixel while drawing.
const BytesPerPixel = 9

// ctxCheckPixels is how many pixels the rasterizer may touch between two
// checks of the context. Tick counts items, but one item (a huge \rule) can
// cost far more than many small glyphs; at roughly 100 Mpx/s this keeps a
// cancelled call from running more than a few milliseconds past the
// deadline, apart from a single fill already in progress.
const ctxCheckPixels = 1 << 18

// FaceFunc returns the text face for a size; see Options.TextFace.
type FaceFunc func(sizePx float64) (font.Face, error)

// Layout is where a scene lands in the image.
type Layout struct {
	W, H     int     // image size
	OriginX  float64 // x of the formula origin (a whole number)
	Baseline int     // y of the baseline
}

// Plan computes the image size for sc with padding: the typographic box
// plus any ink that sticks out of it.
//
// NOTE: the extents are rounded to 1/64 px before floor and ceil, like
// every coordinate drawn, so that the last bits of floating-point results
// (which can differ between CPUs, e.g. with fused multiply-add on arm64)
// do not move the image size by a pixel.
func Plan(sc *scene.Scene, pad int) Layout {
	s := func(v float64) float64 { return math.Round(v*64) / 64 }
	left := math.Floor(s(min(0, sc.InkMinX)))
	right := math.Ceil(s(max(sc.Width, sc.InkMaxX)))
	top := math.Ceil(s(max(sc.Height, -sc.InkMinY)))
	bottom := math.Ceil(s(max(sc.Depth, sc.InkMaxY)))
	return Layout{
		W:        clampInt(right-left) + 2*pad,
		H:        clampInt(top+bottom) + 2*pad,
		OriginX:  float64(pad) - left,
		Baseline: pad + clampInt(top),
	}
}

// maxSide bounds the width and the height of an image, whatever
// MaxPixels allows: see Render.
const maxSide = 1 << 24

// maxExtent bounds image extents converted to int: far beyond any pixel
// limit, and small enough for a 32-bit int with padding added, where an
// out-of-range conversion would wrap and slip past the size checks.
const maxExtent = 1 << 30

func clampInt(v float64) int {
	return int(max(-maxExtent, min(v, maxExtent)))
}

func snap(v float64) float64 { return math.Round(v*64) / 64 }

// Render draws sc. bg nil means transparent. maxPixels bounds the image
// size (W*H); zero or negative means DefaultMaxPixels. The context in g is
// checked both per item and per amount of pixels drawn, and once more at
// the end, so a cancelled call does not return an image.
func Render(g *guard.Guard, sc *scene.Scene, lay Layout, f *mathfont.Font, faces FaceFunc, bg color.Color, maxPixels int) (*image.RGBA, error) {
	if maxPixels <= 0 {
		maxPixels = DefaultMaxPixels
	}
	if lay.W <= 0 || lay.H <= 0 {
		lay.W, lay.H = max(lay.W, 1), max(lay.H, 1)
	}
	// The rasterizer takes float32 offsets within the image and glyph
	// points are 26.6 fixed point (to 2^25 px): beyond 2^24 px on a side
	// the image would come out wrong or blank.
	if lay.W > maxSide || lay.H > maxSide {
		return nil, fmt.Errorf("%w: %d×%d pixels exceed %d on a side", guard.ErrTooLarge, lay.W, lay.H, maxSide)
	}
	// Compare before multiplying: W*H can overflow int.
	if lay.W > maxPixels/lay.H {
		return nil, fmt.Errorf("%w: %d×%d pixels exceed limit %d", guard.ErrTooLarge, lay.W, lay.H, maxPixels)
	}
	if err := checkWork(sc, f, maxPixels); err != nil {
		return nil, err
	}
	img := image.NewRGBA(image.Rect(0, 0, lay.W, lay.H))
	if bg != nil {
		draw.Draw(img, img.Bounds(), image.NewUniform(bg), image.Point{}, draw.Src)
	}
	if err := g.Err(); err != nil {
		return nil, err
	}
	r := &renderer{g: g, img: img, ox: lay.OriginX, oy: float64(lay.Baseline), f: f, ras: vector.NewRasterizer(1, 1),
		maxFlush: min(maxPixels, math.MaxInt/workFactor) * workFactor}
	for i := 0; i < len(sc.Items); i++ {
		if err := g.Tick(); err != nil {
			return nil, err
		}
		if r.err != nil {
			return nil, r.err
		}
		var err error
		switch it := sc.Items[i].(type) {
		case *scene.Glyph:
			gls := []*scene.Glyph{it}
			for it.Group != 0 && i+1 < len(sc.Items) {
				next, ok := sc.Items[i+1].(*scene.Glyph)
				if !ok || next.Group != it.Group || next.Color != it.Color {
					break
				}
				if err := g.Tick(); err != nil {
					return nil, err
				}
				gls = append(gls, next)
				i++
			}
			err = r.glyphs(gls)
		case *scene.Rect:
			r.rect(it)
		case *scene.Line:
			r.line(it)
		case *scene.Text:
			if r.flush(); r.err != nil {
				return nil, r.err
			}
			err = r.text(it, faces)
		}
		if err != nil {
			return nil, err
		}
	}
	if r.flush(); r.err != nil {
		return nil, r.err
	}
	if err := g.Err(); err != nil {
		return nil, err
	}
	return img, nil
}

// workFactor bounds the pixels the rasterizer touches in one call, summed
// over all items, as a multiple of the image size limit. Overlapping
// glyphs (for example with negative kerns) can make this far larger than
// the image.
const workFactor = 4

func checkWork(sc *scene.Scene, f *mathfont.Font, maxPixels int) error {
	maxWork := float64(workFactor) * float64(maxPixels)
	work := 0.0
	// Each product is converted explicitly so that it is not fused into
	// the sum (arm64, GOAMD64=v3): the result decides ErrTooLarge, which
	// must not depend on the machine.
	for _, it := range sc.Items {
		switch it := it.(type) {
		case *scene.Glyph:
			m := f.Metrics(it.Glyph)
			work += float64((m.MaxX - m.MinX + 2) * (m.Height + m.Depth + 2) * it.Scale * it.Scale)
		case *scene.Rect:
			work += float64((it.X1 - it.X0 + 2) * (it.Y1 - it.Y0 + 2))
		case *scene.Line:
			work += float64((math.Abs(it.X1-it.X0) + 2) * (math.Abs(it.Y1-it.Y0) + 2))
		case *scene.Text:
			// The face's metrics are unknown here: count an em square
			// per character.
			work += float64(float64(utf8.RuneCountInString(it.Text)) * (it.SizePx + 2) * (it.SizePx + 2))
		}
		if work > maxWork {
			return fmt.Errorf("%w: drawing touches more than %.0f pixels", guard.ErrTooLarge, maxWork)
		}
	}
	return nil
}

// renderer draws items of one color into a coverage mask and composites
// the mask when the color changes. Parts that overlap (glyph assembly
// connectors, a brace's pieces) then do not darken where they meet when
// the color is translucent.
type renderer struct {
	g    *guard.Guard
	err  error // context error found while filling; checked per item
	work int   // pixels touched since the last context check
	// flushed counts the pixels composited by flush, bounded by maxFlush:
	// a color change composites the union of everything drawn since the
	// last one, which checkWork does not see.
	flushed, maxFlush int
	img               *image.RGBA
	ox, oy            float64
	f                 *mathfont.Font
	ras               *vector.Rasterizer

	mask  *image.Alpha
	cur   color.NRGBA
	dirty image.Rectangle // part of mask drawn since the last flush
}

// flush composites the pending mask in the current color and clears it.
// It records ErrTooLarge or a context error in r.err.
func (r *renderer) flush() {
	if r.dirty.Empty() || r.err != nil {
		return
	}
	n := r.dirty.Dx() * r.dirty.Dy()
	if r.flushed += n; r.flushed > r.maxFlush {
		r.err = fmt.Errorf("%w: compositing touches more than %d pixels", guard.ErrTooLarge, r.maxFlush)
		return
	}
	if r.work += n; r.work >= ctxCheckPixels {
		r.work = 0
		if r.err = r.g.Err(); r.err != nil {
			return
		}
	}
	draw.DrawMask(r.img, r.dirty, image.NewUniform(r.cur), image.Point{}, r.mask, r.dirty.Min, draw.Over)
	draw.Draw(r.mask, r.dirty, image.Transparent, image.Point{}, draw.Src)
	r.dirty = image.Rectangle{}
}

// pt is a device point. Points stay float64, which holds multiples of
// 1/64 exactly far beyond any image size; only the offsets from the
// filled box, which are small, go to the float32 rasterizer. (float32
// holds multiples of 1/64 exactly only up to 2^18.)
type pt struct{ x, y float64 }

// fill rasterizes closed polygons/paths given as a callback within box.
func (r *renderer) fill(minX, minY, maxX, maxY float64, c color.NRGBA, path func(dx, dy float64)) {
	box := image.Rect(int(math.Floor(minX)), int(math.Floor(minY)),
		int(math.Ceil(maxX)), int(math.Ceil(maxY))).Intersect(r.img.Bounds())
	if box.Empty() || c.A == 0 || r.err != nil {
		return
	}
	if r.work += box.Dx() * box.Dy(); r.work >= ctxCheckPixels {
		r.work = 0
		if r.err = r.g.Err(); r.err != nil {
			return
		}
	}
	if c != r.cur {
		if r.flush(); r.err != nil {
			return
		}
		r.cur = c
	}
	if r.mask == nil {
		r.mask = image.NewAlpha(r.img.Bounds())
	}
	// vector switches to float32 accumulation above maxTile pixels in
	// either direction, and its assembly and pure Go versions round that
	// differently (amd64 and arm64 would give different pixels). Large
	// boxes are drawn in tiles, each with the whole path: the rasterizer
	// skips the rows outside a tile and carries the coverage of lines left
	// of it into its first column.
	for ty := box.Min.Y; ty < box.Max.Y; ty += maxTile {
		for tx := box.Min.X; tx < box.Max.X; tx += maxTile {
			tile := image.Rect(tx, ty, min(tx+maxTile, box.Max.X), min(ty+maxTile, box.Max.Y))
			r.ras.Reset(tile.Dx(), tile.Dy())
			r.ras.DrawOp = draw.Over
			path(float64(tile.Min.X), float64(tile.Min.Y))
			r.ras.Draw(r.mask, tile, image.Opaque, tile.Min)
		}
	}
	r.dirty = r.dirty.Union(box)
}

// maxTile is the largest side that vector.Rasterizer draws with integer
// (fixed point) accumulation, which is the same on every architecture.
const maxTile = 512

// outline returns the outline of gl as closed polygons in device pixels,
// all points multiples of 1/64, and the bounds of its points.
func (r *renderer) outline(gl *scene.Glyph) (polys [][]pt, minX, minY, maxX, maxY float64, err error) {
	segs, err := r.f.Outline(gl.Glyph)
	if err != nil {
		return nil, 0, 0, 0, 0, fmt.Errorf("glyph %d: %w", gl.Glyph, err)
	}
	x0, y0 := gl.X+r.ox, gl.Y+r.oy
	sx := 1.0
	if gl.Mirror {
		sx = -1
	}
	// conv maps a font-unit point to device 26.6 fixed point. The explicit
	// float64 conversion of the product keeps the compiler from fusing it
	// with the addition (arm64, GOAMD64=v3), which could move a point
	// across a 1/64 rounding boundary on some machines only.
	conv := func(p fixed.Point26_6) fixed.Point26_6 {
		return fixed.Point26_6{
			X: fixed.Int26_6(math.Round((x0 + float64(float64(p.X)/64*gl.Scale*sx)) * 64)),
			Y: fixed.Int26_6(math.Round((y0 + float64(float64(p.Y)/64*gl.Scale)) * 64)),
		}
	}
	minX, minY = math.MaxFloat64, math.MaxFloat64
	maxX, maxY = -minX, -minY
	for _, s := range segs {
		for i := 0; i < nArgs(s.Op); i++ {
			p := conv(s.Args[i])
			x, y := float64(p.X)/64, float64(p.Y)/64
			minX, minY, maxX, maxY = min(minX, x), min(minY, y), max(maxX, x), max(maxY, y)
		}
	}
	// Curves are flattened here with integer arithmetic (internal/curve)
	// and the rasterizer only gets straight lines: its own float32
	// flattening gives different pixels when multiply-adds are fused.
	var cur []pt
	lineTo := func(p fixed.Point26_6) {
		if cur == nil {
			// A path without a MoveTo (a broken font) starts at its
			// first point.
			polys = append(polys, nil)
		}
		cur = append(cur, pt{float64(p.X) / 64, float64(p.Y) / 64})
		polys[len(polys)-1] = cur
	}
	var pen fixed.Point26_6
	for _, s := range segs {
		a := conv(s.Args[0])
		switch s.Op {
		case sfnt.SegmentOpMoveTo:
			cur = nil
			lineTo(a)
			pen = a
		case sfnt.SegmentOpLineTo:
			lineTo(a)
			pen = a
		case sfnt.SegmentOpQuadTo:
			b := conv(s.Args[1])
			curve.Quad(pen, a, b, lineTo)
			pen = b
		case sfnt.SegmentOpCubeTo:
			b, c := conv(s.Args[1]), conv(s.Args[2])
			curve.Cube(pen, a, b, c, lineTo)
			pen = c
		}
	}
	return polys, minX, minY, maxX, maxY, nil
}

// glyphs draws glyphs of one color in one pass of the rasterizer: the
// parts of a glyph assembly, each cut to its band, or a single glyph.
// Coverage is added up before compositing, so parts whose bands touch
// join as one shape.
func (r *renderer) glyphs(gls []*scene.Glyph) error {
	var all [][]pt
	minX, minY := math.MaxFloat64, math.MaxFloat64
	maxX, maxY := -minX, -minY
	for _, gl := range gls {
		polys, x0, y0, x1, y1, err := r.outline(gl)
		if err != nil {
			return err
		}
		if len(polys) == 0 {
			continue
		}
		if b := gl.Clip; b != nil {
			off := r.ox
			if b.Vert {
				off = r.oy
			}
			lo, hi := b.Lo+off, b.Hi+off
			for i, p := range polys {
				polys[i] = clipBand(p, b.Vert, lo, hi)
			}
		}
		all = append(all, polys...)
		minX, minY, maxX, maxY = min(minX, x0), min(minY, y0), max(maxX, x1), max(maxY, y1)
	}
	if len(all) == 0 {
		return nil
	}
	r.fill(minX, minY, maxX, maxY, gls[0].Color, func(dx, dy float64) {
		// The offsets from the box are computed in float64, where they
		// are exact, and are small multiples of 1/64, exact in float32
		// too.
		for _, poly := range all {
			if len(poly) == 0 {
				continue
			}
			r.ras.MoveTo(float32(poly[0].x-dx), float32(poly[0].y-dy))
			for _, p := range poly[1:] {
				r.ras.LineTo(float32(p.x-dx), float32(p.y-dy))
			}
			r.ras.ClosePath()
		}
	})
	return nil
}

// clipBand cuts the closed polygon poly to lo <= y <= hi when vert, else
// to lo <= x <= hi (Sutherland–Hodgman). Infinite limits do not cut.
func clipBand(poly []pt, vert bool, lo, hi float64) []pt {
	if !math.IsInf(lo, 0) {
		poly = clipHalf(poly, vert, lo, true)
	}
	if !math.IsInf(hi, 0) {
		poly = clipHalf(poly, vert, hi, false)
	}
	return poly
}

// clipHalf keeps the part of poly on one side of the line at c: where the
// coordinate is at least c when above, else at most c. New points are
// rounded to 1/64 like all others.
func clipHalf(poly []pt, vert bool, c float64, above bool) []pt {
	coord := func(p pt) (v, u float64) {
		if vert {
			return p.y, p.x
		}
		return p.x, p.y
	}
	inside := func(p pt) bool {
		v, _ := coord(p)
		if above {
			return v >= c
		}
		return v <= c
	}
	cross := func(p, q pt) pt {
		pv, pu := coord(p)
		qv, qu := coord(q)
		// The product is converted explicitly so that it is not fused
		// with the addition (see conv in outline).
		u := snap(pu + float64((c-pv)/(qv-pv)*(qu-pu)))
		if vert {
			return pt{u, c}
		}
		return pt{c, u}
	}
	var out []pt
	for i, cur := range poly {
		prev := poly[(i+len(poly)-1)%len(poly)]
		switch in, prevIn := inside(cur), inside(prev); {
		case in && !prevIn:
			out = append(out, cross(prev, cur), cur)
		case in:
			out = append(out, cur)
		case prevIn:
			out = append(out, cross(prev, cur))
		}
	}
	return out
}

func nArgs(op sfnt.SegmentOp) int {
	switch op {
	case sfnt.SegmentOpQuadTo:
		return 2
	case sfnt.SegmentOpCubeTo:
		return 3
	}
	return 1
}

// rect fills a rule. Thin rules are snapped to whole pixels so that
// fraction bars stay crisp; they are at least one pixel thick.
func (r *renderer) rect(rc *scene.Rect) {
	x0, y0, x1, y1 := rc.X0+r.ox, rc.Y0+r.oy, rc.X1+r.ox, rc.Y1+r.oy
	if x1-x0 >= y1-y0 {
		y0, y1 = snapRule(y0, y1)
	} else {
		x0, x1 = snapRule(x0, x1)
	}
	poly := []pt{{snap(x0), snap(y0)}, {snap(x1), snap(y0)}, {snap(x1), snap(y1)}, {snap(x0), snap(y1)}}
	r.polygon(poly, rc.Color)
}

func snapRule(a, b float64) (float64, float64) {
	t := math.Max(1, math.Round(b-a))
	c := (a + b) / 2
	a = math.Round(c - t/2)
	return a, a + t
}

func (r *renderer) polygon(poly []pt, c color.NRGBA) {
	minX, minY, maxX, maxY := poly[0].x, poly[0].y, poly[0].x, poly[0].y
	for _, p := range poly {
		minX, minY, maxX, maxY = min(minX, p.x), min(minY, p.y), max(maxX, p.x), max(maxY, p.y)
	}
	r.fill(minX, minY, maxX, maxY, c, func(dx, dy float64) {
		r.ras.MoveTo(float32(poly[0].x-dx), float32(poly[0].y-dy))
		for _, p := range poly[1:] {
			r.ras.LineTo(float32(p.x-dx), float32(p.y-dy))
		}
		r.ras.ClosePath()
	})
}

func (r *renderer) line(l *scene.Line) {
	x0, y0, x1, y1 := l.X0+r.ox, l.Y0+r.oy, l.X1+r.ox, l.Y1+r.oy
	dx, dy := x1-x0, y1-y0
	// Not math.Hypot: its pure Go version (arm64) fuses a multiply-add,
	// while amd64 uses assembly, so the result differs by an ulp between
	// machines, and GOAMD64=v3 does not show it. Sqrt is correctly rounded
	// everywhere; coordinates stay far below overflow.
	n := math.Sqrt(float64(dx*dx) + float64(dy*dy))
	if n == 0 {
		return
	}
	w := math.Max(l.Width, 0.8) / 2
	// The explicit float64 conversions keep the compiler from fusing these
	// products with the additions below (arm64, GOAMD64=v3), as in conv.
	nx, ny := float64(-dy/n*w), float64(dx/n*w)
	r.polygon([]pt{
		{snap(x0 + nx), snap(y0 + ny)}, {snap(x1 + nx), snap(y1 + ny)},
		{snap(x1 - nx), snap(y1 - ny)}, {snap(x0 - nx), snap(y0 - ny)},
	}, l.Color)
}

func (r *renderer) text(t *scene.Text, faces FaceFunc) error {
	if faces == nil {
		return nil
	}
	if r.work += utf8.RuneCountInString(t.Text) * int(t.SizePx+2) * int(t.SizePx+2); r.work >= ctxCheckPixels {
		r.work = 0
		if err := r.g.Err(); err != nil {
			return err
		}
	}
	face, err := faces(t.SizePx)
	if err != nil {
		return err
	}
	d := &font.Drawer{
		Dst:  r.img,
		Src:  image.NewUniform(t.Color),
		Face: face,
		Dot: fixed.Point26_6{
			X: fixed.Int26_6(math.Round((t.X + r.ox) * 64)),
			Y: fixed.Int26_6(math.Round((t.Y + r.oy) * 64)),
		},
	}
	d.DrawString(t.Text)
	return nil
}
