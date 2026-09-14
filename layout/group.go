package layout

import (
	"math"
	"sort"
)

// OverlapFraction e o limiar minimo de sobreposicao vertical -- como
// fracao da altura da menor das duas caixas -- para duas palavras
// contarem como da mesma linha.
const OverlapFraction = 0.5

// MaxDriftFactor limita o quanto a faixa vertical de uma linha pode
// crescer conforme novas palavras entram nela, como multiplo da altura da
// primeira palavra (a ancora original) -- ver o comentario de GroupLines
// sobre por que a faixa cresce, e por que precisa de um teto.
const MaxDriftFactor = 2.0

// GroupLines agrupa palavras soltas em linhas de texto usando so a
// geometria (sobreposicao vertical das caixas), nunca o conteudo. Depois
// ordena as palavras de cada linha da esquerda para a direita, e as
// proprias linhas de cima para baixo -- a ordem de leitura de um bloco de
// texto de uma coluna so.
//
// Cada linha "abre" ancorada na faixa vertical [yMin,yMax] da primeira
// palavra atribuida a ela. Palavras novas sao comparadas contra a faixa
// ATUAL da linha (que comeca igual a ancora e pode crescer a cada palavra
// que entra), nao so contra a ancora original -- do contrario uma cadeia
// de palavras real (a mesma linha impressa, mas com ruido de deteccao
// suficiente para a do meio nao bater os 50% contra a primeira) quebra em
// duas: achado com um item de tabela real, onde a celula do valor (~30px
// de altura) e a celula "UN" (~31px, uns pixels mais abaixo) individualmente
// batem a sobreposicao contra a descricao do produto no meio, mas "UN"
// sozinha nao bate os 50% contra a ancora ORIGINAL do outro lado da
// cadeia -- so contra a faixa que ja cresceu para incluir a descricao.
//
// A faixa so cresce ate MaxDriftFactor vezes a altura da ancora original:
// sem esse teto, uma cadeia de palavras cada uma so um pouco mais abaixo
// que a anterior acabaria "derivando" verticalmente sem limite e engolindo
// a linha vizinha de baixo aos poucos -- a preocupacao original que fez
// este pacote nascer sem extensao nenhuma. Isso assume que as palavras de
// uma mesma linha tem altura razoavelmente uniforme; um titulo bem maior
// que o resto do paragrafo ao lado pode abrir a propria linha mesmo
// estando geometricamente colado.
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
		ancoraAltura       float64 // altura da PRIMEIRA palavra -- nunca muda; so limita o teto de MaxDriftFactor
		atualMin, atualMax float64 // faixa usada na comparacao -- comeca igual a ancora, pode crescer ate o teto
		palavras           []Word
	}
	var abertas []linhaAberta

	for _, w := range ordenadas {
		wMin, wMax := w.yRange()

		melhor := -1
		melhorFracao := 0.0
		for i, l := range abertas {
			sobreposicao := overlap(wMin, wMax, l.atualMin, l.atualMax)
			menorAltura := math.Min(wMax-wMin, l.atualMax-l.atualMin)
			if menorAltura <= 0 {
				continue
			}
			fracao := sobreposicao / menorAltura
			if fracao <= OverlapFraction {
				continue
			}
			// Sobreposicao vertical alta tambem acontece entre duas linhas
			// FISICAS DIFERENTES empilhadas de perto (itens de tabela com
			// pouco espaco entre linhas, por exemplo) -- geometricamente
			// quase identico ao caso que MaxDriftFactor existe para
			// resolver. O que distingue os dois: duas palavras da MESMA
			// linha impressa ocupam colunas (faixas de X) diferentes; duas
			// linhas empilhadas tendem a repetir a mesma coluna (o codigo
			// do proximo item comeca na mesma posicao X que o item
			// anterior). Por isso so aceita a palavra se ela nao disputar a
			// faixa de X de nenhuma palavra ja aceita nesta linha.
			if sobrepoeXComAlguma(w, l.palavras) {
				continue
			}
			if fracao > melhorFracao {
				melhor, melhorFracao = i, fracao
			}
		}

		if melhor == -1 {
			abertas = append(abertas, linhaAberta{ancoraAltura: wMax - wMin, atualMin: wMin, atualMax: wMax, palavras: []Word{w}})
		} else {
			l := &abertas[melhor]
			l.palavras = append(l.palavras, w)

			novoMin, novoMax := math.Min(l.atualMin, wMin), math.Max(l.atualMax, wMax)
			if novoMax-novoMin <= l.ancoraAltura*MaxDriftFactor {
				l.atualMin, l.atualMax = novoMin, novoMax
			}
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

// sobrepoeXComAlguma informa se a faixa horizontal de w se sobrepoe com a
// de alguma palavra em palavras -- o sinal de "mesma coluna", usado para
// recusar juntar duas linhas fisicas diferentes que só por sobreposicao
// vertical pareceriam a mesma linha (ver o comentario em GroupLines).
func sobrepoeXComAlguma(w Word, palavras []Word) bool {
	wMin, wMax := w.xRange()
	for _, p := range palavras {
		pMin, pMax := p.xRange()
		if overlap(wMin, wMax, pMin, pMax) > 0 {
			return true
		}
	}
	return false
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
