// Package symbols holds the read-only command tables: symbols and their atom
// classes, operators, delimiters, accents and the Unicode math alphabets.
// The coverage target is KaTeX's list of supported functions.
package symbols

import "github.com/lea-151107/dal/internal/ast"

// Sym is a symbol command.
type Sym struct {
	Text  string
	Class ast.Class
}

// Commands maps symbol commands (without the backslash) to characters.
var Commands = map[string]Sym{
	// Lowercase Greek. NOTE(katex-compat): \phi is ϕ and \varphi is φ, as
	// in TeX.
	"alpha": {"α", ast.ClassOrd}, "beta": {"β", ast.ClassOrd}, "gamma": {"γ", ast.ClassOrd}, "delta": {"δ", ast.ClassOrd},
	"epsilon": {"ϵ", ast.ClassOrd}, "varepsilon": {"ε", ast.ClassOrd}, "zeta": {"ζ", ast.ClassOrd}, "eta": {"η", ast.ClassOrd},
	"theta": {"θ", ast.ClassOrd}, "vartheta": {"ϑ", ast.ClassOrd}, "iota": {"ι", ast.ClassOrd}, "kappa": {"κ", ast.ClassOrd},
	"varkappa": {"ϰ", ast.ClassOrd}, "lambda": {"λ", ast.ClassOrd}, "mu": {"μ", ast.ClassOrd}, "nu": {"ν", ast.ClassOrd},
	"xi": {"ξ", ast.ClassOrd}, "omicron": {"ο", ast.ClassOrd}, "pi": {"π", ast.ClassOrd}, "varpi": {"ϖ", ast.ClassOrd},
	"rho": {"ρ", ast.ClassOrd}, "varrho": {"ϱ", ast.ClassOrd}, "sigma": {"σ", ast.ClassOrd}, "varsigma": {"ς", ast.ClassOrd},
	"tau": {"τ", ast.ClassOrd}, "upsilon": {"υ", ast.ClassOrd}, "phi": {"ϕ", ast.ClassOrd}, "varphi": {"φ", ast.ClassOrd},
	"chi": {"χ", ast.ClassOrd}, "psi": {"ψ", ast.ClassOrd}, "omega": {"ω", ast.ClassOrd}, "digamma": {"ϝ", ast.ClassOrd},
	// Uppercase Greek (upright, as in TeX).
	"Gamma": {"Γ", ast.ClassOrd}, "Delta": {"Δ", ast.ClassOrd}, "Theta": {"Θ", ast.ClassOrd}, "Lambda": {"Λ", ast.ClassOrd},
	"Xi": {"Ξ", ast.ClassOrd}, "Pi": {"Π", ast.ClassOrd}, "Sigma": {"Σ", ast.ClassOrd}, "Upsilon": {"Υ", ast.ClassOrd},
	"Phi": {"Φ", ast.ClassOrd}, "Psi": {"Ψ", ast.ClassOrd}, "Omega": {"Ω", ast.ClassOrd},
	"varGamma": {"𝛤", ast.ClassOrd}, "varDelta": {"𝛥", ast.ClassOrd}, "varTheta": {"𝛩", ast.ClassOrd}, "varLambda": {"𝛬", ast.ClassOrd},
	"varXi": {"𝛯", ast.ClassOrd}, "varPi": {"𝛱", ast.ClassOrd}, "varSigma": {"𝛴", ast.ClassOrd}, "varUpsilon": {"𝛶", ast.ClassOrd},
	"varPhi": {"𝛷", ast.ClassOrd}, "varPsi": {"𝛹", ast.ClassOrd}, "varOmega": {"𝛺", ast.ClassOrd},

	// Ordinary symbols.
	"infty": {"∞", ast.ClassOrd}, "partial": {"∂", ast.ClassOrd}, "nabla": {"∇", ast.ClassOrd}, "forall": {"∀", ast.ClassOrd},
	"exists": {"∃", ast.ClassOrd}, "nexists": {"∄", ast.ClassOrd}, "emptyset": {"∅", ast.ClassOrd}, "varnothing": {"∅", ast.ClassOrd},
	"hbar": {"ℏ", ast.ClassOrd}, "hslash": {"ℏ", ast.ClassOrd}, "ell": {"ℓ", ast.ClassOrd}, "Re": {"ℜ", ast.ClassOrd},
	"Im": {"ℑ", ast.ClassOrd}, "aleph": {"ℵ", ast.ClassOrd}, "beth": {"ℶ", ast.ClassOrd}, "wp": {"℘", ast.ClassOrd},
	"prime": {"′", ast.ClassOrd}, "angle": {"∠", ast.ClassOrd}, "triangle": {"△", ast.ClassOrd}, "neg": {"¬", ast.ClassOrd},
	"lnot": {"¬", ast.ClassOrd}, "top": {"⊤", ast.ClassOrd}, "bot": {"⊥", ast.ClassOrd}, "imath": {"ı", ast.ClassOrd},
	"jmath": {"ȷ", ast.ClassOrd}, "degree": {"°", ast.ClassOrd}, "dagger": {"†", ast.ClassBin}, "ddagger": {"‡", ast.ClassBin},
	"ldots": {"…", ast.ClassInner}, "dots": {"…", ast.ClassInner}, "cdots": {"⋯", ast.ClassInner}, "vdots": {"⋮", ast.ClassOrd},
	"ddots": {"⋱", ast.ClassInner}, "dotsc": {"…", ast.ClassInner}, "dotsb": {"⋯", ast.ClassInner}, "dotsm": {"⋯", ast.ClassInner},
	"dotsi": {"⋯", ast.ClassInner}, "dotso": {"…", ast.ClassInner},
	"clubsuit": {"♣", ast.ClassOrd}, "diamondsuit": {"♢", ast.ClassOrd}, "heartsuit": {"♡", ast.ClassOrd}, "spadesuit": {"♠", ast.ClassOrd},
	"flat": {"♭", ast.ClassOrd}, "natural": {"♮", ast.ClassOrd}, "sharp": {"♯", ast.ClassOrd}, "surd": {"√", ast.ClassOrd},
	"complement": {"∁", ast.ClassOrd}, "Box": {"□", ast.ClassOrd}, "square": {"□", ast.ClassOrd}, "blacksquare": {"■", ast.ClassOrd},
	"Diamond": {"◊", ast.ClassOrd}, "lozenge": {"◊", ast.ClassOrd}, "checkmark": {"✓", ast.ClassOrd}, "S": {"§", ast.ClassOrd},
	"P": {"¶", ast.ClassOrd}, "dag": {"†", ast.ClassOrd}, "ddag": {"‡", ast.ClassOrd}, "infin": {"∞", ast.ClassOrd},
	"backprime": {"‵", ast.ClassOrd}, "mho": {"℧", ast.ClassOrd}, "eth": {"ð", ast.ClassOrd}, "Finv": {"Ⅎ", ast.ClassOrd},
	"Game": {"⅁", ast.ClassOrd}, "vartriangle": {"△", ast.ClassRel},
	"colon": {":", ast.ClassPunct}, "lbrack": {"[", ast.ClassOpen}, "rbrack": {"]", ast.ClassClose},

	// Additions for KaTeX's table (docs/plans/math-completion.md, phase 1).
	"natnums": {"ℕ", ast.ClassOrd}, "integers": {"ℤ", ast.ClassOrd}, "reals": {"ℝ", ast.ClassOrd},
	"Complex": {"ℂ", ast.ClassOrd}, "lt": {"<", ast.ClassRel}, "gt": {">", ast.ClassRel},
	"cdotp": {"⋅", ast.ClassPunct}, "ldotp": {".", ast.ClassPunct}, "And": {"&", ast.ClassBin},
	"notni": {"∌", ast.ClassRel}, "mapsfrom": {"↤", ast.ClassRel},
	"Alpha": {"A", ast.ClassOrd}, "Beta": {"B", ast.ClassOrd}, "Epsilon": {"E", ast.ClassOrd},
	"Zeta": {"Z", ast.ClassOrd}, "Eta": {"H", ast.ClassOrd}, "Iota": {"I", ast.ClassOrd},
	"Kappa": {"K", ast.ClassOrd}, "Mu": {"M", ast.ClassOrd}, "Nu": {"N", ast.ClassOrd},
	"Omicron": {"O", ast.ClassOrd}, "Rho": {"P", ast.ClassOrd}, "Tau": {"T", ast.ClassOrd},
	"Chi":     {"X", ast.ClassOrd},
	"bigstar": {"★", ast.ClassOrd}, "blacktriangle": {"▴", ast.ClassOrd}, "blacktriangledown": {"▾", ast.ClassOrd},
	"blacktriangleleft": {"◀", ast.ClassRel}, "blacktriangleright": {"▶", ast.ClassRel}, "blacklozenge": {"⧫", ast.ClassOrd},
	"maltese": {"✠", ast.ClassOrd}, "measuredangle": {"∡", ast.ClassOrd}, "sphericalangle": {"∢", ast.ClassOrd},
	"diagup": {"╱", ast.ClassOrd}, "diagdown": {"╲", ast.ClassOrd}, "minuso": {"⦵", ast.ClassOrd},
	"circledR": {"®", ast.ClassOrd}, "circledS": {"Ⓢ", ast.ClassOrd}, "copyright": {"©", ast.ClassOrd},
	"pounds": {"£", ast.ClassOrd}, "mathsterling": {"£", ast.ClassOrd}, "yen": {"¥", ast.ClassOrd},
	"Bbbk": {"𝕜", ast.ClassOrd}, "daleth": {"ℸ", ast.ClassOrd}, "gimel": {"ℷ", ast.ClassOrd},
	"imageof": {"⊷", ast.ClassRel}, "origof": {"⊶", ast.ClassRel}, "ratio": {"∶", ast.ClassRel},
	"triangledown": {"▿", ast.ClassOrd}, "mathellipsis": {"…", ast.ClassInner},
	"leqq": {"≦", ast.ClassRel}, "geqq": {"≧", ast.ClassRel}, "lessapprox": {"⪅", ast.ClassRel},
	"gtrapprox": {"⪆", ast.ClassRel}, "lessdot": {"⋖", ast.ClassBin}, "gtrdot": {"⋗", ast.ClassBin},
	"lesseqgtr": {"⋚", ast.ClassRel}, "gtreqless": {"⋛", ast.ClassRel}, "lesseqqgtr": {"⪋", ast.ClassRel},
	"gtreqqless": {"⪌", ast.ClassRel}, "llless": {"⋘", ast.ClassRel}, "gggtr": {"⋙", ast.ClassRel},
	"curlyeqprec": {"⋞", ast.ClassRel}, "curlyeqsucc": {"⋟", ast.ClassRel}, "eqslantless": {"⪕", ast.ClassRel},
	"eqslantgtr": {"⪖", ast.ClassRel}, "eqcirc": {"≖", ast.ClassRel}, "backepsilon": {"∍", ast.ClassRel},
	"smallfrown": {"⌢", ast.ClassRel}, "smallsmile": {"⌣", ast.ClassRel}, "preccurlyeq": {"≼", ast.ClassRel},
	"succcurlyeq": {"≽", ast.ClassRel}, "precapprox": {"⪷", ast.ClassRel}, "succapprox": {"⪸", ast.ClassRel},
	"precnapprox": {"⪹", ast.ClassRel}, "succnapprox": {"⪺", ast.ClassRel}, "precneqq": {"⪵", ast.ClassRel},
	"succneqq": {"⪶", ast.ClassRel}, "precnsim": {"⋨", ast.ClassRel}, "succnsim": {"⋩", ast.ClassRel},
	"lnapprox": {"⪉", ast.ClassRel}, "gnapprox": {"⪊", ast.ClassRel}, "lneqq": {"≨", ast.ClassRel},
	"gneqq": {"≩", ast.ClassRel}, "lnsim": {"⋦", ast.ClassRel}, "gnsim": {"⋧", ast.ClassRel},
	"npreceq": {"⋠", ast.ClassRel}, "nsucceq": {"⋡", ast.ClassRel},
	"ntriangleleft": {"⋪", ast.ClassRel}, "ntriangleright": {"⋫", ast.ClassRel},
	"ntrianglelefteq": {"⋬", ast.ClassRel}, "ntrianglerighteq": {"⋭", ast.ClassRel},
	"subseteqq": {"⫅", ast.ClassRel}, "supseteqq": {"⫆", ast.ClassRel}, "subsetneqq": {"⫋", ast.ClassRel},
	"supsetneqq": {"⫌", ast.ClassRel}, "doublebarwedge": {"⩞", ast.ClassBin}, "doublecap": {"⋒", ast.ClassBin},
	"doublecup": {"⋓", ast.ClassBin}, "leftthreetimes": {"⋋", ast.ClassBin}, "rightthreetimes": {"⋌", ast.ClassBin},
	"Vvdash": {"⊪", ast.ClassRel}, "dashleftarrow": {"⇠", ast.ClassRel}, "dashrightarrow": {"⇢", ast.ClassRel},
	"leftrightsquigarrow": {"↭", ast.ClassRel}, "dblcolon": {"∷", ast.ClassRel},
	// Colon combinations (mathtools): a vertically centered colon (∶)
	// next to a relation, set as one relation.
	"Coloneq": {"∷−", ast.ClassRel}, "coloneq": {"∶−", ast.ClassRel}, "eqcolon": {"−∶", ast.ClassRel},
	"Eqcolon": {"−∷", ast.ClassRel}, "Eqqcolon": {"=∷", ast.ClassRel}, "colonsim": {"∶∼", ast.ClassRel},
	"Colonsim": {"∷∼", ast.ClassRel}, "colonapprox": {"∶≈", ast.ClassRel}, "Colonapprox": {"∷≈", ast.ClassRel},
	"colonminus": {"∶−", ast.ClassRel}, "coloncolonminus": {"∷−", ast.ClassRel}, "minuscolon": {"−∶", ast.ClassRel},
	"minuscoloncolon": {"−∷", ast.ClassRel}, "approxcolon": {"≈∶", ast.ClassRel}, "approxcoloncolon": {"≈∷", ast.ClassRel},
	"coloncolonapprox": {"∷≈", ast.ClassRel}, "coloncolonsim": {"∷∼", ast.ClassRel},
	"lq": {"‘", ast.ClassOrd}, "rq": {"’", ast.ClassOrd},
	"simcolon": {"∼∶", ast.ClassRel}, "simcoloncolon": {"∼∷", ast.ClassRel}, "equalscoloncolon": {"=∷", ast.ClassRel},
	"_": {"_", ast.ClassOrd}, "&": {"&", ast.ClassOrd}, "#": {"#", ast.ClassOrd}, "%": {"%", ast.ClassOrd}, "$": {"$", ast.ClassOrd},

	// Binary operators.
	"pm": {"±", ast.ClassBin}, "mp": {"∓", ast.ClassBin}, "times": {"×", ast.ClassBin}, "div": {"÷", ast.ClassBin},
	"cdot": {"⋅", ast.ClassBin}, "ast": {"∗", ast.ClassBin}, "star": {"⋆", ast.ClassBin}, "circ": {"∘", ast.ClassBin},
	"bullet": {"∙", ast.ClassBin}, "cap": {"∩", ast.ClassBin}, "cup": {"∪", ast.ClassBin}, "uplus": {"⊎", ast.ClassBin},
	"sqcap": {"⊓", ast.ClassBin}, "sqcup": {"⊔", ast.ClassBin}, "vee": {"∨", ast.ClassBin}, "lor": {"∨", ast.ClassBin},
	"wedge": {"∧", ast.ClassBin}, "land": {"∧", ast.ClassBin}, "setminus": {"∖", ast.ClassBin}, "smallsetminus": {"∖", ast.ClassBin},
	"wr": {"≀", ast.ClassBin}, "diamond": {"⋄", ast.ClassBin}, "bigtriangleup": {"△", ast.ClassBin}, "bigtriangledown": {"▽", ast.ClassBin},
	"triangleleft": {"◃", ast.ClassBin}, "triangleright": {"▹", ast.ClassBin}, "oplus": {"⊕", ast.ClassBin}, "ominus": {"⊖", ast.ClassBin},
	"otimes": {"⊗", ast.ClassBin}, "oslash": {"⊘", ast.ClassBin}, "odot": {"⊙", ast.ClassBin}, "bigcirc": {"◯", ast.ClassBin},
	"amalg": {"⨿", ast.ClassBin}, "centerdot": {"⋅", ast.ClassBin}, "dotplus": {"∔", ast.ClassBin}, "ltimes": {"⋉", ast.ClassBin},
	"rtimes": {"⋊", ast.ClassBin}, "boxplus": {"⊞", ast.ClassBin}, "boxminus": {"⊟", ast.ClassBin}, "boxtimes": {"⊠", ast.ClassBin},
	"boxdot": {"⊡", ast.ClassBin}, "intercal": {"⊺", ast.ClassBin}, "barwedge": {"⊼", ast.ClassBin}, "veebar": {"⊻", ast.ClassBin},
	"curlywedge": {"⋏", ast.ClassBin}, "curlyvee": {"⋎", ast.ClassBin}, "Cap": {"⋒", ast.ClassBin}, "Cup": {"⋓", ast.ClassBin},
	"divideontimes": {"⋇", ast.ClassBin}, "circledast": {"⊛", ast.ClassBin}, "circledcirc": {"⊚", ast.ClassBin},
	"circleddash": {"⊝", ast.ClassBin},

	// Relations.
	"leq": {"≤", ast.ClassRel}, "le": {"≤", ast.ClassRel}, "geq": {"≥", ast.ClassRel}, "ge": {"≥", ast.ClassRel},
	"neq": {"≠", ast.ClassRel}, "ne": {"≠", ast.ClassRel}, "approx": {"≈", ast.ClassRel}, "equiv": {"≡", ast.ClassRel},
	"sim": {"∼", ast.ClassRel}, "simeq": {"≃", ast.ClassRel}, "cong": {"≅", ast.ClassRel}, "propto": {"∝", ast.ClassRel},
	"ll": {"≪", ast.ClassRel}, "gg": {"≫", ast.ClassRel}, "lll": {"⋘", ast.ClassRel}, "ggg": {"⋙", ast.ClassRel},
	"in": {"∈", ast.ClassRel}, "notin": {"∉", ast.ClassRel}, "ni": {"∋", ast.ClassRel}, "owns": {"∋", ast.ClassRel},
	"subset": {"⊂", ast.ClassRel}, "supset": {"⊃", ast.ClassRel}, "subseteq": {"⊆", ast.ClassRel}, "supseteq": {"⊇", ast.ClassRel},
	"subsetneq": {"⊊", ast.ClassRel}, "supsetneq": {"⊋", ast.ClassRel}, "nsubseteq": {"⊈", ast.ClassRel}, "nsupseteq": {"⊉", ast.ClassRel},
	"sqsubset": {"⊏", ast.ClassRel}, "sqsupset": {"⊐", ast.ClassRel}, "sqsubseteq": {"⊑", ast.ClassRel}, "sqsupseteq": {"⊒", ast.ClassRel},
	"prec": {"≺", ast.ClassRel}, "succ": {"≻", ast.ClassRel}, "preceq": {"⪯", ast.ClassRel}, "succeq": {"⪰", ast.ClassRel},
	"mid": {"∣", ast.ClassRel}, "nmid": {"∤", ast.ClassRel}, "parallel": {"∥", ast.ClassRel}, "nparallel": {"∦", ast.ClassRel},
	"perp": {"⊥", ast.ClassRel}, "vdash": {"⊢", ast.ClassRel}, "dashv": {"⊣", ast.ClassRel}, "models": {"⊨", ast.ClassRel},
	"vDash": {"⊨", ast.ClassRel}, "Vdash": {"⊩", ast.ClassRel}, "smile": {"⌣", ast.ClassRel}, "frown": {"⌢", ast.ClassRel},
	"asymp": {"≍", ast.ClassRel}, "doteq": {"≐", ast.ClassRel}, "bowtie": {"⋈", ast.ClassRel}, "Join": {"⋈", ast.ClassRel},
	"lesssim": {"≲", ast.ClassRel}, "gtrsim": {"≳", ast.ClassRel}, "leqslant": {"⩽", ast.ClassRel}, "geqslant": {"⩾", ast.ClassRel},
	"lessgtr": {"≶", ast.ClassRel}, "gtrless": {"≷", ast.ClassRel}, "nless": {"≮", ast.ClassRel}, "ngtr": {"≯", ast.ClassRel},
	"nleq": {"≰", ast.ClassRel}, "ngeq": {"≱", ast.ClassRel}, "nsim": {"≁", ast.ClassRel}, "ncong": {"≇", ast.ClassRel},
	"approxeq": {"≊", ast.ClassRel}, "thicksim": {"∼", ast.ClassRel}, "thickapprox": {"≈", ast.ClassRel}, "triangleq": {"≜", ast.ClassRel},
	"coloneqq": {"≔", ast.ClassRel}, "eqqcolon": {"≕", ast.ClassRel}, "Coloneqq": {"⩴", ast.ClassRel},
	"therefore": {"∴", ast.ClassRel}, "because": {"∵", ast.ClassRel}, "lhd": {"⊲", ast.ClassBin}, "rhd": {"⊳", ast.ClassBin},
	"unlhd": {"⊴", ast.ClassBin}, "unrhd": {"⊵", ast.ClassBin}, "trianglelefteq": {"⊴", ast.ClassRel}, "trianglerighteq": {"⊵", ast.ClassRel},
	"vartriangleleft": {"⊲", ast.ClassRel}, "vartriangleright": {"⊳", ast.ClassRel}, "lneq": {"⪇", ast.ClassRel}, "gneq": {"⪈", ast.ClassRel},
	"precsim": {"≾", ast.ClassRel}, "succsim": {"≿", ast.ClassRel}, "nprec": {"⊀", ast.ClassRel}, "nsucc": {"⊁", ast.ClassRel},
	"circeq": {"≗", ast.ClassRel}, "bumpeq": {"≏", ast.ClassRel}, "Bumpeq": {"≎", ast.ClassRel}, "risingdotseq": {"≓", ast.ClassRel},
	"fallingdotseq": {"≒", ast.ClassRel}, "doteqdot": {"≑", ast.ClassRel}, "Doteq": {"≑", ast.ClassRel}, "backsim": {"∽", ast.ClassRel},
	"backsimeq": {"⋍", ast.ClassRel}, "pitchfork": {"⋔", ast.ClassRel}, "between": {"≬", ast.ClassRel}, "Subset": {"⋐", ast.ClassRel},
	"Supset": {"⋑", ast.ClassRel}, "shortmid": {"∣", ast.ClassRel}, "shortparallel": {"∥", ast.ClassRel}, "nshortmid": {"∤", ast.ClassRel},
	"varpropto": {"∝", ast.ClassRel}, "sqsubsetneq": {"⋤", ast.ClassRel}, "eqsim": {"≂", ast.ClassRel}, "nvdash": {"⊬", ast.ClassRel},
	"nvDash": {"⊭", ast.ClassRel}, "nVdash": {"⊮", ast.ClassRel}, "nVDash": {"⊯", ast.ClassRel},

	// Arrows (relations).
	"to": {"→", ast.ClassRel}, "rightarrow": {"→", ast.ClassRel}, "gets": {"←", ast.ClassRel}, "leftarrow": {"←", ast.ClassRel},
	"leftrightarrow": {"↔", ast.ClassRel}, "Rightarrow": {"⇒", ast.ClassRel}, "Leftarrow": {"⇐", ast.ClassRel},
	"Leftrightarrow": {"⇔", ast.ClassRel}, "iff": {"⟺", ast.ClassRel}, "implies": {"⟹", ast.ClassRel}, "impliedby": {"⟸", ast.ClassRel},
	"mapsto": {"↦", ast.ClassRel}, "longmapsto": {"⟼", ast.ClassRel}, "longrightarrow": {"⟶", ast.ClassRel},
	"longleftarrow": {"⟵", ast.ClassRel}, "longleftrightarrow": {"⟷", ast.ClassRel}, "Longrightarrow": {"⟹", ast.ClassRel},
	"Longleftarrow": {"⟸", ast.ClassRel}, "Longleftrightarrow": {"⟺", ast.ClassRel}, "uparrow": {"↑", ast.ClassRel},
	"downarrow": {"↓", ast.ClassRel}, "updownarrow": {"↕", ast.ClassRel}, "Uparrow": {"⇑", ast.ClassRel}, "Downarrow": {"⇓", ast.ClassRel},
	"Updownarrow": {"⇕", ast.ClassRel}, "nearrow": {"↗", ast.ClassRel}, "searrow": {"↘", ast.ClassRel}, "swarrow": {"↙", ast.ClassRel},
	"nwarrow": {"↖", ast.ClassRel}, "hookrightarrow": {"↪", ast.ClassRel}, "hookleftarrow": {"↩", ast.ClassRel},
	"rightharpoonup": {"⇀", ast.ClassRel}, "rightharpoondown": {"⇁", ast.ClassRel}, "leftharpoonup": {"↼", ast.ClassRel},
	"leftharpoondown": {"↽", ast.ClassRel}, "rightleftharpoons": {"⇌", ast.ClassRel}, "leftrightharpoons": {"⇋", ast.ClassRel},
	"leadsto": {"⇝", ast.ClassRel}, "rightsquigarrow": {"⇝", ast.ClassRel}, "twoheadrightarrow": {"↠", ast.ClassRel},
	"twoheadleftarrow": {"↞", ast.ClassRel}, "rightarrowtail": {"↣", ast.ClassRel}, "leftarrowtail": {"↢", ast.ClassRel},
	"rightrightarrows": {"⇉", ast.ClassRel}, "leftleftarrows": {"⇇", ast.ClassRel}, "rightleftarrows": {"⇄", ast.ClassRel},
	"leftrightarrows": {"⇆", ast.ClassRel}, "curvearrowright": {"↷", ast.ClassRel}, "curvearrowleft": {"↶", ast.ClassRel},
	"circlearrowright": {"↻", ast.ClassRel}, "circlearrowleft": {"↺", ast.ClassRel}, "nrightarrow": {"↛", ast.ClassRel},
	"nleftarrow": {"↚", ast.ClassRel}, "nRightarrow": {"⇏", ast.ClassRel}, "nLeftarrow": {"⇍", ast.ClassRel},
	"nleftrightarrow": {"↮", ast.ClassRel}, "nLeftrightarrow": {"⇎", ast.ClassRel}, "Rrightarrow": {"⇛", ast.ClassRel},
	"Lleftarrow": {"⇚", ast.ClassRel}, "upuparrows": {"⇈", ast.ClassRel}, "downdownarrows": {"⇊", ast.ClassRel},
	"restriction": {"↾", ast.ClassRel}, "upharpoonright": {"↾", ast.ClassRel}, "upharpoonleft": {"↿", ast.ClassRel},
	"downharpoonright": {"⇂", ast.ClassRel}, "downharpoonleft": {"⇃", ast.ClassRel}, "multimap": {"⊸", ast.ClassRel},
	"Lsh": {"↰", ast.ClassRel}, "Rsh": {"↱", ast.ClassRel}, "looparrowright": {"↬", ast.ClassRel}, "looparrowleft": {"↫", ast.ClassRel},
}

