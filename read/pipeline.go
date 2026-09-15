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
	"github.com/iafrotamacedo-cloud/era-read/layout"
	"github.com/iafrotamacedo-cloud/era-read/recog"
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

	// FatorDeformacaoRelativo escala Thresholds.RetoPx/CurvoPx pela altura
	// de CADA região, em vez de usar só o valor absoluto de Thresholds --
	// o maior dos dois vale (nunca fica mais apertado que o absoluto).
	//
	// Achado num documento real: um campo curto (um valor monetário de 6
	// caracteres, ~30px de altura) tem ruído geométrico de contorno da
	// mesma ordem de grandeza absoluta que uma linha de texto inteira --
	// alguns pixels -- mas esses mesmos pixels são uma fração muito maior
	// da altura de um campo pequeno. Com só o limiar absoluto (1,5px),
	// dois valores de "Total a pagar:" de um documento real, perfeitamente
	// retos, saíam classificados N3 (curvatura irregular) e eram
	// descartados inteiros -- N3 ainda não tem implementação. Relativo à
	// altura, o mesmo campo passa; uma linha de texto normal (bem mais
	// alta) ganha uma folga maior em pixels absolutos, mas segue
	// proporcional ao próprio tamanho.
	//
	// Não é calibração medida contra curvatura de verdade -- os dois
	// documentos reais que validam isto são de papel liso, nenhum com
	// dobra ou curvatura de fato para testar se o limiar mais largo deixa
	// passar uma curva que devia ser N3. Ponto de partida, como
	// dewarp.DefaultThresholds já se declara.
	FatorDeformacaoRelativo float64
}

// DefaultOptions combina os padrões que cada fase já define sozinha.
func DefaultOptions() Options {
	return Options{
		Detector:                detect.DefaultOptions(),
		PreDetect:               detect.DefaultPreprocessOptions(),
		PreRecog:                recog.DefaultPreprocessOptions(),
		Thresholds:              dewarp.DefaultThresholds(),
		PontosPorBorda:          8,
		FatorDeformacaoRelativo: 0.3,
	}
}

// Page le uma pagina inteira e devolve as linhas agrupadas. Para regioes,
// page_level e identidade use PageScan.
func Page(src image.Image, detGraph, recGraph *graph.Graph, cs recog.Charset, opts Options) ([]layout.Line, error) {
	scan, err := PageScan(src, detGraph, recGraph, cs, opts)
	if err != nil {
		return nil, err
	}
	return scan.Lines, nil
}

// ProcessarRegioes retifica e reconhece regiões já detectadas -- o que
// sobra de Page depois de tirar a rede de detecção. Separado para poder
// testar a retificação e o reconhecimento sem depender de uma rede de
// detecção de verdade, e para quem já tem as regiões vindas de outro lugar.
func ProcessarRegioes(src image.Image, regioes []detect.Result, escala detect.Scale, recGraph *graph.Graph, cs recog.Charset, opts Options) ([]layout.Line, error) {
	scan, err := processarRegioesScan(src, regioes, escala, recGraph, cs, opts)
	if err != nil {
		return nil, err
	}
	return scan.Lines, nil
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
	med, mediu := medirRegiao(regiao, escala, opts)
	if !mediu {
		return nil, "", 0, false
	}
	return reconhecerRegiaoMedida(src, med, recGraph, cs, opts)
}

// limiaresEfetivos escala RetoPx/CurvoPx pela altura da regiao (ver o
// comentario de Options.FatorDeformacaoRelativo) -- o maior entre o
// absoluto de base e o relativo vale, nunca o menor: uma regiao muito
// pequena SO fica mais tolerante, nunca mais apertada que o padrao.
func limiaresEfetivos(base dewarp.Thresholds, altura, fator float64) dewarp.Thresholds {
	if r := altura * fator; r > base.RetoPx {
		base.RetoPx = r
	}
	if r := altura * fator; r > base.CurvoPx {
		base.CurvoPx = r
	}
	return base
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
