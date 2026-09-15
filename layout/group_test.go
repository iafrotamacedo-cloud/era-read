package layout

import (
	"math/rand"
	"testing"

	"github.com/iafrotamacedo-cloud/era-read/geom"
)

// caixa constroi um retangulo axis-aligned como poligono de 4 vertices --
// o suficiente para Box.Bounds() extrair a faixa vertical e horizontal
// certa, que e tudo que GroupLines usa.
func caixa(x0, y0, x1, y1 float64) geom.Polygon {
	return geom.Polygon{
		{X: x0, Y: y0}, {X: x1, Y: y0}, {X: x1, Y: y1}, {X: x0, Y: y1},
	}
}

func palavra(texto string, x0, y0, x1, y1 float64) Word {
	return Word{Box: caixa(x0, y0, x1, y1), Text: texto}
}

func TestGroupLinesVazio(t *testing.T) {
	if got := GroupLines(nil); got != nil {
		t.Errorf("GroupLines(nil) = %v, quero nil", got)
	}
}

func TestGroupLinesUmaPalavra(t *testing.T) {
	w := palavra("so", 0, 0, 10, 10)
	linhas := GroupLines([]Word{w})
	if len(linhas) != 1 || len(linhas[0].Words) != 1 {
		t.Fatalf("GroupLines = %+v, quero 1 linha com 1 palavra", linhas)
	}
	if linhas[0].Text() != "so" {
		t.Errorf("Text() = %q, quero %q", linhas[0].Text(), "so")
	}
}

// TestGroupLinesDuasLinhasEmbaralhadas e o teste principal: duas linhas
// bem separadas, tres palavras cada, entregues em ordem embaralhada.
// GroupLines tem que devolver as linhas de cima para baixo e, dentro de
// cada uma, as palavras da esquerda para a direita -- nunca a ordem em que
// foram passadas.
func TestGroupLinesDuasLinhasEmbaralhadas(t *testing.T) {
	entrada := []Word{
		palavra("123", 85, 10, 110, 20),
		palavra("R$", 45, 50, 55, 60),
		palavra("Nota", 0, 10, 30, 20),
		palavra("500,00", 60, 50, 110, 60),
		palavra("Fiscal", 35, 10, 80, 20),
		palavra("Total", 0, 50, 40, 60),
	}
	rnd := rand.New(rand.NewSource(5))
	rnd.Shuffle(len(entrada), func(i, j int) { entrada[i], entrada[j] = entrada[j], entrada[i] })

	linhas := GroupLines(entrada)
	if len(linhas) != 2 {
		t.Fatalf("GroupLines achou %d linhas, quero 2", len(linhas))
	}
	if got := linhas[0].Text(); got != "Nota Fiscal 123" {
		t.Errorf("linha 0 = %q, quero %q", got, "Nota Fiscal 123")
	}
	if got := linhas[1].Text(); got != "Total R$ 500,00" {
		t.Errorf("linha 1 = %q, quero %q", got, "Total R$ 500,00")
	}
}

// TestGroupLinesToleraJitterVertical confere que duas palavras com faixas
// verticais levemente diferentes (o normal numa deteccao real -- nem toda
// palavra da mesma linha tem exatamente a mesma altura) ainda caem juntas,
// desde que a sobreposicao passe do limiar.
func TestGroupLinesToleraJitterVertical(t *testing.T) {
	entrada := []Word{
		palavra("A", 0, 10, 20, 20),  // faixa [10,20], altura 10
		palavra("B", 25, 12, 45, 22), // faixa [12,22], altura 10 -- sobreposicao [12,20]=8, fracao 0.8
	}
	linhas := GroupLines(entrada)
	if len(linhas) != 1 {
		t.Fatalf("GroupLines achou %d linhas, quero 1 (jitter dentro do limiar)", len(linhas))
	}
	if got := linhas[0].Text(); got != "A B" {
		t.Errorf("linha = %q, quero %q", got, "A B")
	}
}

// TestGroupLinesSeparaSobreposicaoPequena e o caso oposto: sobreposicao
// abaixo do limiar tem que abrir duas linhas, nao uma.
func TestGroupLinesSeparaSobreposicaoPequena(t *testing.T) {
	entrada := []Word{
		palavra("A", 0, 10, 20, 20), // faixa [10,20], altura 10
		palavra("B", 0, 19, 20, 29), // faixa [19,29], altura 10 -- sobreposicao [19,20]=1, fracao 0.1
	}
	linhas := GroupLines(entrada)
	if len(linhas) != 2 {
		t.Fatalf("GroupLines achou %d linhas, quero 2 (sobreposicao abaixo do limiar)", len(linhas))
	}
}

