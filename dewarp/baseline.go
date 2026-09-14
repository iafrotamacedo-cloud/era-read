package dewarp

import (
	"fmt"

	"github.com/iafrotamacedo-cloud/era-read/geom"
)

// ExtractBaseline devolve os pontos que aproximam a base do texto -- onde
// as letras se apoiam -- a partir do poligono que delimita uma linha
// detectada, da esquerda para a direita.
//
// Segue a convencao usual de deteccao de texto curvo (DBNet e os conjuntos
// que o treinam, TotalText e CTW1500): os vertices do poligono sao a borda
// de cima, da esquerda para a direita, seguidos pela borda de baixo, da
// direita para a esquerda -- as duas metades tem o mesmo tanto de pontos.
// A baseline e a segunda metade invertida.
//
// Um quadrilatero simples (4 vertices: superior-esquerdo, superior-direito,
// inferior-direito, inferior-esquerdo -- o caso comum de uma linha sem
// curvatura) cai no mesmo esquema: a segunda metade e [inferior-direito,
// inferior-esquerdo], invertida vira [inferior-esquerdo, inferior-direito].
// So 2 pontos de baseline nao bastam para medir curvatura -- e correto:
// um detector que devolveu so 4 vertices ja esta dizendo que considera a
// linha reta. MeasureLine trata esse caso sem erro.
func ExtractBaseline(p geom.Polygon) ([]geom.Point, error) {
	n := len(p)
	if n < 4 || n%2 != 0 {
		return nil, fmt.Errorf("dewarp: poligono de linha precisa de numero par de vertices >= 4, tem %d", n)
	}

	metade := n / 2
	borda := p[metade:]
	baseline := make([]geom.Point, metade)
	for i, pt := range borda {
		baseline[metade-1-i] = pt
	}

	// detect.ToLinePolygon divide um contorno DENSO em cima/baixo cortando
	// o mesmo laco em dois arcos, do vertice mais a esquerda ao mais a
	// direita e vice-versa -- os dois arcos comecam e terminam EXATAMENTE
	// no mesmo par de vertices (p[metade-1]==p[metade] e p[n-1]==p[0]).
	// Para uma mancha mais larga que alta (o caso normal), esses dois
	// vertices compartilhados ficam perto do MEIO da altura da mancha --
	// onde ela e mais larga -- nao em cima nem embaixo. Sem descartar os
	// dois, a baseline sai com as pontas puxadas para o meio da altura,
	// mesmo numa linha perfeitamente reta: MeasureLine ve um "salto" nas
	// pontas e Classify erra para N2/N3.
	//
	// Um quadrilatero simples (n=4) NAO tem esse compartilhamento -- os 4
	// vertices sao 4 cantos distintos -- entao a checagem de igualdade so
	// dispara para o caso de contorno denso, e so remove quando sobram
	// pelo menos 2 pontos depois (metade >= 4).
	if metade >= 4 && p[metade-1] == p[metade] && p[n-1] == p[0] {
		baseline = baseline[1 : metade-1]
	}

	return baseline, nil
}
