package parse

import (
	"reflect"

	"github.com/lea-151107/dal/internal/ast"
)

var nodeType = reflect.TypeFor[ast.Node]()

// countNodes returns the number of nodes in the tree under v (an
// *ast.Expr). The parser counts nodes as it makes them to stop early, but
// some constructs make several nodes at once (\varliminf, \pmod, scripts);
// this exact count is what Limits.MaxNodes bounds.
func countNodes(v reflect.Value) int {
	n := 0
	switch v.Kind() {
	case reflect.Interface, reflect.Pointer:
		if v.IsNil() {
			return 0
		}
		if v.Kind() == reflect.Pointer && v.Type().Implements(nodeType) {
			n++
		}
		return n + countNodes(v.Elem())
	case reflect.Struct:
		for i := range v.NumField() {
			n += countNodes(v.Field(i))
		}
	case reflect.Slice, reflect.Array:
		// Only containers of nodes; skip []byte, strings and numbers.
		if !containsNodes(v.Type().Elem(), nil) {
			return 0
		}
		for i := range v.Len() {
			n += countNodes(v.Index(i))
		}
	}
	return n
}

// containsNodes reports whether values of t can hold nodes. seen holds
// the struct types being looked at, so that a type that refers to itself
// ends the search.
func containsNodes(t reflect.Type, seen []reflect.Type) bool {
	switch t.Kind() {
	case reflect.Interface:
		return t == nodeType
	case reflect.Pointer, reflect.Slice, reflect.Array:
		return containsNodes(t.Elem(), seen)
	case reflect.Struct:
		for _, s := range seen {
			if s == t {
				return false
			}
		}
		seen = append(seen, t)
		for i := range t.NumField() {
			if containsNodes(t.Field(i).Type, seen) {
				return true
			}
		}
	}
	return false
}
