package layout

import "github.com/iafrotamacedo-cloud/era-read/geom"

// DefaultGapFactor e o ponto de partida para SplitCells: um vao entre
// palavras maior que DefaultGapFactor vezes a altura da linha conta como
// fronteira de coluna.
//
// Nao e uma calibracao medida contra tabela real rotulada a mao -- so uma
// leitura das duas notas fiscais reais que este motor ja usa para
// validar as outras fases (ver README): vaos de ate ~1,5x a altura da
// linha apareceram entre pedacos do MESMO campo que o detector partiu em
// duas regioes ("Bairro:" virou duas regioes vizinhas, vao pequeno); vaos
// acima de ~3x apareceram so entre campos genuinamente diferentes lado a
// lado. O numero relativo (multiplo da altura), nao um valor fixo de
// pixels, e de proposito: a mesma pagina escaneada em 150 DPI ou 300 DPI
// muda todo espacamento em pixels na mesma proporcao que muda a altura da
// fonte, entao um limiar por pixel quebraria de resolucao para resolucao.
const DefaultGapFactor = 3.0

// Cell e um grupo de palavras vizinhas dentro da MESMA linha, com vao
// horizontal pequeno entre elas -- candidatas a serem o mesmo campo,
// possivelmente partido em mais de uma regiao pelo detector.
type Cell struct {
	Words []Word
}

// Text junta o texto das palavras da celula, na mesma convencao de
// Line.Text.
func (c Cell) Text() string {
	return Line{Words: c.Words}.Text()
}

// Bounds devolve o retangulo que envolve as palavras da celula, na mesma
// convencao de Line.Bounds.
func (c Cell) Bounds() (min, max geom.Point) {
	return Line{Words: c.Words}.Bounds()
}

// SplitCells corta a linha em celulas por espacamento horizontal: um vao
// entre palavras consecutivas maior que gapFactor vezes a altura da
// propria linha conta como fronteira de coluna.
//
// Pressupoe Words ja ordenado esquerda para direita -- a mesma ordem que
// GroupLines ja deixa. Uma linha vazia ou de uma palavra so devolve uma
// unica celula (nada para cortar).
//
// O que isso NAO faz: alinhar celulas de linhas diferentes na mesma
// coluna (a tabela de verdade, onde "codigo" de uma linha cai embaixo de
// "codigo" da linha anterior). SplitCells so olha para DENTRO de uma
// linha; juntar colunas ao longo de varias linhas fica para quando algum
// documento real pedir isso -- calibrar esse alinhamento sem dado real
// para testar contra seria chutar um numero, o mesmo motivo que atrasou
// SplitCells ate agora.
func (l Line) SplitCells(gapFactor float64) []Cell {
	if len(l.Words) == 0 {
		return nil
	}
	if len(l.Words) == 1 {
		return []Cell{{Words: l.Words}}
	}

	limiar := gapFactor * alturaDaLinha(l)

	var celulas []Cell
	atual := []Word{l.Words[0]}
	for i := 1; i < len(l.Words); i++ {
		_, maxAnterior := l.Words[i-1].Box.Bounds()
		minAtual, _ := l.Words[i].Box.Bounds()
		vao := minAtual.X - maxAnterior.X

		if vao > limiar {
			celulas = append(celulas, Cell{Words: atual})
			atual = nil
		}
		atual = append(atual, l.Words[i])
	}
	celulas = append(celulas, Cell{Words: atual})
	return celulas
}

// alturaDaLinha devolve a altura da linha inteira -- a mesma medida usada
// como referencia de escala para o limiar de SplitCells.
func alturaDaLinha(l Line) float64 {
	lo, hi := l.Bounds()
	return hi.Y - lo.Y
}
