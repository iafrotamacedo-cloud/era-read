package read

import (
	"regexp"
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/era-read/extract"
	"github.com/iafrotamacedo-cloud/era-read/layout"
)

// DANFE e os campos do "Documento Auxiliar da Nota Fiscal Eletrônica" --
// ao contrário da DAV (documento próprio de um sistema de PDV, sem padrão
// entre empresas), o DANFE tem leiaute único, definido pelo CONFAZ e
// publicado no Manual de Orientação do Contribuinte da NF-e (Anexo II).
// Os rótulos abaixo são os desse leiaute oficial, conferidos na fonte
// primária (o Anexo II reproduzido em normas estaduais, ex. RICMS) e
// contra vários exemplos reais em 14/09/2026 -- não são um palpite.
//
// Diferença importante em relação a ExtrairDAV: ESTE schema ainda não foi
// testado contra um DANFE real passando pelo pipeline de detecção e
// reconhecimento deste motor -- nenhum documento desse tipo estava
// disponível nesta sessão. Os testes usam texto sintético fiel ao leiaute
// oficial, não a saída real de `read.Page`. Fica registrado como a
// diferença de confiança em relação à DAV, não escondido.
type DANFE struct {
	Emitente         Empresa // Nome fica vazio -- ver o comentário de ExtrairDANFE
	Destinatario     Empresa
	NaturezaOperacao string
	NumeroNF         string
	Serie            string
	ChaveAcesso      string // 44 dígitos, sem separador
	Protocolo        string
	DataEmissao      *time.Time
	DataSaidaEntrada *time.Time
	Impostos         map[string]extract.Money
	Itens            layout.Table
}

// EtiquetasImpostoDANFE são os rótulos do quadro "Cálculo do Imposto" do
// leiaute oficial do DANFE. Ordem não importa para a busca -- cada um é
// procurado independente, só a primeira ocorrência de cada conta (mesma
// regra de EtiquetasValor).
var EtiquetasImpostoDANFE = []string{
	"VALOR TOTAL DOS PRODUTOS",
	"VALOR TOTAL DA NOTA",
	"BASE DE CÁLCULO DO ICMS SUBSTITUIÇÃO",
	"VALOR DO ICMS SUBSTITUIÇÃO",
	"BASE DE CÁLCULO DO ICMS",
	"VALOR DO ICMS",
	"VALOR DO FRETE",
	"VALOR DO SEGURO",
	"DESCONTO",
	"OUTRAS DESPESAS ACESSÓRIAS",
	"VALOR DO IPI",
}

// chaveAcessoPattern acha a chave de acesso da NF-e: 44 dígitos, impressos
// corridos ou em grupos separados por espaço (a forma como o próprio
// leiaute e a maioria dos geradores de DANFE mostram, ex. "3508 0599
// 9990 9091 0270 5500 1000 0000 0151 8005 1273").
var chaveAcessoPattern = regexp.MustCompile(`\b(?:\d{4}\s*){10}\d{4}\b`)

// MaxGapProximaLinha limita, em múltiplos da altura da própria linha, a
// distância vertical que valorNaProximaLinha aceita entre o rótulo e a
// linha seguinte antes de considerá-la "a linha de valor" -- sem isso,
// duas linhas de seções diferentes que calham de vir em sequência no
// slice (ex. o canhoto e o cabeçalho principal do DANFE) seriam tratadas
// como rótulo+valor por engano. Medido contra um DANFE real em
// 15/09/2026: o vão entre um cabeçalho de campo e a linha de valor logo
// abaixo ficou sempre menor que a própria altura da linha (ex. 4px de vão
// para 21px de altura); o vão entre o canhoto e o cabeçalho principal (
// seções diferentes) ficou bem maior que a altura (48px de vão para 29px
// de altura) -- 1,0 separa os dois casos com folga em ambas as direções.
const MaxGapProximaLinha = 1.0

