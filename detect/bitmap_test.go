package detect

import (
	"testing"

	"github.com/iafrotamacedo-cloud/era-read/imgproc"
)

func TestBinarize(t *testing.T) {
	g := imgproc.NewGray(3, 1)
	g.Set(0, 0, 0.1)
	g.Set(1, 0, 0.3) // exatamente no limiar: conta como texto
	g.Set(2, 0, 0.5)

	b := Binarize(g, 0.3)
	casos := []struct {
		x    int
		want bool
	}{
		{0, false},
		{1, true},
		{2, true},
	}
	for _, c := range casos {
		if got := b.At(c.x, 0); got != c.want {
			t.Errorf("At(%d,0) = %v, quero %v", c.x, got, c.want)
		}
	}
}

func TestBitmapInBounds(t *testing.T) {
	b := NewBitmap(4, 3)
	casos := []struct {
		x, y int
		want bool
	}{
		{0, 0, true}, {3, 2, true},
		{-1, 0, false}, {4, 0, false}, {0, -1, false}, {0, 3, false},
	}
	for _, c := range casos {
		if got := b.In(c.x, c.y); got != c.want {
			t.Errorf("In(%d,%d) = %v, quero %v", c.x, c.y, got, c.want)
		}
	}
}