// Aliases maps alternative command names (KaTeX's) to the command they
// stand for. The parser resolves them before any other lookup.
var Aliases = map[string]string{
	"N": "natnums", "Z": "integers", "R": "reals", "Reals": "reals", "cnums": "Complex",
	"real": "Re", "image": "Im", "weierp": "wp", "alef": "aleph", "alefsym": "aleph",
	"thetasym": "vartheta", "exist": "exists", "isin": "in", "empty": "emptyset",
	"plusmn": "pm", "sdot": "cdot", "bull": "bullet", "Dagger": "ddagger",
	"sub": "subset", "sube": "subseteq", "supe": "supseteq",
	"clubs": "clubsuit", "diamonds": "diamondsuit", "hearts": "heartsuit", "spades": "spadesuit",
	"lparen": "(", "rparen": ")", "lang": "langle", "rang": "rangle",
	"rarr": "rightarrow", "Rarr": "Rightarrow", "rArr": "Rightarrow",
	"larr": "leftarrow", "Larr": "Leftarrow", "lArr": "Leftarrow",
	"lrarr": "leftrightarrow", "Lrarr": "Leftrightarrow", "lrArr": "Leftrightarrow",
	"harr": "leftrightarrow", "Harr": "Leftrightarrow", "hArr": "Leftrightarrow",
	"uarr": "uparrow", "Uarr": "Uparrow", "uArr": "Uparrow",
	"darr": "downarrow", "Darr": "Downarrow", "dArr": "Downarrow",
	"colonequals": "coloneqq", "coloncolonequals": "Coloneqq", "equalscolon": "eqqcolon",
	"coloncolon": "dblcolon", "vcentcolon": "ratio", "varsubsetneq": "subsetneq",
	"varsupsetneq": "supsetneq", "varsubsetneqq": "subsetneqq", "varsupsetneqq": "supsetneqq",
	"lvertneqq": "lneqq", "gvertneqq": "gneqq", "ngeqq": "ngeq", "nleqq": "nleq",
	"ngeqslant": "ngeq", "nleqslant": "nleq", "nshortparallel": "nparallel",
	"nsubseteqq": "nsubseteq", "nsupseteqq": "nsupseteq",
}