// valorNaProximaLinha acha etiqueta como palavra (ou parte de uma palavra)
// de linhas[i] e devolve o texto das palavras da linha seguinte cuja
// faixa X se sobrepõe à da etiqueta -- o valor de um campo impresso como
// cabeçalho de coluna numa linha, com o dado correspondente alinhado por
// baixo na linha seguinte.
//
// Esse jeito de imprimir é comum no DANFE (testado contra um documento
// real em 15/09/2026: "NOME/RAZÃO SOCIAL ... DATA DA EMISSÃO" numa linha,
// os valores correspondentes na linha de baixo) e ExtrairDANFE tenta isso
// ANTES de apósEtiqueta (rótulo e valor na mesma linha) -- ver comentário
// de ExtrairDANFE sobre por quê.
func valorNaProximaLinha(linhas []layout.Line, i int, etiqueta string) (string, bool) {
	if i < 0 || i+1 >= len(linhas) {
		return "", false
	}
	atual, prox := linhas[i], linhas[i+1]

	amin, amax := atual.Bounds()
	pmin, _ := prox.Bounds()
	altura := amax.Y - amin.Y
	if altura <= 0 || pmin.Y-amax.Y > altura*MaxGapProximaLinha {
		return "", false
	}

	achouEtiqueta := false
	var xmin, xmax float64
	for _, w := range atual.Words {
		if indexNormalizado(w.Text, etiqueta) < 0 {
			continue
		}
		wmin, wmax := w.Box.Bounds()
		if !achouEtiqueta || wmin.X < xmin {
			xmin = wmin.X
		}
		if !achouEtiqueta || wmax.X > xmax {
			xmax = wmax.X
		}
		achouEtiqueta = true
	}
	if !achouEtiqueta {
		return "", false
	}

	var partes []string
	for _, w := range prox.Words {
		wmin, wmax := w.Box.Bounds()
		if wmax.X < xmin || wmin.X > xmax {
			continue // fora da faixa X da etiqueta -- outra coluna
		}
		partes = append(partes, w.Text)
	}
	if len(partes) == 0 {
		return "", false
	}
	return strings.Join(partes, " "), true
}

