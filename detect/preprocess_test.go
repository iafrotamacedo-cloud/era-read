package detect

import (
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/iafrotamacedo-cloud/era-read/geom"
)

// TestTamanhoRedimensionado usa os tres casos que separam os caminhos do
// algoritmo do PaddleOCR: imagem maior que o limite (encolhe), menor
// (mantem, so arredonda), e menor que 32 (bate no minimo).
func TestTamanhoRedimensionado(t *testing.T) {
	casos := []struct {
		nome   string
		w, h   int
		limite float64
		wantW  int
		wantH  int
	}{
		// 1920x1080, limite 960: maior=1920>960, ratio=0.5.
		// h: 1080*0.5=540 -> 540/32=16.875 -> arredonda 17 -> 544.
		// w: 1920*0.5=960 -> 960/32=30 exato -> 960.
		{"encolhe (paisagem)", 1920, 1080, 960, 960, 544},

		// 500x300, limite 960: maior=500<960, ratio=1 (nao encolhe).
		// h: 300/32=9.375 -> arredonda 9 -> 288.
		// w: 500/32=15.625 -> arredonda 16 -> 512.
		{"nao encolhe, so arredonda", 500, 300, 960, 512, 288},

		// 10x10, bem menor que 32: bate no minimo dos dois eixos.
		{"minimo de 32", 10, 10, 960, 32, 32},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			gotW, gotH := tamanhoRedimensionado(c.w, c.h, c.limite)
			if gotW != c.wantW || gotH != c.wantH {
				t.Errorf("tamanhoRedimensionado(%d,%d,%v) = (%d,%d), quero (%d,%d)",
					c.w, c.h, c.limite, gotW, gotH, c.wantW, c.wantH)
			}
			// os dois eixos sempre multiplos de 32.
			if gotW%32 != 0 || gotH%32 != 0 {
				t.Errorf("(%d,%d) nao sao multiplos de 32", gotW, gotH)
			}
		})
	}
}

// TestPreprocessOrdemDeCanalENormalizacao confere, com uma imagem 32x32 de
// cor solida (menor que o limite, entao sem redimensionar de verdade -- so
// arredondar, que para 32x32 ja e multiplo), que o tensor de saida tem os
// tres canais na ordem B,G,R e cada um normalizado com a media/desvio do
// seu proprio canal -- a inversao de ordem e o bug que nenhum teste de
// forma pegaria.
func TestPreprocessOrdemDeCanalENormalizacao(t *testing.T) {
	const w, h = 32, 32
	src := image.NewRGBA(image.Rect(0, 0, w, h))
	corR, corG, corB := uint8(255), uint8(128), uint8(0)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			src.Set(x, y, color.RGBA{R: corR, G: corG, B: corB, A: 255})
		}
	}

	tns, scale, err := Preprocess(src, DefaultPreprocessOptions())
	if err != nil {
		t.Fatalf("Preprocess: %v", err)
	}
	if len(tns.Shape) != 4 || tns.Shape[0] != 1 || tns.Shape[1] != 3 || tns.Shape[2] != h || tns.Shape[3] != w {
		t.Fatalf("forma = %v, quero [1,3,%d,%d]", tns.Shape, h, w)
	}
	if math.Abs(scale.X-1) > 1e-9 || math.Abs(scale.Y-1) > 1e-9 {
		t.Errorf("scale = %v, quero {1,1} (32x32 ja e multiplo de 32, sem encolher)", scale)
	}

	wantB := (float32(corB)/255 - meanBGR[0]) / stdBGR[0]
	wantG := (float32(corG)/255 - meanBGR[1]) / stdBGR[1]
	wantR := (float32(corR)/255 - meanBGR[2]) / stdBGR[2]

	got := tns.Flat()
	plano := w * h
	if got := got[0*plano]; abs32(got-wantB) > 0.01 {
		t.Errorf("canal 0 (deveria ser B) = %v, quero %v", got, wantB)
	}
	if got := got[1*plano]; abs32(got-wantG) > 0.01 {
		t.Errorf("canal 1 (deveria ser G) = %v, quero %v", got, wantG)
	}
	if got := got[2*plano]; abs32(got-wantR) > 0.01 {
		t.Errorf("canal 2 (deveria ser R) = %v, quero %v", got, wantR)
	}
}

func TestRescale(t *testing.T) {
	poly := geom.Polygon{{X: 100, Y: 200}, {X: 300, Y: 400}}
	s := Scale{X: 0.5, Y: 0.25}
	got := Rescale(poly, s)
	want := geom.Polygon{{X: 200, Y: 800}, {X: 600, Y: 1600}}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Rescale[%d] = %v, quero %v", i, got[i], want[i])
		}
	}
}
