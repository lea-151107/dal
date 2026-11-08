package parse

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/lea-151107/dal/internal/diag"
	"github.com/lea-151107/dal/internal/guard"
)

// maxExpandedTokens bounds the tokens produced by macro expansion, so that a
// few short macros cannot blow up exponentially within MaxExpansions.
const maxExpandedTokens = 1 << 16

// maxScopes bounds the nesting of groups the expander tracks for macro
// scoping; deeper input fails in the parser's depth check anyway.
const maxScopes = 1 << 12

// macro is a user definition. delims[0] are the tokens that must follow
// the macro name before the first parameter, and delims[i] (i ≥ 1) the
// tokens that end parameter #i; an empty delims[i] means an undelimited
// parameter (a group or one token). For \newcommand with an optional first
// argument, opt holds its default.
type macro struct {
	nargs  int
	delims [][]token
	body   []token
	opt    []token
	hasOpt bool
}

// expander delivers tokens with user macros expanded.
type expander struct {
	toks    []token
	pos     int
	pending []token // pushed-back or expanded tokens; the last one is next
	// macros holds the visible definition of each name on top of the
	// ones it shadows, so that a lookup does not depend on how many
	// groups are open. scopes[d] lists the names defined locally in the
	// open group of depth d; scopes[0] is the global level.
	macros map[string][]binding
	scopes [][]string
	// floor is the depth below which popScope does not go: the expander
	// of an \edef body shares macros with the outer one, and an
	// unbalanced } there must not drop the outer definitions.
	floor  int
	eofOff int

	expansions    int
	maxExpansions int
	produced      int
	// edef is set while expanding the body of \edef: definitions are
	// not carried out but kept in the body, as TeX does with
	// unexpandable commands.
	edef bool
}

func newExpander(toks []token, eofOff, maxExpansions int) *expander {
	return &expander{toks: toks, macros: map[string][]binding{}, scopes: [][]string{nil}, eofOff: eofOff, maxExpansions: maxExpansions}
}

// raw returns the next unexpanded token.
func (e *expander) raw() token {
	if n := len(e.pending); n > 0 {
		t := e.pending[n-1]
		e.pending = e.pending[:n-1]
		return t
	}
	if e.pos < len(e.toks) {
		t := e.toks[e.pos]
		e.pos++
		return t
	}
	return token{kind: tEOF, off: e.eofOff}
}

func (e *expander) unread(t token) { e.pending = append(e.pending, t) }

// pushScope and popScope open and close a group for local definitions
// where the parser sees one that has no braces: an array cell, an
// environment, \left ... \right.
func (e *expander) pushScope() {
	if len(e.scopes) < maxScopes {
		e.scopes = append(e.scopes, nil)
	}
}

func (e *expander) popScope() {
	if len(e.scopes) > max(e.floor, 1) {
		d := len(e.scopes) - 1
		for _, name := range e.scopes[d] {
			// A \global definition since then has replaced the stack.
			if st := e.macros[name]; len(st) > 0 && st[len(st)-1].depth == d {
				if st = st[:len(st)-1]; len(st) == 0 {
					delete(e.macros, name)
				} else {
					e.macros[name] = st
				}
			}
		}
		e.scopes = e.scopes[:d]
	}
}

// binding is a definition made at a group depth.
type binding struct {
	depth int
	m     *macro
}

func (e *expander) lookup(name string) (*macro, bool) {
	st := e.macros[name]
	if len(st) == 0 {
		return nil, false
	}
	m := st[len(st)-1].m
	return m, m != nil
}

