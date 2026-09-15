// Package contrato e o JSON que o chamador (FrotaHub) persiste no
// lancamento e que o calibrador compara. Este pacote so descreve a forma
// -- nao abre banco, nao chama IA, nao sabe que o REGEN existe.
//
// A leitura original e sempre a deste motor, congelada: nao o que o
// operador digitou, nao o OCR, nao o Gemini. Reexecutar o motor sobre
// nota antiga quebra o contrato com o REGEN.
//
// Espelha o schema do calibrador (ERA AUDITOR, contrato.LeituraERA) sem
// importar aquele modulo -- copia de tipos, mesmos nomes de campo JSON.
package contrato

import "time"

// VersaoContrato identifica a forma deste JSON. Qualquer mudanca
// incompativel (renomear campo, mudar unidade de total) vira a1.
const VersaoContrato = "a0"

// Campos de negocio comparados campo a campo. Chaves estaveis: o prompt
// Gemini e o OCR normalizado usam as mesmas.
const (
	CampoCNPJEmitente     = "cnpj_emitente"
	CampoCNPJDestinatario = "cnpj_destinatario"
	CampoCPF              = "cpf"
	CampoData             = "data"
	CampoTotal            = "total"
)

// Point e um pixel da imagem, mesma convencao do geom.Point: X direita,
// Y para baixo.
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Polygon e a caixa do detector: 4 vertices se a palavra e reta, mais se
// o papel curvou. Nao e retangulo axis-aligned.
type Polygon []Point

// DetectOptions espelha detect.Options (literatura DBNet). O filtro
// publicado pelo REGEN manda estes valores de volta na leitura seguinte.
type DetectOptions struct {
	Threshold   float32 `json:"threshold"`
	MinArea     float64 `json:"min_area"`
	MinScore    float32 `json:"min_score"`
	UnclipRatio float64 `json:"unclip_ratio"`
}

// DefaultDetectOptions sao os valores de detect.DefaultOptions.
func DefaultDetectOptions() DetectOptions {
	return DetectOptions{
		Threshold:   0.3,
		MinArea:     9,
		MinScore:    0.5,
		UnclipRatio: 1.5,
	}
}

// DewarpThresholds espelha dewarp.Thresholds.
type DewarpThresholds struct {
	RetoPx    float64 `json:"reto_px"`
	CurvoPx   float64 `json:"curvo_px"`
	AnguloRad float64 `json:"angulo_rad"`
}

// DefaultDewarpThresholds sao os valores de dewarp.DefaultThresholds
// (1,5 px de folga, 1 grau de inclinacao).
func DefaultDewarpThresholds() DewarpThresholds {
	return DewarpThresholds{
		RetoPx:    1.5,
		CurvoPx:   1.5,
		AnguloRad: 0.017453292519943295, // pi/180
	}
}

// LayoutOptions guarda os limiares publicos do agrupamento de linhas.
type LayoutOptions struct {
	OverlapFraction float64 `json:"overlap_fraction"`
	MaxDriftFactor  float64 `json:"max_drift_factor,omitempty"`
}

// DefaultLayoutOptions sao os valores de layout.OverlapFraction e
// layout.MaxDriftFactor.
func DefaultLayoutOptions() LayoutOptions {
	return LayoutOptions{
		OverlapFraction: 0.5,
		MaxDriftFactor:  2.0,
	}
}

// Identidade diz qual motor, quais pesos e quais limiares produziram a
// leitura. Sem isso o REGEN nao sabe o que treinar contra o que.
type Identidade struct {
	MotorVersao    string           `json:"motor_versao"`
	ONNXDetHash    string           `json:"onnx_det_hash"`
	ONNXRecHash    string           `json:"onnx_rec_hash,omitempty"`
	Detect         DetectOptions    `json:"detect"`
	Dewarp         DewarpThresholds `json:"dewarp"`
	Layout         LayoutOptions    `json:"layout"`
	ContratoVersao string           `json:"contrato_versao"`
}

// LineShape espelha dewarp.LineShape.
type LineShape struct {
	Angle     float64 `json:"angle"`
	LinearBow float64 `json:"linear_bow"`
	CurveBow  float64 `json:"curve_bow"`
}

// Region e uma detect.Result com o texto e a confianca CTC quando o
// reconhecedor leu a caixa. Caixa detectada que o recog descartou (N3,
// contorno degenerado) entra mesmo assim, com Text vazio -- o calibrador
// compara caixa, nao so campo.
type Region struct {
	Polygon       Polygon `json:"polygon"`
	Score         float32 `json:"score"`
	Text          string  `json:"text,omitempty"`
	CTCConfidence float32 `json:"ctc_confidence,omitempty"`
}

// ItemLinha e uma linha de tabela no JSON do contrato.
type ItemLinha struct {
	Descricao     string `json:"descricao"`
	Quantidade    string `json:"quantidade,omitempty"`
	ValorCentavos *int64 `json:"valor_centavos,omitempty"`
}

// Campos tipados do documento. O motor preenche via ExtrairDocumento
// (DAV/DANFE) -- nao fica mais vazio como no A0 do calibrador, quando
// ainda nao havia recog. O funil OCR x ERA 100% nos campos canonicos
// passa a ser caminho real no lancamento.
type Campos struct {
	CNPJEmitente     string      `json:"cnpj_emitente,omitempty"`
	CNPJDestinatario string      `json:"cnpj_destinatario,omitempty"`
	CPF              string      `json:"cpf,omitempty"`
	Data             string      `json:"data,omitempty"` // YYYY-MM-DD quando parseado
	TotalCentavos    *int64      `json:"total_centavos,omitempty"`
	Itens            []ItemLinha `json:"itens,omitempty"`
}

// Vazio devolve true se nenhum campo de negocio foi preenchido.
func (c Campos) Vazio() bool {
	return c.CNPJEmitente == "" &&
		c.CNPJDestinatario == "" &&
		c.CPF == "" &&
		c.Data == "" &&
		c.TotalCentavos == nil &&
		len(c.Itens) == 0
}

// LeituraERA e a leitura original. O FrotaHub grava no lancamento e nao
// reescreve. O motor preenche Identidade, Regioes, PageLevel, FormasLinha
// e Campos; o chamador preenche id, nota, uri, operador e instante.
type LeituraERA struct {
	ID          string      `json:"id"`
	NotaID      string      `json:"nota_id"`
	ImagemURI   string      `json:"imagem_uri"`
	LancadoPor  string      `json:"lancado_por"`
	LancadoEm   time.Time   `json:"lancado_em"`
	Identidade  Identidade  `json:"identidade"`
	Regioes     []Region    `json:"regioes"`
	PageLevel   string      `json:"page_level"` // N0, N1, N2, N3
	FormasLinha []LineShape `json:"formas_linha,omitempty"`
	Campos      Campos      `json:"campos"`
}

// MinScore devolve o menor Score das regioes, ou 0 se nao houver nenhuma
// -- pagina vazia e duvida.
func (l LeituraERA) MinScore() float32 {
	if len(l.Regioes) == 0 {
		return 0
	}
	min := l.Regioes[0].Score
	for _, r := range l.Regioes[1:] {
		if r.Score < min {
			min = r.Score
		}
	}
	return min
}
