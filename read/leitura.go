package read

import (
	"image"

	"github.com/iafrotamacedo-cloud/era-read/contrato"
	"github.com/iafrotamacedo-cloud/era-read/detect"
	"github.com/iafrotamacedo-cloud/era-read/dewarp"
	"github.com/iafrotamacedo-cloud/era-read/geom"
	"github.com/iafrotamacedo-cloud/era-read/graph"
	"github.com/iafrotamacedo-cloud/era-read/layout"
	"github.com/iafrotamacedo-cloud/era-read/recog"
)

// Pagina e o produto de uma passagem do motor: linhas na ordem de leitura
// mais o que o contrato de lancamento precisa (caixas, scores, deformacao).
type Pagina struct {
	Linhas      []layout.Line
	Regioes     []contrato.Region
	FormasLinha []contrato.LineShape
	PageLevel   dewarp.Level
}

// ProcessarPagina e ProcessarRegioes com a evidencia extra: toda caixa
// detectada entra em Regioes (mesmo se o recog falhar), e as formas
// medidas alimentam PageLevel.
func ProcessarPagina(src image.Image, regioes []detect.Result, escala detect.Scale, recGraph *graph.Graph, cs recog.Charset, opts Options) (Pagina, error) {
	if err := exigeEntradaSaidaUnica(recGraph, "reconhecimento"); err != nil {
		return Pagina{}, err
	}

	var (
		palavras []layout.Word
		outReg   []contrato.Region
		formas   []contrato.LineShape
		pior     dewarp.Level
	)
	for _, regiao := range regioes {
		poly := detect.Rescale(regiao.Polygon, escala)
		reg := contrato.Region{
			Polygon: polygonContrato(poly),
			Score:   regiao.Score,
		}

		linha, texto, confianca, forma, nivel, ok := reconhecerRegiao(src, regiao, escala, recGraph, cs, opts)
		if ok || nivel == dewarp.N3 {
			formas = append(formas, contrato.LineShape{
				Angle:     forma.Angle,
				LinearBow: forma.LinearBow,
				CurveBow:  forma.CurveBow,
			})
			if nivel > pior {
				pior = nivel
			}
		}
		if ok && texto != "" {
			reg.Text = texto
			reg.CTCConfidence = confianca
			palavras = append(palavras, layout.Word{Box: linha, Text: texto, Confidence: confianca})
		}
		outReg = append(outReg, reg)
	}

	return Pagina{
		Linhas:      layout.GroupLinesOpts(palavras, opts.Layout),
		Regioes:     outReg,
		FormasLinha: formas,
		PageLevel:   pior,
	}, nil
}

// MontarLeitura fecha o JSON que o FrotaHub congela em leitura_era.
// id, nota_id, imagem_uri, lancado_por e lancado_em ficam vazios -- o
// chamador preenche, o motor nao tem esses dados.
func MontarLeitura(p Pagina, opts Options) contrato.LeituraERA {
	doc := ExtrairDocumento(p.Linhas)
	return contrato.LeituraERA{
		Identidade:  identidadeDe(opts),
		Regioes:     p.Regioes,
		PageLevel:   p.PageLevel.String(),
		FormasLinha: p.FormasLinha,
		Campos:      CamposDoDocumento(doc),
	}
}

func identidadeDe(opts Options) contrato.Identidade {
	layoutOpts := contrato.LayoutOptions{
		OverlapFraction: opts.Layout.OverlapFraction,
		MaxDriftFactor:  opts.Layout.MaxDriftFactor,
	}
	if layoutOpts.OverlapFraction == 0 {
		layoutOpts = contrato.DefaultLayoutOptions()
	}
	return contrato.Identidade{
		MotorVersao: opts.MotorVersao,
		ONNXDetHash: opts.ONNXDetHash,
		ONNXRecHash: opts.ONNXRecHash,
		Detect: contrato.DetectOptions{
			Threshold:   opts.Detector.Threshold,
			MinArea:     opts.Detector.MinArea,
			MinScore:    opts.Detector.MinScore,
			UnclipRatio: opts.Detector.UnclipRatio,
		},
		Dewarp: contrato.DewarpThresholds{
			RetoPx:    opts.Thresholds.RetoPx,
			CurvoPx:   opts.Thresholds.CurvoPx,
			AnguloRad: opts.Thresholds.AnguloRad,
		},
		Layout:         layoutOpts,
		ContratoVersao: contrato.VersaoContrato,
	}
}

func polygonContrato(p geom.Polygon) contrato.Polygon {
	out := make(contrato.Polygon, len(p))
	for i, pt := range p {
		out[i] = contrato.Point{X: pt.X, Y: pt.Y}
	}
	return out
}
