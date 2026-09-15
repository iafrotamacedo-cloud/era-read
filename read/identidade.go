package read

import (
	"github.com/iafrotamacedo-cloud/era-read/detect"
	"github.com/iafrotamacedo-cloud/era-read/dewarp"
)

// MotorVersao identifica o binario do ERA READ que produziu uma leitura.
// O REGEN usa isto para saber contra qual motor treinar.
const MotorVersao = "era-read-0.4.0"

// ContratoVersao e a versao do JSON que o FrotaHub persiste (espelho do
// pacote contrato do era-regen).
const ContratoVersao = "a0"

// OverlapLayout e o unico limiar publico do layout de uma coluna -- o
// valor que GroupLines usa internamente hoje.
const OverlapLayout = 0.5

// Identidade descreve motor, pesos e limiares de uma leitura. O
// orquestrador copia isto para contrato.LeituraERA sem o READ importar o
// era-regen.
type Identidade struct {
	MotorVersao    string
	ONNXDetHash    string
	ONNXRecHash    string
	Detect         detect.Options
	Dewarp         dewarp.Thresholds
	OverlapLayout  float64
	ContratoVersao string
}

// IdentidadeDe monta a identidade a partir das opcoes efetivas e dos
// hashes dos arquivos .onnx usados na leitura.
func IdentidadeDe(opts Options, detHash, recHash string) Identidade {
	return Identidade{
		MotorVersao:    MotorVersao,
		ONNXDetHash:    detHash,
		ONNXRecHash:    recHash,
		Detect:         opts.Detector,
		Dewarp:         opts.Thresholds,
		OverlapLayout:  OverlapLayout,
		ContratoVersao: ContratoVersao,
	}
}
