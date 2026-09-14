package detect

import (
	"github.com/iafrotamacedo-cloud/era-read/geom"
	"github.com/iafrotamacedo-cloud/era-read/imgproc"
)

// RegionScore mede a confianca media de um contorno: a media dos valores
// do mapa de probabilidade nos pixels dentro do poligono.
//
// Um poligono degenerado (mancha de 1 pixel, sem area) nunca satisfaz
// Contains, entao cai para o valor do pixel mais proximo do centroide em
// vez de devolver zero -- devolver zero para uma mancha de 1 pixel
// descartaria pelo motivo errado (poligono degenerado, nao confianca
// baixa de verdade).
func RegionScore(prob *imgproc.Gray, poly geom.Polygon) float32 {
	if len(poly) == 0 {
		return 0
	}

	minP, maxP := poly.Bounds()
	x0 := clampInt(int(minP.X), 0, prob.W-1)
	y0 := clampInt(int(minP.Y), 0, prob.H-1)
	x1 := clampInt(int(maxP.X), 0, prob.W-1)
	y1 := clampInt(int(maxP.Y), 0, prob.H-1)

	var soma float64
	var n int
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if poly.Contains(geom.Point{X: float64(x), Y: float64(y)}) {
				soma += float64(prob.At(x, y))
				n++
			}
		}
	}
	if n == 0 {
		c := poly.Centroid()
		cx := clampInt(int(c.X), 0, prob.W-1)
		cy := clampInt(int(c.Y), 0, prob.H-1)
		return prob.At(cx, cy)
	}
	return float32(soma / float64(n))
}
