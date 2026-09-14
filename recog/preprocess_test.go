package recog

import (
	"image"
	"image/color"
	"testing"
)

func imagemSolida(w, h int, r, g, b uint8) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
	return img
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// TestPreprocessLinhaCurta cobre o caso comum: uma linha bem mais larga que
// alta, mas ainda proporcionalmente mais estreita que a razao padrao do
// modelo (320/48 ~ 6,67) -- a largura alvo fica no minimo (320), e sobra
// preenchimento em zero a direita.
func TestPreprocessLinhaCurta(t *testing.T) {
	img := imagemSolida(48, 48, 100, 150, 200) // razao 1:1
	tns, err := Preprocess(img, DefaultPreprocessOptions())
	if err != nil {
		t.Fatal(err)
	}

	if want := []int{1, 3, 48, 320}; !intsIguais(tns.Shape, want) {
		t.Fatalf("forma = %v, quero %v", tns.Shape, want)
	}

	const larguraRedim = 48 // ceil(48 * 48/48)
	plano := 48 * 320
	flat := tns.Flat()

	// canal 0 = B, canal 1 = G, canal 2 = R -- mesma convencao de detect.Preprocess.
	quero := [3]float32{
		float32(200)/255*2 - 1, // B
		float32(150)/255*2 - 1, // G
		float32(100)/255*2 - 1, // R
	}
	for c := 0; c < 3; c++ {
		for y := 0; y < 48; y++ {
			for x := 0; x < larguraRedim; x++ {
				got := flat[c*plano+y*320+x]
				if abs32(got-quero[c]) > 0.02 {
					t.Fatalf("canal %d, pixel (%d,%d) = %v, quero %v", c, x, y, got, quero[c])
				}
			}
			// preenchimento a direita: exatamente zero.
			for x := larguraRedim; x < 320; x++ {
				if got := flat[c*plano+y*320+x]; got != 0 {
					t.Fatalf("canal %d, preenchimento (%d,%d) = %v, quero 0", c, x, y, got)
				}
			}
		}
	}
}

// TestPreprocessLinhaLarga cobre uma linha proporcionalmente mais larga que
// a razao padrao do modelo -- a largura alvo cresce alem de 320, e nao
// sobra preenchimento nenhum.
func TestPreprocessLinhaLarga(t *testing.T) {
	img := imagemSolida(960, 48, 10, 20, 30) // razao 20:1, acima de 320/48
	tns, err := Preprocess(img, DefaultPreprocessOptions())
	if err != nil {
		t.Fatal(err)
	}
	if want := []int{1, 3, 48, 960}; !intsIguais(tns.Shape, want) {
		t.Fatalf("forma = %v, quero %v (sem preenchimento, a linha ja ocupa tudo)", tns.Shape, want)
	}
}

func TestPreprocessRecusaEntradaInvalida(t *testing.T) {
	t.Run("imagem vazia", func(t *testing.T) {
		img := image.NewRGBA(image.Rect(0, 0, 0, 0))
		if _, err := Preprocess(img, DefaultPreprocessOptions()); err == nil {
			t.Error("imagem vazia deveria dar erro")
		}
	})

	t.Run("opcoes invalidas", func(t *testing.T) {
		img := imagemSolida(10, 10, 0, 0, 0)
		if _, err := Preprocess(img, PreprocessOptions{Height: 0, DefaultWidth: 320}); err == nil {
			t.Error("Height=0 deveria dar erro")
		}
		if _, err := Preprocess(img, PreprocessOptions{Height: 48, DefaultWidth: 0}); err == nil {
			t.Error("DefaultWidth=0 deveria dar erro")
		}
	})
}

func intsIguais(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
