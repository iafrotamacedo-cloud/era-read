package detect

import (
	"math"
	"testing"

	"github.com/iafrotamacedo-cloud/era-read/geom"
)

// TestUnclipQuadrado usa a propriedade exata de um quadrado: com cantos de
// 90 graus, expandir cada aresta por "distancia" faz o quadrado crescer
// exatamente "distancia" para cada lado -- da para prever a area e os
// limites exatos do resultado, sem depender de detalhe de implementacao
// (por exemplo, o sentido em que os vertices ficam armazenados).
func TestUnclipQuadrado(t *testing.T) {
	poly := geom.Polygon{{X: 0, Y: 0}, {X: 4, Y: 0}, {X: 4, Y: 4}, {X: 0, Y: 4}}
	// area 16, perimetro 16, ratio 1 -> distancia = 16*1/16 = 1
	got := Unclip(poly, 1.0)

	min, max := got.Bounds()
	quero := geom.Point{X: -1, Y: -1}
	queroMax := geom.Point{X: 5, Y: 5}
	const tol = 1e-6
	if math.Abs(min.X-quero.X) > tol || math.Abs(min.Y-quero.Y) > tol {
		t.Errorf("min = %v, quero %v", min, quero)
	}
	if math.Abs(max.X-queroMax.X) > tol || math.Abs(max.Y-queroMax.Y) > tol {
		t.Errorf("max = %v, quero %v", max, queroMax)
	}

	gotArea := got.Area()
	if gotArea < 0 {
		gotArea = -gotArea
	}
	if math.Abs(gotArea-36) > 1e-6 {
		t.Errorf("area expandida = %v, quero 36 (quadrado 6x6)", gotArea)
	}
}

func TestUnclipCresce(t *testing.T) {
	poly := geom.Polygon{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 3}, {X: 0, Y: 3}}
	original := poly.Area()
	if original < 0 {
		original = -original
	}

	expandido := Unclip(poly, 1.5)
	novaArea := expandido.Area()
	if novaArea < 0 {
		novaArea = -novaArea
	}
	if novaArea <= original {
		t.Errorf("area apos Unclip = %v, esperava maior que a original %v", novaArea, original)
	}
}

func TestUnclipPoligonoPequenoDemaisDevolveIgual(t *testing.T) {
	poly := geom.Polygon{{X: 0, Y: 0}, {X: 1, Y: 1}}
	got := Unclip(poly, 1.5)
	if len(got) != len(poly) {
		t.Errorf("Unclip de poligono com %d vertices mudou o tamanho para %d", len(poly), len(got))
	}
}
