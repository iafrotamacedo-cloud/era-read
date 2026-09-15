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
func ExtrairDANFE(linhas []layout.Line) DANFE {
	var d DANFE
	d.Impostos = make(map[string]extract.Money)

	inicioItens, fimItens := -1, -1
	achouDestinatario := false

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

		if d.Destinatario.CNPJOuCPF == "" {
			if resto, ok := apósEtiqueta(texto, "RAZÃO SOCIAL"); ok {
				d.Destinatario.Nome = strings.TrimSpace(cortarAntesDe(resto, "CNPJ"))
				d.Destinatario.CNPJOuCPF = primeiroCNPJOuCPFApos(texto, "RAZÃO SOCIAL")
			}
		}

		if d.NaturezaOperacao == "" {
			if resto, ok := apósEtiqueta(texto, "NATUREZA DA OPERAÇÃO"); ok {
				d.NaturezaOperacao = strings.TrimSpace(primeiraPalavraOuFrase(resto))
			}
		}

		if d.NumeroNF == "" {
			if resto, ok := apósEtiqueta(texto, "N.º"); ok {
				d.NumeroNF = strings.TrimSpace(cortarAntesDe(resto, "SÉRIE"))
			} else if resto, ok := apósEtiqueta(texto, "Nº"); ok {
				d.NumeroNF = strings.TrimSpace(cortarAntesDe(resto, "SÉRIE"))
			}
		}
		if d.Serie == "" {
			if resto, ok := apósEtiqueta(texto, "SÉRIE"); ok {
				d.Serie = strings.TrimSpace(primeiraPalavraOuFrase(resto))
			}
		}

		if d.ChaveAcesso == "" {
			if m := chaveAcessoPattern.FindString(texto); m != "" {
				d.ChaveAcesso = strings.ReplaceAll(m, " ", "")
			}
		}

		if d.Protocolo == "" {
			if resto, ok := apósEtiqueta(texto, "PROTOCOLO DE AUTORIZAÇÃO DE USO"); ok {
				d.Protocolo = strings.TrimSpace(resto)
			}
		}

		if d.DataEmissao == nil {
			if resto, ok := apósEtiqueta(texto, "DATA DA EMISSÃO"); ok {
				if t, err := extract.ParseDateBR(resto); err == nil {
					d.DataEmissao = &t
				}
			}
		}
		if d.DataSaidaEntrada == nil {
			for _, rotulo := range []string{"DATA DA SAÍDA/ENTRADA", "DATA DE SAÍDA/ENTRADA", "DATA DA SAÍDA", "DATA DA ENTRADA"} {
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
			resto, ok := apósEtiqueta(texto, etiqueta)
			if !ok {
				continue
			}
			itemEncontrado = true
			if v, err := extract.ParseMoney(resto); err == nil {
				d.Impostos[etiqueta] = v
			}
		}

		// A tabela de itens fica entre o cabeçalho "DADOS DO PRODUTO" e a
		// primeira linha do quadro "CÁLCULO DO IMPOSTO" (que já é
		// procurado acima) -- o mesmo esquema de início/fim de ExtrairDAV.
		if inicioItens == -1 && strings.Contains(strings.ToLower(texto), "dados do produto") {
			inicioItens = i + 1
		}
		if inicioItens != -1 && fimItens == -1 && itemEncontrado {
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

// primeiraPalavraOuFrase devolve s ate o primeiro vao duplo de espaco (o
// que separa um campo do proximo quando dois campos ficam colados na
// mesma linha sem outro rotulo entre eles) -- ou s inteiro, se não achar.
// Usado para campos curtos (natureza da operação, série) que não têm um
// rótulo seguinte conhecido para cortar antes, ao contrário de
// cortarAntesDe.
func primeiraPalavraOuFrase(s string) string {
	if idx := strings.Index(s, "  "); idx >= 0 {
		return s[:idx]
	}
	return s
}
