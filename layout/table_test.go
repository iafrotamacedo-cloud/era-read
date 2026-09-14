package layout

import "testing"

// linhaDeGrade monta uma Line com celulas nas posicoes X dadas (largura
// fixa de 30) -- cada posicao vira uma Word isolada (vao grande entre
// elas, de proposito, para SplitCells sempre separar em celulas distintas
// dentro da linha).
func linhaDeGrade(y float64, textos []string, xs []float64) Line {
	return linhaDeGradeComLargura(y, textos, xs, 30)
}

func linhaDeGradeComLargura(y float64, textos []string, xs []float64, largura float64) Line {
	ws := make([]Word, len(textos))
	for i, texto := range textos {
		ws[i] = Word{Box: caixa(xs[i], y, xs[i]+largura, y+20), Text: texto}
	}
	return Line{Words: ws}
}

func TestGroupTableGradeLimpa(t *testing.T) {
	// 3 linhas, 3 colunas cada, as mesmas faixas X em todas -- o caso
	// ideal de tabela bem desenhada.
	linhas := []Line{
		linhaDeGrade(0, []string{"Codigo", "Qtde", "Preco"}, []float64{0, 500, 700}),
		linhaDeGrade(30, []string{"001", "2", "10,00"}, []float64{0, 500, 700}),
		linhaDeGrade(60, []string{"002", "1", "5,00"}, []float64{0, 500, 700}),
	}

	tbl := GroupTable(linhas, DefaultGapFactor)
	if got := tbl.NumColumns(); got != 3 {
		t.Fatalf("NumColumns = %d, quero 3", got)
	}
	if len(tbl.Rows) != 3 {
		t.Fatalf("len(Rows) = %d, quero 3", len(tbl.Rows))
	}

	quero := [][]string{
		{"Codigo", "Qtde", "Preco"},
		{"001", "2", "10,00"},
		{"002", "1", "5,00"},
	}
	for i, linha := range quero {
		for j, texto := range linha {
			if got := tbl.Rows[i][j].Text(); got != texto {
				t.Errorf("Rows[%d][%d] = %q, quero %q", i, j, got, texto)
			}
		}
	}
}

func TestGroupTableLinhaComMenosCelulas(t *testing.T) {
	// a segunda linha nao tem nada na 3a coluna -- um total sem valor de
	// desconto, por exemplo.
	linhas := []Line{
		linhaDeGrade(0, []string{"A", "B", "C"}, []float64{0, 500, 700}),
		linhaDeGrade(30, []string{"X", "Y"}, []float64{0, 500}),
	}
	tbl := GroupTable(linhas, DefaultGapFactor)
	if got := tbl.NumColumns(); got != 3 {
		t.Fatalf("NumColumns = %d, quero 3", got)
	}
	if got := tbl.Rows[1][2].Text(); got != "" {
		t.Errorf("Rows[1][2] = %q, quero vazio (linha nao tem celula nessa coluna)", got)
	}
	if got := tbl.Rows[1][0].Text(); got != "X" {
		t.Errorf("Rows[1][0] = %q, quero %q", got, "X")
	}
}

func TestGroupTableColunaDeslocada(t *testing.T) {
	// a largura da celula 0 varia entre as linhas (texto mais longo numa
	// linha, mais curto na outra) mas as faixas X ainda se sobrepoem --
	// deve continuar na mesma coluna, nao abrir uma nova.
	linhas := []Line{
		linhaDeGradeComLargura(0, []string{"AAAA", "1"}, []float64{0, 500}, 150),
		linhaDeGradeComLargura(30, []string{"B", "2"}, []float64{0, 500}, 30),
	}
	tbl := GroupTable(linhas, DefaultGapFactor)
	if got := tbl.NumColumns(); got != 2 {
		t.Fatalf("NumColumns = %d, quero 2 (as duas linhas devem cair nas mesmas 2 colunas)", got)
	}
	if got := tbl.Rows[0][0].Text(); got != "AAAA" {
		t.Errorf("Rows[0][0] = %q, quero %q", got, "AAAA")
	}
	if got := tbl.Rows[1][0].Text(); got != "B" {
		t.Errorf("Rows[1][0] = %q, quero %q", got, "B")
	}
}

// TestGroupTableWordsAlinhaCampoDeItem cobre o caso real que motivou
// GroupTableWords: uma linha de item de tabela ja corrigida pelo quarto
// bug (codigo+descricao, unidade, quantidade, preco, tudo numa Line so),
// onde os vaos entre campos vizinhos sao parecidos demais para SplitCells
// achar uma fronteira -- GroupTable (via SplitCells) colapsaria tudo numa
// celula so; GroupTableWords usa cada palavra ja detectada separadamente
// como sua propria celula.
func TestGroupTableWordsAlinhaCampoDeItem(t *testing.T) {
	// duas linhas de item, mesmas colunas (descricao, unidade, quantidade,
	// preco), vaos pequenos e parecidos entre campos vizinhos -- o que
	// faria SplitCells juntar tudo numa celula so.
	linhas := []Line{
		linhaDeGrade(0, []string{"COD1 - PRODUTO A", "UN", "1,000", "10,00"}, []float64{0, 50, 100, 150}),
		linhaDeGrade(30, []string{"COD2 - PRODUTO B", "UN", "2,000", "5,00"}, []float64{0, 50, 100, 150}),
	}

	// confere a premissa: SplitCells colapsa mesmo (vaos pequenos demais
	// para o gapFactor padrao separar).
	if len(linhas[0].SplitCells(DefaultGapFactor)) != 1 {
		t.Fatal("premissa do teste furou: SplitCells nao deveria achar fronteira nenhuma aqui")
	}

	tbl := GroupTableWords(linhas)
	if got := tbl.NumColumns(); got != 4 {
		t.Fatalf("NumColumns = %d, quero 4 (uma por palavra)", got)
	}
	quero := [][]string{
		{"COD1 - PRODUTO A", "UN", "1,000", "10,00"},
		{"COD2 - PRODUTO B", "UN", "2,000", "5,00"},
	}
	for i, linha := range quero {
		for j, texto := range linha {
			if got := tbl.Rows[i][j].Text(); got != texto {
				t.Errorf("Rows[%d][%d] = %q, quero %q", i, j, got, texto)
			}
		}
	}
}

func TestGroupTableWordsSemLinhas(t *testing.T) {
	tbl := GroupTableWords(nil)
	if len(tbl.Rows) != 0 {
		t.Errorf("Rows = %v, quero vazio", tbl.Rows)
	}
}

func TestGroupTableSemLinhas(t *testing.T) {
	tbl := GroupTable(nil, DefaultGapFactor)
	if len(tbl.Rows) != 0 {
		t.Errorf("Rows = %v, quero vazio", tbl.Rows)
	}
}

func TestGroupTableLinhasSemPalavras(t *testing.T) {
	tbl := GroupTable([]Line{{}, {}}, DefaultGapFactor)
	if len(tbl.Rows) != 2 {
		t.Fatalf("len(Rows) = %d, quero 2", len(tbl.Rows))
	}
	if tbl.NumColumns() != 0 {
		t.Errorf("NumColumns = %d, quero 0", tbl.NumColumns())
	}
}
