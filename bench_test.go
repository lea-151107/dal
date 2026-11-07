package dal

import (
	"context"
	"testing"
)

const benchSrc = `\sum_{i=1}^n \frac{1}{i^2}`

func BenchmarkRender(b *testing.B) {
	for b.Loop() {
		if _, err := Render(context.Background(), benchSrc, Options{Display: true}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRenderText(b *testing.B) {
	for b.Loop() {
		if _, err := RenderText(context.Background(), benchSrc, TextOptions{Display: true}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRenderLine(b *testing.B) {
	for b.Loop() {
		if _, err := RenderLine(benchSrc, LineOptions{}); err != nil {
			b.Fatal(err)
		}
	}
}
