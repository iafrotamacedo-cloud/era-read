package read

import (
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/iafrotamacedo-cloud/era-read/detect"
	"github.com/iafrotamacedo-cloud/era-read/dewarp"
	"github.com/iafrotamacedo-cloud/era-read/geom"
	"github.com/iafrotamacedo-cloud/era-read/graph"
	"github.com/iafrotamacedo-cloud/era-read/imgproc"
	"github.com/iafrotamacedo-cloud/era-read/onnx"
	"github.com/iafrotamacedo-cloud/era-read/recog"
)

// redeReconhecimentoDeBrinquedo monta um grafo minusculo, sem nenhuma
// pretensao de reconhecer texto de verdade: GlobalAveragePool reduz a
// entrada [1,3,H,W] a uma media por canal, Flatten junta os 3 canais numa
// linha, e Gemm projeta em 3 classes (branco, "X", "Y") com pesos
// escolhidos a mao para que a media dos pixels decida a classe vencedora --
// media alta (~+1, imagem clara) vence "X", media baixa (~-1, imagem
// escura) vence "Y". So serve para testar a FIACAO
// (Preprocess -> grafo -> DecodeCTC -> layout), nao reconhecimento de
// verdade -- essa validacao ja existe contra o modelo real (ver README).
func redeReconhecimentoDeBrinquedo(t *testing.T) *graph.Graph {
	t.Helper()

	no := func(opType, nome string, ins, outs []string, attrs ...*onnx.Attribute) *onnx.Node {
		return &onnx.Node{OpType: opType, Name: nome, Inputs: ins, Outputs: outs, Attributes: attrs}
	}
	aInt := func(nome string, v int64) *onnx.Attribute {
		return &onnx.Attribute{Name: nome, Type: onnx.AttrInt, I: v}
	}
	peso := func(nome string, dims []int64, vals []float32) *onnx.Tensor {
		return &onnx.Tensor{Name: nome, DataType: onnx.Float, Dims: dims, FloatData: vals}
	}

	m := &onnx.Model{
		IRVersion:    8,
		OpsetImports: []onnx.OpsetID{{Version: 13}},
		Graph: &onnx.Graph{
			Name: "brinquedo",
			Nodes: []*onnx.Node{
				no("GlobalAveragePool", "pool", []string{"x"}, []string{"pooled"}),
				no("Flatten", "flat", []string{"pooled"}, []string{"flat"}, aInt("axis", 1)),
				no("Gemm", "fc", []string{"flat", "w", "b"}, []string{"logits"}, aInt("transB", 1)),
				no("Unsqueeze", "t", []string{"logits"}, []string{"y"}, &onnx.Attribute{
					Name: "axes", Type: onnx.AttrInts, Ints: []int64{1},
				}),
			},
			Initializers: []*onnx.Tensor{
				// linha 0 (branco): sempre 0. linha 1 ("X"): media dos 3
				// canais (pesos 1/3). linha 2 ("Y"): o negativo disso.
				peso("w", []int64{3, 3}, []float32{
					0, 0, 0,
					1.0 / 3, 1.0 / 3, 1.0 / 3,
					-1.0 / 3, -1.0 / 3, -1.0 / 3,
				}),
				peso("b", []int64{3}, []float32{0, 0, 0}),
			},
			Inputs:  []*onnx.ValueInfo{{Name: "x"}},
			Outputs: []*onnx.ValueInfo{{Name: "y"}},
		},
	}

	g, err := graph.New(m)
	if err != nil {
		t.Fatalf("montar rede de brinquedo: %v", err)
	}
	return g
}

func imagemDeFundo(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	cinza := color.NRGBA{128, 128, 128, 255}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, cinza)
		}
	}
	return img
}

func preencherRetangulo(img *image.NRGBA, x0, y0, x1, y1 int, c color.NRGBA) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
}

