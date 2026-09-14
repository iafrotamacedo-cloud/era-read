package detect

import (
	"testing"

	"github.com/iafrotamacedo-cloud/era-read/geom"
	"github.com/iafrotamacedo-cloud/era-read/imgproc"
)

func TestRegionScoreMediaDentroDoPoligono(t *testing.T) {
	g := imgproc.NewGray(10, 10)
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			g.Set(x, y, 0.1) // fundo: confianca baixa
		}
	}
	// quadrado 4x4 com confianca alta
	for y := 2; y <= 5; y++ {
		for x := 2; x <= 5; x++ {
			g.Set(x, y, 0.9)
		}
	}

	poly := geom.Polygon{{X: 2, Y: 2}, {X: 6, Y: 2}, {X: 6, Y: 6}, {X: 2, Y: 6}}
	got := RegionScore(g, poly)
	if got < 0.85 || got > 0.95 {
		t.Errorf("RegionScore = %v, esperava perto de 0.9", got)
	}
}

func TestRegionScorePoligonoDegenerado(t *testing.T) {
	g := imgproc.NewGray(5, 5)
	g.Set(2, 2, 0.77)

	// um unico ponto: Contains nunca e verdadeiro, cai para o centroide.
	poly := geom.Polygon{{X: 2, Y: 2}}
	if got := RegionScore(g, poly); abs32(got-0.77) > 1e-6 {
		t.Errorf("RegionScore degenerado = %v, quero 0.77 (valor no centroide)", got)
	}
}

func TestRegionScorePoligonoVazio(t *testing.T) {
	g := imgproc.NewGray(3, 3)
	if got := RegionScore(g, nil); got != 0 {
		t.Errorf("RegionScore(nil) = %v, quero 0", got)
	}
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
