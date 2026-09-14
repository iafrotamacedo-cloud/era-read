// Package detect transforma o mapa de probabilidade que a rede de deteccao
// devolve (um valor entre 0 e 1 por pixel, "isto e texto?") em poligonos --
// as regioes discretas que o resto do pipeline (dewarp, layout, recog)
// sabe manipular.
//
// O caminho, em ordem:
//
//	mapa de probabilidade
//	  -> Binarize        (corta num limiar: texto ou nao)
//	  -> label            (agrupa pixels vizinhos na mesma mancha)
//	  -> FindContours     (traca o contorno de cada mancha)
//	  -> RegionScore      (mede a confianca media de cada contorno)
//	  -> filtra           (descarta mancha pequena ou de confianca baixa)
//	  -> Unclip           (expande o contorno de volta ao tamanho real)
//
// Nada aqui e rede neural -- e processamento de imagem e geometria
// classicos, a parte do problema que sempre foi determinística.
//
// # Por que a rede nao devolve poligono direto
//
// Uma rede tem saida de tamanho fixo, mas o numero de palavras numa pagina
// varia, e o formato de cada uma tambem (reto ou curvo, o problema que o
// dewarp existe para resolver). A saida de tamanho fixo que funciona e "um
// valor de probabilidade por pixel" -- dai o nome do pacote: ele destila
// esse mapa continuo nas regioes discretas que sao, de fato, o produto
// desta fase.
//
// # Por que o modelo preve a mancha encolhida
//
// O DBNet e treinado para prever cada regiao de texto um pouco menor do
// que o texto real -- de proposito, para que duas palavras vizinhas nao
// colem numa mancha so na binarizacao. Unclip desfaz isso, expandindo o
// contorno de volta.
package detect

import (
	"github.com/iafrotamacedo-cloud/era-read/geom"
	"github.com/iafrotamacedo-cloud/era-read/imgproc"
)

// Options ajusta os limiares do pos-processamento. Os valores default sao
// os da literatura do DBNet/PP-OCR, nao uma calibracao medida contra
// documento real -- este motor ainda nao tem imagem real para calibrar
// contra (mesma ressalva que documents/layout ja registra para o limiar de
// coluna).
type Options struct {
	// Threshold corta o mapa de probabilidade: pixel >= Threshold conta
	// como texto.
	Threshold float32

	// MinArea descarta contornos com area (em pixels, na resolucao do
	// mapa de probabilidade) menor que isso -- ruido de binarizacao.
	MinArea float64

	// MinScore descarta contornos cuja confianca media (RegionScore)
	// fique abaixo disso.
	MinScore float32

	// UnclipRatio controla o quanto o contorno cresce em Unclip:
	// distancia = area * UnclipRatio / perimetro.
	UnclipRatio float64
}

// DefaultOptions devolve os limiares da literatura DBNet/PP-OCR.
func DefaultOptions() Options {
	return Options{
		Threshold:   0.3,
		MinArea:     9,
		MinScore:    0.5,
		UnclipRatio: 1.5,
	}
}

// Result e uma regiao de texto detectada.
type Result struct {
	Polygon geom.Polygon
	Score   float32
}

// Detect roda o pipeline inteiro: binariza, acha os contornos, filtra por
// area e confianca, e expande cada um de volta ao tamanho real.
func Detect(prob *imgproc.Gray, opts Options) []Result {
	bmp := Binarize(prob, opts.Threshold)
	contornos := FindContours(bmp)

	var out []Result
	for _, c := range contornos {
		if absArea(c) < opts.MinArea {
			continue
		}
		score := RegionScore(prob, c)
		if score < opts.MinScore {
			continue
		}
		expandido := Unclip(c, opts.UnclipRatio)
		out = append(out, Result{Polygon: expandido, Score: score})
	}
	return out
}

// absArea e a area do poligono sem sinal -- Polygon.Area e assinada pela
// orientacao dos vertices, e o rumo em que FindContours traca nao importa
// para filtrar por tamanho.
func absArea(p geom.Polygon) float64 {
	a := p.Area()
	if a < 0 {
		return -a
	}
	return a
}
