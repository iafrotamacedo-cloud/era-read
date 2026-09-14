// Package layout organiza palavras soltas (posicao + texto) em linhas de
// texto, na ordem em que uma pessoa leria a pagina, e cada linha em
// campos (SplitCells) quando o espacamento horizontal denuncia mais de um
// campo colado na mesma linha.
//
// So usa geometria -- a posicao das caixas -- nunca o conteudo do texto.
// Isso cobre corretamente um bloco de texto de uma coluna so: a maioria
// dos campos de uma nota fiscal, boleto ou ordem de compra (cabecalho,
// dados do fornecedor, totais, rotulo e valor lado a lado numa mesma
// linha). O que ainda fica de fora: alinhar celulas de LINHAS diferentes
// na mesma coluna -- a tabela de verdade, onde a segunda palavra de toda
// linha de item forma a coluna "quantidade" -- ver a ressalva em
// SplitCells.
package layout

import (
	"strings"

	"github.com/iafrotamacedo-cloud/era-read/geom"
)

// Word e uma palavra reconhecida: a caixa que a delimita, o texto que o
// reconhecedor devolveu para ela, e a confianca media que ele reportou
// (ver recog.Resultado -- este pacote so organiza o que a fase 5 produz,
// nao recalcula nada a partir do conteudo).
type Word struct {
	Box        geom.Polygon
	Text       string
	Confidence float32
}

func (w Word) yRange() (min, max float64) {
	lo, hi := w.Box.Bounds()
	return lo.Y, hi.Y
}

func (w Word) xMin() float64 {
	lo, _ := w.Box.Bounds()
	return lo.X
}

// Line e uma sequencia de palavras da mesma linha de texto, em ordem de
// leitura esquerda para direita.
type Line struct {
	Words []Word
}

// Bounds devolve o retangulo que envolve todas as palavras da linha.
func (l Line) Bounds() (min, max geom.Point) {
	if len(l.Words) == 0 {
		return geom.Point{}, geom.Point{}
	}
	min, max = l.Words[0].Box.Bounds()
	for _, w := range l.Words[1:] {
		wMin, wMax := w.Box.Bounds()
		if wMin.X < min.X {
			min.X = wMin.X
		}
		if wMin.Y < min.Y {
			min.Y = wMin.Y
		}
		if wMax.X > max.X {
			max.X = wMax.X
		}
		if wMax.Y > max.Y {
			max.Y = wMax.Y
		}
	}
	return min, max
}

// Text junta o texto das palavras da linha, separado por espaco, na ordem
// em que Words ja esta -- esquerda para direita.
func (l Line) Text() string {
	partes := make([]string, len(l.Words))
	for i, w := range l.Words {
		partes[i] = w.Text
	}
	return strings.Join(partes, " ")
}
