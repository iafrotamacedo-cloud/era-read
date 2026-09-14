package read

import (
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/era-read/extract"
	"github.com/iafrotamacedo-cloud/era-read/layout"
)

// Empresa e uma pessoa juridica (ou fisica) identificada num documento --
// o nome que aparece perto de um CNPJ ou CPF no texto.
type Empresa struct {
	Nome      string
	CNPJOuCPF string
}

// DAV e os campos de uma "Documento Auxiliar de Venda" tipica de comercio
// brasileiro -- o tipo de documento usado para validar este schema (ver
// ExtrairDAV).
//
// E um schema ESPECIFICO deste tipo de documento, nao um parser universal
// de nota fiscal: os rotulos que ExtrairDAV procura ("Social:", "Nome:",
// "Documento:"...) sao os que apareceram nos dois documentos reais da
// Frota Macedo usados para validar isto (ver README), nao um catalogo de
// todo jeito de layout de DAV que existe. Um documento com outros rotulos
// simplesmente deixa os campos correspondentes vazios -- ExtrairDAV nunca
// adivinha.
type DAV struct {
	Emitente        Empresa
	Destinatario    Empresa
	NumeroDocumento string
	DataEmissao     *time.Time
	Totais          map[string]extract.Money
	Itens           layout.Table
}

// rotuloDocumento e o texto que precede o numero do documento -- variantes
// vistas nos documentos reais ("N° do Documento:", sem o "N°" quando o
// reconhecedor perde o simbolo de ordinal, ver README fase 5).
var rotulosNumeroDocumento = []string{"do Documento:", "Documento:"}

// ExtrairDAV reconstitui os campos de uma DAV a partir das linhas ja
// lidas e agrupadas (a saida de read.Page). Cada campo que nao encontrar
// o rotulo esperado fica no valor zero -- nunca um palpite.
func ExtrairDAV(linhas []layout.Line) DAV {
	var d DAV
	d.Totais = make(map[string]extract.Money)

	inicioItens, fimItens := -1, -1

	for i, l := range linhas {
		texto := l.Text()

		if d.Emitente.CNPJOuCPF == "" {
			if resto, ok := apósEtiqueta(texto, "Social:"); ok {
				d.Emitente.Nome = strings.TrimSpace(cortarAntesDe(resto, "CNPJ"))
				if cnpjs := extract.FindCNPJs(texto); len(cnpjs) > 0 {
					d.Emitente.CNPJOuCPF = cnpjs[0]
				}
			}
		}

		if d.Destinatario.CNPJOuCPF == "" {
			if resto, ok := apósEtiqueta(texto, "Nome:"); ok {
				d.Destinatario.Nome = strings.TrimSpace(cortarAntesDe(resto, "CPF"))
				achado := primeiroCNPJOuCPFApos(texto, "Nome:")
				d.Destinatario.CNPJOuCPF = achado
			}
		}

		if d.NumeroDocumento == "" {
			for _, rotulo := range rotulosNumeroDocumento {
				if resto, ok := apósEtiqueta(texto, rotulo); ok {
					d.NumeroDocumento = strings.TrimSpace(resto)
					break
				}
			}
		}

		if d.DataEmissao == nil {
			if resto, ok := apósEtiqueta(texto, "Emis:"); ok {
				if t, err := extract.ParseDateBR(resto); err == nil {
					d.DataEmissao = &t
				}
			}
		}

		etiquetaEncontrada := false
		for _, etiqueta := range EtiquetasValor {
			if _, ja := d.Totais[etiqueta]; ja {
				continue
			}
			resto, ok := apósEtiqueta(texto, etiqueta)
			if !ok {
				continue
			}
			etiquetaEncontrada = true
			if v, err := extract.ParseMoney(resto); err == nil {
				d.Totais[etiqueta] = v
			}
		}

		// A tabela de itens fica entre o cabecalho (a linha que titula as
		// colunas: "Produto/Endereco Embalagem Quantidade...") e a
		// primeira linha de totais -- a mesma etiqueta que EtiquetasValor
		// ja procura marca o fim. O cabecalho e identificado por
		// "endereco"/"quantidade" em vez de "produto": num documento real
		// esse rotulo saiu truncado ("oduto/Endere", faltando o "Pr" do
		// inicio -- ver README, fase 5) e "produto" sozinho nao bate mais.
		if inicioItens == -1 && (strings.Contains(strings.ToLower(texto), "endere") ||
			strings.Contains(strings.ToLower(texto), "quantidade")) {
			inicioItens = i + 1
		}
		if inicioItens != -1 && fimItens == -1 && etiquetaEncontrada {
			fimItens = i
		}
	}

	if inicioItens != -1 {
		fim := fimItens
		if fim == -1 || fim <= inicioItens {
			fim = len(linhas)
		}
		if inicioItens < fim {
			d.Itens = layout.GroupTable(linhas[inicioItens:fim], layout.DefaultGapFactor)
		}
	}

	return d
}

// cortarAntesDe devolve o prefixo de s antes da primeira ocorrencia de
// marcador (sem diferenciar maiusculas/minusculas); devolve s inteiro se
// marcador nao aparecer.
func cortarAntesDe(s, marcador string) string {
	idx := strings.Index(strings.ToLower(s), strings.ToLower(marcador))
	if idx < 0 {
		return s
	}
	return s[:idx]
}

// primeiroCNPJOuCPFApos acha o primeiro CNPJ ou CPF valido no texto que
// vem depois de rotulo -- prefere CNPJ quando os dois aparecem (o campo
// "CPF/CNPJ:" de uma pessoa juridica sempre traz CNPJ).
func primeiroCNPJOuCPFApos(texto, rotulo string) string {
	resto, ok := apósEtiqueta(texto, rotulo)
	if !ok {
		return ""
	}
	if cnpjs := extract.FindCNPJs(resto); len(cnpjs) > 0 {
		return cnpjs[0]
	}
	if cpfs := extract.FindCPFs(resto); len(cpfs) > 0 {
		return cpfs[0]
	}
	return ""
}
