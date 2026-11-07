package dal

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// TestASTReexported checks that every exported type and constant of
// internal/ast is re-exported by ast.go, so that callers of Parse can name
// every node they may get back.
func TestASTReexported(t *testing.T) {
	fset := token.NewFileSet()
	internal, err := parser.ParseFile(fset, "internal/ast/ast.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	root, err := parser.ParseFile(fset, "ast.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	// want: exported names declared in internal/ast.
	want := map[string]bool{}
	for _, d := range internal.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, s := range gd.Specs {
			switch s := s.(type) {
			case *ast.TypeSpec:
				if s.Name.IsExported() {
					want[s.Name.Name] = true
				}
			case *ast.ValueSpec:
				if gd.Tok == token.CONST {
					for _, n := range s.Names {
						if n.IsExported() {
							want[n.Name] = true
						}
					}
				}
			}
		}
	}
	// got: names referenced as ast.X on the right-hand side in ast.go.
	got := map[string]bool{}
	ast.Inspect(root, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "ast" {
				got[sel.Sel.Name] = true
			}
		}
		return true
	})
	for n := range want {
		if !got[n] {
			t.Errorf("internal/ast.%s is not re-exported in ast.go", n)
		}
	}
}
