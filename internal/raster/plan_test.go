package raster

import (
	"testing"

	"github.com/lea-151107/dal/internal/scene"
)

// TestPlanClamp checks that extents beyond any pixel limit stay large and
// positive after conversion to int, also with a 32-bit int (GOARCH=386),
// so that the size checks reject them instead of seeing a wrapped value.
func TestPlanClamp(t *testing.T) {
	for _, sc := range []*scene.Scene{
		{Width: 3e9, Height: 10},
		{Width: 10, Height: 3e9, Depth: 3e9},
		{Width: 1e300, Height: 1e300, Depth: 1e300},
	} {
		lay := Plan(sc, 1024)
		if lay.W < 1<<20 && sc.Width > 1<<20 || lay.H < 1<<20 && sc.Height > 1<<20 || lay.W <= 0 || lay.H <= 0 || lay.Baseline <= 0 {
			t.Errorf("Plan(%+v) = %+v", *sc, lay)
		}
	}
}