// expandAfter carries out \expandafter, whose name was just read: it
// expands the token after the next one once and puts the next one back in
// front. When that token is \expandafter itself, expanding it once is
// doing this step again, so chains (\expandafter\a\expandafter{\b})
// work as in TeX.
func (e *expander) expandAfter() error {
	// The chain is walked with a loop, not recursion, so that a long
	// chain cannot exhaust the stack: firsts holds the tokens to put back,
	// outermost first.
	var firsts []token
	for {
		first := e.raw()
		second := e.raw()
		firsts = append(firsts, first)
		if second.kind == tCmd && !second.seen && second.text == "expandafter" {
			continue
		}
		expanded := false
		if second.kind == tCmd && !second.seen {
			if m, ok := e.lookup(second.text); ok {
				if err := e.expand(second, m); err != nil {
					return err
				}
				expanded = true
			}
		}
		if !expanded {
			e.unread(second)
		}
		break
	}
	for i := len(firsts) - 1; i >= 0; i-- {
		e.unread(firsts[i])
	}
	return nil
}

// next returns the next token after macro expansion. Definitions are
// processed here and never reach the parser. Braces open and close the
// scopes of local definitions.
func (e *expander) next() (token, error) {
	for {
		t := e.raw()
		if t.seen {
			return t, nil
		}
		switch t.kind {
		case tChar:
			switch t.text {
			case "{":
				e.pushScope()
			case "}":
				e.popScope()
			}
			t.seen = true
			return t, nil
		case tCmd:
		default:
			t.seen = true
			return t, nil
		}
		if e.edef {
			switch t.text {
			case "begingroup", "bgroup", "endgroup", "egroup", "long", "global", "newcommand", "renewcommand",
				"providecommand", "def", "gdef", "edef", "xdef", "let", "futurelet":
				t.seen = true
				return t, nil
			}
		}
		switch t.text {
		case "begingroup", "bgroup":
			e.unread(token{kind: tChar, text: "{", off: t.off})
			continue
		case "endgroup", "egroup":
			e.unread(token{kind: tChar, text: "}", off: t.off})
			continue
		case "long":
			continue // \long only matters to TeX's paragraph checks
		case "global":
			if err := e.definition(e.skipSpaceRaw(), true); err != nil {
				return token{}, err
			}
			continue
		case "newcommand", "renewcommand", "providecommand", "def", "gdef", "edef", "xdef", "let", "futurelet":
			if err := e.definition(t, false); err != nil {
				return token{}, err
			}
			continue
		case "expandafter":
			if err := e.expandAfter(); err != nil {
				return token{}, err
			}
			continue
		case "noexpand":
			n := e.raw()
			n.seen = true
			return n, nil
		}
		m, ok := e.lookup(t.text)
		if !ok {
			t.seen = true
			return t, nil
		}
		if err := e.expand(t, m); err != nil {
			return token{}, err
		}
	}
}

func (e *expander) peek() (token, error) {
	t, err := e.next()
	if err != nil {
		return t, err
	}
	e.unread(t)
	return t, nil
}

// skipSpaceRaw returns the next unexpanded non-space token.
func (e *expander) skipSpaceRaw() token {
	for {
		if t := e.raw(); t.kind != tSpace {
			return t
		}
	}
}

// group reads a balanced {...} group of unexpanded tokens whose opening
// brace was already consumed.
func (e *expander) group(open token) ([]token, error) {
	var out []token
	depth := 0
	for {
		t := e.raw()
		switch {
		case t.kind == tEOF:
			return nil, diag.Syntaxf(open.off, "missing }")
		case t.isChar("{"):
			depth++
		case t.isChar("}"):
			if depth == 0 {
				return out, nil
			}
			depth--
		}
		out = append(out, t)
		if len(out) > maxExpandedTokens {
			return nil, fmt.Errorf("%w: macro argument longer than %d tokens", guard.ErrTooLarge, maxExpandedTokens)
		}
	}
}

// arg reads one undelimited macro argument: a group or a single token.
func (e *expander) arg(call token) ([]token, error) {
	t := e.skipSpaceRaw()
	switch {
	case t.kind == tEOF:
		return nil, diag.Syntaxf(call.off, "missing argument for \\%s", call.text)
	case t.isChar("{"):
		return e.group(t)
	case t.isChar("}"):
		return nil, diag.Syntaxf(t.off, "missing argument for \\%s", call.text)
	case t.kind == tURL:
		// The lexer took {...} after \url verbatim; a user macro named
		// \url reads it as ordinary tokens, as TeX would.
		return relex(t)
	}
	return []token{t}, nil
}

