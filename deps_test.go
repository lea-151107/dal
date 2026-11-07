package dal

import (
	"os/exec"
	"strings"
	"testing"
)

// TestDependencies enforces the pure Go policy: no cgo, no external
// commands, no network, and only the standard library plus x/image.
func TestDependencies(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go command not available")
	}
	out, err := exec.Command("go", "list", "-deps", "-f", "{{.ImportPath}} {{.Standard}} {{.Module}}", ".").CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, out)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.Fields(line)
		pkg, std := f[0], f[1] == "true"
		switch {
		case pkg == "runtime/cgo", pkg == "os/exec", pkg == "net", strings.HasPrefix(pkg, "net/"), pkg == "plugin":
			t.Errorf("forbidden dependency %s", pkg)
		case std:
		case strings.HasPrefix(pkg, "github.com/lea/dal"), strings.HasPrefix(pkg, "golang.org/x/image/"):
		case pkg == "golang.org/x/sys/cpu", strings.HasPrefix(pkg, "golang.org/x/text/"):
			// Pure Go packages imported by x/image itself (CPU feature
			// detection for vector, legacy encodings for sfnt).
		default:
			t.Errorf("unexpected third-party dependency %s", pkg)
		}
	}

	// Our own packages must not import unsafe or perform file I/O.
	out, err = exec.Command("go", "list", "-f", "{{.ImportPath}} {{join .Imports \" \"}}", "./...").CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, out)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.Fields(line)
		if strings.Contains(f[0], "/cmd/") {
			continue
		}
		for _, imp := range f[1:] {
			if imp == "unsafe" || imp == "os" || imp == "os/exec" || imp == "syscall" {
				t.Errorf("%s imports %s", f[0], imp)
			}
		}
	}
}

// layers lists, for every internal package, the internal packages it may
// import (CODING_GUIDELINES.md §3). A new package must be added here (and
// to the guideline's diagram) before it can be used. box, textgrid and
// linear are separate outputs of the same AST and must not import each
// other.
var layers = map[string][]string{
	"parse":    {"ast", "diag", "guard", "symbols"},
	"box":      {"ast", "diag", "guard", "mathfont", "scene", "symbols"},
	"raster":   {"curve", "guard", "mathfont", "scene"},
	"curve":    nil,
	"textgrid": {"ast", "diag", "guard", "symbols"},
	"linear":   {"ast", "symbols"},
	"symbols":  {"ast"},
	"mathfont": nil,
	"scene":    nil,
	"ast":      nil,
	"diag":     nil,
	"guard":    nil,
}

// TestLayering enforces the one-way dependency direction between internal
// packages.
func TestLayering(t *testing.T) {
	checkLayering(t, "github.com/lea/dal/internal/", layers)
}
