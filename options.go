package dal

import (
	"image/color"

	"golang.org/x/image/font"
)

// Default limits, used when a Limits field is zero or negative.
const (
	DefaultMaxInputBytes = 16 << 10
	DefaultMaxNodes      = 5000
	DefaultMaxDepth      = 64
	DefaultMaxExpansions = 1000
	// DefaultMaxPixels bounds the image area (width × height) of Render.
	// Drawing needs about 9 bytes per pixel at its peak, so the default
	// allows about 150 MB per call.
	DefaultMaxPixels = 16 << 20
)

// DefaultSizePx is the text size used when Options.SizePx is zero.
const DefaultSizePx = 20

// minSizePx, maxSizePx and maxPaddingPx bound the image options.
const (
	minSizePx    = 1
	maxSizePx    = 1024
	maxPaddingPx = 1024
)

// Limits bounds the work done for one call. Zero fields use the defaults.
type Limits struct {
	MaxInputBytes int // input length in bytes
	MaxNodes      int // syntax tree nodes
	MaxDepth      int // parser recursion depth; one level of braces or arguments uses one or two
	MaxExpansions int // macro expansions (\newcommand, \def)
	// MaxPixels bounds the image area (width × height, padding included)
	// of Render; larger images give ErrTooLarge. Peak memory of one call is
	// about 9 bytes per pixel of the image, so this also bounds memory:
	// lower it to bound memory, for example with many calls in parallel.
	// Other functions ignore it.
	MaxPixels int
}

func (l Limits) withDefaults() Limits {
	if l.MaxInputBytes <= 0 {
		l.MaxInputBytes = DefaultMaxInputBytes
	}
	if l.MaxNodes <= 0 {
		l.MaxNodes = DefaultMaxNodes
	}
	if l.MaxDepth <= 0 {
		l.MaxDepth = DefaultMaxDepth
	}
	if l.MaxExpansions <= 0 {
		l.MaxExpansions = DefaultMaxExpansions
	}
	if l.MaxPixels <= 0 {
		l.MaxPixels = DefaultMaxPixels
	}
	return l
}

// ParseOptions configures Parse.
type ParseOptions struct {
	Limits Limits
}

// Options configures Render. The zero value renders inline math at 20 px
// in black on a transparent background.
type Options struct {
	// Display selects display style ($$...$$): big operators get larger and
	// take their limits above and below, fractions use the full size.
	Display bool
	// SizePx is the size of the surrounding text in pixels (one em). Zero
	// means DefaultSizePx; negative, NaN and infinite values and sizes
	// below 1 are errors, and sizes above 1024 return ErrTooLarge.
	SizePx float64
	// Foreground is the default ink color; nil means black.
	Foreground color.Color
	// Background fills the image; nil means transparent.
	Background color.Color
	// PaddingPx is added on every side of the image. It must not be
	// negative; more than 1024 returns ErrTooLarge.
	PaddingPx int
	// MaxWidth is the maximum image width in pixels; 0 means unlimited and
	// a negative value is an error.
	// Wider formulas fail with ErrTooLarge instead of being scaled down,
	// unless Wrap breaks them into lines. With Display, equation tags are
	// put at the right end of MaxWidth.
	MaxWidth int
	// TextFace returns the face for characters the math font lacks: runs
	// in \text{...} (e.g. Japanese), and single characters written in
	// math mode (定, ½, \digamma), which it draws upright and does not
	// stretch. Only such characters are drawn with it; letters, digits
	// and other characters the math font has keep the math font, so
	// \text{otherwise} looks the same with or without a TextFace. It is called only when such a run occurs, at
	// most once per size and call, and must return a face that is not
	// used concurrently elsewhere. Nil means characters missing from the
	// math font give ErrUnsupported.
	TextFace func(sizePx float64) (font.Face, error)
	// Wrap breaks an inline formula (Display false) that is wider than
	// MaxWidth into lines, where TeX would break it: after relations and
	// binary operators, and at \allowbreak. The lines are stacked;
	// Baseline is the first line's. A piece between two break points that
	// is wider than MaxWidth on its own still gives ErrTooLarge. Display
	// formulas are never broken, as in KaTeX.
	Wrap bool
	// Font is an OpenType font with a MATH table to use instead of the
	// embedded Latin Modern Math.
	Font []byte
	// Limits bounds the work done.
	Limits Limits
}

// TextOptions configures RenderText.
type TextOptions struct {
	// Display selects display style: limits above and below big operators.
	Display bool
	// MaxCols is the maximum width in terminal columns (East Asian wide
	// characters count as two); 0 means unlimited. Wider output fails with
	// ErrTooLarge.
	//
	// Characters of East Asian Ambiguous width (box drawing ─ │, √, ∑, →,
	// Greek letters) count as one column. A terminal set to show them wide
	// misaligns the output.
	//
	// With Display, equation tags are put at the right end of MaxCols.
	MaxCols int
	// Wrap breaks an inline formula wider than MaxCols into lines, as
	// Options.Wrap does for images; Baseline is the first line's.
	Wrap bool
	// Limits bounds the work done.
	Limits Limits
}

// LineStyle selects how RenderLine writes scripts.
type LineStyle int

const (
	// LineUnicode uses Unicode superscripts and subscripts when every
	// character of a script has one (x², aᵢ), else x^(…) and a_(…).
	LineUnicode LineStyle = iota
	// LineASCII always writes x^2, a_i and x^(a+b).
	LineASCII
)

// LineOptions configures RenderLine.
type LineOptions struct {
	Style  LineStyle
	Limits Limits
}
