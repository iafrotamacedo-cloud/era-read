package recog

import (
	"fmt"
	"strings"

	"github.com/iafrotamacedo-cloud/era-read/tensor"
)

// Resultado e o texto decodificado de uma linha, com a confianca media dos
// caracteres emitidos (a media da probabilidade do carater vencedor em
// cada posicao de tempo que sobreviveu ao colapso e ao descarte do branco;
// 0 se nenhum caractere sobrou).
type Resultado struct {
	Texto     string
	Confianca float32
}

// DecodeCTC decodifica a saida da rede de reconhecimento -- forma
// [N,T,C], ja com softmax aplicado (a ultima operacao do grafo do
// PP-OCRv3/v4 rec e um Softmax) -- em texto, um Resultado por amostra do
// lote.
//
// Decodificacao gulosa de CTC, do jeito do CTCLabelDecode do PaddleOCR
// (ppocr/postprocess/rec_postprocess.py, conferido na fonte primaria em
// 14/09/2026): para cada posicao no tempo, pega o indice de maior
// probabilidade; colapsa repeticoes consecutivas do MESMO indice CRU
// (inclusive quando o indice repetido e o branco); so DEPOIS descarta o
// branco. A ordem importa -- colapsar antes de descartar e o que separa
// duas letras iguais seguidas (dois picos do mesmo indice, sem branco no
// meio de verdade colapsando errado) de letra-branco-letra (a mesma letra
// duas vezes, de proposito).
func DecodeCTC(saida *tensor.Tensor, cs Charset) ([]Resultado, error) {
	if saida.Rank() != 3 {
		return nil, fmt.Errorf("recog: DecodeCTC espera forma [N,T,C], recebeu %v", saida.Shape)
	}
	n, t, c := saida.Shape[0], saida.Shape[1], saida.Shape[2]
	if c != len(cs) {
		return nil, fmt.Errorf("recog: a rede tem %d classes, o dicionario tem %d (branco incluso)", c, len(cs))
	}

	flat := saida.Flat()
	out := make([]Resultado, n)

	for amostra := 0; amostra < n; amostra++ {
		var texto strings.Builder
		var somaConf float32
		var contagem int
		anterior := -1 // fora do intervalo valido de indices: o primeiro passo nunca colide

		for passo := 0; passo < t; passo++ {
			base := (amostra*t + passo) * c
			idx, prob := 0, flat[base]
			for k := 1; k < c; k++ {
				if v := flat[base+k]; v > prob {
					idx, prob = k, v
				}
			}

			if idx == anterior {
				continue
			}
			anterior = idx
			if idx == 0 { // token em branco
				continue
			}
			texto.WriteString(cs[idx])
			somaConf += prob
			contagem++
		}

		confianca := float32(0)
		if contagem > 0 {
			confianca = somaConf / float32(contagem)
		}
		out[amostra] = Resultado{Texto: texto.String(), Confianca: confianca}
	}

	return out, nil
}
