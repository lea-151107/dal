package raster

import (
	"os"
	"strings"
	"testing"
)

// TestNoArchMath checks that raster does not call standard functions
// whose result differs between amd64 (assembly) and arm64 (pure Go with
// fused multiply-adds); GOAMD64=v3 does not reproduce that difference.
func TestNoArchMath(t *testing.T) {
	src, err := os.ReadFile("raster.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"math.Hypot(", "math.Exp(", "math.Log(", "math.Pow("} {
		if strings.Contains(string(src), f) {
			t.Errorf("raster.go calls %s", f)
		}
	}
}
