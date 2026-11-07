// Package ast defines the syntax tree produced by the parser and shared by
// all outputs (image, text grid and single line). The root package re-exports
// these types as aliases.
package ast

import "image/color"

// Class is the TeX atom type of a node; it decides inter-atom spacing.
type Class uint8

// Atom classes (TeXbook chapter 17).
const (
	ClassOrd Class = iota
	ClassOp
	ClassBin
	ClassRel
	ClassOpen
	ClassClose
	ClassPunct
	ClassInner
)

func (c Class) String() string {
	return [...]string{"ord", "op", "bin", "rel", "open", "close", "punct", "inner"}[c]
}

// MathStyle is one of TeX's four math styles.
type MathStyle uint8

// Math styles. StyleNone means "inherit".
const (
	StyleNone MathStyle = iota
	StyleDisplay
	StyleText
	StyleScript
	StyleScriptScript
)

// FontStyle selects a math alphabet (\mathbf, \mathbb, ...).
type FontStyle uint8

// Math alphabets. FontDefault means italic letters and upright digits.
const (
	FontDefault FontStyle = iota
	FontRoman
	FontBold
	FontItalic
	FontBoldItalic
	FontSans
	FontMono
	FontBlackboard
	FontCalligraphic
	FontFraktur
)

// Limits controls where a big operator puts its scripts.
type Limits uint8

// Limit placements. LimitsAuto uses the operator default for the style.
const (
	LimitsAuto Limits = iota
	LimitsOn
	LimitsOff
)

// Node is a node of the syntax tree. Pos returns its byte offset in the
// source; for nodes produced by macro expansion it is the offset of the macro
// call.
type Node interface {
	Pos() int
}

// Expr is a parsed formula.
type Expr struct {
	Body []Node
	// Tag is the equation tag (\tag{...}); nil when absent.
	Tag *Tag
}

// Symbol is a single character (letter, digit, operator, relation...).
type Symbol struct {
	Offset int
	Text   string // one Unicode character, e.g. "α" or "−"
	Class  Class
	// Letter is true for symbols that are italic by default (Latin letters
	// and lowercase Greek).
	Letter bool
}

// Group is a braced group {...}. It behaves as one atom of the given class:
// Ord for plain braces, other classes for \mathrel{...} and friends.
type Group struct {
	Offset int
	Class  Class
	Body   []Node
}

// Scripts attaches a superscript and/or subscript to Base. Base may be nil
// (e.g. "^2" at the start of a group). A nil Sup or Sub is absent.
type Scripts struct {
	Offset int
	Base   Node
	Sup    []Node
	Sub    []Node
	HasSup bool
	HasSub bool
}

// Op is a big operator (\sum, \int) or a named function (\sin, \lim,
// \operatorname{...}).
type Op struct {
	Offset int
	Text   string // the symbol ("∑") or the name ("sin")
	Symbol bool   // true for big operator symbols, false for names
	// Body is set instead of Text for \operatorname with more than
	// letters (arg\,max, f_1); it is set upright.
	Body []Node
	// DefaultLimits is true when the operator puts scripts above and below
	// in display style (\sum, \lim), false otherwise (\int, \sin).
	DefaultLimits bool
	Limits        Limits
}

// Frac is \frac, \dfrac, \tfrac, \binom and friends.
type Frac struct {
	Offset      int
	Num, Den    []Node
	Bar         bool   // false for \binom
	Left, Right string // delimiters ("(" and ")" for \binom), "" for none
	Style       MathStyle
	// BarEm is the bar thickness in em given by \above or \genfrac; 0
	// means the font's default.
	BarEm float64
}

// Sqrt is \sqrt[Index]{Body}. Index is nil when absent.
type Sqrt struct {
	Offset int
	Index  []Node
	Body   []Node
}

// LeftRight is \left ... \right. A delimiter "." or "" means none.
type LeftRight struct {
	Offset      int
	Left, Right string
	Body        []Node
}

// Middle is \middle inside \left ... \right.
type Middle struct {
	Offset int
	Delim  string
}

// SizedDelim is \big, \Bigl, \biggr, ... Size is 1 to 4.
type SizedDelim struct {
	Offset int
	Delim  string
	Size   int
	Class  Class
}

