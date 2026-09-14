package detect

import "github.com/iafrotamacedo-cloud/era-read/imgproc"

// Bitmap e uma mascara binaria: verdadeiro onde ha texto, falso onde nao.
type Bitmap struct {
	Pix  []bool
	W, H int
}

// NewBitmap cria uma mascara vazia (tudo falso) de w por h.
func NewBitmap(w, h int) *Bitmap {
	return &Bitmap{Pix: make([]bool, w*h), W: w, H: h}
}

// At devolve o valor em (x, y). Sem checagem de limite -- caminho quente,
// mesma convencao de imgproc.Gray.At.
func (b *Bitmap) At(x, y int) bool { return b.Pix[y*b.W+x] }

// Set atribui o valor em (x, y).
func (b *Bitmap) Set(x, y int, v bool) { b.Pix[y*b.W+x] = v }

// In informa se (x, y) cai dentro da mascara.
func (b *Bitmap) In(x, y int) bool {
	return x >= 0 && x < b.W && y >= 0 && y < b.H
}

// Binarize corta o mapa de probabilidade num limiar: pixel >= threshold
// conta como texto, abaixo conta como fundo.
func Binarize(prob *imgproc.Gray, threshold float32) *Bitmap {
	b := NewBitmap(prob.W, prob.H)
	for i, v := range prob.Pix {
		b.Pix[i] = v >= threshold
	}
	return b
}