// relex lexes the text of a verbatim URL token as ordinary tokens, with
// offsets at the token.
func relex(t token) ([]token, error) {
	toks, err := lex(t.text)
	if err != nil {
		return nil, err
	}
	for i := range toks {
		toks[i].off = t.off
	}
	return toks, nil
}

// sourceText writes tokens back as source text, for a macro argument
// that goes into a verbatim URL (\url{#1}). A token counts as one in the
// expansion budget whatever its length (a long command name), so the text
// is checked against maxExpandedTokens bytes while it is written.
func sourceText(toks []token) (string, error) {
	var b strings.Builder
	for i, t := range toks {
		if b.Len() > maxExpandedTokens {
			return "", errURLTooLong()
		}
		switch t.kind {
		case tCmd:
			b.WriteString("\\" + t.text)
			if isLetter(t.text[0]) && i+1 < len(toks) && toks[i+1].kind == tChar && isLetter(toks[i+1].text[0]) {
				b.WriteByte(' ')
			}
		case tParam:
			b.WriteString("#" + t.text)
		case tURL:
			b.WriteString("{" + t.text + "}")
		case tVerb:
			// The original delimiter is not kept: take one that does not
			// occur in the text.
			d := ""
			for _, c := range "|!+/=:\"@'()<>[]-.,;?^_`~&$" {
				if !strings.ContainsRune(t.text, c) {
					d = string(c)
					break
				}
			}
			if d == "" {
				// Every candidate occurs: use a visible character outside
				// ASCII that does not (no format, control or combining
				// character, which would reach the output).
				for c := rune(0xA1); ; c++ {
					if unicode.IsPrint(c) && !unicode.Is(unicode.M, c) && !isControl(c) && !strings.ContainsRune(t.text, c) {
						d = string(c)
						break
					}
				}
			}
			b.WriteString("\\verb" + d + t.text + d)
		case tSpace:
			b.WriteByte(' ')
		default:
			b.WriteString(t.text)
		}
	}
	if b.Len() > maxExpandedTokens {
		return "", errURLTooLong()
	}
	return b.String(), nil
}

func errURLTooLong() error {
	return fmt.Errorf("%w: verbatim text from macro arguments longer than %d bytes", guard.ErrTooLarge, maxExpandedTokens)
}

// substituteURL replaces the parameters #1 to #9 in the text of a verbatim
// URL token of a macro body by the arguments' source text; ## stays a
// single #, and parameters beyond the macro's arguments stay as they are.
// A result longer than maxExpandedTokens bytes fails before it is built:
// \url{#1#1...} with a long argument would otherwise allocate far beyond
// the expansion budget, which counts URL bytes.
func substituteURL(text string, args [][]token) (string, error) {
	if !strings.Contains(text, "#") {
		return text, nil
	}
	src := make([]*string, len(args))
	var b strings.Builder
	for i := 0; i < len(text); i++ {
		if b.Len() > maxExpandedTokens {
			return "", errURLTooLong()
		}
		c := text[i]
		if c != '#' || i+1 >= len(text) {
			b.WriteByte(c)
			continue
		}
		switch n := text[i+1]; {
		case n == '#':
			b.WriteByte('#')
			i++
		case n >= '1' && n <= '9' && int(n-'1') < len(args):
			k := n - '1'
			if src[k] == nil {
				a, err := sourceText(args[k])
				if err != nil {
					return "", err
				}
				a = unescapeURL(a)
				src[k] = &a
			}
			b.WriteString(*src[k])
			i++
		default:
			b.WriteByte(c)
		}
	}
	if b.Len() > maxExpandedTokens {
		return "", errURLTooLong()
	}
	return b.String(), nil
}