func preencherProb(g *imgproc.Gray, x0, y0, x1, y1 int, v float32) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			g.Set(x, y, v)
		}
	}
}

// preencherCapsulaCor e preencherCapsulaProb desenham a mesma forma nas
// duas imagens: um retangulo com as pontas arredondadas (capsula), a forma
// que uma regiao de texto expandida por Unclip realmente tem -- Unclip
// empurra cada vertice pela bissetriz do canto, entao um contorno
// originalmente reto sai com canto arredondado, nao em esquadro. Um
// retangulo perfeito (canto reto) tem a borda esquerda inteira empatada no
// X minimo, e extremosHorizontais (detect/baseline.go) so acha um unico
// vertice de extremo quando o canto e mesmo arredondado -- com canto reto
// o desempate cai nos vertices do topo, e ExtractBaseline devolve uma
// baseline com as pontas contaminadas pelo Y do topo. Ver o README.
func preencherCapsulaCor(img *image.NRGBA, x0, y0, x1, y1 int, c color.NRGBA) {
	varrerCapsula(x0, y0, x1, y1, func(x, y int) { img.SetNRGBA(x, y, c) })
}

func preencherCapsulaProb(g *imgproc.Gray, x0, y0, x1, y1 int, v float32) {
	varrerCapsula(x0, y0, x1, y1, func(x, y int) { g.Set(x, y, v) })
}

func varrerCapsula(x0, y0, x1, y1 int, pintar func(x, y int)) {
	raio := float64(y1-y0) / 2
	cy := float64(y0+y1) / 2
	for y := y0; y < y1; y++ {
		dy := float64(y) - cy
		// dentro do raio das pontas: recua em x pela metade de um circulo,
		// arredondando as duas pontas em vez de deixar canto reto.
		recuo := 0.0
		if dy*dy < raio*raio {
			recuo = raio - math.Sqrt(raio*raio-dy*dy)
		}
		for x := x0 + int(recuo+0.5); x < x1-int(recuo+0.5); x++ {
			pintar(x, y)
		}
	}
}

// duasRegioesDetectadas monta uma pagina colorida (uma faixa branca e uma
// preta, lado a lado, na mesma altura) e o mapa de probabilidade
// correspondente, e roda detect.Detect de verdade para obter os poligonos
// -- densos, vindos de contorno rastreado e expandidos por Unclip, o
// formato real que ToLinePolygon espera (ao contrario de um quadrilatero
// de 4 vertices desenhado a mao, que quebra a suposicao de reamostragem
// por indice quando pontosPorBorda > 2). Retangulos bem mais largos que
// altos, bem separados, para que a expansao do Unclip nao funda os dois
// nem estoure a proporcao minima do recorte (evitando padding em
// recog.Preprocess).
func duasRegioesDetectadas(t *testing.T) (pagina *image.NRGBA, regioes []detect.Result) {
	t.Helper()
	const w, h = 900, 80
	pagina = imagemDeFundo(w, h)
	prob := imgproc.NewGray(w, h)

	// Retangulos bem mais largos que altos (~15:1) -- mesmo depois da
	// margem que Unclip acrescenta em volta, a proporcao fica alta o
	// bastante para nao esbarrar na limitacao conhecida de
	// detect.ToLinePolygon com formas perto do quadrado (ver nota no
	// pacote detect e no README): a divisao cima/baixo reamostra por
	// INDICE ao longo do contorno, e as pontas verticais (esquerda/direita)
	// de uma regiao pouco alongada acabam competindo por indices com a
	// borda de baixo de verdade, contaminando os pontos da baseline perto
	// das extremidades.
	preencherCapsulaCor(pagina, 10, 15, 310, 35, color.NRGBA{255, 255, 255, 255})
	preencherCapsulaProb(prob, 10, 15, 310, 35, 1.0)

	preencherCapsulaCor(pagina, 500, 15, 800, 35, color.NRGBA{0, 0, 0, 255})
	preencherCapsulaProb(prob, 500, 15, 800, 35, 1.0)

	regioes = detect.Detect(prob, detect.DefaultOptions())
	if len(regioes) != 2 {
		t.Fatalf("detect.Detect achou %d regioes, quero 2 (fixture do teste errada?)", len(regioes))
	}
	return pagina, regioes
}