// Chars gives the class of plain characters; unlisted characters are Ord.
// The minus and asterisk are mapped to their math forms.
var Chars = map[rune]Sym{
	'+': {"+", ast.ClassBin}, '-': {"−", ast.ClassBin}, '*': {"∗", ast.ClassBin},
	'=': {"=", ast.ClassRel}, '<': {"<", ast.ClassRel}, '>': {">", ast.ClassRel}, ':': {":", ast.ClassRel},
	',': {",", ast.ClassPunct}, ';': {";", ast.ClassPunct},
	'(': {"(", ast.ClassOpen}, '[': {"[", ast.ClassOpen}, ')': {")", ast.ClassClose}, ']': {"]", ast.ClassClose},
	'!': {"!", ast.ClassClose}, '?': {"?", ast.ClassClose},
	'|': {"|", ast.ClassOrd}, '/': {"/", ast.ClassOrd}, '.': {".", ast.ClassOrd}, '@': {"@", ast.ClassOrd},
	'"': {"″", ast.ClassOrd}, '`': {"‘", ast.ClassOrd},
	'·': {"⋅", ast.ClassBin}, '×': {"×", ast.ClassBin}, '÷': {"÷", ast.ClassBin}, '±': {"±", ast.ClassBin},
	'≤': {"≤", ast.ClassRel}, '≥': {"≥", ast.ClassRel}, '≠': {"≠", ast.ClassRel}, '→': {"→", ast.ClassRel},
	'∈': {"∈", ast.ClassRel}, '≈': {"≈", ast.ClassRel}, '≡': {"≡", ast.ClassRel}, '←': {"←", ast.ClassRel},
	'⇒': {"⇒", ast.ClassRel}, '⇔': {"⇔", ast.ClassRel}, '−': {"−", ast.ClassBin},
}

