package dal

import (
	"bytes"
	"context"
	"image/color"
	"image/png"
	"path/filepath"
	"strings"
	"testing"
)

// optionCases are golden cases for options the plain golden fixtures do
// not use (widths, wrapping, tags at the line's end). Their files live in
// testdata/golden, so goldenversion_test.go covers them: a change of their
// output needs a new Version like any other.
var optionCases = []struct {
	name, src string
	opt       Options
	text      TextOptions
}{
	{"opt_wrap", `a + b + c + d = e + f + \frac{g}{h} \le i + j + k + l = m \allowbreak n`,
		Options{MaxWidth: 220, Wrap: true}, TextOptions{MaxCols: 20, Wrap: true}},
	{"opt_wrap_tag", `a+b+c+d+e+f+g+h+i+j+k+l+m+n+o+p\tag{1}`,
		Options{MaxWidth: 240, Wrap: true}, TextOptions{MaxCols: 30, Wrap: true}},
	{"opt_tag_right", `E = mc^2 \tag{1}`,
		Options{Display: true, MaxWidth: 400}, TextOptions{Display: true, MaxCols: 40}},
	{"opt_align_tags", `\begin{align} a &= b + c \tag{1} \\ c &= d \tag{10} \end{align}`,
		Options{Display: true, MaxWidth: 400}, TextOptions{Display: true, MaxCols: 40}},
	{"opt_both_tags", `\begin{align}a&=b\tag{1}\\c&=d\tag{2}\end{align}\tag{3}`,
		Options{Display: true, MaxWidth: 400}, TextOptions{Display: true, MaxCols: 40}},
}

func TestGoldenOptions(t *testing.T) {
	for _, c := range optionCases {
		t.Run(c.name, func(t *testing.T) {
			opt := c.opt
			opt.SizePx, opt.PaddingPx, opt.Background = 24, 4, color.White
			res, err := Render(context.Background(), c.src, opt)
			if err != nil {
				t.Fatal(err)
			}
			if opt.MaxWidth > 0 && res.Image.Bounds().Dx() > opt.MaxWidth {
				t.Errorf("image width %d > MaxWidth %d", res.Image.Bounds().Dx(), opt.MaxWidth)
			}
			var buf bytes.Buffer
			if err := png.Encode(&buf, res.Image); err != nil {
				t.Fatal(err)
			}
			checkGolden(t, filepath.Join("testdata", "golden", c.name+".png"), buf.Bytes(), func(want []byte) bool {
				img, err := png.Decode(bytes.NewReader(want))
				return err == nil && pixelsEqual(img, res.Image)
			})
			tr, err := RenderText(context.Background(), c.src, c.text)
			if err != nil {
				t.Fatal(err)
			}
			var b strings.Builder
			for i, l := range tr.Lines {
				mark := "  "
				if i == tr.Baseline {
					mark = "> "
				}
				b.WriteString(mark + l + "\n")
			}
			got := b.String()
			checkGolden(t, filepath.Join("testdata", "golden", c.name+".txt"), []byte(got), func(want []byte) bool {
				return string(want) == got
			})
		})
	}
}