// delimitedArg reads tokens up to delim (which it consumes), keeping
// braces balanced; a lone {...} around the whole argument is removed, as
// in TeX.
func (e *expander) delimitedArg(call token, delim []token) ([]token, error) {
	var out []token
	depth := 0
	// matched is the length of the longest prefix of delim that ends the
	// tokens read at depth 0 (Knuth-Morris-Pratt), so that a long
	// delimiter costs linear time. A delimiter holds no braces, so a
	// brace or a token inside a group restarts the match.
	fail := kmpFailure(delim)
	matched := 0
	for {
		if matched == len(delim) {
			out = out[:len(out)-len(delim)]
			if len(out) >= 2 && out[0].isChar("{") && out[len(out)-1].isChar("}") && balancedInside(out) {
				out = out[1 : len(out)-1]
			}
			return out, nil
		}
		t := e.raw()
		switch {
		case t.kind == tEOF:
			return nil, diag.Syntaxf(call.off, "missing the end of an argument of \\%s", call.text)
		case t.isChar("{"):
			depth++
		case t.isChar("}"):
			if depth == 0 {
				return nil, diag.Syntaxf(t.off, "unbalanced } in an argument of \\%s", call.text)
			}
			depth--
		}
		switch {
		case depth > 0 || t.isChar("{") || t.isChar("}"):
			matched = 0
		default:
			for matched > 0 && !sameToken(delim[matched], t) {
				matched = fail[matched-1]
			}
			if sameToken(delim[matched], t) {
				matched++
			}
		}
		out = append(out, t)
		if len(out) > maxExpandedTokens {
			return nil, fmt.Errorf("%w: macro argument longer than %d tokens", guard.ErrTooLarge, maxExpandedTokens)
		}
	}
}

// balancedInside reports whether the outer braces of out match each other
// ({a}{b} has outer braces that do not).
func balancedInside(out []token) bool {
	depth := 0
	for i, t := range out {
		switch {
		case t.isChar("{"):
			depth++
		case t.isChar("}"):
			depth--
			if depth == 0 && i != len(out)-1 {
				return false
			}
		}
	}
	return true
}

func sameToken(a, b token) bool { return a.kind == b.kind && a.text == b.text }

// kmpFailure returns, for each prefix of delim, the length of its longest
// proper prefix that is also its suffix.
func kmpFailure(delim []token) []int {
	fail := make([]int, len(delim))
	k := 0
	for i := 1; i < len(delim); i++ {
		for k > 0 && !sameToken(delim[k], delim[i]) {
			k = fail[k-1]
		}
		if sameToken(delim[k], delim[i]) {
			k++
		}
		fail[i] = k
	}
	return fail
}

// macroName reads the name being defined: \name or {\name}.
func (e *expander) macroName(def token) (string, error) {
	t := e.skipSpaceRaw()
	if t.isChar("{") {
		inner, err := e.group(t)
		if err != nil {
			return "", err
		}
		var names []token
		for _, x := range inner {
			if x.kind != tSpace {
				names = append(names, x)
			}
		}
		if len(names) != 1 {
			return "", diag.Syntaxf(t.off, "\\%s needs a single command name", def.text)
		}
		t = names[0]
	}
	if t.kind != tCmd {
		return "", diag.Syntaxf(t.off, "\\%s needs a command name", def.text)
	}
	return t.text, nil
}