// OpInfo describes a big operator or named function.
type OpInfo struct {
	Text          string
	Symbol        bool
	DefaultLimits bool
}

// Ops maps operator commands to their properties.
var Ops = map[string]OpInfo{
	"sum": {"∑", true, true}, "prod": {"∏", true, true}, "coprod": {"∐", true, true},
	"bigcup": {"⋃", true, true}, "bigcap": {"⋂", true, true}, "bigvee": {"⋁", true, true},
	"bigwedge": {"⋀", true, true}, "bigoplus": {"⨁", true, true}, "bigotimes": {"⨂", true, true},
	"bigodot": {"⨀", true, true}, "biguplus": {"⨄", true, true}, "bigsqcup": {"⨆", true, true},
	"int": {"∫", true, false}, "iint": {"∬", true, false}, "iiint": {"∭", true, false},
	"oint": {"∮", true, false}, "oiint": {"∯", true, false}, "oiiint": {"∰", true, false},
	"intop": {"∫", true, true}, "smallint": {"∫", false, false},

	"arcsin": {"arcsin", false, false}, "arccos": {"arccos", false, false}, "arctan": {"arctan", false, false},
	"arg": {"arg", false, false}, "cos": {"cos", false, false}, "cosh": {"cosh", false, false},
	"cot": {"cot", false, false}, "coth": {"coth", false, false}, "csc": {"csc", false, false},
	"deg": {"deg", false, false}, "dim": {"dim", false, false}, "exp": {"exp", false, false},
	"hom": {"hom", false, false}, "ker": {"ker", false, false}, "lg": {"lg", false, false},
	"ln": {"ln", false, false}, "log": {"log", false, false}, "sec": {"sec", false, false},
	"sin": {"sin", false, false}, "sinh": {"sinh", false, false}, "tan": {"tan", false, false},
	"tanh": {"tanh", false, false}, "arcctg": {"arcctg", false, false}, "arctg": {"arctg", false, false},
	"ch": {"ch", false, false}, "cosec": {"cosec", false, false}, "cotg": {"cotg", false, false},
	"ctg": {"ctg", false, false}, "cth": {"cth", false, false}, "sh": {"sh", false, false},
	"tg": {"tg", false, false}, "th": {"th", false, false},
	"det": {"det", false, true}, "gcd": {"gcd", false, true}, "inf": {"inf", false, true},
	"lim": {"lim", false, true}, "liminf": {"lim inf", false, true}, "limsup": {"lim sup", false, true},
	"max": {"max", false, true}, "min": {"min", false, true}, "Pr": {"Pr", false, true},
	"sup": {"sup", false, true}, "argmax": {"arg max", false, true}, "argmin": {"arg min", false, true},
	"injlim": {"inj lim", false, true}, "projlim": {"proj lim", false, true},
}