// Accent is \hat, \vec, \widehat, ... Char is the combining accent
// character (U+0302 etc).
type Accent struct {
	Offset int
	Char   rune
	Name   string // command name without backslash ("hat")
	Wide   bool   // stretches over the whole base
	Under  bool   // below the base (\utilde, \underrightarrow)
	Body   []Node
}

// Overline is \overline{Body}; Under selects \underline.
type Overline struct {
	Offset int
	Under  bool
	Body   []Node
}

// Font applies a math alphabet to Body.
type Font struct {
	Offset int
	Style  FontStyle
	Body   []Node
}

// Text is \text{...} and the \textXX family. Body keeps spaces.
type Text struct {
	Offset int
	Text   string
	Style  FontStyle // FontRoman, FontBold, FontItalic, FontSans or FontMono
}

// Space is explicit spacing (\, \quad, ~, ...), in mu (18 mu = 1 em).
type Space struct {
	Offset int
	Mu     float64
}

// Align is the alignment of an array column.
type Align uint8

// Column alignments.
const (
	AlignCenter Align = iota
	AlignLeft
	AlignRight
)

// Column describes an array column; RuleBefore counts vertical rules (|)
// to its left.
type Column struct {
	Align      Align
	RuleBefore int
	// Dashed draws the rules before the column dashed (":" in the
	// column specification).
	Dashed bool
}

// Row is one row of an Array.
type Row struct {
	Cells [][]Node
	Tag   *Tag // per-row tag in align/gather
	// HLineBefore counts \hline commands before the row.
	HLineBefore int
	// HLineDashed draws those rules dashed (\hdashline).
	HLineDashed bool
}

// Array is a matrix-like environment.
type Array struct {
	Offset int
	Env    string // environment name ("pmatrix", "cases", "aligned", ...)
	Rows   []Row
	// Cols gives the alignment of each column. Columns beyond len(Cols)
	// use the last entry, or AlignCenter.
	Cols []Column
	// RuleAfter counts vertical rules after the last column.
	RuleAfter       int
	RuleAfterDashed bool
	// HLineAfter counts \hline commands after the last row.
	HLineAfter       int
	HLineAfterDashed bool
	Left, Right      string    // delimiters around the array
	Style            MathStyle // style of the cells; StyleNone keeps the surrounding style
	// AlignPairs is true for aligned/align: columns alternate right and
	// left and pairs are separated by a wide gap.
	AlignPairs bool
	// ColSepEm is the space between columns in em.
	ColSepEm float64
}

// Color colors Body.
type Color struct {
	Offset int
	Color  color.NRGBA
	Body   []Node
}

// Style switches the math style (\displaystyle, ...) for Body.
type Style struct {
	Offset int
	Style  MathStyle
	Body   []Node
}

// Phantom takes the space of Body without drawing it. NoWidth (\vphantom)
// keeps only the height and depth; NoHeight (\hphantom) only the width.
type Phantom struct {
	Offset   int
	Body     []Node
	NoWidth  bool
	NoHeight bool
}

// Smash draws Body but hides its height (Top) and/or depth (Bottom) from
// the layout.
type Smash struct {
	Offset      int
	Top, Bottom bool
	Body        []Node
}

// Lap draws Body with no width: \rlap (Align AlignLeft, ink to the
// right), \llap (AlignRight) and \clap (AlignCenter).
type Lap struct {
	Offset int
	Align  Align
	Body   []Node
}

// Raise moves Body up by Mu (down when negative). Center instead centers
// Body on the math axis (\vcenter).
type Raise struct {
	Offset int
	Mu     float64
	Center bool
	Body   []Node
}

// ColorBox fills the box of Body with Bg and, when Frame is set, draws a
// frame of that color around it (\colorbox, \fcolorbox).
type ColorBox struct {
	Offset int
	Bg     color.NRGBA
	Frame  *color.NRGBA
	Body   []Node
}

// Reflect mirrors Body left to right (\reflectbox).
type Reflect struct {
	Offset int
	Body   []Node
}

// Rule is a filled rectangle (\rule): width and height in mu, raised by
// RaiseMu.
type Rule struct {
	Offset            int
	WMu, HMu, RaiseMu float64
}

