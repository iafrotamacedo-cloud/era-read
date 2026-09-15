package contrato

import (
	"testing"
)

func TestParseFiltroMinimoDoREGEN(t *testing.T) {
	raw := []byte(`{
		"versao": "2026-09-15T12:00:00Z",
		"motor_versao": "era-read-0.4.0",
		"onnx_det_uri": "s3://x/det.onnx",
		"onnx_rec_uri": "s3://x/rec.onnx",
		"detect": {"threshold": 0.25, "min_area": 10, "min_score": 0.4, "unclip_ratio": 1.6},
		"dewarp": {"reto_px": 2, "curvo_px": 3, "angulo_rad": 0.02},
		"layout": {"overlap_fraction": 0.6},
		"contrato_versao": "a0"
	}`)
	f, err := ParseFiltro(raw)
	if err != nil {
		t.Fatal(err)
	}
	if f.Versao != "2026-09-15T12:00:00Z" {
		t.Errorf("versao = %q", f.Versao)
	}
	if f.Detect.Threshold != 0.25 || f.Detect.MinScore != 0.4 {
		t.Errorf("detect = %+v", f.Detect)
	}
	if f.Dewarp.RetoPx != 2 {
		t.Errorf("dewarp = %+v", f.Dewarp)
	}
	if f.Layout.OverlapFraction != 0.6 {
		t.Errorf("layout = %+v", f.Layout)
	}
	if f.ContratoVersao != VersaoContrato {
		t.Errorf("contrato_versao = %q", f.ContratoVersao)
	}
}

func TestParseFiltroInvalido(t *testing.T) {
	if _, err := ParseFiltro([]byte(`{`)); err == nil {
		t.Error("JSON quebrado deveria falhar")
	}
}

func TestBlocosZerados(t *testing.T) {
	if !(DetectOptions{}).Zerado() || !(DewarpThresholds{}).Zerado() || !(LayoutOptions{}).Zerado() {
		t.Error("zero-value deveria ser Zerado")
	}
	if DefaultDetectOptions().Zerado() {
		t.Error("default de detect nao e zerado")
	}
}
