// Package dal typesets LaTeX math in pure Go. It renders formulas to
// images with an embedded OpenType math font (Latin Modern Math), to
// two-dimensional text for terminals, and to a single line of text.
//
// The typesetter follows TeX's rules (TeXbook appendix G) with parameters
// from the font's MATH table. The supported syntax follows KaTeX.
package dal

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"math"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"

	"github.com/lea-151107/dal/internal/box"
	"github.com/lea-151107/dal/internal/guard"
	"github.com/lea-151107/dal/internal/linear"
	"github.com/lea-151107/dal/internal/mathfont"
	"github.com/lea-151107/dal/internal/parse"
	"github.com/lea-151107/dal/internal/raster"
	"github.com/lea-151107/dal/internal/textgrid"
)

// Result is a rendered formula.
type Result struct {
	// Image is the formula on a transparent (or Background) canvas.
	Image *image.RGBA
	// Baseline is the y of the baseline, in pixels from the top of Image.
	// Align it with the baseline of the surrounding text.
	Baseline int
	// OriginX is the x of the formula's origin (the left end of its
	// baseline), in pixels from the left of Image. It is PaddingPx unless
	// ink sticks out to the left of the box, as with a negative \kern.
	OriginX int
	// Width, Height and Depth are the typographic dimensions in pixels:
	// the advance width, and the extent above and below the baseline.
	// Image is this box plus PaddingPx on each side, grown where ink
	// sticks out of the box (italic overhang).
	//
	// As in TeX they can be negative: a negative \kern or \hspace at the
	// end gives a negative Width (the next text would overlap the formula),
	// and \raisebox or \smash can make Height or Depth negative. The image
	// still covers all the ink, so use Image.Bounds() for its size.
	Width, Height, Depth float64
}

// TextResult is a formula set as text lines.
type TextResult struct {
	// Lines contains no control characters; every line is at most MaxCols
	// columns wide. Trailing spaces are removed, so lines differ in length;
	// use Width for the width of the block.
	Lines []string
	// Width is the width of the widest line in terminal columns (East
	// Asian wide characters count as two).
	Width int
	// Baseline is the index of the line that holds the formula's
	// baseline.
	Baseline int
}

func parseLimits(l Limits) parse.Limits {
	l = l.withDefaults()
	return parse.Limits{MaxInputBytes: l.MaxInputBytes, MaxNodes: l.MaxNodes, MaxDepth: l.MaxDepth,
		MaxExpansions: l.MaxExpansions}
}

// Parse parses src into a syntax tree without rendering it.
func Parse(src string, opt ParseOptions) (e *Expr, err error) {
	defer recoverErr(&err)
	e, err = parse.Parse(src, parseLimits(opt.Limits))
	if err != nil {
		return nil, convertErr(src, err)
	}
	return e, nil
}

func recoverErr(err *error) {
	if r := recover(); r != nil {
		*err = fmt.Errorf("dal: internal error: %v", r)
	}
}

// Render typesets src as an image.
func Render(ctx context.Context, src string, opt Options) (res *Result, err error) {
	defer recoverErr(&err)
	res, err = render(ctx, src, opt)
	if err != nil {
		return nil, convertErr(src, err)
	}
	return res, nil
}

