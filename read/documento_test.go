package read

import (
	"encoding/json"
	"testing"

	"github.com/iafrotamacedo-cloud/era-read/contrato"
	"github.com/iafrotamacedo-cloud/era-read/detect"
	"github.com/iafrotamacedo-cloud/era-read/layout"
	"github.com/iafrotamacedo-cloud/era-read/recog"
)

func TestExtrairDocumentoDAV(t *testing.T) {
	linhas := []layout.Line{
		linhaDeTexto("DOCUMENTO AUXILIAR DE VENDA - PEDIDO"),
		linhaDeTexto("Razao Social: RODRIGUES MATERIAL LTDA-ME CNPJ:14788633000110"),
		linhaDeTexto("Nome: FROTA MACEDO ENGENHARIA EIRELI CPF/CNPJ: 27363223000170"),
		linhaDeTexto("N° do Documento:0000018355"),
		linhaDeTexto("Total a pagar: 100,60"),
		linhaDeTexto("Dt. Emis:18/08/2026"),
	}
	d := ExtrairDocumento(linhas)
	if d.Tipo != TipoDAV {
		t.Fatalf("tipo = %q, quero dav", d.Tipo)
	}
	c := CamposDoDocumento(d)
	if c.CNPJEmitente != "14.788.633/0001-10" {
		t.Errorf("cnpj_emitente = %q", c.CNPJEmitente)
	}
	if c.CNPJDestinatario != "27.363.223/0001-70" {
		t.Errorf("cnpj_destinatario = %q", c.CNPJDestinatario)
	}
	if c.Data != "2026-08-18" {
		t.Errorf("data = %q", c.Data)
	}
	if c.TotalCentavos == nil || *c.TotalCentavos != 10060 {
		t.Errorf("total = %v", c.TotalCentavos)
	}
	if c.Vazio() {
		t.Error("DAV preenchida nao pode sair Campos vazio -- o funil OCR x ERA depende disso")
	}
}

func TestExtrairDocumentoDANFE(t *testing.T) {
	d := ExtrairDocumento(linhasDANFEDeExemplo())
	if d.Tipo != TipoDANFE {
		t.Fatalf("tipo = %q, quero danfe (o titulo tambem diz documento auxiliar)", d.Tipo)
	}
	c := CamposDoDocumento(d)
	if c.CNPJEmitente != "14.788.633/0001-10" {
		t.Errorf("cnpj_emitente = %q", c.CNPJEmitente)
	}
	if c.CNPJDestinatario != "27.363.223/0001-70" {
		t.Errorf("cnpj_destinatario = %q", c.CNPJDestinatario)
	}
	if c.Data != "2026-08-18" {
		t.Errorf("data = %q", c.Data)
	}
	if c.TotalCentavos == nil || *c.TotalCentavos != 2500 {
		t.Errorf("total = %v, quero 2500 (VALOR TOTAL DA NOTA)", c.TotalCentavos)
	}
}

func TestExtrairDocumentoDesconhecidoUsaLivres(t *testing.T) {
	linhas := []layout.Line{
		linhaDeTexto("CNPJ:14788633000110"),
		linhaDeTexto("Dt.Emis:22/07/2026"),
	}
	d := ExtrairDocumento(linhas)
	if d.Tipo != TipoDesconhecido {
		t.Fatalf("tipo = %q, quero vazio (sem rotulo de DAV/DANFE)", d.Tipo)
	}
	c := CamposDoDocumento(d)
	if c.CNPJEmitente != "14.788.633/0001-10" {
		t.Errorf("cnpj_emitente livre = %q", c.CNPJEmitente)
	}
	if c.Data != "2026-07-22" {
		t.Errorf("data livre = %q", c.Data)
	}
}

func TestOptionsFromFiltro(t *testing.T) {
	f := contrato.Filtro{
		MotorVersao: "era-read-test",
		ONNXDetHash: "deadbeef",
		ONNXRecHash: "cafebabe",
		Detect:      contrato.DetectOptions{Threshold: 0.2, MinArea: 12, MinScore: 0.4, UnclipRatio: 1.7},
		Dewarp:      contrato.DewarpThresholds{RetoPx: 2, CurvoPx: 4, AnguloRad: 0.02},
		Layout:      contrato.LayoutOptions{OverlapFraction: 0.7, MaxDriftFactor: 3},
	}
	opts := OptionsFromFiltro(f)
	if opts.MotorVersao != "era-read-test" || opts.ONNXDetHash != "deadbeef" {
		t.Errorf("identidade = %+v", opts)
	}
	if opts.Detector.Threshold != 0.2 || opts.Detector.MinArea != 12 {
		t.Errorf("detect = %+v", opts.Detector)
	}
	if opts.Thresholds.RetoPx != 2 || opts.Thresholds.CurvoPx != 4 {
		t.Errorf("dewarp = %+v", opts.Thresholds)
	}
	if opts.Layout.OverlapFraction != 0.7 {
		t.Errorf("layout = %+v", opts.Layout)
	}
}

