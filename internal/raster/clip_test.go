package raster

import (
	"math"
	"testing"
)

// area is the signed area of a closed polygon (shoelace formula).
func area(poly []pt) float64 {
	a := 0.0
	for i, p := range poly {
		q := poly[(i+1)%len(poly)]
		a += p.x*q.y - q.x*p.y
	}
	return a / 2
}

func TestClipBand(t *testing.T) {
	square := []pt{{0, 0}, {4, 0}, {4, 4}, {0, 4}}
	triangle := []pt{{0, 0}, {8, 0}, {0, 8}}
	inf := math.Inf(1)
	for _, c := range []struct {
		name   string
		poly   []pt
		vert   bool
		lo, hi float64
		want   float64 // area
	}{
		{"square open", square, true, -inf, inf, 16},
		{"square y band", square, true, 1, 3, 8},
		{"square x band", square, false, 1, 3, 8},
		{"square top half", square, true, -inf, 2, 8},
		{"square bottom half", square, true, 2, inf, 8},
		{"square outside", square, true, 5, 6, 0},
		{"triangle y band", triangle, true, 2, 4, 10}, // widths 6 and 4, height 2
		{"triangle x band", triangle, false, 0, 4, 24},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := clipBand(c.poly, c.vert, c.lo, c.hi)
			if a := math.Abs(area(got)); math.Abs(a-c.want) > 1e-9 {
				t.Errorf("area %v, want %v (%v)", a, c.want, got)
			}
			for _, p := range got {
				v := p.x
				if c.vert {
					v = p.y
				}
				if v < c.lo || v > c.hi {
					t.Errorf("point %v outside [%v, %v]", p, c.lo, c.hi)
				}
			}
		})
	}
	// The two halves of a cut add up to the whole, as the rasterizer adds
	// up the coverage of the parts of an assembly.
	top := clipBand(triangle, true, -inf, 3)
	bottom := clipBand(triangle, true, 3, inf)
	if a := area(top) + area(bottom); math.Abs(a-area(triangle)) > 1e-9 {
		t.Errorf("halves add up to %v, want %v", a, area(triangle))
	}
}
