package read

import (
	"github.com/iafrotamacedo-cloud/era-read/contrato"
	"github.com/iafrotamacedo-cloud/era-read/detect"
	"github.com/iafrotamacedo-cloud/era-read/dewarp"
	"github.com/iafrotamacedo-cloud/era-read/layout"
)

// OptionsFromFiltro aplica o payload de filtro_read em cima dos padroes
// da literatura. Campos omitidos (zero) nao sobrescrevem o default -- o
// REGEN so manda o que mediu. URIs de .onnx sao ignoradas: quem carrega
// o grafo e o chamador.
func OptionsFromFiltro(f contrato.Filtro) Options {
	opts := DefaultOptions()
	opts.MotorVersao = f.MotorVersao
	opts.ONNXDetHash = f.ONNXDetHash
	opts.ONNXRecHash = f.ONNXRecHash
	if !f.Detect.Zerado() {
		opts.Detector = detect.Options{
			Threshold:   f.Detect.Threshold,
			MinArea:     f.Detect.MinArea,
			MinScore:    f.Detect.MinScore,
			UnclipRatio: f.Detect.UnclipRatio,
		}
	}
	if !f.Dewarp.Zerado() {
		opts.Thresholds = dewarp.Thresholds{
			RetoPx:    f.Dewarp.RetoPx,
			CurvoPx:   f.Dewarp.CurvoPx,
			AnguloRad: f.Dewarp.AnguloRad,
		}
	}
	if !f.Layout.Zerado() {
		opts.Layout = layout.Options{
			OverlapFraction: f.Layout.OverlapFraction,
			MaxDriftFactor:  f.Layout.MaxDriftFactor,
		}
	}
	return opts
}