// Delims maps delimiter tokens (a plain character or a command name with the
// backslash) to the delimiter character.
var Delims = map[string]string{
	"(": "(", ")": ")", "[": "[", "]": "]", "|": "|", "/": "/", ".": "",
	"<": "⟨", ">": "⟩",
	`\{`: "{", `\}`: "}", `\lbrace`: "{", `\rbrace`: "}", `\lbrack`: "[", `\rbrack`: "]",
	`\|`: "‖", `\vert`: "|", `\Vert`: "‖", `\lvert`: "|", `\rvert`: "|", `\lVert`: "‖", `\rVert`: "‖",
	`\langle`: "⟨", `\rangle`: "⟩", `\lfloor`: "⌊", `\rfloor`: "⌋", `\lceil`: "⌈", `\rceil`: "⌉",
	`\backslash`: "∖", `\uparrow`: "↑", `\downarrow`: "↓", `\updownarrow`: "↕", `\Uparrow`: "⇑",
	`\Downarrow`: "⇓", `\Updownarrow`: "⇕", `\lgroup`: "⟮", `\rgroup`: "⟯", `\lmoustache`: "⎰",
	`\rmoustache`: "⎱", `\llbracket`: "⟦", `\rrbracket`: "⟧", `\ulcorner`: "⌜", `\urcorner`: "⌝",
	`\llcorner`: "⌞", `\lrcorner`: "⌟", `\lBrace`: "⦃", `\rBrace`: "⦄",
	"⟨": "⟨", "⟩": "⟩", "‖": "‖", "⌊": "⌊", "⌋": "⌋", "⌈": "⌈", "⌉": "⌉",
}