func TestOptionsFromFiltroVazioUsaDefault(t *testing.T) {
	opts := OptionsFromFiltro(contrato.Filtro{})
	def := DefaultOptions()
	if opts.Detector != def.Detector {
		t.Errorf("detect = %+v, quero default %+v", opts.Detector, def.Detector)
	}
	if opts.Thresholds != def.Thresholds {
		t.Errorf("dewarp = %+v, quero default", opts.Thresholds)
	}
	if opts.Layout.OverlapFraction != def.Layout.OverlapFraction {
		t.Errorf("layout = %+v, quero default", opts.Layout)
	}
}

func TestMontarLeituraPreencheCamposEIdentidade(t *testing.T) {
	pagina, regioes := duasRegioesDetectadas(t)
	recGraph := redeReconhecimentoDeBrinquedo(t)
	cs := recog.NewCharset([]string{"X", "Y"})
	opts := DefaultOptions()
	opts.MotorVersao = "test-motor"
	opts.ONNXDetHash = "hash-det"

	p, err := ProcessarPagina(pagina, regioes, detect.Scale{X: 1, Y: 1}, recGraph, cs, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Regioes) != 2 {
		t.Fatalf("regioes = %d, quero 2 (mesmo as que o recog ler)", len(p.Regioes))
	}
	if p.PageLevel.String() != "N0" && p.PageLevel.String() != "N1" {
		t.Errorf("PageLevel = %s, regioes retas deveriam ser N0 ou N1", p.PageLevel)
	}

	leitura := MontarLeitura(p, opts)
	if leitura.Identidade.MotorVersao != "test-motor" {
		t.Errorf("motor_versao = %q", leitura.Identidade.MotorVersao)
	}
	if leitura.Identidade.ONNXDetHash != "hash-det" {
		t.Errorf("onnx_det_hash = %q", leitura.Identidade.ONNXDetHash)
	}
	if leitura.Identidade.ContratoVersao != contrato.VersaoContrato {
		t.Errorf("contrato_versao = %q", leitura.Identidade.ContratoVersao)
	}
	if leitura.PageLevel != p.PageLevel.String() {
		t.Errorf("page_level = %q", leitura.PageLevel)
	}
	if len(leitura.Regioes) != 2 {
		t.Fatalf("leitura.Regioes = %d", len(leitura.Regioes))
	}
	if leitura.Regioes[0].Text == "" && leitura.Regioes[1].Text == "" {
		t.Error("pelo menos uma regiao deveria ter texto (rede de brinquedo)")
	}

	raw, err := json.Marshal(leitura)
	if err != nil {
		t.Fatal(err)
	}
	var got contrato.LeituraERA
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Identidade.Detect.Threshold != opts.Detector.Threshold {
		t.Errorf("roundtrip threshold = %v", got.Identidade.Detect.Threshold)
	}
}

func TestProcessarPaginaMantemCaixaSemTexto(t *testing.T) {
	pagina, regioes := duasRegioesDetectadas(t)
	regioes = append(regioes, detect.Result{
		Polygon: regioes[0].Polygon[:2:2], Score: 0.11,
	})
	recGraph := redeReconhecimentoDeBrinquedo(t)
	cs := recog.NewCharset([]string{"X", "Y"})

	p, err := ProcessarPagina(pagina, regioes, detect.Scale{X: 1, Y: 1}, recGraph, cs, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Regioes) != 3 {
		t.Fatalf("regioes = %d, quero 3 (a degenerada entra sem texto)", len(p.Regioes))
	}
	if p.Regioes[2].Score != 0.11 {
		t.Errorf("score da degenerada = %v", p.Regioes[2].Score)
	}
	if p.Regioes[2].Text != "" {
		t.Errorf("degenerada nao deveria ter texto, tem %q", p.Regioes[2].Text)
	}
}
