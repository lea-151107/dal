package box

import (
	"image/color"
	"math"

	"github.com/lea-151107/dal/internal/mathfont"
	"github.com/lea-151107/dal/internal/scene"
)

// ToScene converts b to drawing commands in pixels. fg is the default
// color.
func ToScene(b *Box, f *mathfont.Font, sizePx float64, fg color.NRGBA) *scene.Scene {
	k := sizePx / f.UnitsPerEm()
	sc := &scene.Scene{Width: float64(b.W * k), Height: float64(b.H * k), Depth: float64(b.D * k)}
	w := &sceneWriter{sc: sc, f: f, k: k, sizePx: sizePx}
	w.walk(b, 0, 0, fg, xform{sx: 1})
	return sc
}

// xform maps an x coordinate of the box tree to the scene: sx*x + tx. sx
// is -1 inside an odd number of mirrored boxes.
type xform struct{ sx, tx float64 }

func (t xform) x(x float64) float64 { return float64(t.sx*x) + t.tx }

type sceneWriter struct {
	sc     *scene.Scene
	f      *mathfont.Font
	k      float64
	sizePx float64
	// group is the scene.Glyph group of the assembly being walked, 0
	// outside assemblies; groups counts the assemblies so far.
	group, groups int
}

// snap rounds to 1/64 px, as the rasterizer positions points.
func snap(v float64) float64 { return math.Round(v*64) / 64 }

// band converts the band of a glyph whose origin is at (x, y) to pixels.
func (w *sceneWriter) band(cb *Band, x, y float64, t xform) *scene.Band {
	k := w.k
	if cb.Horiz {
		a, b := snap(float64(t.x(x+cb.Lo)*k)), snap(float64(t.x(x+cb.Hi)*k))
		return &scene.Band{Lo: min(a, b), Hi: max(a, b)}
	}
	// y grows downwards in the scene.
	return &scene.Band{Vert: true, Lo: snap(float64(-(y + cb.Hi) * k)), Hi: snap(float64(-(y + cb.Lo) * k))}
}

func (w *sceneWriter) walk(b *Box, x, y float64, c color.NRGBA, t xform) {
	if b.Color != nil {
		c = *b.Color
	}
	if b.Mirror {
		// Reflect about the box's middle, x+W/2, then apply t.
		t = xform{sx: -t.sx, tx: t.x(float64(2*x) + b.W)}
	}
	k := w.k
	// span maps the box's horizontal extent [x0, x1] to pixels.
	span := func(x0, x1 float64) (float64, float64) {
		a, b := float64(t.x(x0)*k), float64(t.x(x1)*k)
		return min(a, b), max(a, b)
	}
	switch lf := b.Leaf.(type) {
	case GlyphLeaf:
		px, py, s := float64(t.x(x)*k), float64(-y*k), float64(lf.Scale*k)
		gl := &scene.Glyph{X: px, Y: py, Scale: s, Glyph: lf.Glyph, Color: c, Mirror: t.sx < 0}
		if lf.Clip != nil && w.group != 0 {
			gl.Clip, gl.Group = w.band(lf.Clip, x, y, t), w.group
		}
		w.sc.Items = append(w.sc.Items, gl)
		m := w.f.Metrics(lf.Glyph)
		if m.MaxX > m.MinX {
			x0, x1 := span(x+float64(m.MinX*lf.Scale), x+float64(m.MaxX*lf.Scale))
			w.sc.AddInk(x0, py-float64(m.Height*s), x1, py+float64(m.Depth*s))
		}
	case RuleLeaf:
		x0, x1 := span(x, x+b.W)
		y0, y1 := float64(-(y+b.H)*k), float64(-(y-b.D)*k)
		w.sc.Items = append(w.sc.Items, &scene.Rect{X0: x0, Y0: y0, X1: x1, Y1: y1, Color: c})
		w.sc.AddInk(x0, y0, x1, y1)
	case TextLeaf:
		// NOTE(katex-compat): text drawn with the caller's face is moved
		// but not mirrored inside \reflectbox.
		x0, x1 := span(x, x+b.W)
		w.sc.Items = append(w.sc.Items, &scene.Text{X: x0, Y: float64(-y * k), SizePx: float64(lf.SizeEm * w.sizePx), Text: lf.Text, Color: c})
		w.sc.AddInk(x0, float64(-(y+b.H)*k), x1, float64(-(y-b.D)*k))
	case LineLeaf:
		l := &scene.Line{X0: float64(t.x(x+lf.X0) * k), Y0: float64(-(y + lf.Y0) * k), X1: float64(t.x(x+lf.X1) * k), Y1: float64(-(y + lf.Y1) * k),
			Width: float64(lf.Width * k), Color: c}
		w.sc.Items = append(w.sc.Items, l)
		hw := l.Width / 2
		w.sc.AddInk(min(l.X0, l.X1)-hw, min(l.Y0, l.Y1)-hw, max(l.X0, l.X1)+hw, max(l.Y0, l.Y1)+hw)
	}
	if b.Assembly {
		w.groups++
		outer := w.group
		w.group = w.groups
		defer func() { w.group = outer }()
	}
	for _, kid := range b.Kids {
		w.walk(kid.B, x+kid.X, y+kid.Y, c, t)
	}
}