// DelimClass returns the atom class of a delimiter used on its own.
func DelimClass(d string) ast.Class {
	switch d {
	case "(", "[", "{", "⟨", "⌊", "⌈", "⟮", "⟦", "⌜", "⌞", "⦃":
		return ast.ClassOpen
	case ")", "]", "}", "⟩", "⌋", "⌉", "⟯", "⟧", "⌝", "⌟", "⦄":
		return ast.ClassClose
	}
	return ast.ClassOrd
}

// AccentInfo describes an accent command.
type AccentInfo struct {
	Char  rune
	Wide  bool
	Under bool // set below the base
}

// Accents maps accent commands to their combining characters.
var Accents = map[string]AccentInfo{
	"hat": {0x302, false, false}, "widehat": {0x302, true, false}, "check": {0x30C, false, false}, "widecheck": {0x30C, true, false},
	"tilde": {0x303, false, false}, "widetilde": {0x303, true, false}, "acute": {0x301, false, false}, "grave": {0x300, false, false},
	"dot": {0x307, false, false}, "ddot": {0x308, false, false}, "dddot": {0x20DB, false, false}, "breve": {0x306, false, false},
	"bar": {0x304, false, false}, "vec": {0x20D7, false, false}, "mathring": {0x30A, false, false},
	"overrightarrow": {0x20D7, true, false}, "overleftarrow": {0x20D6, true, false}, "overleftrightarrow": {0x20E1, true, false},
	"ddddot": {0x20DC, false, false}, "overgroup": {0x23E0, true, false}, "Overrightarrow": {0x21D2, true, false},
	"overleftharpoon": {0x20D0, true, false}, "overrightharpoon": {0x20D1, true, false},
	"overlinesegment": {0x20E9, true, false},
	"undergroup":      {0x23E1, true, true}, "utilde": {0x0330, true, true}, "underrightarrow": {0x20EF, true, true},
	"underleftarrow": {0x20EE, true, true}, "underleftrightarrow": {0x034D, true, true},
}