// ExtrairDANFE reconstitui os campos de um DANFE a partir das linhas já
// lidas e agrupadas (a saída de read.Page). Cada campo que não encontrar
// o rótulo esperado fica no valor zero -- nunca um palpite.
//
// Emitente.Nome fica vazio de propósito: ao contrário do destinatário
// (rotulado "NOME/RAZÃO SOCIAL") e da DAV (rotulada "Razão Social:"), o
// leiaute oficial do DANFE NÃO rotula o nome do emitente com texto -- o
// quadro "Identificação do emitente" é só o espaço reservado para a
// empresa colocar seu próprio papel timbrado (logotipo, nome, endereço),
// sem rótulo impresso nenhum precedendo o nome. Não há como achar por
// rótulo o que o documento não rotula; o CNPJ do emitente já sai certo,
// porque esse sim tem rótulo ("CNPJ") na mesma faixa.
//
// A maioria dos campos abaixo tenta valorNaProximaLinha ANTES de
// apósEtiqueta, não depois -- na ordem inversa da DAV. Um DANFE real
// testado em 15/09/2026 mostrou que o leiaute nacional imprime a maior
// parte dos quadros como um cabeçalho com vários rótulos numa linha e os
// valores correspondentes alinhados por coluna na linha de baixo (não
// "rótulo: valor" colado, como a DAV); nessa mesma linha de cabeçalho, o
// texto que vem depois de um rótulo costuma ser só o PRÓXIMO rótulo do
// mesmo cabeçalho, não um valor -- então tentar apósEtiqueta primeiro
// pegaria o rótulo vizinho por engano. Quando o valor de fato mora na
// mesma linha do rótulo (visto no canhoto: "SÉRIE : 1"), a linha seguinte
// não tem palavra alinhada com a coluna do rótulo, valorNaProximaLinha
// devolve false, e apósEtiqueta continua cobrindo esse caso.
func ExtrairDANFE(linhas []layout.Line) DANFE {
	var d DANFE
	d.Impostos = make(map[string]extract.Money)

	inicioItens, fimItens := -1, -1
	achouDestinatario := false
	destinatarioPreenchido := false

	for i, l := range linhas {
		texto := l.Text()

		if strings.Contains(strings.ToLower(texto), "destinat") {
			achouDestinatario = true
		}

		// A faixa "INSCRIÇÃO ESTADUAL ... CNPJ ... CHAVE DE ACESSO" fica
		// ANTES de "DESTINATÁRIO/REMETENTE" no leiaute -- parar de procurar
		// CNPJ do emitente a partir dali evita pegar o CNPJ do
		// destinatário (que tem o próprio rótulo "CNPJ/CPF" mais adiante)
		// como se fosse do emitente.
		if d.Emitente.CNPJOuCPF == "" && !achouDestinatario {
			if cnpjs := extract.FindCNPJs(texto); len(cnpjs) > 0 {
				d.Emitente.CNPJOuCPF = cnpjs[0]
			}
		}

		// destinatarioPreenchido, não "CNPJOuCPF == vazio", trava a busca
		// depois da PRIMEIRA ocorrência de "RAZÃO SOCIAL" -- o quadro
		// TRANSPORTADOR/VOLUMES TRANSPORTADOS, mais abaixo na página, tem
		// o próprio rótulo "RAZÃO SOCIAL" (do transportador); se o CNPJ do
		// destinatário não for achado na primeira ocorrência (ex. porque o
		// reconhecedor perdeu aquela região -- visto no documento real de
		// 15/09/2026), continuar procurando pegaria o nome do
		// transportador como se fosse do destinatário.
		if !destinatarioPreenchido {
			if v, ok := valorNaProximaLinha(linhas, i, "RAZÃO SOCIAL"); ok {
				destinatarioPreenchido = true
				if cnpjs := extract.FindCNPJs(v); len(cnpjs) > 0 {
					d.Destinatario.CNPJOuCPF = cnpjs[0]
					d.Destinatario.Nome = strings.TrimSpace(antesDoPrimeiroDigito(v))
				} else if cpfs := extract.FindCPFs(v); len(cpfs) > 0 {
					d.Destinatario.CNPJOuCPF = cpfs[0]
					d.Destinatario.Nome = strings.TrimSpace(antesDoPrimeiroDigito(v))
				} else {
					d.Destinatario.Nome = strings.TrimSpace(v)
				}
				// O CNPJ do destinatário nem sempre mora na mesma coluna
				// do nome -- testado contra 3 DANFEs reais em 15/09/2026,
				// apareceu em posições diferentes em cada um: dentro da
				// própria coluna do nome (raro), solto em outra coluna da
				// MESMA linha de valor (visto na nota da Madeireira Rio
				// Branco: "FROTA MACEDO ENGENHARIA EIRELI 27.363.223/
				// 0001-70 08/07/2026" tudo numa Line só, mas o CNPJ fora
				// da faixa X do rótulo), ou colado na própria linha do
				// rótulo "RAZÃO SOCIAL" (visto na nota da TIM/JJM). As
				// duas buscas abaixo cobrem esses dois casos que
				// valorNaProximaLinha, sozinho (limitado à coluna do
				// nome), não pegava.
				if d.Destinatario.CNPJOuCPF == "" && i+1 < len(linhas) {
					proxTexto := linhas[i+1].Text()
					if cnpjs := extract.FindCNPJs(proxTexto); len(cnpjs) > 0 {
						d.Destinatario.CNPJOuCPF = cnpjs[0]
					} else if cpfs := extract.FindCPFs(proxTexto); len(cpfs) > 0 {
						d.Destinatario.CNPJOuCPF = cpfs[0]
					}
				}
				if d.Destinatario.CNPJOuCPF == "" {
					d.Destinatario.CNPJOuCPF = primeiroCNPJOuCPFApos(texto, "RAZÃO SOCIAL")
				}
			} else if resto, ok := apósEtiqueta(texto, "RAZÃO SOCIAL"); ok {
				destinatarioPreenchido = true
				d.Destinatario.Nome = strings.TrimSpace(cortarAntesDe(resto, "CNPJ"))
				d.Destinatario.CNPJOuCPF = primeiroCNPJOuCPFApos(texto, "RAZÃO SOCIAL")
			}
		}

		if d.NaturezaOperacao == "" {
			if v, ok := valorNaProximaLinha(linhas, i, "NATUREZA DA OPERAÇÃO"); ok {
				d.NaturezaOperacao = strings.TrimSpace(v)
			} else if resto, ok := apósEtiqueta(texto, "NATUREZA DA OPERAÇÃO"); ok {
				d.NaturezaOperacao = strings.TrimSpace(primeiraPalavraOuFrase(resto))
			}
		}

		if d.NumeroNF == "" {
			if v, ok := valorNaProximaLinha(linhas, i, "N.º"); ok {
				d.NumeroNF = strings.TrimSpace(v)
			} else if v, ok := valorNaProximaLinha(linhas, i, "Nº"); ok {
				d.NumeroNF = strings.TrimSpace(v)
			} else if resto, ok := apósEtiqueta(texto, "N.º"); ok {
				d.NumeroNF = strings.TrimSpace(cortarAntesDe(resto, "SÉRIE"))
			} else if resto, ok := apósEtiqueta(texto, "Nº"); ok {
				d.NumeroNF = strings.TrimSpace(cortarAntesDe(resto, "SÉRIE"))
			}
		}
		if d.Serie == "" {
			if v, ok := valorNaProximaLinha(linhas, i, "SÉRIE"); ok {
				d.Serie = strings.TrimSpace(v)
			} else if resto, ok := apósEtiqueta(texto, "SÉRIE"); ok {
				d.Serie = strings.TrimSpace(primeiraPalavraOuFrase(resto))
			}
		}

		if d.ChaveAcesso == "" {
			if m := chaveAcessoPattern.FindString(texto); m != "" {
				d.ChaveAcesso = strings.ReplaceAll(m, " ", "")
			}
		}

		if d.Protocolo == "" {
			if v, ok := valorNaProximaLinha(linhas, i, "PROTOCOLO DE AUTORIZAÇÃO DE USO"); ok {
				d.Protocolo = strings.TrimSpace(v)
			} else if resto, ok := apósEtiqueta(texto, "PROTOCOLO DE AUTORIZAÇÃO DE USO"); ok {
				d.Protocolo = strings.TrimSpace(resto)
			}
		}

		if d.DataEmissao == nil {
			// "DATA EMISSÃO", sem o "DA", é como a nota da TIM/JJM
			// imprimiu esse rótulo (testado em 15/09/2026) -- variante de
			// redação legítima de outro emissor, não erro de OCR (a nota
			// da Madeireira Rio Branco e o leiaute oficial usam "DATA DA
			// EMISSÃO"); não é substring uma da outra, então não há risco
			// de uma bater na hora errada.
			for _, rotulo := range []string{"DATA DA EMISSÃO", "DATA EMISSÃO"} {
				if v, ok := valorNaProximaLinha(linhas, i, rotulo); ok {
					if t, err := extract.ParseDateBR(v); err == nil {
						d.DataEmissao = &t
						break
					}
				}
				if resto, ok := apósEtiqueta(texto, rotulo); ok {
					if t, err := extract.ParseDateBR(resto); err == nil {
						d.DataEmissao = &t
					}
					break
				}
			}
		}
		if d.DataSaidaEntrada == nil {
			for _, rotulo := range []string{"DATA DA SAÍDA/ENTRADA", "DATA DE SAÍDA/ENTRADA", "DATA DA SAÍDA", "DATA DA ENTRADA"} {
				if v, ok := valorNaProximaLinha(linhas, i, rotulo); ok {
					if t, err := extract.ParseDateBR(v); err == nil {
						d.DataSaidaEntrada = &t
						break
					}
				}
				if resto, ok := apósEtiqueta(texto, rotulo); ok {
					if t, err := extract.ParseDateBR(resto); err == nil {
						d.DataSaidaEntrada = &t
					}
					break
				}
			}
		}

		itemEncontrado := false
		for _, etiqueta := range EtiquetasImpostoDANFE {
			if _, ja := d.Impostos[etiqueta]; ja {
				continue
			}
			// Duas etiquetas da lista são prefixo uma da outra ("VALOR DO
			// ICMS" de "VALOR DO ICMS SUBSTITUIÇÃO"; "BASE DE CÁLCULO DO
			// ICMS" da sua versão "SUBSTITUIÇÃO"). Um DANFE real testado
			// em 15/09/2026 colou as duas num único blob de OCR ("VALOR
			// DO ICMS SUBSTITUICAO ALOR TOTAL DOSPRODUTOS"), e a etiqueta
			// curta batia nesse blob usando a faixa X dele (larga demais,
			// da etiqueta ERRADA) -- "VALOR DO ICMS" saiu 0,00 em vez de
			// 71,71. Se a etiqueta mais longa também aparece nesta linha,
			// deixa ELA reivindicar a linha; a curta espera a vez dela
			// (ou fica sem achar, se só aparecer colada assim).
			if etiquetaTemIrmaMaisLonga(texto, etiqueta, EtiquetasImpostoDANFE) {
				continue
			}
			valor, ok := valorNaProximaLinha(linhas, i, etiqueta)
			if !ok {
				valor, ok = apósEtiqueta(texto, etiqueta)
			}
			if !ok {
				continue
			}
			itemEncontrado = true
			if v, err := extract.ParseMoney(valor); err == nil {
				d.Impostos[etiqueta] = v
			}
		}

		// A tabela de itens começa depois do cabeçalho "DADOS DO PRODUTO" e
		// termina em "DADOS ADICIONAIS", o quadro seguinte no leiaute
		// oficial (mesmo esquema de início/fim de ExtrairDAV, mas com um
		// marcador de fim diferente -- ver o motivo abaixo).
		//
		// itemEncontrado (achar um valor do quadro "CÁLCULO DO IMPOSTO")
		// continua valendo como marcador de fim TAMBÉM, porque a posição
		// relativa desse quadro varia: no leiaute oficial e no texto
		// sintético de danfe_test.go ele vem DEPOIS da tabela de itens
		// (marcando o fim dela, como na DAV), mas um DANFE real testado em
		// 15/09/2026 imprimiu o quadro de imposto ANTES da tabela de itens
		// -- ali "dados adicionais" é o único marcador de fim que
		// realmente aparece depois da tabela.
		if inicioItens == -1 && strings.Contains(strings.ToLower(texto), "dados do produto") {
			inicioItens = i + 1
		}
		if inicioItens != -1 && fimItens == -1 &&
			(itemEncontrado || strings.Contains(strings.ToLower(texto), "dados adicionais")) {
			fimItens = i
		}
	}

	if inicioItens != -1 {
		fim := fimItens
		if fim == -1 || fim <= inicioItens {
			fim = len(linhas)
		}
		if inicioItens < fim {
			d.Itens = layout.GroupTableWords(linhas[inicioItens:fim])
		}
	}

	return d
}