// TestProcessarRegioesFiacaoCompleta cobre a fiacao inteira depois da
// deteccao: retificar (N0/N1, ja que as duas regioes sao retas) e
// reconhecer cada regiao, e agrupar o resultado em linhas de leitura. As
// duas regioes estao na mesma faixa vertical -- devem virar uma linha so,
// na ordem esquerda-direita, a clara ("X") antes da escura ("Y").
func TestProcessarRegioesFiacaoCompleta(t *testing.T) {
	pagina, regioes := duasRegioesDetectadas(t)
	recGraph := redeReconhecimentoDeBrinquedo(t)
	cs := recog.NewCharset([]string{"X", "Y"})

	linhas, err := ProcessarRegioes(pagina, regioes, detect.Scale{X: 1, Y: 1}, recGraph, cs, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if len(linhas) != 1 {
		t.Fatalf("linhas = %d, quero 1 (as duas regioes estao na mesma faixa vertical)", len(linhas))
	}
	if got := linhas[0].Text(); got != "X Y" {
		t.Errorf("texto da linha = %q, quero %q", got, "X Y")
	}
	if len(linhas[0].Words) != 2 {
		t.Fatalf("palavras na linha = %d, quero 2", len(linhas[0].Words))
	}
	if linhas[0].Words[0].Confidence <= 0 {
		t.Errorf("confianca da primeira palavra = %v, queria > 0", linhas[0].Words[0].Confidence)
	}
}

// TestProcessarRegioesDescartaRegiaoDegenerada confere que um poligono que
// nao da para virar linha (menos de 3 vertices) e silenciosamente
// descartado, sem derrubar o resto da pagina -- uma regiao ruim entre
// varias boas nao pode travar tudo.
func TestProcessarRegioesDescartaRegiaoDegenerada(t *testing.T) {
	pagina, regioes := duasRegioesDetectadas(t)
	regioes = append(regioes, detect.Result{
		Polygon: geom.Polygon{{X: 0, Y: 0}, {X: 1, Y: 1}}, Score: 1, // degenerado: 2 vertices
	})

	recGraph := redeReconhecimentoDeBrinquedo(t)
	cs := recog.NewCharset([]string{"X", "Y"})

	linhas, err := ProcessarRegioes(pagina, regioes, detect.Scale{X: 1, Y: 1}, recGraph, cs, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if len(linhas) != 1 || linhas[0].Text() != "X Y" {
		t.Fatalf("a regiao degenerada deveria so ser ignorada, tem %v", linhas)
	}
}

// TestLimiaresEfetivos cobre o bug real achado num documento real: um
// campo curto (um valor monetario de poucos caracteres, ~30px de altura)
// tem ruido geometrico de contorno da mesma ordem de grandeza ABSOLUTA
// que uma linha de texto inteira -- alguns pixels -- mas esses pixels sao
// uma fracao muito maior da altura de um campo pequeno. Dois valores
// reais de "Total a pagar:" (~30px de altura, LinearBow medido de 7,09 e
// 4,03px) saiam classificados N3 com o limiar absoluto de
// dewarp.DefaultThresholds (1,5px) e eram descartados inteiros -- N3
// ainda nao tem implementacao.
func TestLimiaresEfetivos(t *testing.T) {
	base := dewarp.DefaultThresholds() // RetoPx=1.5, CurvoPx=1.5

	t.Run("regiao pequena real: LinearBow que era N3 vira folga suficiente", func(t *testing.T) {
		got := limiaresEfetivos(base, 30, 0.3) // altura ~30px, o caso real medido
		if got.RetoPx < 7.09 {
			t.Errorf("RetoPx = %v, precisa ser >= 7.09 para aceitar o caso real medido", got.RetoPx)
		}
		if got.CurvoPx < 7.09 {
			t.Errorf("CurvoPx = %v, precisa ser >= 7.09", got.CurvoPx)
		}
	})

	t.Run("nunca fica mais apertado que o absoluto", func(t *testing.T) {
		got := limiaresEfetivos(base, 1, 0.3) // altura minuscula: 1*0.3=0.3 < 1.5
		if got.RetoPx != base.RetoPx || got.CurvoPx != base.CurvoPx {
			t.Errorf("limiares = %+v, quero o absoluto de base (%+v) intacto", got, base)
		}
	})

	t.Run("fator zero desliga o ajuste", func(t *testing.T) {
		got := limiaresEfetivos(base, 1000, 0)
		if got.RetoPx != base.RetoPx || got.CurvoPx != base.CurvoPx {
			t.Errorf("limiares = %+v, quero o absoluto de base (%+v) sem ajuste", got, base)
		}
	})

	t.Run("AnguloRad nao muda", func(t *testing.T) {
		got := limiaresEfetivos(base, 30, 0.3)
		if got.AnguloRad != base.AnguloRad {
			t.Errorf("AnguloRad = %v, nao deveria mudar (so RetoPx/CurvoPx escalam)", got.AnguloRad)
		}
	})
}

// TestReconhecerRegiaoCampoPequeno confere, de ponta a ponta, que uma
// regiao pequena e reta (a mesma ordem de grandeza do "100,60" real que
// motivou o ajuste) e reconhecida com as opcoes padrao.
func TestReconhecerRegiaoCampoPequeno(t *testing.T) {
	const w, h = 900, 80
	pagina := imagemDeFundo(w, h)
	prob := imgproc.NewGray(w, h)

	preencherCapsulaCor(pagina, 400, 28, 470, 52, color.NRGBA{255, 255, 255, 255})
	preencherCapsulaProb(prob, 400, 28, 470, 52, 1.0)

	regioes := detect.Detect(prob, detect.DefaultOptions())
	if len(regioes) != 1 {
		t.Fatalf("detect.Detect achou %d regioes, quero 1 (fixture do teste errada?)", len(regioes))
	}

	recGraph := redeReconhecimentoDeBrinquedo(t)
	cs := recog.NewCharset([]string{"X", "Y"})

	_, texto, _, ok := reconhecerRegiao(pagina, regioes[0], detect.Scale{X: 1, Y: 1}, recGraph, cs, DefaultOptions())
	if !ok || texto == "" {
		t.Errorf("regiao pequena e reta deveria ser reconhecida (ok=%v texto=%q)", ok, texto)
	}
}

func TestProcessarRegioesExigeGrafoDeUmaEntradaUmaSaida(t *testing.T) {
	no := func(opType, nome string, ins, outs []string) *onnx.Node {
		return &onnx.Node{OpType: opType, Name: nome, Inputs: ins, Outputs: outs}
	}
	m := &onnx.Model{
		IRVersion:    8,
		OpsetImports: []onnx.OpsetID{{Version: 13}},
		Graph: &onnx.Graph{
			Nodes:   []*onnx.Node{no("Relu", "r", []string{"x"}, []string{"y"})},
			Inputs:  []*onnx.ValueInfo{{Name: "x"}},
			Outputs: []*onnx.ValueInfo{{Name: "y"}, {Name: "x"}}, // 2 saidas, de proposito
		},
	}
	g, err := graph.New(m)
	if err != nil {
		t.Fatal(err)
	}

	pagina := imagemDeFundo(20, 20)
	_, err = ProcessarRegioes(pagina, nil, detect.Scale{X: 1, Y: 1}, g, recog.NewCharset(nil), DefaultOptions())
	if err == nil {
		t.Error("grafo com 2 saidas deveria dar erro")
	}
}
