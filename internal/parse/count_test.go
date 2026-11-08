package parse

import (
	"bufio"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/lea-151107/dal/internal/ast"
)

// allNodes counts every value implementing ast.Node under v, looking
// through maps and struct values too, which countNodes does not. A node
// field added later in a form countNodes skips makes the counts differ.
func allNodes(v reflect.Value) int {
	n := 0
	switch v.Kind() {
	case reflect.Interface, reflect.Pointer:
		if v.IsNil() {
			return 0
		}
		if v.Kind() == reflect.Pointer && v.Type().Implements(nodeType) {
			n++
		}
		return n + allNodes(v.Elem())
	case reflect.Struct:
		if v.Type().Implements(nodeType) {
			n++
		}
		for i := range v.NumField() {
			n += allNodes(v.Field(i))
		}
	case reflect.Slice, reflect.Array:
		for i := range v.Len() {
			n += allNodes(v.Index(i))
		}
	case reflect.Map:
		it := v.MapRange()
		for it.Next() {
			n += allNodes(it.Key()) + allNodes(it.Value())
		}
	}
	return n
}

func TestCountNodes(t *testing.T) {
	f, err := os.Open("../../testdata/katex_coverage.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	lim := Limits{MaxInputBytes: 1 << 20, MaxNodes: 1 << 20, MaxDepth: 64, MaxExpansions: 1000}
	sc := bufio.NewScanner(f)
	parsed := 0
	for sc.Scan() {
		src := sc.Text()
		if src == "" || strings.HasPrefix(src, "#") {
			continue
		}
		e, err := Parse(src, lim)
		if err != nil {
			continue
		}
		parsed++
		v := reflect.ValueOf(e)
		if got, want := countNodes(v), allNodes(v); got != want {
			t.Errorf("%q: countNodes %d, all nodes %d", src, got, want)
		}
	}
	if parsed < 900 {
		t.Errorf("only %d formulas parsed", parsed)
	}
}

// TestContainsNodesCycle checks that a type referring to itself ends the
// search instead of recursing forever.
func TestContainsNodesCycle(t *testing.T) {
	type loop struct{ Next *loop }
	type loopNode struct {
		Next *loopNode
		N    ast.Node
	}
	if containsNodes(reflect.TypeFor[loop](), nil) {
		t.Error("loop: got true")
	}
	if !containsNodes(reflect.TypeFor[loopNode](), nil) {
		t.Error("loopNode: got false")
	}
}
