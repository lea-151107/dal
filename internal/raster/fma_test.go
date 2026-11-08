package raster

import (
	"context"
	"crypto/sha256"
	"fmt"
	"image/color"
	"testing"

	"github.com/lea-151107/dal/internal/guard"
	"github.com/lea-151107/dal/internal/scene"
)

// TestLineNoFusion draws a line whose corner lands on a 1/64 rounding
// boundary: with nx, ny fused into the additions (GOAMD64=v3, arm64) the
// pixels differed. Run it with GOAMD64=v3 and -tags noasm too.
func TestLineNoFusion(t *testing.T) {
	l := &scene.Line{X0: 6.719877571303833, Y0: -7.951629163013407, X1: 7.018899032775687, Y1: -1.5446407590853233,
		Width: 0.7379548184069821, Color: color.NRGBA{A: 255}}
	sc := &scene.Scene{Items: []scene.Item{l}}
	img, err := Render(guard.New(context.Background(), 1<<20), sc, Layout{W: 16, H: 16, Baseline: 12}, nil, nil, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(img.Pix)); got[:16] != wantLineHash {
		t.Errorf("pixels hash %s, want %s...", got[:16], wantLineHash)
	}
}

const wantLineHash = "3758961b1e846c9d"
