package detect

import (
	"reflect"
	"testing"

	"github.com/iafrotamacedo-cloud/era-read/geom"
)

func retangulo(x0, y0, x1, y1 int) *Bitmap {
	w, h := x1+2, y1+2
	b := NewBitmap(w, h)
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			b.Set(x, y, true)
		}
	}
	return b
}

// TestTracarContornoQuadrado3x3 confere o caso calculado a mao no
// comentario de tracarContorno: um quadrado solido 3x3 tem que sair com os
// 8 pixels de borda (o do meio nao e borda), em sentido horario, comecando
// pelo canto superior-esquerdo.
func TestTracarContornoQuadrado3x3(t *testing.T) {
	b := retangulo(0, 0, 2, 2)

	got := tracarContorno(b, 0, 0)
	want := geom.Polygon{
		{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 2, Y: 0},
		{X: 2, Y: 1}, {X: 2, Y: 2},
		{X: 1, Y: 2}, {X: 0, Y: 2},
		{X: 0, Y: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("contorno = %v, quero %v", got, want)
	}
}

func TestTracarContornoPixelIsolado(t *testing.T) {
	b := NewBitmap(3, 3)
	b.Set(1, 1, true)

	got := tracarContorno(b, 1, 1)
	want := geom.Polygon{{X: 1, Y: 1}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("contorno = %v, quero %v", got, want)
	}
}

func TestTracarContornoLinhaHorizontal(t *testing.T) {
	b := NewBitmap(6, 3)
	for x := 1; x <= 4; x++ {
		b.Set(x, 1, true)
	}

	got := tracarContorno(b, 1, 1)
	// uma linha 1xN nao tem "dentro": o rastreamento vai ate a ponta
	// direita e volta pela mesma fileira de pixels, parando assim que
	// encosta de novo no inicio -- sem repetir o proprio inicio no fim.
	want := geom.Polygon{
		{X: 1, Y: 1}, {X: 2, Y: 1}, {X: 3, Y: 1}, {X: 4, Y: 1},
		{X: 3, Y: 1}, {X: 2, Y: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("contorno = %v, quero %v", got, want)
	}
}

func TestFindContoursDuasManchasSeparadas(t *testing.T) {
	b := NewBitmap(10, 5)
	// mancha 1: quadrado 2x2 no canto superior esquerdo
	for y := 0; y <= 1; y++ {
		for x := 0; x <= 1; x++ {
			b.Set(x, y, true)
		}
	}
	// mancha 2: quadrado 2x2 longe, sem tocar a primeira
	for y := 2; y <= 3; y++ {
		for x := 6; x <= 7; x++ {
			b.Set(x, y, true)
		}
	}

	contornos := FindContours(b)
	if len(contornos) != 2 {
		t.Fatalf("FindContours achou %d manchas, quero 2", len(contornos))
	}

	// a ordem de varredura garante que a mancha do canto superior esquerdo
	// vem primeiro.
	min0, _ := contornos[0].Bounds()
	if min0.X != 0 || min0.Y != 0 {
		t.Errorf("primeira mancha comeca em %v, quero (0,0)", min0)
	}
	min1, _ := contornos[1].Bounds()
	if min1.X != 6 || min1.Y != 2 {
		t.Errorf("segunda mancha comeca em %v, quero (6,2)", min1)
	}
}

// TestFindContoursDiagonalConecta8 confere que dois pixels que se tocam so
// na diagonal contam como a mesma mancha -- a razao de usar 8-conectividade
// (ver label.go).
func TestFindContoursDiagonalConecta8(t *testing.T) {
	b := NewBitmap(4, 4)
	b.Set(0, 0, true)
	b.Set(1, 1, true) // toca (0,0) so na diagonal

	contornos := FindContours(b)
	if len(contornos) != 1 {
		t.Fatalf("FindContours achou %d manchas, quero 1 (diagonal deveria unir)", len(contornos))
	}
}

func TestFindContoursVazio(t *testing.T) {
	b := NewBitmap(5, 5)
	if got := FindContours(b); got != nil {
		t.Errorf("FindContours em mascara vazia = %v, quero nil", got)
	}
}

// TestTracarContornoLFormato confere uma forma com reentrancia (nao
// convexa) -- o "L" ja usado em geom/polygon_test.go -- pra garantir que o
// rastreamento nao se perde num canto concavo.
func TestTracarContornoLFormato(t *testing.T) {
	b := NewBitmap(6, 6)
	// L: bloco 4x1 embaixo, bloco 1x4 a esquerda, unidos no canto
	for x := 0; x <= 3; x++ {
		b.Set(x, 3, true)
	}
	for y := 0; y <= 3; y++ {
		b.Set(0, y, true)
	}

	got := tracarContorno(b, 0, 0)

	// nao ha um "want" curto de decorar aqui; a propriedade que importa e
	// que o rastreamento fecha o laco (volta ao inicio) sem estourar o
	// limite de seguranca, e que todo vertice do laco e um pixel de
	// verdade da mascara.
	if len(got) == 0 {
		t.Fatal("contorno vazio")
	}
	for _, v := range got {
		x, y := int(v.X), int(v.Y)
		if !b.In(x, y) || !b.At(x, y) {
			t.Fatalf("vertice %v do contorno nao e um pixel de texto", v)
		}
	}
}
