package read

import (
	"testing"

	"github.com/iafrotamacedo-cloud/era-read/layout"
)

func TestExtrairDAV(t *testing.T) {
	linhas := []layout.Line{
		linhaDeTexto("DOCUMENTO AUXILIAR DE VENDA - PEDIDO"),
		linhaDeTexto("Razao Social: RODRIGUES MATERIAL LTDA-ME CNPJ:14788633000110"),
		linhaDeTexto("Nome: FROTA MACEDO ENGENHARIA EIRELI CPF/CNPJ: 27363223000170"),
		linhaDeTexto("N° do Documento:0000018355"),
		linhaDeTexto("Produto/Endereco Quantidade Preco Unitario"),
		linhaDeTexto("00000000001210 - CAP ESG PVC 40MM"),
		linhaDeTexto("1,000 9,80"),
		linhaDeTexto("Total a pagar: 100,60"),
		linhaDeTexto("Dt. Emis:18/08/2026"),
	}

	d := ExtrairDAV(linhas)

	if want := "RODRIGUES MATERIAL LTDA-ME"; d.Emitente.Nome != want {
		t.Errorf("Emitente.Nome = %q, quero %q", d.Emitente.Nome, want)
	}
	if want := "14.788.633/0001-10"; d.Emitente.CNPJOuCPF != want {
		t.Errorf("Emitente.CNPJOuCPF = %q, quero %q", d.Emitente.CNPJOuCPF, want)
	}

	if want := "FROTA MACEDO ENGENHARIA EIRELI"; d.Destinatario.Nome != want {
		t.Errorf("Destinatario.Nome = %q, quero %q", d.Destinatario.Nome, want)
	}
	if want := "27.363.223/0001-70"; d.Destinatario.CNPJOuCPF != want {
		t.Errorf("Destinatario.CNPJOuCPF = %q, quero %q", d.Destinatario.CNPJOuCPF, want)
	}

	if want := "0000018355"; d.NumeroDocumento != want {
		t.Errorf("NumeroDocumento = %q, quero %q", d.NumeroDocumento, want)
	}

	if d.DataEmissao == nil {
		t.Fatal("DataEmissao = nil, queria uma data")
	}
	if y, m, dia := d.DataEmissao.Date(); y != 2026 || m != 8 || dia != 18 {
		t.Errorf("DataEmissao = %v, quero 2026-08-18", d.DataEmissao)
	}

	if got, ok := d.Totais["Total a pagar"]; !ok || got != 10060 {
		t.Errorf("Totais[Total a pagar] = %v (ok=%v), quero 10060", got, ok)
	}

	if got := d.Itens.NumColumns(); got == 0 {
		t.Error("Itens.NumColumns() = 0, queria pelo menos 1 coluna (a tabela de itens deveria ter sido montada)")
	}
	if len(d.Itens.Rows) != 2 {
		t.Fatalf("len(Itens.Rows) = %d, quero 2 (as duas linhas entre o cabecalho e o total)", len(d.Itens.Rows))
	}
}

func TestExtrairDAVSemRotulosConhecidosFicaZerado(t *testing.T) {
	linhas := []layout.Line{linhaDeTexto("um texto qualquer sem nenhum rotulo conhecido")}
	d := ExtrairDAV(linhas)

	if d.Emitente.Nome != "" || d.Emitente.CNPJOuCPF != "" {
		t.Errorf("Emitente = %+v, queria zerado", d.Emitente)
	}
	if d.Destinatario.Nome != "" || d.Destinatario.CNPJOuCPF != "" {
		t.Errorf("Destinatario = %+v, queria zerado", d.Destinatario)
	}
	if d.NumeroDocumento != "" {
		t.Errorf("NumeroDocumento = %q, queria vazio", d.NumeroDocumento)
	}
	if d.DataEmissao != nil {
		t.Errorf("DataEmissao = %v, queria nil", d.DataEmissao)
	}
	if len(d.Totais) != 0 {
		t.Errorf("Totais = %v, queria vazio", d.Totais)
	}
	if len(d.Itens.Rows) != 0 {
		t.Errorf("Itens.Rows = %v, queria vazio (sem cabecalho de tabela, sem itens)", d.Itens.Rows)
	}
}

// TestExtrairDAVCabecalhoIlegivelNaoPegaEnderecoDoRodape cobre um DAV
// real testado em 15/09/2026: o cabeçalho da tabela de itens
// ("Produto/Endereço Embalagem Quantidade...") saiu tão garbled pelo
// reconhecedor que virou um único blob ilegível, sem "endere" nem
// "quantidade" sobreviverem -- a busca pelo início da tabela continuava
// e achava "Endereço:" no RODAPÉ (o endereço de entrega, campo
// diferente), fazendo a tabela de itens sair cheia com texto do rodapé
// (vendedor, telefone, observação) em vez do item de verdade. Corrigido
// parando a busca do cabeçalho assim que "Plano de Pagamento" ou "Dados
// Complementares" aparece -- os dois só vêm DEPOIS da tabela no leiaute.
func TestExtrairDAVCabecalhoIlegivelNaoPegaEnderecoDoRodape(t *testing.T) {
	linhas := []layout.Line{
		linhaDeTexto("Razao Social: RODRIGUES MATERIAL LTDA-ME CNPJ:14788633000110"),
		linhaDeTexto("Nome: FROTA MACEDO ENGENHARIA EIRELI CPF/CNPJ: 27363223000170"),
		linhaDeTexto("N° do Documento: 0000018860"),
		linhaDeTexto("-nnoaaoennLuantoaoPPrecoUniaroUese"), // cabecalho ilegivel
		linhaDeTexto("00000000001795 - ARGAMASSA AC3 15KG TOP10 UN 2,000 31,90 0,00 63,80"),
		linhaDeTexto("Total a pagar: 63,80"),
		linhaDeTexto("Plano de Pagamento"),
		linhaDeTexto("Dados Complementares"),
		linhaDeTexto("Endereco: AVENIDA ENGENHEIRO HEITOR Bairro: CIDADE DOS FUNCIONAR"),
		linhaDeTexto("Vendedor: PEDRO RODRIGUES Dt. Prev: 06/08/2026"),
	}
	d := ExtrairDAV(linhas)
	if len(d.Itens.Rows) != 0 {
		t.Errorf("Itens.Rows = %v, queria vazio (cabecalho ilegivel -- melhor vazio que pegar o rodape)", d.Itens.Rows)
	}
}

func TestCortarAntesDe(t *testing.T) {
	casos := []struct {
		s, marcador, quero string
	}{
		{"RODRIGUES LTDA CNPJ:123", "CNPJ", "RODRIGUES LTDA "},
		{"sem marcador nenhum", "CNPJ", "sem marcador nenhum"},
		{"MARCADOR em maiuscula CNPJ:1", "cnpj", "MARCADOR em maiuscula "},
	}
	for _, c := range casos {
		if got := cortarAntesDe(c.s, c.marcador); got != c.quero {
			t.Errorf("cortarAntesDe(%q, %q) = %q, quero %q", c.s, c.marcador, got, c.quero)
		}
	}
}
