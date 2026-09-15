package read

import (
	"testing"

	"github.com/iafrotamacedo-cloud/era-read/detect"
	"github.com/iafrotamacedo-cloud/era-read/dewarp"
)

func TestAplicarFiltroSobrepoeDetect(t *testing.T) {
	opts := DefaultOptions()
	f := FiltroRead{
		Detect: detect.Options{Threshold: 0.42, MinArea: 9, MinScore: 0.5, UnclipRatio: 1.5},
		Dewarp: dewarp.Thresholds{RetoPx: 2.5, CurvoPx: 2.5, AnguloRad: 0.02},
	}
	out := AplicarFiltro(opts, f)
	if out.Detector.Threshold != 0.42 {
		t.Fatalf("threshold=%v", out.Detector.Threshold)
	}
	if out.Thresholds.RetoPx != 2.5 {
		t.Fatalf("reto_px=%v", out.Thresholds.RetoPx)
	}
}

func TestIdentidadeDeEspelhaOpts(t *testing.T) {
	opts := DefaultOptions()
	id := IdentidadeDe(opts, "sha256:det", "sha256:rec")
	if id.MotorVersao != MotorVersao || id.ContratoVersao != ContratoVersao {
		t.Fatalf("versao errada: %+v", id)
	}
	if id.ONNXDetHash != "sha256:det" || id.ONNXRecHash != "sha256:rec" {
		t.Fatal("hash")
	}
	if id.OverlapLayout != OverlapLayout {
		t.Fatal("overlap")
	}
}
