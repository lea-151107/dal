package linear_test

import (
	"testing"

	"github.com/lea/dal/internal/linear"
	"github.com/lea/dal/internal/parse"
)

var lim = parse.Limits{MaxInputBytes: 16 << 10, MaxNodes: 5000, MaxDepth: 64, MaxExpansions: 1000}

func TestWrite(t *testing.T) {
	for _, c := range []struct{ src, uni, ascii string }{
		{`x^2 + y_i`, `x² + yᵢ`, `x^2 + y_i`},
		{`x^{a+b}`, `xᵃ⁺ᵇ`, `x^(a+b)`},
		{`x^{\alpha\beta}`, `xᵅᵝ`, `x^(αβ)`},
		{`x^{\pi}`, `x^π`, `x^π`},
		{`\frac{a+b}{c}`, `(a + b)/c`, `(a + b)/c`},
		{`\sqrt{x+1}`, `√(x + 1)`, `√(x + 1)`},
		{`(x^2)^3`, `(x²)³`, `(x^2)^3`},
		{`\begin{pmatrix}a&b\\c&d\end{pmatrix}`, `(a, b; c, d)`, `(a, b; c, d)`},
		{`\cancel{x}`, `cancel(x)`, `cancel(x)`},
		{`a - b`, `a − b`, `a - b`},
		{`\alpha\to\infty`, `α → ∞`, `α → ∞`},
	} {
		e, err := parse.Parse(c.src, lim)
		if err != nil {
			t.Errorf("%s: %v", c.src, err)
			continue
		}
		if got := linear.Write(e, linear.Options{}); got != c.uni {
			t.Errorf("%s: unicode %q, want %q", c.src, got, c.uni)
		}
		if got := linear.Write(e, linear.Options{ASCII: true}); got != c.ascii {
			t.Errorf("%s: ascii %q, want %q", c.src, got, c.ascii)
		}
	}
}
