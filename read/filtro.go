package read

import (
	"encoding/json"
	"fmt"

	"github.com/iafrotamacedo-cloud/era-read/detect"
	"github.com/iafrotamacedo-cloud/era-read/dewarp"
)

// FiltroRead e o que o REGEN publica e o FrotaHub passa ao motor na leitura
// seguinte. Espelha o JSON em docs/REGEN.md do era-regen.
type FiltroRead struct {
	Versao         string              `json:"versao"`
	MotorVersao    string              `json:"motor_versao"`
	ONNXDetURI     string              `json:"onnx_det_uri,omitempty"`
	ONNXRecURI     string              `json:"onnx_rec_uri,omitempty"`
	Detect         detect.Options      `json:"detect"`
	Dewarp         dewarp.Thresholds   `json:"dewarp"`
	Layout         filtroLayout        `json:"layout"`
	ContratoVersao string              `json:"contrato_versao"`
}

type filtroLayout struct {
	OverlapFraction float64 `json:"overlap_fraction"`
}

// ParseFiltroRead decodifica o JSON publicado pelo REGEN.
func ParseFiltroRead(b []byte) (FiltroRead, error) {
	var f FiltroRead
	if err := json.Unmarshal(b, &f); err != nil {
		return FiltroRead{}, fmt.Errorf("read: filtro_read invalido: %w", err)
	}
	return f, nil
}

// AplicarFiltro sobrepoe os limiares de opts com os do filtro vigente.
// Nao troca os grafos ONNX -- quem carrega os pesos certos e o
// orquestrador, usando ONNXDetURI/ONNXRecURI fora desta funcao.
func AplicarFiltro(opts Options, f FiltroRead) Options {
	if f.MotorVersao != "" && f.MotorVersao != MotorVersao {
		// filtro de outra versao: ainda aplica limiares, mas o chamador
		// deve decidir se recusa o filtro inteiro.
	}
	opts.Detector = f.Detect
	opts.Thresholds = f.Dewarp
	return opts
}
