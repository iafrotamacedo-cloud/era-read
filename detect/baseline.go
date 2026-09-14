package detect

import (
	"fmt"

	"github.com/iafrotamacedo-cloud/era-read/geom"
)

// ToLinePolygon converte um contorno cru (o que FindContours devolve, um
// laco fechado generico, um vertice por pixel de borda) para a convencao
// que dewarp.ExtractBaseline espera: metade dos vertices formando a borda
// de cima da esquerda para a direita, a outra metade a borda de baixo da
// direita para a esquerda, as duas com a mesma contagem.
//
// pontos e quantos vertices cada borda tem depois de reamostrada -- o
// contorno cru tem contagem densa e variavel entre manchas; reamostrar
// para um numero fixo por lado e o que garante as duas bordas com a mesma
// contagem, exigencia de ExtractBaseline.
//
// # Como divide em cima/baixo
//
// FindContours sempre traca em sentido horario (Moore, ver contour.go),
// entao andar do vertice mais a esquerda ate o mais a direita SEGUINDO o
// sentido do laco passa pela borda de cima; continuar do mais a direita de
// volta ao mais a esquerda passa pela borda de baixo. Nao ha reordenacao
// nem inversao depois -- a ordem sai certa so por causa do sentido
// consistente do rastreamento.
//
// Isso vale para uma mancha razoavelmente convexa e mais larga que alta --
// o caso normal de uma palavra ou linha de texto. Uma forma exotica
// (textos verticais, uma mancha com reentrancia funda perto do ponto mais
// a esquerda ou mais a direita) pode quebrar a suposicao; e a mesma
// suposicao de "proximo da horizontal" que dewarp.MeasureLine ja registra.
func ToLinePolygon(contour geom.Polygon, pontos int) (geom.Polygon, error) {
	if len(contour) < 3 {
		return nil, fmt.Errorf("detect: contorno com %d vertices nao da para separar em cima/baixo", len(contour))
	}
	if pontos < 2 {
		return nil, fmt.Errorf("detect: pontos por borda tem que ser >= 2, recebeu %d", pontos)
	}

	iEsq, iDir := extremosHorizontais(contour)
	cima, baixo := dividirEmArcos(contour, iEsq, iDir)

	cimaR := reamostrarArco(cima, pontos)
	baixoR := reamostrarArco(baixo, pontos)

	out := make(geom.Polygon, 0, 2*pontos)
	out = append(out, cimaR...)
	out = append(out, baixoR...)
	return out, nil
}

// extremosHorizontais acha o indice do vertice mais a esquerda e do mais a
// direita do poligono.
func extremosHorizontais(p geom.Polygon) (esq, dir int) {
	for i, v := range p {
		if v.X < p[esq].X {
			esq = i
		}
		if v.X > p[dir].X {
			dir = i
		}
	}
	return esq, dir
}

// dividirEmArcos corta o laco fechado p em dois arcos entre os indices a e
// b, cada um percorrido no sentido do laco original (a -> b, depois b -> a
// completando a volta).
func dividirEmArcos(p geom.Polygon, a, b int) (arcoAB, arcoBA geom.Polygon) {
	n := len(p)
	for i := a; ; i = (i + 1) % n {
		arcoAB = append(arcoAB, p[i])
		if i == b {
			break
		}
	}
	for i := b; ; i = (i + 1) % n {
		arcoBA = append(arcoBA, p[i])
		if i == a {
			break
		}
	}
	return arcoAB, arcoBA
}

// reamostrarArco escolhe n pontos igualmente espacados por INDICE ao longo
// do arco -- nao por comprimento de arco geometrico. Simplificacao
// deliberada: o contorno cru ja tem densidade quase uniforme (um vertice
// por passo de pixel ao longo da borda), entao espacar por indice se
// aproxima bem de espacar por distancia sem o custo de medir distancias.
func reamostrarArco(arco geom.Polygon, n int) geom.Polygon {
	if len(arco) == 1 {
		out := make(geom.Polygon, n)
		for i := range out {
			out[i] = arco[0]
		}
		return out
	}
	out := make(geom.Polygon, n)
	for i := 0; i < n; i++ {
		t := float64(i) / float64(n-1)
		idx := int(t * float64(len(arco)-1))
		out[i] = arco[idx]
	}
	return out
}
