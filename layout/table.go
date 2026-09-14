package layout

import "sort"

// Table e uma tabela reconstituida de varias linhas: cada linha virou uma
// sequencia de celulas, e as celulas de todas as linhas foram alinhadas na
// mesma coluna quando ocupam a mesma faixa horizontal.
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

// GroupTable tenta alinhar em colunas as celulas de varias linhas, cada
// linha primeiro cortada em campos por SplitCells (mesmo gapFactor para
// todas).
//
// So funciona quando SplitCells acha vao nenhum grande o bastante para
// separar os campos dentro de uma linha -- uma linha de item de tabela
// com muitos campos numericos vizinhos e vaos parecidos entre todos pode
// nao ter fronteira nenhuma que se destaque, e a linha inteira vira uma
// celula so (ver GroupTableWords para esse caso).
func GroupTable(linhas []Line, gapFactor float64) Table {
	celulasPorLinha := make([][]Cell, len(linhas))
	for i, l := range linhas {
		celulasPorLinha[i] = l.SplitCells(gapFactor)
	}
	return agruparEmColunas(celulasPorLinha)
}

// GroupTableWords e como GroupTable, mas usa cada PALAVRA da linha como a
// sua propria celula, sem passar por SplitCells.
//
// Existe para quando SplitCells nao acha vao nenhum que se destaque
// dentro de uma linha densa (o caso comum de um item de tabela: unidade,
// quantidade, preco e descontos ficam a distancias parecidas umas das
// outras, sem um vao maior que marque fronteira de coluna) -- mas cada um
// desses campos ainda e uma regiao detectada a parte, com posicao propria,
// e dá para alinhar por coluna sem precisar decidir onde cortar a linha
// primeiro.
//
// O preco: uma linha comum (nao tabular), onde duas ou mais palavras
// formam uma frase corrida ("Razão Social: RODRIGUES..."), tambem vira
// uma celula por palavra -- fragmenta o que SplitCells trataria como um
// campo so. Use GroupTable para texto corrido, GroupTableWords quando o
// bloco de linhas já se sabe ser uma tabela.
func GroupTableWords(linhas []Line) Table {
	celulasPorLinha := make([][]Cell, len(linhas))
	for i, l := range linhas {
		celulas := make([]Cell, len(l.Words))
		for j, w := range l.Words {
			celulas[j] = Cell{Words: []Word{w}}
		}
		celulasPorLinha[i] = celulas
	}
	return agruparEmColunas(celulasPorLinha)
}

// agruparEmColunas e o algoritmo de fundir intervalos que GroupTable e
// GroupTableWords compartilham, recebendo as celulas de cada linha ja
// decididas (por SplitCells ou uma por palavra).
//
// Junta todas as celulas de todas as linhas, ordena pela borda esquerda,
// e cada celula ou estende a coluna aberta mais recente (se a borda
// esquerda dela cair dentro da coluna) ou abre uma coluna nova. Celulas de
// linhas DIFERENTES que ocupam a mesma faixa horizontal -- o codigo do
// produto de uma linha embaixo do codigo da linha anterior, por exemplo --
// caem na mesma coluna; a ordem em que as linhas foram percorridas nao
// importa para decidir as colunas, so a posicao.
//
// Isso funciona bem quando as colunas de verdade do documento tem faixas
// horizontais que nao se sobrepoem entre si -- o caso comum de uma tabela
// desenhada com colunas alinhadas. NAO tenta decidir se uma celula que
// sobra "deveria" ter ficado em outra coluna por semantica (um valor que
// vazou pra fora da grade, por exemplo); isso e responsabilidade de quem
// usa o resultado.
func agruparEmColunas(celulasPorLinha [][]Cell) Table {
	type ocorrencia struct {
		linha, ordem int
		min, max     float64
	}

	var todas []ocorrencia
	for i, celulas := range celulasPorLinha {
		for j, c := range celulas {
			min, max := c.Bounds()
			todas = append(todas, ocorrencia{linha: i, ordem: j, min: min.X, max: max.X})
		}
	}
	if len(todas) == 0 {
		return Table{Rows: make([][]Cell, len(celulasPorLinha))}
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

	rows := make([][]Cell, len(celulasPorLinha))
	for i, celulas := range celulasPorLinha {
		row := make([]Cell, len(colunas))
		for j, c := range celulas {
			row[colunaDe[[2]int{i, j}]] = c
		}
		rows[i] = row
	}

	return Table{Rows: rows}
}
