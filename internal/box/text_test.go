package box

import (
	"context"
	"reflect"
	"testing"

	"github.com/lea/dal/internal/guard"
	"github.com/lea/dal/internal/mathfont"
	"github.com/lea/dal/internal/parse"
)

// TestTextRuns checks that \text uses the math font for the characters it
// has and sends only the rest to the text face.
func TestTextRuns(t *testing.T) {
	f, err := mathfont.Load(nil)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		src      string
		measured []string
		glyphs   bool
	}{
		{`\text{otherwise}`, nil, true},
		{`\text{速度 v}`, []string{"速度 "}, true},
		{`\text{a 速}`, []string{"速"}, true},
		{`\text{速` + "́" + `}`, []string{"速́"}, false},
		{`\text{e` + "́" + `}`, nil, true},
		{`\textbf{x 日本}`, []string{"日本"}, true},
		// The face lacks U+0303, so it is set with the math font.
		{`\text{日本語 ` + "\u0303" + `é}`, []string{"日本語 "}, true},
	}
	for _, c := range cases {
		e, err := parse.Parse(c.src, parse.Limits{MaxInputBytes: 1 << 10, MaxNodes: 1 << 10, MaxDepth: 64, MaxExpansions: 1 << 10})
		if err != nil {
			t.Fatalf("%s: %v", c.src, err)
		}
		var got []string
		env := &Env{F: f, G: guard.New(context.Background(), 1<<10), SizePx: 20,
			Measure: func(s string, px float64) (float64, float64, float64, error) {
				got = append(got, s)
				return 10, 8, 2, nil
			},
			Has: func(r rune, px float64) (bool, error) { return r != '\u0303', nil }}
		b, err := Layout(env, e, false)
		if err != nil {
			t.Fatalf("%s: %v", c.src, err)
		}
		if !reflect.DeepEqual(got, c.measured) {
			t.Errorf("%s: measured %q, want %q", c.src, got, c.measured)
		}
		texts, glyphs := leaves(b)
		if texts != len(c.measured) || (glyphs > 0) != c.glyphs {
			t.Errorf("%s: %d text and %d glyph leaves", c.src, texts, glyphs)
		}
	}
}

func leaves(b *Box) (texts, glyphs int) {
	switch b.Leaf.(type) {
	case TextLeaf:
		texts++
	case GlyphLeaf:
		glyphs++
	}
	for _, k := range b.Kids {
		t, g := leaves(k.B)
		texts, glyphs = texts+t, glyphs+g
	}
	return texts, glyphs
}
