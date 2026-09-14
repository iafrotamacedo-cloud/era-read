package read

import (
	"image"
	"image/color"
	"testing"

	"github.com/iafrotamacedo-cloud/era-read/geom"
)

func imagemSolida(w, h int, r, g, b uint8) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: r, G: g, B: b, A: 255})
		}
	}
	return img
}

func abs32(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// corEm le a cor de um pixel como NRGBA, decodificando de volta o que
// FromImageChannel/ComposeBGR fizeram -- serve para conferir, com
// tolerancia de quantizacao de 8 bits, que o recorte preservou a cor.
func corEm(img image.Image, x, y int) (r, g, b uint8) {
	nr, ng, nb, _ := img.At(x, y).RGBA()
	return uint8(nr >> 8), uint8(ng >> 8), uint8(nb >> 8)
}

// TestRecortarQuadPreservaCor confere que recortar um quadrilatero
// alinhado aos eixos, exatamente do tamanho da regiao, reproduz a cor
// original -- prova que os tres canais saem na ordem certa (nao trocados)
// depois de RemapHomography + ComposeBGR.
func TestRecortarQuadPreservaCor(t *testing.T) {
	const w, h = 20, 10
	img := imagemSolida(w, h, 200, 100, 50)

	cantos := [4]geom.Point{
		{X: 0, Y: 0}, {X: w - 1, Y: 0}, {X: w - 1, Y: h - 1}, {X: 0, Y: h - 1},
	}
	out, err := recortarQuad(img, cantos, w, h)
	if err != nil {
		t.Fatal(err)
	}
	if out.Bounds().Dx() != w || out.Bounds().Dy() != h {
		t.Fatalf("forma = %dx%d, quero %dx%d", out.Bounds().Dx(), out.Bounds().Dy(), w, h)
	}

	r, g, b := corEm(out, w/2, h/2)
	if abs32(float64(r)-200) > 2 || abs32(float64(g)-100) > 2 || abs32(float64(b)-50) > 2 {
		t.Errorf("cor = (%d,%d,%d), quero perto de (200,100,50)", r, g, b)
	}
}

// TestRecortarCurvaPreservaCor confere o mesmo para o caminho N2 -- uma
// baseline reta (caso degenerado) sobre uma imagem solida deveria
// reproduzir a mesma cor.
func TestRecortarCurvaPreservaCor(t *testing.T) {
	const w, h = 20, 10
	img := imagemSolida(w, h, 10, 20, 30)

	baseline := []geom.Point{{X: 0, Y: float64(h - 1)}, {X: w - 1, Y: float64(h - 1)}}
	out, err := recortarCurva(img, baseline, w, h, float64(h-1), 0)
	if err != nil {
		t.Fatal(err)
	}

	r, g, b := corEm(out, w/2, h/2)
	if abs32(float64(r)-10) > 2 || abs32(float64(g)-20) > 2 || abs32(float64(b)-30) > 2 {
		t.Errorf("cor = (%d,%d,%d), quero perto de (10,20,30)", r, g, b)
	}
}

// TestRecortarQuadCantosColinearesDaErro cobre o caso em que os 4 cantos
// nao formam um quadrilatero de verdade (todos na mesma reta) -- a
// homografia fica indeterminada, e RemapHomography devolve erro em vez de
// um resultado sem sentido.
func TestRecortarQuadCantosColinearesDaErro(t *testing.T) {
	img := imagemSolida(4, 4, 0, 0, 0)
	cantos := [4]geom.Point{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 2, Y: 0}, {X: 3, Y: 0}}
	if _, err := recortarQuad(img, cantos, 4, 4); err == nil {
		t.Error("cantos colineares deveriam dar erro")
	}
}
