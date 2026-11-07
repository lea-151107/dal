package parse

import (
	"strings"

	"github.com/lea/dal/internal/ast"
	"github.com/lea/dal/internal/diag"
	"github.com/lea/dal/internal/symbols"
)

// textArg reads the argument of \text and its relatives. Plain text becomes
// Text nodes; nested text commands switch the style of their part and
// $...$ switches back to math.
func (p *parser) textArg(cmd token, style ast.FontStyle) ([]ast.Node, error) {
	if err := p.enter(); err != nil {
		return nil, err
	}
	defer p.leave()
	t, err := p.nextNonSpace()
	if err != nil {
		return nil, err
	}
	if !t.isChar("{") {
		if t.kind == tChar && !p.isTerminator(t, false) && t.text != "$" {
			if err := p.count(); err != nil {
				return nil, err
			}
			return []ast.Node{&ast.Text{Offset: t.off, Text: t.text, Style: style}}, nil
		}
		return nil, diag.Syntaxf(cmdOff(cmd, t), "missing argument for %s", cmdName(cmd))
	}
	var out []ast.Node
	var b strings.Builder
	runOff := t.off
	flush := func(next int) error {
		if b.Len() > 0 {
			if err := p.count(); err != nil {
				return err
			}
			out = append(out, &ast.Text{Offset: runOff, Text: b.String(), Style: style})
			b.Reset()
		}
		runOff = next
		return nil
	}
	depth := 0
	for {
		x, err := p.e.next()
		if err != nil {
			return nil, err
		}
		switch {
		case x.kind == tEOF:
			return nil, diag.Syntaxf(t.off, "missing }")
		case x.isChar("{"):
			depth++
		case x.isChar("}"):
			if depth == 0 {
				if err := flush(x.off); err != nil {
					return nil, err
				}
				if len(out) == 0 {
					if err := p.count(); err != nil {
						return nil, err
					}
					out = append(out, &ast.Text{Offset: t.off, Style: style})
				}
				return out, nil
			}
			depth--
		case x.kind == tSpace, x.isChar("~"):
			b.WriteByte(' ')
		case x.isChar("$"):
			if err := flush(x.off); err != nil {
				return nil, err
			}
			p.textMath++
			body, end, err := p.list()
			p.textMath--
			if err != nil {
				return nil, err
			}
			if !end.isChar("$") {
				return nil, diag.Syntaxf(x.off, "missing $ inside %s", cmdName(cmd))
			}
			out = append(out, body...)
		case x.kind == tChar:
			b.WriteString(x.text)
		case x.kind == tVerb:
			if err := flush(x.off); err != nil {
				return nil, err
			}
			if err := p.count(); err != nil {
				return nil, err
			}
			out = append(out, &ast.Text{Offset: x.off, Text: x.text, Style: ast.FontMono})
		case x.kind == tParam:
			return nil, diag.Syntaxf(x.off, "unexpected #")
		case x.kind == tCmd:
			if x.text == "emph" {
				if err := flush(x.off); err != nil {
					return nil, err
				}
				// \emph switches to italic, or back to upright inside
				// italic text.
				st := ast.FontItalic
				if style == ast.FontItalic {
					st = ast.FontRoman
				}
				inner, err := p.textArg(x, st)
				if err != nil {
					return nil, err
				}
				out = append(out, inner...)
				continue
			}
			if fs, ok := symbols.TextFonts[x.text]; ok {
				if err := flush(x.off); err != nil {
					return nil, err
				}
				inner, err := p.textArg(x, fs)
				if err != nil {
					return nil, err
				}
				out = append(out, inner...)
				continue
			}
			if x.text == "textcircled" {
				if err := flush(x.off); err != nil {
					return nil, err
				}
				inner, err := p.textArg(x, style)
				if err != nil {
					return nil, err
				}
				if err := p.count(); err != nil {
					return nil, err
				}
				out = append(out, &ast.Boxed{Offset: x.off, Circle: true, Body: inner})
				continue
			}
			if x.text == "(" {
				if err := flush(x.off); err != nil {
					return nil, err
				}
				p.textParen++
				body, end, err := p.list()
				p.textParen--
				if err != nil {
					return nil, err
				}
				if !end.isCmd(")") {
					return nil, diag.Syntaxf(x.off, "missing \\) inside %s", cmdName(cmd))
				}
				out = append(out, body...)
				continue
			}
			if x.text == "sout" {
				if err := flush(x.off); err != nil {
					return nil, err
				}
				inner, err := p.textArg(x, style)
				if err != nil {
					return nil, err
				}
				if err := p.count(); err != nil {
					return nil, err
				}
				out = append(out, &ast.Cancel{Offset: x.off, Kind: "-", Body: inner})
				continue
			}
			if s, ok := textSymbols[x.text]; ok {
				b.WriteString(s)
				continue
			}
			if _, ok := textAccents[x.text]; ok {
				r, err := p.textAccent(x)
				if err != nil {
					return nil, err
				}
				b.WriteString(r)
				continue
			}
			if x.text == "relax" {
				continue
			}
			return nil, diag.Unsupported("\\"+x.text+" inside "+cmdName(cmd), x.off)
		}
	}
}

