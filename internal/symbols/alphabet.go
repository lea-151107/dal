package symbols

import "github.com/lea-151107/dal/internal/ast"

// Starts of the Latin ranges in the Mathematical Alphanumeric Symbols block
// (capital A; small a follows 26 code points later).
var latinStart = map[ast.FontStyle]rune{
	ast.FontBold:         0x1D400,
	ast.FontItalic:       0x1D434,
	ast.FontBoldItalic:   0x1D468,
	ast.FontCalligraphic: 0x1D49C,
	ast.FontFraktur:      0x1D504,
	ast.FontBlackboard:   0x1D538,
	ast.FontSans:         0x1D5A0,
	ast.FontMono:         0x1D670,
}

// Letters that live outside the block (the "holes" filled by Letterlike
// Symbols).
var latinHoles = map[ast.FontStyle]map[rune]rune{
	ast.FontItalic: {'h': 'ℎ'},
	ast.FontCalligraphic: {'B': 'ℬ', 'E': 'ℰ', 'F': 'ℱ', 'H': 'ℋ', 'I': 'ℐ', 'L': 'ℒ', 'M': 'ℳ', 'R': 'ℛ',
		'e': 'ℯ', 'g': 'ℊ', 'o': 'ℴ'},
	ast.FontFraktur:    {'C': 'ℭ', 'H': 'ℌ', 'I': 'ℑ', 'R': 'ℜ', 'Z': 'ℨ'},
	ast.FontBlackboard: {'C': 'ℂ', 'H': 'ℍ', 'N': 'ℕ', 'P': 'ℙ', 'Q': 'ℚ', 'R': 'ℝ', 'Z': 'ℤ'},
}

var digitStart = map[ast.FontStyle]rune{
	ast.FontBold:       0x1D7CE,
	ast.FontBlackboard: 0x1D7D8,
	ast.FontSans:       0x1D7E2,
	ast.FontMono:       0x1D7F6,
}

var greekStart = map[ast.FontStyle]rune{
	ast.FontBold:       0x1D6A8,
	ast.FontItalic:     0x1D6E2,
	ast.FontBoldItalic: 0x1D71C,
}

// greekIndex returns the position of a Greek letter inside a math Greek
// alphabet (58 code points: 25 capitals with ϴ, ∇, 25 small letters with ς,
// ∂ and six variants).
func greekIndex(r rune) int {
	switch {
	case r >= 0x391 && r <= 0x3A9:
		if r == 0x3A2 {
			return -1
		}
		return int(r - 0x391)
	case r == 0x3F4:
		return 17
	case r == '∇':
		return 25
	case r >= 0x3B1 && r <= 0x3C9:
		return 26 + int(r-0x3B1)
	}
	switch r {
	case '∂':
		return 51
	case 'ϵ':
		return 52
	case 'ϑ':
		return 53
	case 'ϰ':
		return 54
	case 'ϕ':
		return 55
	case 'ϱ':
		return 56
	case 'ϖ':
		return 57
	}
	return -1
}

// IsLetter reports whether r is italic by default in math mode (Latin
// letters and lowercase Greek, as in TeX).
func IsLetter(r rune) bool {
	if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
		return true
	}
	i := greekIndex(r)
	return i >= 26 && i != 51
}

// Alphabet maps r to the math alphabet style. It returns r unchanged when
// the style has no form for it (e.g. digits in \mathcal).
func Alphabet(r rune, style ast.FontStyle) rune {
	if style == ast.FontRoman || style == ast.FontDefault {
		return r
	}
	switch {
	case r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z':
		if h, ok := latinHoles[style][r]; ok {
			return h
		}
		start, ok := latinStart[style]
		if !ok {
			return r
		}
		if r >= 'a' {
			return start + 26 + (r - 'a')
		}
		return start + (r - 'A')
	case r >= '0' && r <= '9':
		if start, ok := digitStart[style]; ok {
			return start + (r - '0')
		}
		if style == ast.FontBoldItalic {
			return digitStart[ast.FontBold] + (r - '0')
		}
		return r
	}
	if i := greekIndex(r); i >= 0 {
		if start, ok := greekStart[style]; ok {
			return start + rune(i)
		}
	}
	return r
}

