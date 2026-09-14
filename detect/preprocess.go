package detect

import (
	"image"
	"math"

	"github.com/iafrotamacedo-cloud/era-read/geom"
	"github.com/iafrotamacedo-cloud/era-read/imgproc"
	"github.com/iafrotamacedo-cloud/era-read/tensor"
)

// meanBGR e stdBGR sao os parametros de normalizacao do PP-OCRv4/DBNet, na
// ordem B, G, R.
//
// Conferido no codigo-fonte do PaddleOCR em 14/09/2026
// (tools/infer/predict_det.py monta a imagem via cv2.imread -- BGR -- e
// passa direto para NormalizeImage; ppocr/data/imaug/operators.py nao tem
// nenhuma conversao BGR->RGB no caminho). Errar essa ordem nao quebra
// nada visivelmente -- a rede roda, produz uma saida de forma certa -- so
// detecta pior, porque cada canal recebe a media/desvio do canal errado.
// E o tipo de bug que so aparece comparando contra imagem real, nunca
// contra teste sintetico.
var meanBGR = [3]float32{0.485, 0.456, 0.406}
var stdBGR = [3]float32{0.229, 0.224, 0.225}

// PreprocessOptions controla o redimensionamento antes de entrar na rede.
type PreprocessOptions struct {
	// LimitSideLen e o lado maior maximo antes de encolher a imagem.
	LimitSideLen float64
}

// DefaultPreprocessOptions devolve o limite do PaddleOCR
// (tools/infer/utility.py: det_limit_side_len=960, det_limit_type="max").
func DefaultPreprocessOptions() PreprocessOptions {
	return PreprocessOptions{LimitSideLen: 960}
}

// Scale e o fator de escala que Preprocess aplicou, separado por eixo (o
// arredondamento para multiplo de 32 pode fazer os dois eixos escalarem
// por fatores levemente diferentes mesmo mantendo a proporcao original
// aproximadamente). Rescale usa isto para converter coordenadas de volta.
type Scale struct {
	X, Y float64
}

// Preprocess prepara uma imagem para a rede de deteccao: redimensiona (lado
// maior no limite, os dois eixos arredondados para o multiplo de 32 mais
// proximo) e normaliza (BGR, media/desvio do ImageNet, escala 1/255).
//
// O arredondamento para multiplo de 32 nao e estetica: o backbone reduz a
// imagem por um fator de 32 em varios estagios e depois amplia de volta
// pela FPN e pelo DBHead; um tamanho que nao seja multiplo exato acumula
// erro de arredondamento de forma a cada estagio, e alguns pares
// reducao/ampliacao simplesmente nao fecham no tamanho certo.
//
// Devolve o tensor pronto para graph.Run, forma [1,3,H,W], e a escala
// aplicada -- ver Rescale para o caminho de volta.
func Preprocess(src image.Image, opts PreprocessOptions) (*tensor.Tensor, Scale, error) {
	b := src.Bounds()
	origW, origH := b.Dx(), b.Dy()

	resizeW, resizeH := tamanhoRedimensionado(origW, origH, opts.LimitSideLen)

	canais := [3]imgproc.Channel{imgproc.ChannelB, imgproc.ChannelG, imgproc.ChannelR}
	plano := resizeH * resizeW
	dados := make([]float32, 3*plano)

	for c, ch := range canais {
		bruto := imgproc.FromImageChannel(src, ch)
		redimensionado := imgproc.Resize(bruto, resizeW, resizeH)
		for i, v := range redimensionado.Pix {
			dados[c*plano+i] = (v - meanBGR[c]) / stdBGR[c]
		}
	}

	t, err := tensor.FromSlice(dados, 1, 3, resizeH, resizeW)
	if err != nil {
		return nil, Scale{}, err
	}

	return t, Scale{
		X: float64(resizeW) / float64(origW),
		Y: float64(resizeH) / float64(origH),
	}, nil
}

// Rescale converte um poligono da resolucao redimensionada (a que a rede
// viu, e portanto a que o mapa de probabilidade e os poligonos detectados
// usam) de volta para a resolucao da imagem original.
func Rescale(poly geom.Polygon, s Scale) geom.Polygon {
	out := make(geom.Polygon, len(poly))
	for i, p := range poly {
		out[i] = geom.Point{X: p.X / s.X, Y: p.Y / s.Y}
	}
	return out
}

// tamanhoRedimensionado replica o algoritmo do PaddleOCR
// (DetResizeForTest.resize_image_type0, limit_type="max"): se o lado maior
// passar do limite, encolhe proporcionalmente; depois arredonda os dois
// lados, cada um por si, para o multiplo de 32 mais proximo -- os dois
// podem sair com fatores de escala levemente diferentes, por isso Scale
// guarda X e Y separados em vez de um numero so. Minimo de 32 nos dois
// eixos, para uma imagem menor que isso nao virar 0.
func tamanhoRedimensionado(w, h int, limite float64) (resizeW, resizeH int) {
	ratio := 1.0
	maior := w
	if h > maior {
		maior = h
	}
	if float64(maior) > limite {
		ratio = limite / float64(maior)
	}

	resizeH = arredondarMultiplo32(int(float64(h) * ratio))
	resizeW = arredondarMultiplo32(int(float64(w) * ratio))
	return resizeW, resizeH
}

func arredondarMultiplo32(v int) int {
	r := int(math.Round(float64(v)/32)) * 32
	if r < 32 {
		r = 32
	}
	return r
}
