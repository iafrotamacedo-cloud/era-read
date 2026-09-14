package detect

import "github.com/iafrotamacedo-cloud/era-read/geom"

// Unclip expande um poligono para fora pela distancia que a formula do
// DBNet manda: area * ratio / perimetro. E o inverso do encolhimento que o
// treino aplica de proposito (ver documentacao do pacote) -- sem isso, o
// contorno detectado fica sistematicamente menor que o texto real.
//
// A expansao empurra cada vertice ao longo da bissetriz das duas arestas
// que se encontram nele, na distancia certa para que as ARESTAS (nao so
// os vertices) fiquem deslocadas por "distancia" -- e o "miter join"
// classico de desenho de contorno. Um canto muito agudo pediria um
// deslocamento enorme para manter essa exatidao; miterLimite trava isso
// num multiplo razoavel da distancia normal, ao custo de o canto ficar
// levemente arredondado em vez de pontudo nesses casos raros.
func Unclip(poly geom.Polygon, ratio float64) geom.Polygon {
	n := len(poly)
	if n < 3 {
		return poly
	}

	area := poly.Area()
	if area < 0 {
		area = -area
	}
	perimetro := poly.Perimeter()
	if perimetro == 0 {
		return poly
	}
	distancia := area * ratio / perimetro

	sentido := sentidoParaFora(poly)

	const miterLimite = 4.0
	out := make(geom.Polygon, n)
	for i := 0; i < n; i++ {
		prev := poly[(i-1+n)%n]
		cur := poly[i]
		next := poly[(i+1)%n]

		nIn := normalDeAresta(cur.Sub(prev), sentido)
		nOut := normalDeAresta(next.Sub(cur), sentido)

		bis := nIn.Add(nOut)
		compr := bis.Len()
		if compr < 1e-9 {
			// arestas quase opostas (dobra de ~180 graus): a bissetriz nao
			// esta definida: usa so a normal de entrada.
			out[i] = cur.Add(nIn.Scale(distancia))
			continue
		}
		bis = bis.Scale(1 / compr)

		cosMeio := bis.Dot(nIn)
		escala := distancia
		if cosMeio > 1e-6 {
			escala = distancia / cosMeio
		}
		if escala > distancia*miterLimite {
			escala = distancia * miterLimite
		}
		out[i] = cur.Add(bis.Scale(escala))
	}
	return out
}

// sentidoParaFora descobre, numa aresta de referencia, qual das duas
// rotacoes de 90 graus aponta para longe do centroide -- e usa a MESMA
// regra em todas as arestas do poligono, valido porque um poligono simples
// tem um sentido de percurso so (a normal "para fora" de uma aresta e
// sempre a mesma rotacao relativa ao sentido de percurso, em todas as
// arestas).
func sentidoParaFora(poly geom.Polygon) float64 {
	centroide := poly.Centroid()
	p0, p1 := poly[0], poly[1%len(poly)]
	aresta := p1.Sub(p0)
	normal := normalDeAresta(aresta, 1)
	meio := p0.Add(p1).Scale(0.5)
	paraFora := meio.Sub(centroide)
	if normal.Dot(paraFora) < 0 {
		return -1
	}
	return 1
}

// normalDeAresta devolve a normal unitaria de um vetor de aresta,
// rotacionada 90 graus para o lado indicado por sentido (+1 ou -1).
func normalDeAresta(aresta geom.Point, sentido float64) geom.Point {
	l := aresta.Len()
	if l < 1e-12 {
		return geom.Point{}
	}
	ux, uy := aresta.X/l, aresta.Y/l
	return geom.Point{X: -uy * sentido, Y: ux * sentido}
}
