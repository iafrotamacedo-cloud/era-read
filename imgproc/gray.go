// Package imgproc fornece o pre-processamento de imagem do ERA READ:
// decodificar, converter para cinza, normalizar iluminacao e amostrar.
//
// A maior parte deste pacote trabalha em escala de cinza, em float32, com
// valores em [0,1]. Cor nao ajuda os ALGORITMOS CLASSICOS daqui -- o
// contraste entre tinta e papel e questao de luminancia, nao de matiz. A
// excecao e FromImageChannel: a rede de deteccao foi treinada com imagem
// colorida de verdade, entao alimentar ela exige os tres canais separados,
// nao a luminancia combinada -- ver detect.Preprocess.
//
// Nada neste pacote conhece documento, papel ou texto. E processamento de
// imagem generico; o significado entra em pacotes posteriores.
package imgproc

import (
	"fmt"
	"image"
	"image/color"
)

// Gray e uma imagem em escala de cinza, float32 em [0,1], linha a linha.
//
// O pixel (x, y) mora em Pix[y*Stride+x]. Guardar Stride em vez de assumir
// Stride == W permite que uma Gray seja uma janela de outra maior sem copiar
// dado -- o mesmo motivo pelo qual tensor.Tensor guarda Strides em vez de
// arrays aninhados.
type Gray struct {
	Pix    []float32
	W, H   int
	Stride int
}

// NewGray cria uma imagem cinza zerada (preta) de w por h.
func NewGray(w, h int) *Gray {
	if w <= 0 || h <= 0 {
		panic(fmt.Sprintf("imgproc: dimensoes invalidas %dx%d", w, h))
	}
	return &Gray{
		Pix:    make([]float32, w*h),
		W:      w,
		H:      h,
		Stride: w,
	}
}

// At devolve o pixel em (x, y). Fora dos limites e erro de quem chama --
// nao ha checagem, pelo mesmo motivo que Tensor.At nao checa: este e o
// caminho quente do pacote, chamado por pixel.
func (g *Gray) At(x, y int) float32 {
	return g.Pix[y*g.Stride+x]
}

// Set atribui o pixel em (x, y).
func (g *Gray) Set(x, y int, v float32) {
	g.Pix[y*g.Stride+x] = v
}

// In informa se (x, y) cai dentro da imagem.
func (g *Gray) In(x, y int) bool {
	return x >= 0 && x < g.W && y >= 0 && y < g.H
}

// pesosLuminancia sao os pesos ITU-R BT.601 para RGB -> luminancia.
//
// BT.601 (em vez do BT.709, mais novo) porque e o que o pacote image da
// stdlib usa em image/color.GrayModel -- manter o mesmo padrao evita que a
// mesma foto produza cinza ligeiramente diferente dependendo de qual
// caminho de conversao foi usado.
const (
	pesoR = 299.0 / 1000.0
	pesoG = 587.0 / 1000.0
	pesoB = 114.0 / 1000.0
)

// FromImage converte uma image.Image (o que image/png e image/jpeg
// decodificam) para Gray, em [0,1].
//
// RGBA() da stdlib devolve componentes em [0,65535] ja com o alpha
// pre-multiplicado. Documento fotografado nao tem transparencia real, mas
// alguns PNGs trazem canal alfa mesmo assim (opaco, alfa=65535) -- nesse
// caso o pre-multiplicado nao muda nada. Se algum dia entrar uma imagem com
// alfa parcial, o pixel sai mais escuro do que devia; nao e o caso de uso
// deste motor.
func FromImage(src image.Image) *Gray {
	b := src.Bounds()
	dst := NewGray(b.Dx(), b.Dy())

	const escala = 1.0 / 65535.0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := src.At(x, y).RGBA()
			lum := pesoR*float64(r) + pesoG*float64(g) + pesoB*float64(bl)
			dst.Set(x-b.Min.X, y-b.Min.Y, float32(lum*escala))
		}
	}
	return dst
}

// Channel seleciona um canal de cor para extrair isoladamente, em
// FromImageChannel.
type Channel int

const (
	ChannelR Channel = iota
	ChannelG
	ChannelB
)

// FromImageChannel extrai um unico canal de cor de src como Gray, em [0,1]
// -- sem misturar com os outros dois, ao contrario de FromImage.
//
// Existe para alimentar uma rede treinada em imagem colorida (o detector de
// texto): achar texto pela via classica e questao de contraste, mas a rede
// aprendeu com cor de verdade, e nao ha como reconstruir isso a partir so
// da luminancia.
func FromImageChannel(src image.Image, ch Channel) *Gray {
	b := src.Bounds()
	dst := NewGray(b.Dx(), b.Dy())

	const escala = 1.0 / 65535.0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := src.At(x, y).RGBA()
			var v uint32
			switch ch {
			case ChannelR:
				v = r
			case ChannelG:
				v = g
			case ChannelB:
				v = bl
			}
			dst.Set(x-b.Min.X, y-b.Min.Y, float32(float64(v)*escala))
		}
	}
	return dst
}

// ComposeBGR reconstroi uma image.Image a partir de tres planos Gray -- B,
// G, R, na mesma ordem e convencao de FromImageChannel -- do mesmo tamanho.
// E o inverso de tres chamadas a FromImageChannel: existe para quando cada
// canal de cor precisou ser retificado (Remap, RemapHomography,
// dewarp.RectifyLine) separadamente, porque essas funcoes so trabalham em
// Gray, e o resultado colorido precisa voltar a ser uma image.Image comum
// para alimentar algo que espera isso -- recog.Preprocess, por exemplo.
func ComposeBGR(b, g, r *Gray) (image.Image, error) {
	if b.W != g.W || b.W != r.W || b.H != g.H || b.H != r.H {
		return nil, fmt.Errorf("imgproc: ComposeBGR com canais de tamanhos diferentes: B=%dx%d G=%dx%d R=%dx%d",
			b.W, b.H, g.W, g.H, r.W, r.H)
	}

	img := image.NewNRGBA(image.Rect(0, 0, b.W, b.H))
	for y := 0; y < b.H; y++ {
		for x := 0; x < b.W; x++ {
			img.SetNRGBA(x, y, color.NRGBA{
				R: paraUint8(r.At(x, y)),
				G: paraUint8(g.At(x, y)),
				B: paraUint8(b.At(x, y)),
				A: 255,
			})
		}
	}
	return img, nil
}

// paraUint8 converte um valor em [0,1] (a faixa que todo Gray usa) para
// [0,255], grudando nas pontas em vez de estourar -- um remap bilinear
// pode extrapolar levemente acima de 1 ou abaixo de 0 perto de uma borda.
func paraUint8(v float32) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 255
	}
	return uint8(v*255 + 0.5)
}
