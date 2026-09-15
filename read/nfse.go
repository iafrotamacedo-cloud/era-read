package read

import (
	"regexp"
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/era-read/extract"
	"github.com/iafrotamacedo-cloud/era-read/layout"
)

// NFSe e os campos do DANFSe ("Documento Auxiliar da NFS-e"), o terceiro
// tipo de documento que a Frota Macedo lê -- ao contrário da DAV
// (documento próprio de PDV, sem padrão) e a exemplo do DANFE, o DANFSe
// tem leiaute único e NACIONAL, mas por um motivo diferente do DANFE: não
// é uma norma tributária estadual reproduzindo um anexo do CONFAZ, é a
// "Nota Técnica Nº 008 -- Especificações Técnicas do DANFSe" (versão
// 1.02, 14/07/2026), publicada pela Secretaria-Executiva do Comitê Gestor
// da Nota Fiscal de Serviço Eletrônica de Padrão Nacional (SE/CGNFS-e) no
// portal oficial gov.br/nfse -- o programa "NFS-e Nacional" que unificou,
// desde 2022, a aparência do documento entre municípios (a arrecadação do
// ISSQN continua municipal; só o LEIAUTE do documento e o formato do XML
// por trás são nacionais agora). Rótulos conferidos direto nessa fonte
// primária e contra um DANFSe real (nota de Antonio Carlos de Lima,
// Fortaleza/CE, competência 07/2026) em 15/09/2026.
//
// Schema modesto de propósito, do mesmo jeito que DAV e DANFE: o DANFSe
// tem MUITO mais campos do que os capturados aqui (toda a seção de
// tributação IBS/CBS da reforma tributária, por exemplo, tem mais de 15
// campos sozinha) -- só os campos relevantes para a conferência da Frota
// Macedo entraram na v1. Um campo que ExtrairNFSe não encontrar fica no
// valor zero, nunca um palpite.
type NFSe struct {
	Prestador        Empresa
	Tomador          Empresa
	Municipio        string // município emissor da NFS-e, ex. "FORTALEZA/CE"
	NumeroNFSe       string
	ChaveAcesso      string // 50 dígitos, sem separador
	Competencia      *time.Time
	DataEmissao      *time.Time
	DescricaoServico string
	ValorServico     extract.Money // "Valor da Operação/Serviço"
	BaseCalculoISSQN extract.Money
	ISSQNApurado     extract.Money
	ValorLiquido     extract.Money // "Valor Líquido da NFS-e"
}

// chaveAcessoNFSePattern acha a chave de acesso da NFS-e: 50 dígitos
// corridos -- ao contrário da chave do DANFE (44 dígitos, quase sempre
// impressa em grupos de 4 separados por espaço), o exemplo real conferido
// em 15/09/2026 imprimiu os 50 dígitos sem separador nenhum, como a Nota
// Técnica nº 008 pede ("impressa em único bloco").
var chaveAcessoNFSePattern = regexp.MustCompile(`\b\d{50}\b`)

