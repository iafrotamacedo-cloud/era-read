package read

import (
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/era-read/extract"
	"github.com/iafrotamacedo-cloud/era-read/layout"
)

// Campos e o que ExtrairCampos consegue achar em texto ja lido e
// organizado em linhas, sem saber o tipo do documento.
//
// CNPJ e CPF tem digito verificador, entao dao para achar em qualquer
// lugar do texto com confianca -- exatamente o que extract.FindCNPJs e
// FindCPFs ja oferecem. Data e valor monetario nao tem verificacao
// equivalente (extract.ParseDateBR e ParseMoney esperam um campo ja
// isolado, nao um buscador de texto livre); aqui "isolar o campo" e achar
// uma etiqueta conhecida (ver EtiquetasValor) na mesma linha e tentar
// parsear o que sobra depois dela.
type Campos struct {
	CNPJs   []string
	CPFs    []string
	Datas   []time.Time
	Valores map[string]extract.Money // etiqueta reconhecida -> valor
}

// EtiquetasValor sao rotulos de campo monetario de um documento comercial
// brasileiro (nota fiscal, DAV) -- a forma de isolar "qual numero e
// dinheiro" que extract.ParseMoney exige, sem ser um buscador de texto
// livre (que colidiria com quantidade, numero de nota, CEP).
//
// Lista curta de proposito, e sem acento: um reconhecedor real erra
// acento antes de errar a letra em si (ver README, fase 5) -- "Razäo" em
// vez de "Razão" ja apareceu num documento real -- entao a etiqueta
// procurada evita depender de acento sair certo.
var EtiquetasValor = []string{
	"Total a pagar",
	"Valor Produtos",
	"Total Bruto Produtos",
}

// ExtrairCampos varre as linhas ja lidas e agrupadas (a saida de
// read.Page ou de layout.GroupLines direto) e devolve os campos tipados
// que dao para achar sem saber a estrutura do documento.
//
// So o primeiro valor achado por etiqueta conta -- um documento real pode
// repetir uma etiqueta parecida em mais de um lugar (subtotal, total),
// e sem uma tabela ja separada em colunas (fase 6, ainda parcial) nao ha
// como saber qual repeticao e a certa; ficar com a primeira e a escolha
// mais previsivel, nao a mais correta em todo caso.
func ExtrairCampos(linhas []layout.Line) Campos {
	c := Campos{Valores: make(map[string]extract.Money)}

	for _, l := range linhas {
		texto := l.Text()

		c.CNPJs = append(c.CNPJs, extract.FindCNPJs(texto)...)
		c.CPFs = append(c.CPFs, extract.FindCPFs(texto)...)

		if t, err := extract.ParseDateBR(texto); err == nil {
			c.Datas = append(c.Datas, t)
		}

		for _, etiqueta := range EtiquetasValor {
			if _, ja := c.Valores[etiqueta]; ja {
				continue
			}
			resto, ok := apósEtiqueta(texto, etiqueta)
			if !ok {
				continue
			}
			if v, err := extract.ParseMoney(resto); err == nil {
				c.Valores[etiqueta] = v
			}
		}
	}

	return c
}

// apósEtiqueta acha etiqueta dentro de texto (sem diferenciar
// maiusculas/minusculas -- o reconhecedor nao garante nem uma coisa nem
// outra) e devolve o que vem depois dela.
func apósEtiqueta(texto, etiqueta string) (string, bool) {
	idx := indexNormalizado(texto, etiqueta)
	if idx < 0 {
		return "", false
	}
	origRunes := []rune(texto)
	fim := idx + len([]rune(etiqueta))
	if fim > len(origRunes) {
		fim = len(origRunes)
	}
	return string(origRunes[fim:]), true
}

// indexNormalizado acha a posicao, em RUNES de s (nao bytes -- acento em
// UTF-8 ocupa mais de 1 byte, e cortar por indice de byte no meio de um
// caractere multi-byte corromperia o texto), onde alvo aparece dentro de
// s, comparando os dois em minusculo e sem acento (ver removerAcentos).
// Devolve -1 se nao achar.
//
// Comparar por rune, nao por byte, e o que garante que a posicao ache
// certo: ToLower e removerAcentos trocam CADA rune por exatamente um
// outro rune (nunca fundem nem quebram um caractere em mais de um), entao
// a posicao de um rune no texto normalizado e a MESMA posicao no texto
// original -- so o valor do rune muda, nunca onde ele esta.
func indexNormalizado(s, alvo string) int {
	rs := []rune(removerAcentos(strings.ToLower(s)))
	ra := []rune(removerAcentos(strings.ToLower(alvo)))
	if len(ra) == 0 || len(ra) > len(rs) {
		return -1
	}
	for i := 0; i+len(ra) <= len(rs); i++ {
		if runesIguais(rs[i:i+len(ra)], ra) {
			return i
		}
	}
	return -1
}

func runesIguais(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// removerAcentos troca vogal acentuada e cedilha pela letra base -- o
// reconhecedor erra acento com frequencia maior que a letra em si
// ("Razäo" em vez de "Razão" ja apareceu num documento real, ver README
// fase 5), entao comparar rotulo ignorando acento acha mais campo do que
// comparar exato. So cobre o alfabeto latino que aparece em portugues;
// nao e um normalizador Unicode geral.
func removerAcentos(s string) string {
	troca := strings.NewReplacer(
		"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a",
		"é", "e", "è", "e", "ê", "e", "ë", "e",
		"í", "i", "ì", "i", "î", "i", "ï", "i",
		"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o",
		"ú", "u", "ù", "u", "û", "u", "ü", "u",
		"ç", "c", "ñ", "n",
	)
	return troca.Replace(s)
}
