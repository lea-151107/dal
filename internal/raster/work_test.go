package raster

import (
	"testing"

	"github.com/lea/dal/internal/scene"
)

// TestCheckWorkNoFusion checks a sum of work that lands within an ulp of
// the limit: with the products fused into the sum (GOAMD64=v3, arm64) it
// crossed the limit on some machines only. Run it with GOAMD64=v3 too.
func TestCheckWorkNoFusion(t *testing.T) {
	sc := &scene.Scene{Items: []scene.Item{
		&scene.Rect{X0: 0, Y0: 0, X1: 0, Y1: 7.440613119011058},
		&scene.Rect{X0: 0, Y0: 0, X1: 4.837282303836957, Y1: 7.5240727043600995},
	}}
	if err := checkWork(sc, nil, 21); err != nil {
		t.Errorf("checkWork: %v", err)
	}
}