// textSymbols are the control symbols and words allowed in text mode.
var textSymbols = map[string]string{
	" ": " ", "space": " ", "quad": " ", "qquad": "  ", ",": " ", ";": " ", ":": " ",
	"nobreakspace": "\u00a0", "{": "{", "}": "}", "$": "$", "%": "%", "&": "&", "_": "_", "#": "#",
	"textbackslash": "\\", "ldots": "…", "dots": "…", "textellipsis": "…",
	"textendash": "–", "textemdash": "—", "textasciitilde": "~", "textasciicircum": "^",
	"textbar": "|", "textless": "<", "textgreater": ">", "S": "§", "P": "¶", "copyright": "©",
	"dag": "†", "ddag": "‡", "textdegree": "°", "o": "ø", "O": "Ø", "ss": "ß", "ae": "æ", "AE": "Æ",
	"oe": "œ", "OE": "Œ", "aa": "å", "AA": "Å", "l": "ł", "L": "Ł", "i": "ı", "j": "ȷ",
	"textdollar": "$", "textunderscore": "_", "textbraceleft": "{", "textbraceright": "}",
	"textregistered": "®", "textquoteleft": "‘", "textquoteright": "’", "textquotedblleft": "“",
	"textquotedblright": "”", "lq": "‘", "rq": "’", "textdagger": "†", "textdaggerdbl": "‡",
	"textsterling": "£", "textbardbl": "‖", "sect": "§", "textbullet": "•", "textperiodcentered": "·",
}

// textAccents maps a text accent command to "base letters" and their
// accented forms, position by position.
var textAccents = map[string][2]string{
	"'":  {"aeiouyAEIOUYcnszCNSZglrkGLRK", "áéíóúýÁÉÍÓÚÝćńśźĆŃŚŹǵĺŕḱǴĹŔḰ"},
	"`":  {"aeiouAEIOU", "àèìòùÀÈÌÒÙ"},
	"^":  {"aeiouAEIOUcgshjwyCGSHJWY", "âêîôûÂÊÎÔÛĉĝŝĥĵŵŷĈĜŜĤĴŴŶ"},
	"\"": {"aeiouyAEIOUY", "äëïöüÿÄËÏÖÜŸ"},
	"~":  {"anoANOiuIU", "ãñõÃÑÕĩũĨŨ"},
	"c":  {"cCsStT", "çÇşŞţŢ"},
	"v":  {"cdenrstzCDENRSTZaiougkhAIOUGKH", "čďěňřšťžČĎĚŇŘŠŤŽǎǐǒǔǧǩȟǍǏǑǓǦǨȞ"},
	"u":  {"agAG", "ăğĂĞ"},
	"=":  {"aeiouAEIOU", "āēīōūĀĒĪŌŪ"},
	".":  {"zZeEgIaAoOcC", "żŻėĖġİȧȦȯȮċĊ"},
	"H":  {"ouOU", "őűŐŰ"},
	"k":  {"aeAE", "ąęĄĘ"},
	"r":  {"auAU", "åůÅŮ"},
}

// textAccentMarks gives the combining character of each text accent, for
// letters without a precomposed form.
var textAccentMarks = map[string]rune{
	"'": 0x301, "`": 0x300, "^": 0x302, "\"": 0x308, "~": 0x303, "c": 0x327, "v": 0x30C,
	"u": 0x306, "=": 0x304, ".": 0x307, "H": 0x30B, "k": 0x328, "r": 0x30A,
}

// textAccent reads the letter after an accent command (x or {x}) and
// returns the precomposed character.
func (p *parser) textAccent(cmd token) (string, error) {
	t, err := p.nextNonSpace()
	if err != nil {
		return "", err
	}
	braced := t.isChar("{")
	if braced {
		if t, err = p.e.next(); err != nil {
			return "", err
		}
	}
	if t.kind != tChar && !(t.kind == tCmd && (t.text == "i" || t.text == "j")) {
		return "", diag.Syntaxf(cmdOff(cmd, t), "missing letter after \\%s", cmd.text)
	}
	letter := t.text
	if t.kind == tCmd { // \'\i is í
		letter = map[string]string{"i": "i", "j": "j"}[t.text]
	}
	if braced {
		c, err := p.e.next()
		if err != nil {
			return "", err
		}
		if !c.isChar("}") {
			return "", diag.Syntaxf(c.off, "expected } after the accented letter")
		}
	}
	tab := textAccents[cmd.text]
	base, acc := []rune(tab[0]), []rune(tab[1])
	for i, r := range base {
		if string(r) == letter {
			return string(acc[i]), nil
		}
	}
	// No precomposed form: the letter and a combining mark.
	return letter + string(textAccentMarks[cmd.text]), nil
}