// AccentText is the spacing (non-combining) form of an accent for text
// outputs.
var AccentText = map[rune]string{
	0x302: "^", 0x30C: "ˇ", 0x303: "~", 0x301: "´", 0x300: "`", 0x307: "˙", 0x308: "¨", 0x20DB: "⋯",
	0x306: "˘", 0x304: "‾", 0x20D7: "→", 0x30A: "˚", 0x20D6: "←", 0x20E1: "↔",
	0x20DC: "⋯", 0x23E0: "⏠", 0x21D2: "⇒", 0x20D0: "↼", 0x20D1: "⇀", 0x20E9: "‾",
	0x23E1: "⏡", 0x0330: "~", 0x20EF: "→", 0x20EE: "←", 0x034D: "↔",
}

// Fonts maps font commands to math alphabets.
var Fonts = map[string]ast.FontStyle{
	"mathrm": ast.FontRoman, "mathbf": ast.FontBold, "mathit": ast.FontItalic, "mathsf": ast.FontSans,
	"mathtt": ast.FontMono, "mathbb": ast.FontBlackboard, "mathcal": ast.FontCalligraphic,
	"mathscr": ast.FontCalligraphic, "mathfrak": ast.FontFraktur, "boldsymbol": ast.FontBoldItalic,
	"bm": ast.FontBoldItalic, "mathnormal": ast.FontDefault, "Bbb": ast.FontBlackboard, "bold": ast.FontBold,
	"frak": ast.FontFraktur,
}

// FontSwitches are the old-style font commands that apply to the rest of
// the group ({\rm d}x).
var FontSwitches = map[string]ast.FontStyle{
	"rm": ast.FontRoman, "bf": ast.FontBold, "it": ast.FontItalic, "sf": ast.FontSans, "tt": ast.FontMono,
	"cal": ast.FontCalligraphic,
}