func (e *expander) define(name string, m *macro, def token, global bool) error {
	switch name {
	case "newcommand", "renewcommand", "providecommand", "def", "gdef", "edef", "xdef", "let", "futurelet",
		"global", "long", "expandafter", "noexpand", "begin", "end", "left", "right",
		"begingroup", "endgroup", "bgroup", "egroup":
		// The expander handles these before looking up definitions, so a
		// definition would be silently ignored.
		return diag.Syntaxf(def.off, "cannot redefine \\%s", name)
	case "csname", "endcsname":
		// \csname is not supported. Defining it would usually come from
		// \expandafter\def\csname name\endcsname{...}, which in TeX
		// defines \name; taking it literally would define \csname with
		// the name as a delimiter and silently drop the definition.
		return diag.Unsupported(`\csname`, def.off)
	}
	for k, t := range m.body {
		if t.kind == tParam && !escapedParam(m.body, k) && int(t.text[0]-'0') > m.nargs {
			return diag.Syntaxf(t.off, "parameter #%s exceeds %d arguments", t.text, m.nargs)
		}
	}
	_, defined := e.lookup(name)
	defined = defined || isBuiltin(name)
	switch {
	case def.text == "providecommand" && defined:
		return nil // only defines what does not exist yet
	case def.text == "newcommand" && defined:
		return diag.Syntaxf(def.off, "\\newcommand: \\%s is already defined (use \\renewcommand)", name)
	}
	if global {
		e.macros[name] = []binding{{0, m}}
		return nil
	}
	d := len(e.scopes) - 1
	st := e.macros[name]
	if len(st) > 0 && st[len(st)-1].depth == d {
		st[len(st)-1].m = m
		return nil
	}
	e.macros[name] = append(st, binding{d, m})
	e.scopes[d] = append(e.scopes[d], name)
	return nil
}

// definition processes a defining command (\def, \newcommand, \let, ...).
// global is set by a preceding \global.
func (e *expander) definition(def token, global bool) error {
	if def.kind != tCmd {
		return diag.Syntaxf(def.off, "\\global must be followed by a definition")
	}
	switch def.text {
	case "newcommand", "renewcommand", "providecommand":
		return e.newcommand(def, global)
	case "def", "edef":
		return e.def(def, global)
	case "gdef", "xdef":
		return e.def(def, true)
	case "let":
		return e.let(def, global)
	case "futurelet":
		return e.futurelet(def, global)
	case "long":
		return e.definition(e.skipSpaceRaw(), global)
	}
	return diag.Syntaxf(def.off, "\\global must be followed by a definition, not \\%s", def.text)
}

func (e *expander) newcommand(def token, global bool) error {
	name, err := e.macroName(def)
	if err != nil {
		return err
	}
	m := &macro{}
	t := e.skipSpaceRaw()
	if t.isChar("[") {
		n := e.skipSpaceRaw()
		if n.kind != tChar || len(n.text) != 1 || n.text[0] < '0' || n.text[0] > '9' {
			return diag.Syntaxf(n.off, "invalid argument count for \\%s", def.text)
		}
		m.nargs = int(n.text[0] - '0')
		if c := e.skipSpaceRaw(); !c.isChar("]") {
			return diag.Syntaxf(c.off, "missing ] in \\%s", def.text)
		}
		t = e.skipSpaceRaw()
		if t.isChar("[") {
			if m.nargs == 0 {
				return diag.Syntaxf(t.off, "a default value for \\%s needs at least one argument", def.text)
			}
			opt, err := e.delimitedArg(def, []token{{kind: tChar, text: "]"}})
			if err != nil {
				return err
			}
			m.opt, m.hasOpt = opt, true
			t = e.skipSpaceRaw()
		}
	}
	if !t.isChar("{") {
		return diag.Syntaxf(t.off, "missing body for \\%s", def.text)
	}
	if m.body, err = e.group(t); err != nil {
		return err
	}
	return e.define(name, m, def, global)
}

