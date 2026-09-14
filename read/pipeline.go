// Package read liga as fases já prontas numa única passagem: detecta
// regiões de texto numa página, retifica cada uma conforme a deformação
// medida e reconhece o texto, devolvendo linhas já na ordem de leitura.
//
// Cada fase continua podendo ser usada sozinha -- este pacote só é a cola
// entre elas, sem lógica nova de detecção, retificação, reconhecimento ou
// layout.
package read

import (
	"fmt"
	"image"
	"math"

	"github.com/iafrotamacedo-cloud/era-read/detect"
	"github.com/iafrotamacedo-cloud/era-read/dewarp"
	"github.com/iafrotamacedo-cloud/era-read/geom"
	"github.com/iafrotamacedo-cloud/era-read/graph"
	"github.com/iafrotamacedo-cloud/era-read/imgproc"
	"github.com/iafrotamacedo-cloud/era-read/layout"
	"github.com/iafrotamacedo-cloud/era-read/nn"
	"github.com/iafrotamacedo-cloud/era-read/recog"
	"github.com/iafrotamacedo-cloud/era-read/tensor"
)

// Options agrupa os ajustes de cada fase que este pacote encadeia.
type Options struct {
	Detector   detect.Options
	PreDetect  detect.PreprocessOptions
	PreRecog   recog.PreprocessOptions
	Thresholds dewarp.Thresholds

	// PontosPorBorda controla em quantos pontos cada borda (cima e baixo)
	// de uma região detectada é reamostrada antes de medir a deformação e
	// (se for N2) ajustar a curva -- ver detect.ToLinePolygon.
	PontosPorBorda int
}

// DefaultOptions combina os padrões que cada fase já define sozinha.
func DefaultOptions() Options {
	return Options{
		Detector:       detect.DefaultOptions(),
		PreDetect:      detect.DefaultPreprocessOptions(),
		PreRecog:       recog.DefaultPreprocessOptions(),
		Thresholds:     dewarp.DefaultThresholds(),
		PontosPorBorda: 8,
	}
}

// Page lê uma página inteira: roda a rede de detecção, retifica cada
// região encontrada conforme a deformação medida, roda a rede de
// reconhecimento em cada uma, e agrupa o resultado em linhas de leitura.
//
// detGraph e recGraph já vêm montados (graph.New) -- este pacote não
// carrega `.onnx` nenhum, pela mesma razão que nenhuma outra fase carrega:
// quem usa a biblioteca decide qual modelo trazer. cs é o dicionário do
// modelo de reconhecimento (ver recog.NewCharset).
func Page(src image.Image, detGraph, recGraph *graph.Graph, cs recog.Charset, opts Options) ([]layout.Line, error) {
	if err := exigeEntradaSaidaUnica(detGraph, "deteccao"); err != nil {
		return nil, err
	}

	entradaDet, escala, err := detect.Preprocess(src, opts.PreDetect)
	if err != nil {
		return nil, fmt.Errorf("read: pre-processar deteccao: %w", err)
	}

	ws := nn.NewWorkspace()
	saidasDet, err := detGraph.Run(ws, map[string]*tensor.Tensor{detGraph.Inputs()[0]: entradaDet})
	if err != nil {
		return nil, fmt.Errorf("read: rodar grafo de deteccao: %w", err)
	}
	saidaDet := saidasDet[detGraph.Outputs()[0]]
	if saidaDet.Rank() != 4 {
		return nil, fmt.Errorf("read: saida da deteccao tem forma %v, esperava [N,1,H,W]", saidaDet.Shape)
	}
	prob := &imgproc.Gray{
		Pix: saidaDet.Flat(), W: saidaDet.Shape[3], H: saidaDet.Shape[2], Stride: saidaDet.Shape[3],
	}

	regioes := detect.Detect(prob, opts.Detector)

	return ProcessarRegioes(src, regioes, escala, recGraph, cs, opts)
}

// ProcessarRegioes retifica e reconhece regiões já detectadas -- o que
// sobra de Page depois de tirar a rede de detecção. Separado para poder
// testar a retificação e o reconhecimento sem depender de uma rede de
// detecção de verdade, e para quem já tem as regiões vindas de outro lugar.
func ProcessarRegioes(src image.Image, regioes []detect.Result, escala detect.Scale, recGraph *graph.Graph, cs recog.Charset, opts Options) ([]layout.Line, error) {
	if err := exigeEntradaSaidaUnica(recGraph, "reconhecimento"); err != nil {
		return nil, err
	}

	var palavras []layout.Word
	for _, regiao := range regioes {
		linha, texto, confianca, ok := reconhecerRegiao(src, regiao, escala, recGraph, cs, opts)
		if !ok || texto == "" {
			continue
		}
		palavras = append(palavras, layout.Word{Box: linha, Text: texto, Confidence: confianca})
	}

	return layout.GroupLines(palavras), nil
}

