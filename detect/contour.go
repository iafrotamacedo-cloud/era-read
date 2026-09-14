package detect

import "github.com/iafrotamacedo-cloud/era-read/geom"

// direcoes sao os 8 deslocamentos ao redor de um pixel, em ordem horaria,
// comecando por Oeste. A ordem importa: o rastreamento de contorno busca
// o proximo pixel de borda percorrendo esta lista a partir de onde parou.
var direcoes = [8][2]int{
	{-1, 0},  // 0: O
	{-1, -1}, // 1: NO
	{0, -1},  // 2: N
	{1, -1},  // 3: NE
	{1, 0},   // 4: L
	{1, 1},   // 5: SE
	{0, 1},   // 6: S
	{-1, 1},  // 7: SO
}

// FindContours acha o contorno de cada mancha de texto na mascara,
// devolvendo um poligono por mancha.
//
// Poligonos de mancha isolada de 1 pixel saem com um unico vertice; quem
// consumir isso (RegionScore, Unclip) precisa aceitar poligonos
// degenerados ou filtra-los antes, por area.
func FindContours(b *Bitmap) []geom.Polygon {
	labels, n := label(b)
	if n == 0 {
		return nil
	}

	// acha o pixel superior-esquerdo de cada rotulo, na ordem de
	// varredura -- e o ponto de partida certo para o rastreamento (ver
	// tracarContorno).
	inicio := make([]int, n+1) // indexado por rotulo, 1..n
	for i := range inicio {
		inicio[i] = -1
	}
	for idx, l := range labels {
		if l != 0 && inicio[l] == -1 {
			inicio[l] = idx
		}
	}

	contornos := make([]geom.Polygon, n)
	for l := 1; l <= n; l++ {
		x, y := inicio[l]%b.W, inicio[l]/b.W
		contornos[l-1] = tracarContorno(b, x, y)
	}
	return contornos
}

// tracarContorno segue a borda de uma mancha a partir de (x0, y0) --
// obrigatoriamente o pixel mais acima e mais a esquerda da mancha, achado
// por varredura -- pelo algoritmo de Moore: em cada pixel de borda,
// procura o proximo pixel de texto percorrendo os 8 vizinhos em sentido
// horario a partir de onde a busca anterior parou, nunca do zero.
//
// Como (x0, y0) e o pixel superior-esquerdo por varredura, seu vizinho a
// oeste e garantidamente fundo (se fosse texto, teria sido achado antes na
// varredura) -- e por isso a busca comeca sempre assumindo que "chegamos
// pela direcao oeste".
//
// Simplificacao registrada: o rastreamento para assim que volta ao pixel
// inicial, sem conferir tambem o segundo ponto (o criterio de Jacob, mais
// rigoroso). Isso e suficiente para uma mancha de texto normal; uma forma
// patologica que toca a si mesma no pixel inicial poderia parar cedo
// demais. Nao chegou a importar na pratica para manchas de texto.
func tracarContorno(b *Bitmap, x0, y0 int) geom.Polygon {
	contorno := geom.Polygon{{X: float64(x0), Y: float64(y0)}}

	cx, cy := x0, y0
	entrada := 0 // comeca como se tivesse chegado pela direcao Oeste (indice 0)

	limite := len(b.Pix)*8 + 8 // guarda de seguranca contra loop infinito
	for passo := 0; passo < limite; passo++ {
		achou := false
		var nx, ny, novaEntrada int

		for k := 1; k <= 8; k++ {
			idx := (entrada + k) % 8
			tx, ty := cx+direcoes[idx][0], cy+direcoes[idx][1]
			if b.In(tx, ty) && b.At(tx, ty) {
				nx, ny = tx, ty
				novaEntrada = (idx + 4) % 8 // direcao de volta a este pixel
				achou = true
				break
			}
		}

		if !achou {
			// pixel isolado: nao ha vizinho de texto nenhum.
			break
		}

		cx, cy = nx, ny
		entrada = novaEntrada

		if cx == x0 && cy == y0 {
			break
		}
		contorno = append(contorno, geom.Point{X: float64(cx), Y: float64(cy)})
	}

	return contorno
}
