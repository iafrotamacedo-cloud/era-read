package layout

import (
	"math"
	"sort"
)

// OverlapFraction e o limiar minimo de sobreposicao vertical -- como
// fracao da altura da menor das duas caixas -- para duas palavras
// contarem como da mesma linha.
const OverlapFraction = 0.5

// GroupLines agrupa palavras soltas em linhas de texto usando so a
// geometria (sobreposicao vertical das caixas), nunca o conteudo. Depois
// ordena as palavras de cada linha da esquerda para a direita, e as
// proprias linhas de cima para baixo -- a ordem de leitura de um bloco de
// texto de uma coluna so.
//
// Cada linha "abre" ancorada na faixa vertical [yMin,yMax] da primeira
// palavra atribuida a ela, e essa ancora nao muda depois -- ao contrario
// de ir estendendo a faixa a cada palavra nova (o que deixaria a linha
// "derivar" verticalmente e engolir a linha vizinha de baixo aos poucos).
// Isso assume que as palavras de uma mesma linha tem altura razoavelmente
// uniforme; um titulo bem maior que o resto do paragrafo ao lado pode
// abrir a propria linha mesmo estando geometricamente colado.
//
// SO agrupa por faixa vertical: duas palavras que pertencem, na pagina de
// verdade, a colunas diferentes lado a lado (um rotulo "Total:" e o valor
// numa coluna bem separada, por exemplo) caem na mesma Line se estiverem
// na mesma altura -- GroupLines nao sabe que sao campos diferentes.
// SplitCells (columns.go) resolve isso depois, cortando a Line de volta
// em Cell por espacamento horizontal; separado em duas funcoes porque sao
// duas perguntas diferentes ("que palavras formam esta linha" e "que
// palavras formam este campo dentro da linha"), cada uma com o seu limiar
// calibrado à parte.
func GroupLines(words []Word) []Line {
	if len(words) == 0 {
		return nil
	}

	ordenadas := append([]Word(nil), words...)
	sort.SliceStable(ordenadas, func(i, j int) bool {
		iMin, _ := ordenadas[i].yRange()
		jMin, _ := ordenadas[j].yRange()
		return iMin < jMin
	})

	type linhaAberta struct {
		ancoraMin, ancoraMax float64
		palavras             []Word
	}
	var abertas []linhaAberta

	for _, w := range ordenadas {
		wMin, wMax := w.yRange()

		melhor := -1
		melhorFracao := 0.0
		for i, l := range abertas {
			sobreposicao := overlap(wMin, wMax, l.ancoraMin, l.ancoraMax)
			menorAltura := math.Min(wMax-wMin, l.ancoraMax-l.ancoraMin)
			if menorAltura <= 0 {
				continue
			}
			fracao := sobreposicao / menorAltura
			if fracao > OverlapFraction && fracao > melhorFracao {
				melhor, melhorFracao = i, fracao
			}
		}

		if melhor == -1 {
			abertas = append(abertas, linhaAberta{ancoraMin: wMin, ancoraMax: wMax, palavras: []Word{w}})
		} else {
			abertas[melhor].palavras = append(abertas[melhor].palavras, w)
		}
	}

	linhas := make([]Line, len(abertas))
	for i, l := range abertas {
		palavras := append([]Word(nil), l.palavras...)
		sort.SliceStable(palavras, func(a, b int) bool {
			return palavras[a].xMin() < palavras[b].xMin()
		})
		linhas[i] = Line{Words: palavras}
	}
	sort.SliceStable(linhas, func(i, j int) bool {
		iMin, _ := linhas[i].Bounds()
		jMin, _ := linhas[j].Bounds()
		return iMin.Y < jMin.Y
	})
	return linhas
}

// overlap devolve o comprimento da intersecao entre [aMin,aMax] e
// [bMin,bMax], ou 0 se nao se tocarem.
func overlap(aMin, aMax, bMin, bMax float64) float64 {
	lo := math.Max(aMin, bMin)
	hi := math.Min(aMax, bMax)
	if hi <= lo {
		return 0
	}
	return hi - lo
}
