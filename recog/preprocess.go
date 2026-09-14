// Package recog prepara uma linha de texto ja recortada para a rede de
// reconhecimento (fase 5) e decodifica a saida dela em texto.
//
// O que vem antes -- achar a linha na pagina -- e trabalho de detect e
// layout. Este pacote so cuida do que acontece depois que a caixa da linha
// ja existe: redimensionar para o formato que a rede espera e traduzir a
// saida de volta para texto.
package recog

import (
	"fmt"
	"image"
	"math"

	"github.com/iafrotamacedo-cloud/era-read/imgproc"
	"github.com/iafrotamacedo-cloud/era-read/tensor"
)

// PreprocessOptions controla o redimensionamento de uma linha de texto
// recortada antes de entrar na rede de reconhecimento.
type PreprocessOptions struct {
	// Height e a altura fixa que a rede espera.
	Height int

	// DefaultWidth e a proporcao largura/altura que o modelo foi treinado
	// esperando por padrao -- controla o preenchimento MINIMO de uma linha
	// curta, nao um limite rigido: uma linha proporcionalmente mais larga
	// que isso cresce alem dele (a rede aceita largura dinamica).
	DefaultWidth int
}

// DefaultPreprocessOptions devolve os valores do PaddleOCR para a familia
// PP-OCRv3/v4 (tools/infer/predict_rec.py: rec_image_shape="3,48,320",
// conferido na fonte primaria em 14/09/2026).
func DefaultPreprocessOptions() PreprocessOptions {
	return PreprocessOptions{Height: 48, DefaultWidth: 320}
}

// Preprocess redimensiona uma linha de texto ja recortada para a altura
// fixa da rede, preservando a proporcao, e preenche com zero a direita ate
// a largura alvo.
//
// Replica resize_norm_img de predict_rec.py -- a funcao que roda de
// verdade para o algoritmo "SVTR_LCNet" (a familia PP-OCRv3/v4, o caso
// deste pacote). Existe outra funcao no mesmo arquivo,
// resize_norm_img_svtr, com nome quase igual, mas ela so roda para uma
// lista diferente de algoritmos ("SVTR", "SATRN", "ParseQ", "CPPD") -- as
// duas foram conferidas lado a lado na fonte em 14/09/2026 para nao trocar
// uma pela outra.
//
// Normalizacao: (pixel/255 - 0,5) / 0,5, igual nos tres canais -- diferente
// da rede de deteccao, que usa media/desvio do ImageNet por canal (ver
// detect.Preprocess). Ordem de canal continua BGR sem conversao, pela
// mesma razao ja documentada la: cv2.imread nunca converte para RGB no
// caminho de inferencia do PaddleOCR, nem aqui na recognicao.
func Preprocess(src image.Image, opts PreprocessOptions) (*tensor.Tensor, error) {
	if opts.Height <= 0 || opts.DefaultWidth <= 0 {
		return nil, fmt.Errorf("recog: Height e DefaultWidth precisam ser positivos, recebi %+v", opts)
	}

	b := src.Bounds()
	origW, origH := b.Dx(), b.Dy()
	if origW <= 0 || origH <= 0 {
		return nil, fmt.Errorf("recog: imagem vazia (%dx%d)", origW, origH)
	}

	razao := float64(origW) / float64(origH)
	razaoMaxima := math.Max(float64(opts.DefaultWidth)/float64(opts.Height), razao)
	larguraAlvo := int(float64(opts.Height) * razaoMaxima)

	larguraRedim := int(math.Ceil(float64(opts.Height) * razao))
	if larguraRedim > larguraAlvo {
		larguraRedim = larguraAlvo
	}
	if larguraRedim < 1 {
		larguraRedim = 1
	}

	canais := [3]imgproc.Channel{imgproc.ChannelB, imgproc.ChannelG, imgproc.ChannelR}
	plano := opts.Height * larguraAlvo
	// Zerado de proposito: alem do canal 0..larguraRedim, o resto da
	// largura alvo fica em zero -- e o preenchimento a direita que
	// resize_norm_img faz com np.zeros antes de copiar a imagem redimensionada.
	dados := make([]float32, 3*plano)

	for c, ch := range canais {
		bruto := imgproc.FromImageChannel(src, ch)
		redimensionado := imgproc.Resize(bruto, larguraRedim, opts.Height)
		for y := 0; y < opts.Height; y++ {
			linha := dados[c*plano+y*larguraAlvo:]
			for x := 0; x < larguraRedim; x++ {
				linha[x] = redimensionado.At(x, y)*2 - 1 // (v-0.5)/0.5
			}
		}
	}

	return tensor.FromSlice(dados, 1, 3, opts.Height, larguraAlvo)
}