// ExtrairNFSe reconstitui os campos de um DANFSe a partir das linhas já
// lidas e agrupadas (a saída de read.Page). Segue a mesma estratégia do
// ExtrairDANFE -- tenta valorNaProximaLinha (rótulo numa linha, valor
// alinhado por coluna na linha seguinte) antes de apósEtiqueta (mesma
// linha) -- porque o DANFSe usa o mesmo estilo de quadro com cabeçalho de
// colunas que o DANFE, confirmado no documento real usado para validar
// isto.
func ExtrairNFSe(linhas []layout.Line) NFSe {
	var n NFSe
	achouTomador := false

	for i, l := range linhas {
		texto := l.Text()

		if strings.Contains(strings.ToLower(texto), "tomador/adquirente") {
			achouTomador = true
		}

		// O gate confere Nome E CNPJ, não só um dos dois -- os rótulos
		// "Nome/Nome Empresarial" e "CNPJ/CPF/NIF" ficam em sub-quadros de
		// cabeçalho DIFERENTES dentro do mesmo bloco (ver comentário de
		// preencherEmpresaNFSe), então um pode aparecer em linha bem
		// depois do outro -- travar assim que só o CNPJ aparecesse
		// deixaria o Nome, se vier depois, sem nunca ser procurado.
		if !achouTomador {
			if n.Prestador.Nome == "" || n.Prestador.CNPJOuCPF == "" {
				preencherEmpresaNFSe(&n.Prestador, linhas, i, texto)
			}
		} else {
			if n.Tomador.Nome == "" || n.Tomador.CNPJOuCPF == "" {
				preencherEmpresaNFSe(&n.Tomador, linhas, i, texto)
			}
		}

		if n.Municipio == "" {
			if resto, ok := apósEtiqueta(texto, "Município:"); ok {
				n.Municipio = strings.TrimSpace(primeiraPalavraOuFrase(resto))
			}
		}

		if n.NumeroNFSe == "" {
			if v, ok := valorNaProximaLinha(linhas, i, "NÚMERO DA NFS-E"); ok {
				n.NumeroNFSe = strings.TrimSpace(primeiraPalavraOuFrase(v))
			} else if resto, ok := apósEtiqueta(texto, "NÚMERO DA NFS-E"); ok {
				n.NumeroNFSe = strings.TrimSpace(primeiraPalavraOuFrase(resto))
			}
		}

		if n.ChaveAcesso == "" {
			if m := chaveAcessoNFSePattern.FindString(texto); m != "" {
				n.ChaveAcesso = m
			}
		}

		if n.Competencia == nil {
			if v, ok := valorNaProximaLinha(linhas, i, "COMPETÊNCIA DA NFS-E"); ok {
				if t, err := extract.ParseDateBR(v); err == nil {
					n.Competencia = &t
				}
			}
			if n.Competencia == nil {
				if resto, ok := apósEtiqueta(texto, "COMPETÊNCIA DA NFS-E"); ok {
					if t, err := extract.ParseDateBR(resto); err == nil {
						n.Competencia = &t
					}
				}
			}
		}

		if n.DataEmissao == nil {
			if v, ok := valorNaProximaLinha(linhas, i, "DATA E HORA DA EMISSÃO DA NFS-E"); ok {
				if t, err := extract.ParseDateBR(v); err == nil {
					n.DataEmissao = &t
				}
			}
			if n.DataEmissao == nil {
				if resto, ok := apósEtiqueta(texto, "DATA E HORA DA EMISSÃO DA NFS-E"); ok {
					if t, err := extract.ParseDateBR(resto); err == nil {
						n.DataEmissao = &t
					}
				}
			}
		}

		if n.DescricaoServico == "" {
			if resto, ok := apósEtiqueta(texto, "Descrição do Serviço"); ok {
				n.DescricaoServico = strings.TrimSpace(resto)
			}
		}

		if n.ValorServico == 0 {
			if v, ok := valorNaProximaLinha(linhas, i, "Valor da Operação/Serviço"); ok {
				if m, err := extract.ParseMoney(v); err == nil {
					n.ValorServico = m
				}
			} else if resto, ok := apósEtiqueta(texto, "Valor da Operação/Serviço"); ok {
				if m, err := extract.ParseMoney(resto); err == nil {
					n.ValorServico = m
				}
			}
		}

		if n.BaseCalculoISSQN == 0 {
			if v, ok := valorNaProximaLinha(linhas, i, "BC ISSQN"); ok {
				if m, err := extract.ParseMoney(v); err == nil {
					n.BaseCalculoISSQN = m
				}
			} else if resto, ok := apósEtiqueta(texto, "BC ISSQN"); ok {
				if m, err := extract.ParseMoney(resto); err == nil {
					n.BaseCalculoISSQN = m
				}
			}
		}

		if n.ISSQNApurado == 0 {
			if v, ok := valorNaProximaLinha(linhas, i, "ISSQN Apurado"); ok {
				if m, err := extract.ParseMoney(v); err == nil {
					n.ISSQNApurado = m
				}
			} else if resto, ok := apósEtiqueta(texto, "ISSQN Apurado"); ok {
				if m, err := extract.ParseMoney(resto); err == nil {
					n.ISSQNApurado = m
				}
			}
		}

		if n.ValorLiquido == 0 {
			if v, ok := valorNaProximaLinha(linhas, i, "Valor Líquido da NFS-e"); ok {
				if m, err := extract.ParseMoney(v); err == nil {
					n.ValorLiquido = m
				}
			} else if resto, ok := apósEtiqueta(texto, "Valor Líquido da NFS-e"); ok {
				if m, err := extract.ParseMoney(resto); err == nil {
					n.ValorLiquido = m
				}
			}
		}
	}

	return n
}

// preencherEmpresaNFSe procura, a partir da linha i, o nome e o CNPJ/CPF
// de uma Empresa (Prestador ou Tomador) -- os dois rótulos ("Nome/Nome
// Empresarial" e "CNPJ/CPF/NIF") aparecem em quadros de cabeçalho
// separados dentro do mesmo bloco (ver comentário de ExtrairNFSe), então
// são buscados independentemente, cada um só quando ainda não achado.
func preencherEmpresaNFSe(emp *Empresa, linhas []layout.Line, i int, texto string) {
	if emp.Nome == "" {
		if v, ok := valorNaProximaLinha(linhas, i, "Nome/Nome Empresarial"); ok {
			emp.Nome = strings.TrimSpace(v)
		} else if resto, ok := apósEtiqueta(texto, "Nome/Nome Empresarial"); ok {
			emp.Nome = strings.TrimSpace(primeiraPalavraOuFrase(resto))
		}
	}
	if emp.CNPJOuCPF == "" {
		if v, ok := valorNaProximaLinha(linhas, i, "CNPJ/CPF/NIF"); ok {
			if cnpjs := extract.FindCNPJs(v); len(cnpjs) > 0 {
				emp.CNPJOuCPF = cnpjs[0]
			} else if cpfs := extract.FindCPFs(v); len(cpfs) > 0 {
				emp.CNPJOuCPF = cpfs[0]
			}
		} else if resto, ok := apósEtiqueta(texto, "CNPJ/CPF/NIF"); ok {
			if cnpjs := extract.FindCNPJs(resto); len(cnpjs) > 0 {
				emp.CNPJOuCPF = cnpjs[0]
			} else if cpfs := extract.FindCPFs(resto); len(cpfs) > 0 {
				emp.CNPJOuCPF = cpfs[0]
			}
		}
	}
}
