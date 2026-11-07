package parse

import "github.com/lea/dal/internal/symbols"

// parserCommands are the commands the parser handles itself (the cases of
// command and switchCommand, and the infix and structural ones).
// TestParserCommandsListed keeps it in step with the source.
var parserCommands = map[string]bool{
	// Structure read by the parser outside command (env.go, text.go).
	"\\": true, "cr": true, "newline": true, "hdashline": true, "textcircled": true, "(": true, ")": true,
	// Handled by the expander itself.
	"begingroup": true, "endgroup": true, "bgroup": true, "egroup": true, "long": true, "global": true,
	"expandafter": true, "noexpand": true, "newcommand": true, "renewcommand": true, "providecommand": true,
	"def": true, "gdef": true, "edef": true, "xdef": true, "let": true, "futurelet": true, "verb": true,
	"Big": true, "Bigg": true, "Biggl": true, "Biggm": true, "Biggr": true,
	"Bigl": true, "Bigm": true, "Bigr": true, "Bra": true, "Braket": true,
	"KaTeX": true, "Ket": true, "LaTeX": true, "Set": true, "TeX": true,
	"above": true, "allowbreak": true, "angl": true, "angln": true, "atop": true,
	"bcancel": true, "begin": true, "big": true, "bigg": true, "biggl": true,
	"biggm": true, "biggr": true, "bigl": true, "bigm": true, "bigr": true,
	"binom": true, "bmod": true, "boxed": true, "bra": true, "brace": true,
	"brack": true, "braket": true, "cancel": true, "cfrac": true, "char": true,
	"choose": true, "clap": true, "color": true, "colorbox": true, "dbinom": true,
	"dfrac": true, "displaylimits": true, "displaystyle": true, "end": true, "fbox": true,
	"fcolorbox": true, "frac": true, "genfrac": true, "hline": true, "hphantom": true,
	"href": true, "hskip": true, "hspace": true, "htmlClass": true, "htmlData": true,
	"htmlId": true, "htmlStyle": true, "kern": true, "ket": true, "left": true,
	"limits": true, "llap": true, "mathchoice": true, "mathclap": true, "mathllap": true,
	"mathreflectbox": true, "mathrlap": true, "mathstrut": true, "middle": true, "mkern": true,
	"mod": true, "mskip": true, "nobreak": true, "nolimits": true, "nonumber": true,
	"not": true, "notag": true, "operatorname": true, "operatornamewithlimits": true, "over": true,
	"overbrace": true, "overbracket": true, "overline": true, "overset": true, "phantom": true,
	"phase": true, "plim": true, "pmb": true, "pmod": true, "pod": true,
	"raisebox": true, "reflectbox": true, "relax": true, "right": true, "rlap": true,
	"rule": true, "scriptscriptstyle": true, "scriptstyle": true, "set": true, "smash": true,
	"sout": true, "sqrt": true, "stackrel": true, "substack": true, "tag": true,
	"tbinom": true, "textcolor": true, "textstyle": true, "tfrac": true, "underbar": true,
	"underbrace": true, "underbracket": true, "underline": true, "underlinesegment": true, "underset": true,
	"url": true, "varinjlim": true, "varliminf": true, "varlimsup": true, "varprojlim": true,
	"vcenter": true, "vphantom": true, "xcancel": true,
}

// isBuiltin reports whether name is a command dal defines, so that
// \newcommand refuses it and \providecommand leaves it alone.
//
// NOTE(katex-compat): KaTeX's aliases (\R, \N, \Z, \rarr, ...) do not
// count: LaTeX has none of them and documents commonly define \R
// themselves, which KaTeX rejects.
func isBuiltin(name string) bool {
	if parserCommands[name] || classCommands[name] != 0 || name == "mathord" {
		return true
	}
	if _, ok := symbols.Commands[name]; ok {
		return true
	}
	if _, ok := symbols.Delims["\\"+name]; ok {
		return true
	}
	if _, ok := symbols.Ops[name]; ok {
		return true
	}
	if _, ok := symbols.Spaces[name]; ok {
		return true
	}
	if _, ok := symbols.Accents[name]; ok {
		return true
	}
	if _, ok := symbols.Fonts[name]; ok {
		return true
	}
	if _, ok := symbols.FontSwitches[name]; ok {
		return true
	}
	if _, ok := symbols.TextFonts[name]; ok {
		return true
	}
	if _, ok := symbols.XArrows[name]; ok {
		return true
	}
	if _, ok := symbols.Sizes[name]; ok {
		return true
	}
	_, ok := textSymbols[name]
	return ok
}
