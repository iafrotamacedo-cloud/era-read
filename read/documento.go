package read

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/iafrotamacedo-cloud/era-read/contrato"
	"github.com/iafrotamacedo-cloud/era-read/extract"
	"github.com/iafrotamacedo-cloud/era-read/layout"
)

// TipoDocumento e a classificacao grosseira que ExtrairDocumento usa para
// escolher o schema. Nao e um classificador treinado -- so rotulos que o
// proprio documento imprime.
type TipoDocumento string

const (
	TipoDesconhecido TipoDocumento = ""
	TipoDAV          TipoDocumento = "dav"
	TipoDANFE        TipoDocumento = "danfe"
)

// Documento e o resultado de ExtrairDocumento: o tipo escolhido, o schema
// correspondente e os achados livres (sempre preenchidos, mesmo quando o
// tipo e conhecido -- o funil pode cruzar os dois).
type Documento struct {
	Tipo   TipoDocumento
	DAV    DAV
	DANFE  DANFE
	Livres Campos
}

var quantidadeSolta = regexp.MustCompile(`^\d+([.,]\d+)?$`)

// ExtrairDocumento escolhe DAV ou DANFE pelos rotulos da pagina e preenche
// o schema. Pagina sem rotulo conhecido fica TipoDesconhecido, com so os
// achados livres de ExtrairCampos.
func ExtrairDocumento(linhas []layout.Line) Documento {
	d := Documento{Livres: ExtrairCampos(linhas)}
	switch classificarTipo(linhas) {
	case TipoDANFE:
		d.Tipo = TipoDANFE
		d.DANFE = ExtrairDANFE(linhas)
	case TipoDAV:
		d.Tipo = TipoDAV
		d.DAV = ExtrairDAV(linhas)
	}
	return d
}

func classificarTipo(linhas []layout.Line) TipoDocumento {
	var b strings.Builder
	for _, l := range linhas {
		b.WriteString(l.Text())
		b.WriteByte('\n')
	}
	norm := removerAcentos(strings.ToLower(b.String()))

	if strings.Contains(norm, "danfe") ||
		strings.Contains(norm, "natureza da operacao") ||
		strings.Contains(norm, "chave de acesso") ||
		chaveAcessoPattern.FindString(b.String()) != "" {
		return TipoDANFE
	}
	if strings.Contains(norm, "documento auxiliar") ||
		strings.Contains(norm, "social:") ||
		strings.Contains(norm, "total a pagar") {
		return TipoDAV
	}
	return TipoDesconhecido
}

// CamposDoDocumento projeta Documento nos campos canonicos do contrato
// (cnpj_emitente, cnpj_destinatario, cpf, data, total, itens). Prefere o
// schema tipado; cai nos achados livres se o tipo nao preencheu o campo.
func CamposDoDocumento(d Documento) contrato.Campos {
	c := contrato.Campos{}

	switch d.Tipo {
	case TipoDAV:
		preencherIdentidade(&c, d.DAV.Emitente.CNPJOuCPF, true)
		preencherIdentidade(&c, d.DAV.Destinatario.CNPJOuCPF, false)
		if d.DAV.DataEmissao != nil {
			c.Data = d.DAV.DataEmissao.Format("2006-01-02")
		}
		c.TotalCentavos = primeiroTotal(d.DAV.Totais, EtiquetasValor)
		c.Itens = itensDaTabela(d.DAV.Itens)
	case TipoDANFE:
		preencherIdentidade(&c, d.DANFE.Emitente.CNPJOuCPF, true)
		preencherIdentidade(&c, d.DANFE.Destinatario.CNPJOuCPF, false)
		if d.DANFE.DataEmissao != nil {
			c.Data = d.DANFE.DataEmissao.Format("2006-01-02")
		}
		c.TotalCentavos = primeiroTotal(d.DANFE.Impostos, []string{
			"VALOR TOTAL DA NOTA",
			"VALOR TOTAL DOS PRODUTOS",
		})
		c.Itens = itensDaTabela(d.DANFE.Itens)
	}

	if c.CNPJEmitente == "" && len(d.Livres.CNPJs) > 0 {
		c.CNPJEmitente = d.Livres.CNPJs[0]
	}
	if c.CNPJDestinatario == "" && len(d.Livres.CNPJs) > 1 {
		c.CNPJDestinatario = d.Livres.CNPJs[1]
	}
	if c.CPF == "" && len(d.Livres.CPFs) > 0 {
		c.CPF = d.Livres.CPFs[0]
	}
	if c.Data == "" && len(d.Livres.Datas) > 0 {
		c.Data = d.Livres.Datas[0].Format("2006-01-02")
	}
	if c.TotalCentavos == nil {
		c.TotalCentavos = primeiroTotal(d.Livres.Valores, EtiquetasValor)
	}
	return c
}

func preencherIdentidade(c *contrato.Campos, id string, emitente bool) {
	if id == "" {
		return
	}
	n := len(somenteDigitosLocal(id))
	switch {
	case n == 14:
		if emitente {
			c.CNPJEmitente = id
		} else {
			c.CNPJDestinatario = id
		}
	case n == 11:
		if c.CPF == "" {
			c.CPF = id
		}
	}
}

func primeiroTotal(vals map[string]extract.Money, ordem []string) *int64 {
	for _, k := range ordem {
		if v, ok := vals[k]; ok {
			n := int64(v)
			return &n
		}
	}
	return nil
}

func itensDaTabela(t layout.Table) []contrato.ItemLinha {
	if len(t.Rows) == 0 {
		return nil
	}
	var out []contrato.ItemLinha
	for _, row := range t.Rows {
		item := contrato.ItemLinha{}
		var desc []string
		for _, cell := range row {
			tx := strings.TrimSpace(cell.Text())
			if tx == "" {
				continue
			}
			if n := cellMoney(tx); n != nil {
				item.ValorCentavos = n
				continue
			}
			if item.Quantidade == "" && quantidadeSolta.MatchString(tx) {
				item.Quantidade = tx
				continue
			}
			desc = append(desc, tx)
		}
		item.Descricao = strings.Join(desc, " ")
		if item.Descricao == "" && item.Quantidade == "" && item.ValorCentavos == nil {
			continue
		}
		out = append(out, item)
	}
	return out
}

// cellMoney so trata como dinheiro o que tem R$ ou centavos com 2 digitos
// (9,80). "1,000" e "5" numa celula de tabela sao quantidade, nao total --
// ParseMoney aceitaria os dois (casa o prefixo numerico), e o item
// perderia a quantidade.
func cellMoney(tx string) *int64 {
	low := strings.ToLower(tx)
	temRS := strings.Contains(low, "r$")
	temCentavos := false
	if i := strings.LastIndex(tx, ","); i >= 0 && i+3 == len(tx) {
		temCentavos = unicode.IsDigit(rune(tx[i+1])) && unicode.IsDigit(rune(tx[i+2]))
	}
	if !temRS && !temCentavos {
		return nil
	}
	v, err := extract.ParseMoney(tx)
	if err != nil {
		return nil
	}
	n := int64(v)
	return &n
}

func somenteDigitosLocal(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