// Superscripts maps characters to their Unicode superscript forms.
var Superscripts = map[rune]rune{
	'0': '⁰', '1': '¹', '2': '²', '3': '³', '4': '⁴', '5': '⁵', '6': '⁶', '7': '⁷', '8': '⁸', '9': '⁹',
	'+': '⁺', '−': '⁻', '-': '⁻', '=': '⁼', '(': '⁽', ')': '⁾', 'n': 'ⁿ', 'i': 'ⁱ',
	'a': 'ᵃ', 'b': 'ᵇ', 'c': 'ᶜ', 'd': 'ᵈ', 'e': 'ᵉ', 'f': 'ᶠ', 'g': 'ᵍ', 'h': 'ʰ', 'j': 'ʲ', 'k': 'ᵏ',
	'l': 'ˡ', 'm': 'ᵐ', 'o': 'ᵒ', 'p': 'ᵖ', 'r': 'ʳ', 's': 'ˢ', 't': 'ᵗ', 'u': 'ᵘ', 'v': 'ᵛ', 'w': 'ʷ',
	'x': 'ˣ', 'y': 'ʸ', 'z': 'ᶻ',
	'A': 'ᴬ', 'B': 'ᴮ', 'D': 'ᴰ', 'E': 'ᴱ', 'G': 'ᴳ', 'H': 'ᴴ', 'I': 'ᴵ', 'J': 'ᴶ', 'K': 'ᴷ', 'L': 'ᴸ',
	'M': 'ᴹ', 'N': 'ᴺ', 'O': 'ᴼ', 'P': 'ᴾ', 'R': 'ᴿ', 'T': 'ᵀ', 'U': 'ᵁ', 'V': 'ⱽ', 'W': 'ᵂ',
	'α': 'ᵅ', 'β': 'ᵝ', 'γ': 'ᵞ', 'δ': 'ᵟ', 'φ': 'ᵠ', 'ϕ': 'ᵠ', 'χ': 'ᵡ', 'θ': 'ᶿ',
	'′': '′', '″': '″', '∘': '°',
}

// Subscripts maps characters to their Unicode subscript forms.
var Subscripts = map[rune]rune{
	'0': '₀', '1': '₁', '2': '₂', '3': '₃', '4': '₄', '5': '₅', '6': '₆', '7': '₇', '8': '₈', '9': '₉',
	'+': '₊', '−': '₋', '-': '₋', '=': '₌', '(': '₍', ')': '₎',
	'a': 'ₐ', 'e': 'ₑ', 'o': 'ₒ', 'x': 'ₓ', 'h': 'ₕ', 'k': 'ₖ', 'l': 'ₗ', 'm': 'ₘ', 'n': 'ₙ', 'p': 'ₚ',
	's': 'ₛ', 't': 'ₜ', 'i': 'ᵢ', 'j': 'ⱼ', 'r': 'ᵣ', 'u': 'ᵤ', 'v': 'ᵥ',
	'β': 'ᵦ', 'γ': 'ᵧ', 'ρ': 'ᵨ', 'φ': 'ᵩ', 'ϕ': 'ᵩ', 'χ': 'ᵪ',
}

// TextAlphabet maps r for text outputs: only the alphabets whose look is
// the point of the command (ℝ, ℒ, 𝔤) are converted.
// NOTE(katex-compat): bold, italic, sans and mono letters stay ASCII in text
// output because terminal fonts often lack the math alphanumerics and they
// hurt searchability.
func TextAlphabet(r rune, style ast.FontStyle) rune {
	switch style {
	case ast.FontBlackboard, ast.FontCalligraphic, ast.FontFraktur:
		return Alphabet(r, style)
	}
	return r
}
