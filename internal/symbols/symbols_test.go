package symbols

import (
	"github.com/lea-151107/dal/internal/ast"
	"testing"
	"unicode/utf8"
)

// TestTables checks the consistency of the command tables.
func TestTables(t *testing.T) {
	for name, s := range Commands {
		if n := utf8.RuneCountInString(s.Text); n != 1 && !(n == 2 && s.Class.String() == "rel") {
			t.Errorf(`\%s: %q is not one character or a two-character relation`, name, s.Text)
		}
		if _, dup := Ops[name]; dup {
			t.Errorf(`\%s is both a symbol and an operator`, name)
		}
		if _, dup := Accents[name]; dup {
			t.Errorf(`\%s is both a symbol and an accent`, name)
		}
	}
	for alias, target := range Aliases {
		_, sym := Commands[target]
		_, op := Ops[target]
		_, del := Delims[`\`+target]
		_, del2 := Delims[target]
		if !sym && !op && !del && !del2 {
			t.Errorf(`alias \%s: target %q is not a command`, alias, target)
		}
		if _, dup := Commands[alias]; dup {
			t.Errorf(`alias \%s shadows a symbol`, alias)
		}
	}
	for key, d := range Delims {
		if d != "" && utf8.RuneCountInString(d) != 1 {
			t.Errorf("delimiter %s: %q is not one character", key, d)
		}
	}
	// Every opening delimiter class has a closing counterpart.
	open, closing := 0, 0
	seen := map[string]bool{}
	for _, d := range Delims {
		if seen[d] {
			continue
		}
		seen[d] = true
		switch DelimClass(d).String() {
		case "open":
			open++
		case "close":
			closing++
		}
	}
	if open != closing {
		t.Errorf("%d opening and %d closing delimiters", open, closing)
	}
	for r, sub := range ImageSubstitutes {
		if sub == "" || sub == string(r) {
			t.Errorf("substitute for %c is empty or itself", r)
		}
	}
	for name, a := range Accents {
		if _, ok := AccentText[a.Char]; !ok {
			t.Errorf(`\%s: no text form for U+%04X`, name, a.Char)
		}
	}
}

func TestAlphabet(t *testing.T) {
	for _, c := range []struct {
		r    rune
		fs   ast.FontStyle
		want rune
	}{
		{'a', ast.FontRoman, 'a'},
		{'a', ast.FontItalic, '𝑎'},
		{'h', ast.FontItalic, 'ℎ'}, // a hole in the Unicode block
		{'A', ast.FontBold, '𝐀'},
		{'R', ast.FontBlackboard, 'ℝ'},
		{'C', ast.FontCalligraphic, '𝒞'},
		{'1', ast.FontBold, '𝟏'},
		{'α', ast.FontItalic, '𝛼'},
	} {
		if got := Alphabet(c.r, c.fs); got != c.want {
			t.Errorf("Alphabet(%c, %d) = %c, want %c", c.r, c.fs, got, c.want)
		}
	}
}