// reconhecerRegiao mede a deformação de uma região, retifica no nível
// certo (N0/N1 por homografia, N2 por curva) e roda o reconhecedor.
// Devolve ok=false para qualquer região que não dê para processar --
// contorno degenerado demais para virar linha, N3 (ainda não implementado,
// precisa de rede), ou erro na própria retificação/reconhecimento --
// porque uma região ruim não pode derrubar a página inteira.
//
// linha (a região já convertida para o formato cima/baixo e na escala da
// página original) volta junto mesmo em caso de sucesso, para
// ProcessarRegioes não precisar refazer a mesma conversão.
func reconhecerRegiao(src image.Image, regiao detect.Result, escala detect.Scale, recGraph *graph.Graph, cs recog.Charset, opts Options) (linha geom.Polygon, texto string, confianca float32, ok bool) {
	linhaRede, err := detect.ToLinePolygon(regiao.Polygon, opts.PontosPorBorda)
	if err != nil {
		return nil, "", 0, false
	}
	linha = detect.Rescale(linhaRede, escala)

	baseline, err := dewarp.ExtractBaseline(linha)
	if err != nil {
		return nil, "", 0, false
	}
	forma, err := dewarp.MeasureLine(baseline)
	if err != nil {
		return nil, "", 0, false
	}
	nivel := dewarp.Classify(forma, opts.Thresholds)

	min, max := linha.Bounds()
	largura := arredondaPositivo(max.X - min.X)
	altura := arredondaPositivo(max.Y - min.Y)

	var recorte image.Image
	switch nivel {
	case dewarp.N0, dewarp.N1:
		// Cantos do retangulo que envolve a regiao inteira -- nao os 4
		// pontos "de cima/baixo" da propria linha (uma versao anterior
		// usava esses, via uma cantosDaLinha que foi removida): os pontos
		// que sobram depois de descartar as pontas contaminadas (a mesma
		// contaminacao corrigida em dewarp.ExtractBaseline) ficam alguns
		// pixels para DENTRO das bordas verdadeiras da regiao. Usar esses
		// pontos como canto da homografia mapeava o retangulo de saida
		// inteiro para uma faixa mais estreita da origem, cortando o
		// inicio/fim do texto -- um bug real, achado comparando o texto
		// lido contra o documento de verdade (ver README). O retangulo
		// envolvente nunca perde conteudo; o preco e nao corrigir a
		// perspectiva de uma linha N1 com precisao (so inclui uma margem
		// de fundo a mais nos cantos tortos), o que recog tolera bem.
		cantos := [4]geom.Point{
			{X: min.X, Y: min.Y}, {X: max.X, Y: min.Y},
			{X: max.X, Y: max.Y}, {X: min.X, Y: max.Y},
		}
		recorte, err = recortarQuad(src, cantos, largura, altura)
	case dewarp.N2:
		acima := max.Y - min.Y
		abaixo := acima * 0.15 // margem para descendentes -- ver RectifyLine
		outH := arredondaPositivo(acima + abaixo)
		recorte, err = recortarCurva(src, baseline, largura, outH, acima, abaixo)
	default: // N3: precisa de rede, ainda nao implementado (ver README)
		return nil, "", 0, false
	}
	if err != nil {
		return nil, "", 0, false
	}

	entrada, err := recog.Preprocess(recorte, opts.PreRecog)
	if err != nil {
		return nil, "", 0, false
	}

	ws := nn.NewWorkspace()
	saidas, err := recGraph.Run(ws, map[string]*tensor.Tensor{recGraph.Inputs()[0]: entrada})
	if err != nil {
		return nil, "", 0, false
	}
	saida := saidas[recGraph.Outputs()[0]]

	resultados, err := recog.DecodeCTC(saida, cs)
	if err != nil || len(resultados) == 0 {
		return nil, "", 0, false
	}
	return linha, resultados[0].Texto, resultados[0].Confianca, true
}

func arredondaPositivo(v float64) int {
	n := int(math.Round(v))
	if n < 1 {
		return 1
	}
	return n
}

func exigeEntradaSaidaUnica(g *graph.Graph, papel string) error {
	if len(g.Inputs()) != 1 {
		return fmt.Errorf("read: grafo de %s tem %d entradas, esperava 1", papel, len(g.Inputs()))
	}
	if len(g.Outputs()) != 1 {
		return fmt.Errorf("read: grafo de %s tem %d saidas, esperava 1", papel, len(g.Outputs()))
	}
	return nil
}
