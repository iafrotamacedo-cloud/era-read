package contrato

import (
	"encoding/json"
	"fmt"
)

// Filtro e o payload da tabela filtro_read: o que o REGEN publica e o
// FrotaHub passa ao motor na leitura seguinte. O motor so consome --
// nunca grava de volta, nunca baixa .onnx, nunca abre o Postgres.
//
// URIs de peso sao para o chamador carregar o grafo; hashes e limiares
// entram em Identidade e em read.Options.
type Filtro struct {
	Versao         string           `json:"versao"`
	MotorVersao    string           `json:"motor_versao"`
	ONNXDetURI     string           `json:"onnx_det_uri,omitempty"`
	ONNXRecURI     string           `json:"onnx_rec_uri,omitempty"`
	ONNXDetHash    string           `json:"onnx_det_hash,omitempty"`
	ONNXRecHash    string           `json:"onnx_rec_hash,omitempty"`
	Detect         DetectOptions    `json:"detect"`
	Dewarp         DewarpThresholds `json:"dewarp"`
	Layout         LayoutOptions    `json:"layout"`
	ContratoVersao string           `json:"contrato_versao"`
}

// ParseFiltro decodifica o JSONB de filtro_read. Nao aplica default --
// quem transforma em read.Options e read.OptionsFromFiltro, que completa
// zero com os padroes da literatura.
func ParseFiltro(raw []byte) (Filtro, error) {
	var f Filtro
	if err := json.Unmarshal(raw, &f); err != nil {
		return Filtro{}, fmt.Errorf("contrato: filtro_read: %w", err)
	}
	return f, nil
}

// DetectZerado e true se o bloco detect veio vazio no JSON (todos os
// campos no zero-value). Distingue "omitido, usa default" de "mandou
// threshold 0 de proposito" -- zero nao e um corte util no mapa de
// probabilidade, entao trata como omitido.
func (d DetectOptions) Zerado() bool {
	return d.Threshold == 0 && d.MinArea == 0 && d.MinScore == 0 && d.UnclipRatio == 0
}

// DewarpZerado e true se o bloco dewarp veio vazio no JSON.
func (d DewarpThresholds) Zerado() bool {
	return d.RetoPx == 0 && d.CurvoPx == 0 && d.AnguloRad == 0
}

// LayoutZerado e true se o bloco layout veio vazio no JSON.
func (l LayoutOptions) Zerado() bool {
	return l.OverlapFraction == 0 && l.MaxDriftFactor == 0
}