// etiquetaTemIrmaMaisLonga informa se, entre todas, existe uma etiqueta
// mais longa da qual etiqueta é prefixo normalizado E que também aparece
// em texto -- ver o comentário no laço de Impostos em ExtrairDANFE sobre
// por que isso evita reivindicar a linha errada quando duas etiquetas
// aparecem coladas na mesma região de OCR.
func etiquetaTemIrmaMaisLonga(texto, etiqueta string, todas []string) bool {
	for _, outra := range todas {
		if outra == etiqueta || len(outra) <= len(etiqueta) {
			continue
		}
		if indexNormalizado(outra, etiqueta) != 0 {
			continue // etiqueta nao e prefixo de outra
		}
		if indexNormalizado(texto, outra) >= 0 {
			return true
		}
	}
	return false
}

// antesDoPrimeiroDigito devolve s ate o primeiro digito -- usado para
// separar nome de CNPJ/CPF quando os dois vem juntos, sem rotulo entre
// eles, na mesma faixa de coluna (ver valorNaProximaLinha). Corta pelo
// digito em si, não pelo texto formatado que extract.FindCNPJs/FindCPFs
// devolve, porque esse texto (com pontuacao) pode nao aparecer literal no
// OCR bruto -- nome de empresa nao tem digito, CNPJ/CPF sempre comeca com
// um.
func antesDoPrimeiroDigito(s string) string {
	for i, r := range s {
		if r >= '0' && r <= '9' {
			return s[:i]
		}
	}
	return s
}

// primeiraPalavraOuFrase devolve s ate o primeiro vao duplo de espaco (o
// que separa um campo do proximo quando dois campos ficam colados na
// mesma linha sem outro rotulo entre eles) -- ou s inteiro, se não achar.
// Usado para campos curtos (natureza da operação, série) que não têm um
// rótulo seguinte conhecido para cortar antes, ao contrário de
// cortarAntesDe.
//
// O TrimLeft inicial descarta um separador solto entre rótulo e valor
// (":", com ou sem espaço em volta) -- no leiaute oficial não existe
// esse separador (é só o nome do campo, sem dois-pontos), mas um DANFE
// real testado em 14/09/2026 imprimiu "SÉRIE : 1" (a borda da caixa do
// campo, lida como ":"), e sem isso o valor extraído ficava ": 1" em
// vez de "1".
func primeiraPalavraOuFrase(s string) string {
	s = strings.TrimLeft(s, " :")
	if idx := strings.Index(s, "  "); idx >= 0 {
		return s[:idx]
	}
	return s
}