// TestGroupLinesCadeiaDeSobreposicao cobre o bug real achado num item de
// tabela: 3 regioes da MESMA linha impressa (o valor de uma coluna, a
// descricao do produto no meio, a unidade de outra coluna), com faixas Y
// que se sobrepoem em cadeia mas nao todas contra a PRIMEIRA por ordem de
// Y -- os numeros sao os medidos de verdade em nota_real-1.png.
//
//	"Valor Total"   y=[447,477] altura=30  (a 1a por ordem de Y -- vira ancora)
//	descricao       y=[461,494] altura=33  (sobrepoe a ancora em 53%)
//	"UN"            y=[467,498] altura=31  (sobrepoe a ancora em so 33% --
//	                                        mas sobrepoe a descricao em 87%)
//
// Sem estender a faixa da linha conforme a descricao entra, "UN" cai numa
// linha separada -- exatamente o defeito que motivou a extensao com teto.
func TestGroupLinesCadeiaDeSobreposicao(t *testing.T) {
	entrada := []Word{
		palavra("VValorTotal", 1400, 447, 1500, 477),
		palavra("descricao", 0, 461, 400, 494),
		palavra("UN", 666, 467, 700, 498),
	}
	linhas := GroupLines(entrada)
	if len(linhas) != 1 {
		t.Fatalf("GroupLines achou %d linhas, quero 1 (as 3 regioes sao a mesma linha impressa)", len(linhas))
	}
	if got := linhas[0].Text(); got != "descricao UN VValorTotal" {
		t.Errorf("linha = %q, quero %q", got, "descricao UN VValorTotal")
	}
}

// TestGroupLinesNaoDerivaSemLimite e o contrapeso do teste acima: uma
// cadeia de palavras cada uma um pouco mais abaixo que a anterior, longa
// o bastante para que a faixa cresceria sem parar se não houvesse teto --
// e a preocupacao original que fez este pacote nascer sem extensao
// nenhuma. Com MaxDriftFactor, a cadeia tem que parar de crescer e abrir
// uma linha nova, nao engolir a pagina inteira numa linha so.
func TestGroupLinesNaoDerivaSemLimite(t *testing.T) {
	const altura = 10.0
	const passo = 4.0 // sobreposicao de 6 em 10 = 60%, acima do limiar
	var entrada []Word
	for i := 0; i < 12; i++ {
		y0 := float64(i) * passo
		entrada = append(entrada, palavra("w", float64(i)*15, y0, float64(i)*15+10, y0+altura))
	}

	linhas := GroupLines(entrada)
	// sem teto, a cadeia inteira (sobreposicao de 60% em cada passo) viraria
	// UMA linha so, do topo ao fim -- 12 palavras, ~54px de faixa vertical.
	// Com MaxDriftFactor, a faixa para de crescer depois de um tanto e uma
	// palavra distante o bastante abre linha nova.
	if len(linhas) < 2 {
		t.Fatalf("GroupLines achou %d linha(s), quero mais de uma -- a cadeia nao pode derivar sem limite", len(linhas))
	}
	if len(linhas[0].Words) == len(entrada) {
		t.Errorf("a primeira linha engoliu as %d palavras -- o teto de MaxDriftFactor nao freou a cadeia", len(entrada))
	}
}

// TestGroupLinesNaoFundeLinhasEmpilhadas cobre o outro lado do bug real:
// duas linhas de ITEM DIFERENTES, empilhadas de perto (pouco espaco entre
// elas, comum numa tabela), tem sobreposicao vertical alta -- geometricamente
// quase identica ao caso que motivou a extensao de MaxDriftFactor. A faixa
// de X e o que distingue: a mesma linha impressa ocupa colunas diferentes
// (nao se sobrepoe em X); duas linhas empilhadas repetem a coluna (o
// codigo do proximo item comeca na mesma posicao X do anterior).
func TestGroupLinesNaoFundeLinhasEmpilhadas(t *testing.T) {
	entrada := []Word{
		// item A: descricao (x=[0,300]) e quantidade (x=[400,450])
		palavra("00000000007105 - PANO DE CHAO", 0, 597, 300, 639),
		palavra("2,000", 400, 611, 450, 650),
		// item B, logo abaixo, mesma coluna de descricao -- sobreposicao Y
		// alta com o item A, mas MESMA faixa de X da descricao dele.
		palavra("00000000000442 - SERRA STARRETT", 0, 611, 300, 660),
	}
	linhas := GroupLines(entrada)
	if len(linhas) != 2 {
		t.Fatalf("GroupLines achou %d linha(s), quero 2 (sao dois itens diferentes empilhados)", len(linhas))
	}
}

func TestLineBounds(t *testing.T) {
	l := Line{Words: []Word{
		palavra("a", 0, 5, 10, 15),
		palavra("b", 20, 0, 30, 20),
	}}
	min, max := l.Bounds()
	if min != (geom.Point{X: 0, Y: 0}) {
		t.Errorf("min = %v, quero {0,0}", min)
	}
	if max != (geom.Point{X: 30, Y: 20}) {
		t.Errorf("max = %v, quero {30,20}", max)
	}
}

func TestGroupLinesOptsLimiarApertadoSepara(t *testing.T) {
	entrada := []Word{
		palavra("A", 0, 10, 20, 20),  // faixa [10,20]
		palavra("B", 25, 12, 45, 22), // sobreposicao 0.8 -- passa o default 0.5
	}
	if n := len(GroupLines(entrada)); n != 1 {
		t.Fatalf("default agrupou %d linhas, quero 1", n)
	}
	linhas := GroupLinesOpts(entrada, Options{OverlapFraction: 0.85})
	if len(linhas) != 2 {
		t.Fatalf("limiar 0.85 agrupou %d linhas, quero 2 (o filtro_read precisa conseguir apertar)", len(linhas))
	}
}