// TextFonts maps text commands to styles.
var TextFonts = map[string]ast.FontStyle{
	"text": ast.FontRoman, "textrm": ast.FontRoman, "textnormal": ast.FontRoman, "textup": ast.FontRoman,
	"textbf": ast.FontBold, "textit": ast.FontItalic, "textsf": ast.FontSans, "texttt": ast.FontMono,
	"mbox": ast.FontRoman, "hbox": ast.FontRoman, "textmd": ast.FontRoman, "emph": ast.FontItalic,
}

// Sizes maps the size commands to their scale factor (KaTeX's sizes
// relative to \normalsize).
var Sizes = map[string]float64{
	"tiny": 0.5, "sixptsize": 0.6, "scriptsize": 0.7, "footnotesize": 0.8, "small": 0.9,
	"normalsize": 1, "large": 1.2, "Large": 1.44, "LARGE": 1.728, "huge": 2.074, "Huge": 2.488,
}

// Spaces maps spacing commands to their width in mu.
var Spaces = map[string]float64{
	",": 3, "thinspace": 3, ":": 4, ">": 4, "medspace": 4, ";": 5, "thickspace": 5,
	"!": -3, "negthinspace": -3, "negmedspace": -4, "negthickspace": -5,
	"quad": 18, "qquad": 36, " ": 6, "enspace": 9, "space": 6, "nobreakspace": 6,
}

// XArrows maps extensible arrow commands to their arrow character.
var XArrows = map[string]rune{
	"xrightarrow": '→', "xleftarrow": '←', "xleftrightarrow": '↔', "xRightarrow": '⇒',
	"xLeftarrow": '⇐', "xLeftrightarrow": '⇔', "xmapsto": '↦', "xhookrightarrow": '↪',
	"xhookleftarrow": '↩', "xtwoheadrightarrow": '↠', "xtwoheadleftarrow": '↞',
	"xrightharpoonup": '⇀', "xrightharpoondown": '⇁', "xleftharpoonup": '↼', "xleftharpoondown": '↽',
	"xrightleftharpoons": '⇌', "xlongequal": '=', "xleftrightharpoons": '⇋', "xtofrom": '⇄',
}

// ImageSubstitutes gives characters that look the same as a character the
// math font lacks, for images. Characters with neither a glyph nor a
// substitute need the caller's text face.
var ImageSubstitutes = map[rune]string{
	'◃': "◁", '▹': "▷", '⩴': "∷=", '▴': "▲", '▾': "▼", '▿': "▽", '╱': "∕", '╲': "∖",
}

// Negations maps the symbol following \not to its negated form.
var Negations = map[string]string{
	"=": "≠", "<": "≮", ">": "≯", "≤": "≰", "≥": "≱", "∈": "∉", "∋": "∌", "⊂": "⊄", "⊃": "⊅",
	"⊆": "⊈", "⊇": "⊉", "∼": "≁", "≃": "≄", "≅": "≇", "≈": "≉", "≡": "≢", "∣": "∤", "∥": "∦",
	"≺": "⊀", "≻": "⊁", "⊢": "⊬", "⊨": "⊭", "→": "↛", "←": "↚", "↔": "↮", "⇒": "⇏", "⇐": "⇍",
	"⇔": "⇎", "∃": "∄",
}

// Circled returns the circled form of a letter or digit (ⓐ, Ⓐ, ①).
func Circled(r rune) (rune, bool) {
	switch {
	case r >= 'a' && r <= 'z':
		return 0x24D0 + r - 'a', true
	case r >= 'A' && r <= 'Z':
		return 0x24B6 + r - 'A', true
	case r >= '1' && r <= '9':
		return 0x2460 + r - '1', true
	case r == '0':
		return 0x24EA, true
	}
	return 0, false
}

// Decompose splits a precomposed Latin letter (á, ǎ) into its base letter
// and combining mark, for fonts that have the parts but not the letter.
func Decompose(r rune) (base, mark rune, ok bool) {
	d, ok := decompositions[r]
	return d[0], d[1], ok
}

var decompositions = func() map[rune][2]rune {
	// Base letters and their accented forms, position by position, per
	// combining mark (the accents of \text: \' \` \^ \" \~ \c \v \u \= \. \H \k \r).
	table := []struct {
		mark        rune
		base, forms string
	}{
		{0x301, "aeiouyAEIOUYcnszCNSZglrkGLRK", "áéíóúýÁÉÍÓÚÝćńśźĆŃŚŹǵĺŕḱǴĹŔḰ"},
		{0x300, "aeiouAEIOU", "àèìòùÀÈÌÒÙ"},
		{0x302, "aeiouAEIOUcgshjwyCGSHJWY", "âêîôûÂÊÎÔÛĉĝŝĥĵŵŷĈĜŜĤĴŴŶ"},
		{0x308, "aeiouyAEIOUY", "äëïöüÿÄËÏÖÜŸ"},
		{0x303, "anoANOiuIU", "ãñõÃÑÕĩũĨŨ"},
		{0x327, "cCsStT", "çÇşŞţŢ"},
		{0x30C, "cdenrstzCDENRSTZaiougkhAIOUGKH", "čďěňřšťžČĎĚŇŘŠŤŽǎǐǒǔǧǩȟǍǏǑǓǦǨȞ"},
		{0x306, "agAG", "ăğĂĞ"},
		{0x304, "aeiouAEIOU", "āēīōūĀĒĪŌŪ"},
		{0x307, "zZeEgIaAoOcC", "żŻėĖġİȧȦȯȮċĊ"},
		{0x30B, "ouOU", "őűŐŰ"},
		{0x328, "aeAE", "ąęĄĘ"},
		{0x30A, "auAU", "åůÅŮ"},
	}
	m := map[rune][2]rune{}
	for _, t := range table {
		b, f := []rune(t.base), []rune(t.forms)
		for i := range b {
			m[f[i]] = [2]rune{b[i], t.mark}
		}
	}
	return m
}()
