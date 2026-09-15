package read

import (
	"fmt"
	"image"

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

// Regiao e uma detect.Result na escala da pagina, com medicao de dewarp e
// (quando o recog acerta) o texto lido.
type Regiao struct {
	Polygon       geom.Polygon
	Score         float32
	Level         dewarp.Level
	Shape         dewarp.LineShape
	Text          string
	CTCConfidence float32
}

// Scan e a saida completa de uma pagina: regioes do detector, nivel da
// pagina, formas medidas e linhas agrupadas. O FrotaHub e o era-regen
// usam isto para montar contrato.LeituraERA sem o READ importar o
// calibrador.
type Scan struct {
	Regioes   []Regiao
	Formas    []dewarp.LineShape
	PageLevel dewarp.Level
	Lines     []layout.Line
}

// PageScan igual a Page, mas devolve regioes e metadados de dewarp junto
// com as linhas reconhecidas.
func PageScan(src image.Image, detGraph, recGraph *graph.Graph, cs recog.Charset, opts Options) (Scan, error) {
	if err := exigeEntradaSaidaUnica(detGraph, "deteccao"); err != nil {
		return Scan{}, err
	}

	entradaDet, escala, err := detect.Preprocess(src, opts.PreDetect)
	if err != nil {
		return Scan{}, fmt.Errorf("read: pre-processar deteccao: %w", err)
	}

	ws := nn.NewWorkspace()
	saidasDet, err := detGraph.Run(ws, map[string]*tensor.Tensor{detGraph.Inputs()[0]: entradaDet})
	if err != nil {
		return Scan{}, fmt.Errorf("read: rodar grafo de deteccao: %w", err)
	}
	saidaDet := saidasDet[detGraph.Outputs()[0]]
	if saidaDet.Rank() != 4 {
		return Scan{}, fmt.Errorf("read: saida da deteccao tem forma %v, esperava [N,1,H,W]", saidaDet.Shape)
	}
	prob := &imgproc.Gray{
		Pix: saidaDet.Flat(), W: saidaDet.Shape[3], H: saidaDet.Shape[2], Stride: saidaDet.Shape[3],
	}

	regioes := detect.Detect(prob, opts.Detector)
	return processarRegioesScan(src, regioes, escala, recGraph, cs, opts)
}

func processarRegioesScan(src image.Image, regioes []detect.Result, escala detect.Scale, recGraph *graph.Graph, cs recog.Charset, opts Options) (Scan, error) {
	if err := exigeEntradaSaidaUnica(recGraph, "reconhecimento"); err != nil {
		return Scan{}, err
	 }

	var scan Scan
	var palavras []layout.Word

	for _, regiao := range regioes {
		poly := detect.Rescale(regiao.Polygon, escala)
		r := Regiao{Polygon: poly, Score: regiao.Score}

		med, ok := medirRegiao(regiao, escala, opts)
		if ok {
			r.Level = med.nivel
			r.Shape = med.forma
			scan.Formas = append(scan.Formas, med.forma)

			linha, texto, confianca, reconheceu := reconhecerRegiaoMedida(src, med, recGraph, cs, opts)
			if reconheceu && texto != "" {
				r.Text = texto
				r.CTCConfidence = confianca
				palavras = append(palavras, layout.Word{Box: linha, Text: texto, Confidence: confianca})
			}
		}

		scan.Regioes = append(scan.Regioes, r)
	}

	scan.PageLevel = dewarp.PageLevel(scan.Formas, opts.Thresholds)
	scan.Lines = layout.GroupLines(palavras)
	return scan, nil
}

type regiaoMedida struct {
	linha geom.Polygon
	forma dewarp.LineShape
	nivel dewarp.Level
	largura int
	altura  int
	minX    float64
	minY    float64
	maxX    float64
	maxY    float64
}

func medirRegiao(regiao detect.Result, escala detect.Scale, opts Options) (regiaoMedida, bool) {
	linhaRede, err := detect.ToLinePolygon(regiao.Polygon, opts.PontosPorBorda)
	if err != nil {
		return regiaoMedida{}, false
	}
	linha := detect.Rescale(linhaRede, escala)

	baseline, err := dewarp.ExtractBaseline(linha)
	if err != nil {
		return regiaoMedida{}, false
	}
	forma, err := dewarp.MeasureLine(baseline)
	if err != nil {
		return regiaoMedida{}, false
	}

	min, max := linha.Bounds()
	th := limiaresEfetivos(opts.Thresholds, max.Y-min.Y, opts.FatorDeformacaoRelativo)
	nivel := dewarp.Classify(forma, th)

	return regiaoMedida{
		linha:   linha,
		forma:   forma,
		nivel:   nivel,
		largura: arredondaPositivo(max.X - min.X),
		altura:  arredondaPositivo(max.Y - min.Y),
		minX:    min.X,
		minY:    min.Y,
		maxX:    max.X,
		maxY:    max.Y,
	}, true
}

func reconhecerRegiaoMedida(src image.Image, med regiaoMedida, recGraph *graph.Graph, cs recog.Charset, opts Options) (geom.Polygon, string, float32, bool) {
	var recorte image.Image
	var err error

	switch med.nivel {
	case dewarp.N0, dewarp.N1:
		cantos := [4]geom.Point{
			{X: med.minX, Y: med.minY}, {X: med.maxX, Y: med.minY},
			{X: med.maxX, Y: med.maxY}, {X: med.minX, Y: med.maxY},
		}
		recorte, err = recortarQuad(src, cantos, med.largura, med.altura)
	case dewarp.N2:
		baseline, err := dewarp.ExtractBaseline(med.linha)
		if err != nil {
			return nil, "", 0, false
		}
		acima := med.maxY - med.minY
		abaixo := acima * 0.15
		outH := arredondaPositivo(acima + abaixo)
		recorte, err = recortarCurva(src, baseline, med.largura, outH, acima, abaixo)
	default:
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
	return med.linha, resultados[0].Texto, resultados[0].Confianca, true
}