func (e *expander) def(def token, global bool) error {
	t := e.skipSpaceRaw()
	if t.kind != tCmd {
		return diag.Syntaxf(t.off, "\\%s needs a command name", def.text)
	}
	name := t.text
	m := &macro{delims: [][]token{nil}}
	for {
		p := e.raw()
		switch {
		case p.isChar("{"):
			body, err := e.group(p)
			if err != nil {
				return err
			}
			if def.text == "edef" || def.text == "xdef" {
				if body, err = e.expandAll(body); err != nil {
					return err
				}
			}
			m.body = body
			return e.define(name, m, def, global)
		case p.kind == tParam:
			if int(p.text[0]-'0') != m.nargs+1 {
				return diag.Syntaxf(p.off, "parameters of \\%s must be numbered #1, #2, ... in order", def.text)
			}
			m.nargs++
			m.delims = append(m.delims, nil)
		case p.kind == tEOF:
			return diag.Syntaxf(def.off, "missing body for \\%s", def.text)
		case p.isChar("#"):
			return diag.Syntaxf(p.off, "unexpected #")
		case p.isChar("}"):
			return diag.Syntaxf(p.off, "unexpected } in the parameters of \\%s", def.text)
		default:
			// Parameter text: tokens that must follow the name (before #1)
			// or that end the preceding parameter.
			m.delims[m.nargs] = append(m.delims[m.nargs], p)
			if len(m.delims[m.nargs]) > maxExpandedTokens {
				return fmt.Errorf("%w: parameter text longer than %d tokens", guard.ErrTooLarge, maxExpandedTokens)
			}
		}
	}
}

// expandAll expands body completely (for \edef): user macros are replaced
// by their expansion, other tokens are kept.
func (e *expander) expandAll(body []token) ([]token, error) {
	sub := &expander{toks: body, macros: e.macros, scopes: e.scopes, floor: len(e.scopes), eofOff: e.eofOff, edef: true,
		expansions: e.expansions, maxExpansions: e.maxExpansions, produced: e.produced}
	var out []token
	for {
		t, err := sub.next()
		if err != nil {
			return nil, err
		}
		if t.kind == tEOF {
			break
		}
		t.seen = false
		out = append(out, t)
		if len(out) > maxExpandedTokens {
			return nil, fmt.Errorf("%w: macro expansion produced more than %d tokens", guard.ErrTooLarge, maxExpandedTokens)
		}
	}
	e.expansions, e.produced = sub.expansions, sub.produced
	return out, nil
}

// letTarget reads the token after \let\name, skipping an optional = and
// one optional space, and returns the macro that \name becomes.
func (e *expander) letTarget() (*macro, error) {
	t := e.skipSpaceRaw()
	if t.isChar("=") {
		t = e.raw()
		if t.kind == tSpace {
			t = e.raw()
		}
	}
	if t.kind == tEOF {
		return nil, diag.Syntaxf(t.off, "missing the token for \\let")
	}
	return e.meaning(t), nil
}

// meaning returns a macro that behaves like t: a copy of t's definition
// when t is a user macro, else a macro that produces t (a built-in command
// or a character).
func (e *expander) meaning(t token) *macro {
	if t.kind == tCmd {
		if m, ok := e.lookup(t.text); ok {
			return m
		}
	}
	t.seen = true // not re-expanded, so \let\a=\a does not loop
	return &macro{delims: [][]token{nil}, body: []token{t}}
}

func (e *expander) let(def token, global bool) error {
	t := e.skipSpaceRaw()
	if t.kind != tCmd {
		return diag.Syntaxf(t.off, "\\let needs a command name")
	}
	m, err := e.letTarget()
	if err != nil {
		return err
	}
	return e.define(t.text, m, def, global)
}

// futurelet: \futurelet\cs a b lets \cs be b, then continues with a b.
func (e *expander) futurelet(def token, global bool) error {
	t := e.skipSpaceRaw()
	if t.kind != tCmd {
		return diag.Syntaxf(t.off, "\\futurelet needs a command name")
	}
	a := e.raw()
	b := e.raw()
	if err := e.define(t.text, e.meaning(b), def, global); err != nil {
		return err
	}
	e.unread(b)
	e.unread(a)
	return nil
}

