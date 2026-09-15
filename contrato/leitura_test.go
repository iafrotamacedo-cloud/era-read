package contrato

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestVersaoContratoA0(t *testing.T) {
	if VersaoContrato != "a0" {
		t.Errorf("VersaoContrato = %q, o calibrador ainda fala a0", VersaoContrato)
	}
}

func TestCamposVazio(t *testing.T) {
	if !(Campos{}).Vazio() {
		t.Error("Campos{} deveria ser vazio")
	}
	c := Campos{CNPJEmitente: "14.788.633/0001-10"}
	if c.Vazio() {
		t.Error("campo preenchido nao e vazio")
	}
}

func TestLeituraERAJSONChavesCanonicas(t *testing.T) {
	total := int64(10060)
	l := LeituraERA{
		Identidade: Identidade{
			MotorVersao:    "test",
			ONNXDetHash:    "abc",
			Detect:         DefaultDetectOptions(),
			Dewarp:         DefaultDewarpThresholds(),
			Layout:         DefaultLayoutOptions(),
			ContratoVersao: VersaoContrato,
		},
		Regioes: []Region{{
			Polygon:       Polygon{{X: 1, Y: 2}, {X: 3, Y: 4}},
			Score:         0.97,
			Text:          "CNPJ",
			CTCConfidence: 0.9,
		}},
		PageLevel: "N0",
		Campos: Campos{
			CNPJEmitente:     "14.788.633/0001-10",
			CNPJDestinatario: "27.363.223/0001-70",
			Data:             "2026-08-18",
			TotalCentavos:    &total,
			Itens:            []ItemLinha{{Descricao: "CAP ESG PVC 40MM", Quantidade: "1,000", ValorCentavos: &total}},
		},
	}

	raw, err := json.Marshal(l)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, chave := range []string{
		`"cnpj_emitente"`,
		`"cnpj_destinatario"`,
		`"total_centavos"`,
		`"ctc_confidence"`,
		`"page_level"`,
		`"overlap_fraction"`,
		`"reto_px"`,
		`"min_score"`,
		`"contrato_versao"`,
	} {
		if !strings.Contains(s, chave) {
			t.Errorf("JSON sem a chave %s: %s", chave, s)
		}
	}

	var got LeituraERA
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Campos.CNPJEmitente != l.Campos.CNPJEmitente {
		t.Errorf("roundtrip CNPJ emitente = %q", got.Campos.CNPJEmitente)
	}
	if got.Campos.TotalCentavos == nil || *got.Campos.TotalCentavos != total {
		t.Errorf("roundtrip total = %v", got.Campos.TotalCentavos)
	}
	if got.Identidade.ContratoVersao != VersaoContrato {
		t.Errorf("contrato_versao = %q", got.Identidade.ContratoVersao)
	}
}

func TestMinScore(t *testing.T) {
	if (LeituraERA{}).MinScore() != 0 {
		t.Error("sem regioes, MinScore deveria ser 0")
	}
	l := LeituraERA{Regioes: []Region{{Score: 0.9}, {Score: 0.4}, {Score: 0.7}}}
	if l.MinScore() != 0.4 {
		t.Errorf("MinScore = %v, quero 0.4", l.MinScore())
	}
}
