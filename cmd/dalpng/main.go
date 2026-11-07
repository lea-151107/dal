// Command dalpng renders a LaTeX formula read from standard input.
//
//	echo '\frac{a}{b}' | dalpng --display > out.png
//	echo '\frac{a}{b}' | dalpng --text
//	echo '\frac{a}{b}' | dalpng --line
//
// Options are written with two dashes (--text). The flag package also
// accepts one dash, so -text keeps working.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"image/color"
	"image/png"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"

	"github.com/lea/dal"
)

func main() {
	display := flag.Bool("display", false, "display style ($$...$$)")
	size := flag.Float64("size", 32, "text size in pixels")
	fg := flag.String("fg", "000000", "foreground color (RRGGBB)")
	bg := flag.String("bg", "", "background color (RRGGBB); empty for transparent")
	pad := flag.Int("pad", 4, "padding in pixels")
	width := flag.Int("width", 0, "maximum image width in pixels (0: unlimited)")
	mathFont := flag.String("font", "", "OpenType math font to use instead of Latin Modern Math")
	textFont := flag.String("textfont", "", "font for \\text{...} (e.g. a Japanese font)")
	text := flag.Bool("text", false, "write two-dimensional text instead of PNG")
	cols := flag.Int("cols", 0, "maximum columns for --text (0: unlimited)")
	wrap := flag.Bool("wrap", false, "break an inline formula wider than --width or --cols into lines")
	line := flag.Bool("line", false, "write a single line of text instead of PNG")
	ascii := flag.Bool("ascii", false, "with --line, write x^2 instead of x²")
	out := flag.String("output", "", "output file (default: standard output)")
	flag.StringVar(out, "o", "", "short for --output")
	timeout := flag.Duration("timeout", 10*time.Second, "time limit")
	flag.Usage = usage
	flag.Parse()

	src, err := io.ReadAll(os.Stdin)
	if err != nil {
		fail(err)
	}
	input := strings.TrimSpace(string(src))
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	w := os.Stdout
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			fail(err)
		}
		defer f.Close()
		w = f
	}

	switch {
	case *line:
		style := dal.LineUnicode
		if *ascii {
			style = dal.LineASCII
		}
		s, err := dal.RenderLine(input, dal.LineOptions{Style: style})
		if err != nil {
			fail(err)
		}
		fmt.Fprintln(w, s)
	case *text:
		res, err := dal.RenderText(ctx, input, dal.TextOptions{Display: *display, MaxCols: *cols, Wrap: *wrap})
		if err != nil {
			fail(err)
		}
		for _, l := range res.Lines {
			fmt.Fprintln(w, l)
		}
	default:
		opt := dal.Options{Display: *display, SizePx: *size, PaddingPx: *pad, MaxWidth: *width, Wrap: *wrap}
		if opt.Foreground, err = parseColor(*fg); err != nil {
			fail(err)
		}
		if *bg != "" {
			if opt.Background, err = parseColor(*bg); err != nil {
				fail(err)
			}
		}
		if *mathFont != "" {
			if opt.Font, err = os.ReadFile(*mathFont); err != nil {
				fail(err)
			}
		}
		if *textFont != "" {
			if opt.TextFace, err = loadFace(*textFont); err != nil {
				fail(err)
			}
		}
		res, err := dal.Render(ctx, input, opt)
		if err != nil {
			fail(err)
		}
		if err := png.Encode(w, res.Image); err != nil {
			fail(err)
		}
		fmt.Fprintf(os.Stderr, "%dx%d baseline=%d origin=%d\n", res.Image.Bounds().Dx(), res.Image.Bounds().Dy(), res.Baseline, res.OriginX)
	}
}

// usage is flag.PrintDefaults with options written as --name.
func usage() {
	w := flag.CommandLine.Output()
	fmt.Fprintf(w, "usage: dalpng [options] < formula.tex\n\noptions:\n")
	flag.VisitAll(func(f *flag.Flag) {
		if f.Name == "o" {
			return // listed with --output
		}
		name, help := flag.UnquoteUsage(f)
		head := "  --" + f.Name
		if f.Name == "output" {
			head = "  -o, --output"
		}
		if name != "" {
			head += " " + name
		}
		fmt.Fprintf(w, "%s\n    \t%s", head, help)
		if f.DefValue != "" && f.DefValue != "false" && f.DefValue != "0" {
			fmt.Fprintf(w, " (default %v)", f.DefValue)
		}
		fmt.Fprintln(w)
	})
}

func parseColor(s string) (color.Color, error) {
	s = strings.TrimPrefix(s, "#")
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil || len(s) != 6 {
		return nil, fmt.Errorf("invalid color %q", s)
	}
	return color.NRGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}, nil
}

func loadFace(path string) (func(float64) (font.Face, error), error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f, err := opentype.Parse(data)
	if err != nil {
		c, cerr := opentype.ParseCollection(data)
		if cerr != nil {
			return nil, err
		}
		if f, err = c.Font(0); err != nil {
			return nil, err
		}
	}
	return func(size float64) (font.Face, error) {
		return opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingNone})
	}, nil
}

func fail(err error) {
	var se *dal.SyntaxError
	var ue *dal.UnsupportedError
	switch {
	case errors.As(err, &se):
		fmt.Fprintln(os.Stderr, "syntax error:", err)
	case errors.As(err, &ue):
		fmt.Fprintln(os.Stderr, "unsupported:", err)
	case errors.Is(err, dal.ErrTooLarge):
		fmt.Fprintln(os.Stderr, "too large:", err)
	default:
		fmt.Fprintln(os.Stderr, "error:", err)
	}
	os.Exit(1)
}
