package detect

import (
	"testing"

	"github.com/iafrotamacedo-cloud/era-read/imgproc"
)

// TestDetectManchaUnica monta um mapa de probabilidade sintetico com uma
// unica mancha de confianca alta sobre fundo de confianca baixa -- o caso
// mais simples que ainda exercita o pipeline inteiro: binariza, acha o
// contorno, mede a confianca, expande.
func TestDetectManchaUnica(t *testing.T) {
	g := imgproc.NewGray(30, 20)
	for i := range g.Pix {
		g.Pix[i] = 0.05
	}
	for y := 5; y <= 12; y++ {
		for x := 5; x <= 20; x++ {
			g.Set(x, y, 0.9)
		}
	}

	got := Detect(g, DefaultOptions())
	if len(got) != 1 {
		t.Fatalf("Detect achou %d regioes, quero 1: %+v", len(got), got)
	}

	r := got[0]
	if r.Score < 0.8 {
		t.Errorf("Score = %v, esperava perto de 0.9", r.Score)
	}

	// Unclip expande: a area do poligono devolvido tem que ser maior que
	// a mancha original (16*8 = 128).
	area := r.Polygon.Area()
	if area < 0 {
		area = -area
	}
	if area <= 128 {
		t.Errorf("area do poligono = %v, esperava maior que 128 (mancha original, antes de expandir)", area)
	}
}

func TestDetectNadaAcimaDoLimiar(t *testing.T) {
	g := imgproc.NewGray(10, 10)
	for i := range g.Pix {
		g.Pix[i] = 0.05
	}
	if got := Detect(g, DefaultOptions()); len(got) != 0 {
		t.Errorf("Detect em mapa sem texto achou %d regioes, quero 0", len(got))
	}
}

// TestDetectDescartaManchaPequena confere que MinArea filtra ruido de
// binarizacao -- um punhado de pixels isolados acima do limiar que nao e
// texto de verdade.
func TestDetectDescartaManchaPequena(t *testing.T) {
	g := imgproc.NewGray(20, 20)
	for i := range g.Pix {
		g.Pix[i] = 0.05
	}
	g.Set(10, 10, 0.9) // 1 pixel isolado

	opts := DefaultOptions()
	got := Detect(g, opts)
	if len(got) != 0 {
		t.Errorf("Detect com mancha de 1 pixel (area < MinArea=%v) achou %d regioes, quero 0", opts.MinArea, len(got))
	}
}

func TestDetectDescartaConfiancaBaixa(t *testing.T) {
	g := imgproc.NewGray(20, 20)
	for i := range g.Pix {
		g.Pix[i] = 0.05
	}
	// mancha grande, mas so um pouco acima do limiar de binarizacao --
	// confianca baixa, deveria ser descartada por MinScore.
	for y := 5; y <= 12; y++ {
		for x := 5; x <= 12; x++ {
			g.Set(x, y, 0.35)
		}
	}

	got := Detect(g, DefaultOptions())
	if len(got) != 0 {
		t.Errorf("Detect com confianca baixa (0.35 < MinScore=0.5) achou %d regioes, quero 0", len(got))
	}
}
