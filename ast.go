package dal

import "github.com/lea-151107/dal/internal/ast"

// The syntax tree returned by Parse. The types are defined in an internal
// package and re-exported here; see the field documentation there.
type (
	// Expr is a parsed formula.
	Expr = ast.Expr
	// Node is a node of the syntax tree.
	Node = ast.Node
	// Class is the TeX atom type of a node.
	Class = ast.Class
	// MathStyle is display, text, script or scriptscript style.
	MathStyle = ast.MathStyle
	// FontStyle is a math alphabet.
	FontStyle = ast.FontStyle
	// LimitsMode controls the placement of big operator scripts.
	LimitsMode = ast.Limits
	// Align is the alignment of an array column.
	Align = ast.Align
	// Column describes an array column.
	Column = ast.Column
	// Row is one row of an Array.
	Row = ast.Row

	// Symbol is a single character.
	Symbol = ast.Symbol
	// Group is a braced group.
	Group = ast.Group
	// Scripts attaches superscripts and subscripts.
	Scripts = ast.Scripts
	// Op is a big operator or named function.
	Op = ast.Op
	// Frac is a fraction or binomial.
	Frac = ast.Frac
	// Sqrt is a radical.
	Sqrt = ast.Sqrt
	// LeftRight is \left ... \right.
	LeftRight = ast.LeftRight
	// Middle is \middle.
	Middle = ast.Middle
	// SizedDelim is \big and friends.
	SizedDelim = ast.SizedDelim
	// Accent is \hat and friends.
	Accent = ast.Accent
	// Overline is \overline or \underline.
	Overline = ast.Overline
	// Font applies a math alphabet.
	Font = ast.Font
	// Text is \text{...}.
	Text = ast.Text
	// Space is explicit spacing.
	Space = ast.Space
	// Array is a matrix-like environment.
	Array = ast.Array
	// Color colors its body.
	Color = ast.Color
	// Style switches the math style.
	Style = ast.Style
	// Phantom reserves space.
	Phantom = ast.Phantom
	// Not is \not before a symbol without a negated form.
	Not = ast.Not
	// Boxed draws a frame.
	Boxed = ast.Boxed
	// Cancel strikes out its body.
	Cancel = ast.Cancel
	// Brace is \overbrace or \underbrace.
	Brace = ast.Brace
	// OverUnder is \overset, \underset or \stackrel.
	OverUnder = ast.OverUnder
	// XArrow is an extensible arrow.
	XArrow = ast.XArrow
	// Tag is an equation tag.
	Tag = ast.Tag
	// Smash hides the height and/or depth of its body (\smash).
	Smash = ast.Smash
	// Lap draws its body with no width (\rlap, \llap, \clap).
	Lap = ast.Lap
	// Raise moves its body up or down, or centers it on the axis
	// (\raisebox, \vcenter).
	Raise = ast.Raise
	// ColorBox fills and optionally frames its body (\colorbox, \fcolorbox).
	ColorBox = ast.ColorBox
	// Reflect mirrors its body left to right (\reflectbox).
	Reflect = ast.Reflect
	// Rule is a filled rectangle (\rule).
	Rule = ast.Rule
	// Size scales its body (\tiny ... \Huge).
	Size = ast.Size
	// Choice picks a body by math style (\mathchoice).
	Choice = ast.Choice
	// Enclose draws a notation around its body (\angl, \phase).
	Enclose = ast.Enclose
	// Logo is \TeX, \LaTeX or \KaTeX.
	Logo = ast.Logo
	// Break is \allowbreak or \nobreak.
	Break = ast.Break
)

// Math styles. StyleNone means "inherit".
const (
	StyleNone         = ast.StyleNone
	StyleDisplay      = ast.StyleDisplay
	StyleText         = ast.StyleText
	StyleScript       = ast.StyleScript
	StyleScriptScript = ast.StyleScriptScript
)

// Math alphabets. FontDefault means italic letters and upright digits.
const (
	FontDefault      = ast.FontDefault
	FontRoman        = ast.FontRoman
	FontBold         = ast.FontBold
	FontItalic       = ast.FontItalic
	FontBoldItalic   = ast.FontBoldItalic
	FontSans         = ast.FontSans
	FontMono         = ast.FontMono
	FontBlackboard   = ast.FontBlackboard
	FontCalligraphic = ast.FontCalligraphic
	FontFraktur      = ast.FontFraktur
)

// Limit placements of big operator scripts. LimitsAuto uses the operator
// default for the style.
const (
	LimitsAuto = ast.LimitsAuto
	LimitsOn   = ast.LimitsOn
	LimitsOff  = ast.LimitsOff
)

// Column alignments.
const (
	AlignCenter = ast.AlignCenter
	AlignLeft   = ast.AlignLeft
	AlignRight  = ast.AlignRight
)

// Atom classes.
const (
	ClassOrd   = ast.ClassOrd
	ClassOp    = ast.ClassOp
	ClassBin   = ast.ClassBin
	ClassRel   = ast.ClassRel
	ClassOpen  = ast.ClassOpen
	ClassClose = ast.ClassClose
	ClassPunct = ast.ClassPunct
	ClassInner = ast.ClassInner
)