func render(ctx context.Context, src string, opt Options) (*Result, error) {
	lim := parseLimits(opt.Limits)
	g := guard.New(ctx, lim.MaxNodes)
	if err := g.Err(); err != nil {
		return nil, err
	}
	size := opt.SizePx
	switch {
	case math.IsNaN(size) || math.IsInf(size, 0) || size < 0:
		return nil, fmt.Errorf("invalid SizePx %v", size)
	case size == 0:
		size = DefaultSizePx
	case size < minSizePx:
		// Tiny sizes (down to subnormal numbers) make every dimension
		// round to nothing or overflow when divided by.
		return nil, fmt.Errorf("invalid SizePx %v (below %d)", size, minSizePx)
	case size > maxSizePx:
		return nil, fmt.Errorf("%w: size %.0f px exceeds limit %d", guard.ErrTooLarge, size, maxSizePx)
	}
	if opt.PaddingPx < 0 || opt.MaxWidth < 0 {
		return nil, fmt.Errorf("invalid PaddingPx %d or MaxWidth %d (must not be negative)", opt.PaddingPx, opt.MaxWidth)
	}
	if opt.PaddingPx > maxPaddingPx {
		return nil, fmt.Errorf("%w: padding %d px exceeds limit %d", guard.ErrTooLarge, opt.PaddingPx, maxPaddingPx)
	}
	pad := opt.PaddingPx
	e, err := parse.Parse(src, lim)
	if err != nil {
		return nil, err
	}
	f, err := mathfont.Load(opt.Font)
	if err != nil {
		return nil, err
	}
	faces := &faceCache{fn: opt.TextFace, faces: map[float64]font.Face{}}
	env := &box.Env{F: f, G: g, SizePx: size, Wrap: opt.Wrap}
	if opt.MaxWidth > 0 {
		// Two pixels of slack keep the tag's ink and the rounding of the
		// image size within MaxWidth.
		env.LineWidth = float64(opt.MaxWidth-2*pad-2) / size * f.UnitsPerEm()
	}
	if opt.TextFace != nil {
		env.Measure = faces.measure
		env.Has = faces.has
	}
	b, err := box.Layout(env, e, opt.Display)
	if err != nil {
		return nil, err
	}
	fg := color.NRGBA{A: 255}
	if opt.Foreground != nil {
		fg = color.NRGBAModel.Convert(opt.Foreground).(color.NRGBA)
	}
	sc := box.ToScene(b, f, size, fg)
	lay := raster.Plan(sc, pad)
	if opt.MaxWidth > 0 && lay.W > opt.MaxWidth {
		return nil, fmt.Errorf("%w: image width %d px exceeds MaxWidth %d", guard.ErrTooLarge, lay.W, opt.MaxWidth)
	}
	if err := g.Err(); err != nil {
		return nil, err
	}
	img, err := raster.Render(g, sc, lay, f, faces.face, opt.Background, opt.Limits.withDefaults().MaxPixels)
	if err != nil {
		return nil, err
	}
	return &Result{Image: img, Baseline: lay.Baseline, OriginX: int(lay.OriginX), Width: sc.Width, Height: sc.Height, Depth: sc.Depth}, nil
}

// faceCache creates at most one text face per size for one call.
type faceCache struct {
	fn    func(sizePx float64) (font.Face, error)
	faces map[float64]font.Face
}

func (c *faceCache) face(sizePx float64) (font.Face, error) {
	if c.fn == nil {
		return nil, nil
	}
	if f, ok := c.faces[sizePx]; ok {
		return f, nil
	}
	f, err := c.fn(sizePx)
	if err != nil {
		return nil, fmt.Errorf("TextFace: %w", err)
	}
	if f == nil {
		return nil, fmt.Errorf("TextFace returned nil")
	}
	c.faces[sizePx] = f
	return f, nil
}

func (c *faceCache) measure(s string, sizePx float64) (adv, asc, desc float64, err error) {
	f, err := c.face(sizePx)
	if err != nil {
		return 0, 0, 0, err
	}
	m := f.Metrics()
	return toPx(font.MeasureString(f, s)), toPx(m.Ascent), toPx(m.Descent), nil
}

func (c *faceCache) has(r rune, sizePx float64) (bool, error) {
	f, err := c.face(sizePx)
	if err != nil {
		return false, err
	}
	_, ok := f.GlyphAdvance(r)
	return ok, nil
}

func toPx(v fixed.Int26_6) float64 { return float64(v) / 64 }

// RenderText typesets src on a character grid, with fractions, limits and
// matrices laid out over several lines. Constructs that cannot be drawn
// with characters give ErrUnsupported, so that callers can fall back to
// RenderLine.
func RenderText(ctx context.Context, src string, opt TextOptions) (res *TextResult, err error) {
	defer recoverErr(&err)
	res, err = renderText(ctx, src, opt)
	if err != nil {
		return nil, convertErr(src, err)
	}
	return res, nil
}

func renderText(ctx context.Context, src string, opt TextOptions) (*TextResult, error) {
	lim := parseLimits(opt.Limits)
	g := guard.New(ctx, lim.MaxNodes)
	if err := g.Err(); err != nil {
		return nil, err
	}
	e, err := parse.Parse(src, lim)
	if err != nil {
		return nil, err
	}
	lines, base, err := textgrid.Render(g, e, opt.Display, opt.MaxCols, opt.Wrap)
	if err != nil {
		return nil, err
	}
	w := textgrid.Width(lines)
	if opt.MaxCols > 0 && w > opt.MaxCols {
		return nil, fmt.Errorf("%w: %d columns exceed MaxCols %d", guard.ErrTooLarge, w, opt.MaxCols)
	}
	if err := g.Err(); err != nil {
		return nil, err
	}
	return &TextResult{Lines: lines, Baseline: base, Width: w}, nil
}

// RenderLine writes src as a single line of text (x², aᵢ, (a+b)/c,
// √(x+1)). Constructs without a compact form are linearized, so it fails
// only for invalid input and unknown commands.
func RenderLine(src string, opt LineOptions) (s string, err error) {
	defer recoverErr(&err)
	e, err := parse.Parse(src, parseLimits(opt.Limits))
	if err != nil {
		return "", convertErr(src, err)
	}
	return linear.Write(e, linear.Options{ASCII: opt.Style == LineASCII}), nil
}