// Size scales Body by Factor (\tiny ... \Huge).
type Size struct {
	Offset int
	Factor float64
	Body   []Node
}

// Choice picks one of four bodies by the current math style
// (\mathchoice{display}{text}{script}{scriptscript}).
type Choice struct {
	Offset int
	Bodies [4][]Node
}

// Not strikes Body through with a slash (\not before a symbol that has no
// negated form of its own).
type Not struct {
	Offset int
	Body   []Node
}

// Boxed draws a frame around Body; Circle draws a circle instead
// (\textcircled).
type Boxed struct {
	Offset int
	Circle bool
	Body   []Node
}

// Cancel strikes Body out. Kind is "/" (\cancel), "\\" (\bcancel), "X"
// (\xcancel) or "-" (\sout).
type Cancel struct {
	Offset int
	Kind   string
	Body   []Node
}

// Brace is \overbrace (Under false) or \underbrace (Under true). Its label is
// given as a superscript or subscript and laid out as a limit.
type Brace struct {
	Offset int
	Under  bool
	Square bool // \overbracket, \underbracket
	Body   []Node
}

// OverUnder is \overset, \underset and \stackrel.
type OverUnder struct {
	Offset int
	Base   []Node
	Over   []Node // nil when absent
	Under  []Node // nil when absent
	Class  Class
}

// XArrow is \xrightarrow, \xleftarrow, ...
type XArrow struct {
	Offset int
	Arrow  rune
	Over   []Node
	Under  []Node // nil when absent
}

// Tag is an equation tag. Star is true for \tag*, which has no parentheses.
type Tag struct {
	Offset int
	Body   []Node
	Star   bool
}

// Pos implementations.

func (n *Symbol) Pos() int     { return n.Offset }
func (n *Group) Pos() int      { return n.Offset }
func (n *Scripts) Pos() int    { return n.Offset }
func (n *Op) Pos() int         { return n.Offset }
func (n *Frac) Pos() int       { return n.Offset }
func (n *Sqrt) Pos() int       { return n.Offset }
func (n *LeftRight) Pos() int  { return n.Offset }
func (n *Middle) Pos() int     { return n.Offset }
func (n *SizedDelim) Pos() int { return n.Offset }
func (n *Accent) Pos() int     { return n.Offset }
func (n *Overline) Pos() int   { return n.Offset }
func (n *Font) Pos() int       { return n.Offset }
func (n *Text) Pos() int       { return n.Offset }
func (n *Space) Pos() int      { return n.Offset }
func (n *Array) Pos() int      { return n.Offset }
func (n *Color) Pos() int      { return n.Offset }
func (n *Style) Pos() int      { return n.Offset }
func (n *Phantom) Pos() int    { return n.Offset }
func (n *Not) Pos() int        { return n.Offset }
func (n *Boxed) Pos() int      { return n.Offset }
func (n *Cancel) Pos() int     { return n.Offset }
func (n *Brace) Pos() int      { return n.Offset }
func (n *OverUnder) Pos() int  { return n.Offset }
func (n *XArrow) Pos() int     { return n.Offset }
func (n *Tag) Pos() int        { return n.Offset }
func (n *Smash) Pos() int      { return n.Offset }
func (n *Lap) Pos() int        { return n.Offset }
func (n *Raise) Pos() int      { return n.Offset }
func (n *ColorBox) Pos() int   { return n.Offset }
func (n *Reflect) Pos() int    { return n.Offset }
func (n *Rule) Pos() int       { return n.Offset }
func (n *Size) Pos() int       { return n.Offset }
func (n *Choice) Pos() int     { return n.Offset }

// Enclose draws a notation around Body: Kind "angl" (actuarial angle: a
// bar above and a rule on the right) or "phase" (an angle below and to
// the left).
type Enclose struct {
	Offset int
	Kind   string
	Body   []Node
}

// Logo is \TeX, \LaTeX or \KaTeX (Name without the backslash).
type Logo struct {
	Offset int
	Name   string
}

func (n *Enclose) Pos() int { return n.Offset }
func (n *Logo) Pos() int    { return n.Offset }

// Break is \allowbreak (Allow) or \nobreak: a hint for line breaking.
type Break struct {
	Offset int
	Allow  bool
}

func (n *Break) Pos() int { return n.Offset }
