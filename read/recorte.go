package read

import (
	"fmt"
	"image"

	"github.com/iafrotamacedo-cloud/era-read/dewarp"
	"github.com/iafrotamacedo-cloud/era-read/geom"
	"github.com/iafrotamacedo-cloud/era-read/imgproc"
)

// recortarQuad extrai um recorte colorido reto a partir de 4 cantos
// (superior-esquerdo, superior-direito, inferior-direito, inferior-esquerdo
// -- a ordem que geom.RemapHomography espera) fotografados em perspectiva:
// os niveis N0 (ja reto -- a homografia sai perto da identidade) e N1
// (torto ou em perspectiva) do roteiro.
//
// RemapHomography so trabalha em imgproc.Gray; por isso cada canal de cor e
// retificado em separado e depois recombinado com imgproc.ComposeBGR --
// mesma ideia de detect.Preprocess, so que na direcao contraria (lá extrai
// canal pra alimentar rede, aqui recompõe canal depois de mexer em cada um).
func recortarQuad(src image.Image, cantos [4]geom.Point, outW, outH int) (image.Image, error) {
	extrair := func(ch imgproc.Channel) (*imgproc.Gray, error) {
		bruto := imgproc.FromImageChannel(src, ch)
		return geom.RemapHomography(bruto, cantos, outW, outH)
	}

	b, err := extrair(imgproc.ChannelB)
	if err != nil {
		return nil, fmt.Errorf("read: retificar canal B: %w", err)
	}
	g, err := extrair(imgproc.ChannelG)
	if err != nil {
		return nil, fmt.Errorf("read: retificar canal G: %w", err)
	}
	r, err := extrair(imgproc.ChannelR)
	if err != nil {
		return nil, fmt.Errorf("read: retificar canal R: %w", err)
	}
	return imgproc.ComposeBGR(b, g, r)
}

// recortarCurva extrai um recorte colorido reto a partir de uma linha
// curva -- o nível N2, via dewarp.RectifyLine em cada canal separadamente.
func recortarCurva(src image.Image, baseline []geom.Point, outW, outH int, acima, abaixo float64) (image.Image, error) {
	extrair := func(ch imgproc.Channel) (*imgproc.Gray, error) {
		bruto := imgproc.FromImageChannel(src, ch)
		return dewarp.RectifyLine(bruto, baseline, outW, outH, acima, abaixo)
	}

	b, err := extrair(imgproc.ChannelB)
	if err != nil {
		return nil, fmt.Errorf("read: retificar canal B: %w", err)
	}
	g, err := extrair(imgproc.ChannelG)
	if err != nil {
		return nil, fmt.Errorf("read: retificar canal G: %w", err)
	}
	r, err := extrair(imgproc.ChannelR)
	if err != nil {
		return nil, fmt.Errorf("read: retificar canal R: %w", err)
	}
	return imgproc.ComposeBGR(b, g, r)
}