func (e *expander) expand(call token, m *macro) error {
	e.expansions++
	if e.expansions > e.maxExpansions {
		return fmt.Errorf("%w: more than %d macro expansions", guard.ErrTooLarge, e.maxExpansions)
	}
	args := make([][]token, m.nargs)
	if len(m.delims) > 0 {
		for _, want := range m.delims[0] {
			got := e.raw()
			if got.kind != want.kind || got.text != want.text {
				return diag.Syntaxf(call.off, "use of \\%s does not match its definition", call.text)
			}
		}
	}
	for i := range args {
		var a []token
		var err error
		switch {
		case i == 0 && m.hasOpt:
			a = m.opt
			if t := e.skipSpaceRaw(); t.isChar("[") {
				a, err = e.delimitedArg(call, []token{{kind: tChar, text: "]"}})
			} else {
				e.unread(t)
			}
		case len(m.delims) > i+1 && len(m.delims[i+1]) > 0:
			a, err = e.delimitedArg(call, m.delims[i+1])
		default:
			a, err = e.arg(call)
		}
		if err != nil {
			return err
		}
		args[i] = a
	}
	// Check the size before building the expansion: a body with many #1
	// times a long argument would otherwise allocate far beyond the limit.
	// A verbatim URL or \verb is one token whatever its length, and
	// \url{#1#1} doubles it, so its bytes count as tokens too.
	n, cost := 0, 0
	if err := expandBody(m.body, args, func(ts ...token) error {
		n += len(ts)
		cost += len(ts)
		for _, t := range ts {
			if t.kind == tURL || t.kind == tVerb {
				cost += len(t.text)
			}
		}
		if e.produced+cost > maxExpandedTokens {
			return fmt.Errorf("%w: macro expansion produced more than %d tokens", guard.ErrTooLarge, maxExpandedTokens)
		}
		return nil
	}); err != nil {
		return err
	}
	e.produced += cost
	out := make([]token, 0, n)
	_ = expandBody(m.body, args, func(ts ...token) error {
		out = append(out, ts...)
		return nil
	})
	// Errors inside expanded text point at the call site.
	for i := len(out) - 1; i >= 0; i-- {
		t := out[i]
		t.off = call.off
		e.pending = append(e.pending, t)
	}
	return nil
}

// escapedParam reports whether the parameter token body[k] belongs to a
// definition inside the body (##1) rather than to this macro: TeX pairs
// the #s before it, so it is escaped when an odd number of # precede it.
func escapedParam(body []token, k int) bool {
	c := 0
	for k-1-c >= 0 && body[k-1-c].kind == tChar && body[k-1-c].text == "#" {
		c++
	}
	return c%2 == 1
}

// expandBody emits the expansion of a macro body for the arguments. A run
// of # characters before a parameter follows TeX's rule that ## is #:
// with an even number of # in all, the parameter stays (for a definition
// inside the body) after half of them less one; with an odd number, the
// argument follows half of the others.
func expandBody(body []token, args [][]token, emit func(...token) error) error {
	hash := func(t token) bool { return t.kind == tChar && t.text == "#" }
	for k := 0; k < len(body); k++ {
		t := body[k]
		switch {
		case hash(t):
			r := 0
			for k+r < len(body) && hash(body[k+r]) {
				r++
			}
			k += r - 1
			if k+1 < len(body) && body[k+1].kind == tParam {
				p := body[k+1]
				k++
				h := r + 1 // the parameter's own # counts
				for range (h - 1) / 2 {
					if err := emit(t); err != nil {
						return err
					}
				}
				if h%2 == 0 {
					if err := emit(p); err != nil {
						return err
					}
				} else if err := emit(args[p.text[0]-'1']...); err != nil {
					return err
				}
				continue
			}
			for range (r + 1) / 2 {
				if err := emit(t); err != nil {
					return err
				}
			}
		case t.kind == tParam:
			if err := emit(args[t.text[0]-'1']...); err != nil {
				return err
			}
		case t.kind == tURL:
			var err error
			if t.text, err = substituteURL(t.text, args); err != nil {
				return err
			}
			if err := emit(t); err != nil {
				return err
			}
		default:
			if err := emit(t); err != nil {
				return err
			}
		}
	}
	return nil
}

// unescapeURL turns \% and the other escaped specials into the characters,
// as lexURL does for a URL written directly.
func unescapeURL(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && strings.IndexByte("#$%&~_^{}\\", s[i+1]) >= 0 {
			i++
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
