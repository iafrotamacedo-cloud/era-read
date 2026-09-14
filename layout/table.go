package layout

import "sort"

// Table e uma tabela reconstituida de varias linhas: cada linha virou uma
// sequencia de celulas (SplitCells), e as celulas de todas as linhas foram
// alinhadas na mesma coluna quando ocupam a mesma faixa horizontal.
//
// Rows[i][j] e a celula da linha i na coluna j -- Cell{} (sem palavras) se
// aquela linha nao tiver nada na coluna j. As colunas saem na ordem
// esquerda para direita.
type Table struct {
	Rows [][]Cell
}

// NumColumns devolve quantas colunas a tabela tem.
func (t Table) NumColumns() int {
	max := 0
	for _, r := range t.Rows {
		if len(r) > max {
			max = len(r)
		}
	}
	return max
}

// coluna e uma faixa horizontal [min,max] que uma ou mais celulas
// ocupam -- o intervalo cresce conforme mais celulas se juntam a ele.
type coluna struct {
	min, max float64
}

// GroupTable tenta alinhar em colunas as celulas de varias linhas (cada
// linha ja cortada por SplitCells com o mesmo gapFactor).
//
// O algoritmo e o de fundir intervalos: junta todas as celulas de todas as
// linhas, ordena pela borda esquerda, e cada celula ou estende a coluna
// aberta mais recente (se a borda esquerda dela cair dentro da coluna) ou
// abre uma coluna nova. Celulas de linhas DIFERENTES que ocupam a mesma
// faixa horizontal -- o codigo do produto de uma linha embaixo do codigo
// da linha anterior, por exemplo -- caem na mesma coluna; a ordem em que
// as linhas foram percorridas nao importa para decidir as colunas, so a
// posicao.
//
// Isso funciona bem quando as colunas de verdade do documento tem faixas
// horizontais que nao se sobrepoem entre si -- o caso comum de uma tabela
// desenhada com colunas alinhadas. NAO tenta decidir se uma celula que
// sobra "deveria" ter ficado em outra coluna por semantica (um valor que
// vazou pra fora da grade, por exemplo); isso e responsabilidade de quem
// usa o resultado.
func GroupTable(linhas []Line, gapFactor float64) Table {
	type ocorrencia struct {
		linha, ordem int
		cel          Cell
		min, max     float64
	}

	var todas []ocorrencia
	celulasPorLinha := make([][]Cell, len(linhas))
	for i, l := range linhas {
		celulas := l.SplitCells(gapFactor)
		celulasPorLinha[i] = celulas
		for j, c := range celulas {
			min, max := c.Bounds()
			todas = append(todas, ocorrencia{linha: i, ordem: j, cel: c, min: min.X, max: max.X})
		}
	}
	if len(todas) == 0 {
		return Table{Rows: make([][]Cell, len(linhas))}
	}

	sort.SliceStable(todas, func(a, b int) bool { return todas[a].min < todas[b].min })

	var colunas []coluna
	colunaDe := make(map[[2]int]int, len(todas)) // (linha,ordem) -> indice da coluna
	for _, oc := range todas {
		idx := len(colunas) - 1
		if idx >= 0 && oc.min <= colunas[idx].max {
			if oc.max > colunas[idx].max {
				colunas[idx].max = oc.max
			}
		} else {
			colunas = append(colunas, coluna{min: oc.min, max: oc.max})
			idx = len(colunas) - 1
		}
		colunaDe[[2]int{oc.linha, oc.ordem}] = idx
	}

	rows := make([][]Cell, len(linhas))
	for i, celulas := range celulasPorLinha {
		row := make([]Cell, len(colunas))
		for j, c := range celulas {
			row[colunaDe[[2]int{i, j}]] = c
		}
		rows[i] = row
	}

	return Table{Rows: rows}
}
