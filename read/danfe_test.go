package read

import (
	"testing"

	"github.com/iafrotamacedo-cloud/era-read/layout"
)

// As linhas abaixo imitam o texto que read.Page devolveria para um DANFE
// real -- na ordem e com os rotulos do leiaute oficial (Anexo II do
// Manual de Orientacao do Contribuinte da NF-e, conferido em 14/09/2026),
// nao a saida de um documento de verdade passado pelo pipeline (nenhum
// estava disponivel nesta sessao -- ver o comentario de DANFE).
func linhasDANFEDeExemplo() []layout.Line {
	return []layout.Line{
		linhaDeTexto("DANFE Documento Auxiliar da NOTA FISCAL ELETRONICA"),
		linhaDeTexto("INSCRICAO ESTADUAL 123456789 CNPJ 14788633000110 CHAVE DE ACESSO DA NF-e 3508 0599 9990 9091 0270 5500 1000 0000 0151 8005 1273"),
		linhaDeTexto("NATUREZA DA OPERACAO Venda de mercadoria  N.º 000123456"),
		linhaDeTexto("SÉRIE 001"),
		linhaDeTexto("DESTINATARIO/REMETENTE"),
		linhaDeTexto("NOME/RAZÃO SOCIAL FROTA MACEDO ENGENHARIA EIRELI CNPJ/CPF 27363223000170"),
		linhaDeTexto("DATA DA EMISSÃO 18/08/2026"),
		linhaDeTexto("DATA DA SAÍDA/ENTRADA 19/08/2026"),
		linhaDeTexto("PROTOCOLO DE AUTORIZAÇÃO DE USO 135260000012345 14/09/2026 10:30:00"),
		linhaDeTexto("DADOS DO PRODUTO"),
		linhaDeTexto("00001 PRODUTO A UN 2 10,00 20,00"),
		linhaDeTexto("00002 PRODUTO B UN 1 5,00 5,00"),
		linhaDeTexto("VALOR TOTAL DOS PRODUTOS 25,00"),
		linhaDeTexto("VALOR TOTAL DA NOTA 25,00"),
	}
}

func TestExtrairDANFE(t *testing.T) {
	d := ExtrairDANFE(linhasDANFEDeExemplo())

	if want := "14.788.633/0001-10"; d.Emitente.CNPJOuCPF != want {
		t.Errorf("Emitente.CNPJOuCPF = %q, quero %q", d.Emitente.CNPJOuCPF, want)
	}
	if d.Emitente.Nome != "" {
		t.Errorf("Emitente.Nome = %q, quero vazio (o leiaute nao rotula o nome do emitente)", d.Emitente.Nome)
	}

	if want := "FROTA MACEDO ENGENHARIA EIRELI"; d.Destinatario.Nome != want {
		t.Errorf("Destinatario.Nome = %q, quero %q", d.Destinatario.Nome, want)
	}
	if want := "27.363.223/0001-70"; d.Destinatario.CNPJOuCPF != want {
		t.Errorf("Destinatario.CNPJOuCPF = %q, quero %q", d.Destinatario.CNPJOuCPF, want)
	}

	if want := "Venda de mercadoria"; d.NaturezaOperacao != want {
		t.Errorf("NaturezaOperacao = %q, quero %q", d.NaturezaOperacao, want)
	}
	if want := "000123456"; d.NumeroNF != want {
		t.Errorf("NumeroNF = %q, quero %q", d.NumeroNF, want)
	}
	if want := "001"; d.Serie != want {
		t.Errorf("Serie = %q, quero %q", d.Serie, want)
	}

	if want := "35080599999090910270550010000000015180051273"; d.ChaveAcesso != want {
		t.Errorf("ChaveAcesso = %q, quero %q", d.ChaveAcesso, want)
	}
	if want := "135260000012345 14/09/2026 10:30:00"; d.Protocolo != want {
		t.Errorf("Protocolo = %q, quero %q", d.Protocolo, want)
	}

	if d.DataEmissao == nil {
		t.Fatal("DataEmissao = nil, queria uma data")
	}
	if y, m, dia := d.DataEmissao.Date(); y != 2026 || m != 8 || dia != 18 {
		t.Errorf("DataEmissao = %v, quero 2026-08-18", d.DataEmissao)
	}
	if d.DataSaidaEntrada == nil {
		t.Fatal("DataSaidaEntrada = nil, queria uma data")
	}
	if y, m, dia := d.DataSaidaEntrada.Date(); y != 2026 || m != 8 || dia != 19 {
		t.Errorf("DataSaidaEntrada = %v, quero 2026-08-19", d.DataSaidaEntrada)
	}

	quero := map[string]int64{
		"VALOR TOTAL DOS PRODUTOS": 2500,
		"VALOR TOTAL DA NOTA":      2500,
	}
	for etiqueta, v := range quero {
		got, ok := d.Impostos[etiqueta]
		if !ok {
			t.Errorf("faltou o imposto %q", etiqueta)
			continue
		}
		if int64(got) != v {
			t.Errorf("%q = %v, quero %v centavos", etiqueta, got, v)
		}
	}

	if len(d.Itens.Rows) != 2 {
		t.Fatalf("len(Itens.Rows) = %d, quero 2 (as duas linhas entre o cabecalho e o primeiro imposto)", len(d.Itens.Rows))
	}
}

func TestExtrairDANFESemRotulosConhecidosFicaZerado(t *testing.T) {
	linhas := []layout.Line{linhaDeTexto("um texto qualquer sem nenhum rotulo conhecido")}
	d := ExtrairDANFE(linhas)

	if d.Emitente.CNPJOuCPF != "" || d.Destinatario.CNPJOuCPF != "" {
		t.Errorf("Emitente/Destinatario = %+v / %+v, queria zerados", d.Emitente, d.Destinatario)
	}
	if d.NumeroNF != "" || d.Serie != "" || d.ChaveAcesso != "" || d.Protocolo != "" {
		t.Errorf("NumeroNF=%q Serie=%q ChaveAcesso=%q Protocolo=%q, queria todos vazios",
			d.NumeroNF, d.Serie, d.ChaveAcesso, d.Protocolo)
	}
	if d.DataEmissao != nil || d.DataSaidaEntrada != nil {
		t.Error("datas deveriam ser nil")
	}
	if len(d.Impostos) != 0 {
		t.Errorf("Impostos = %v, queria vazio", d.Impostos)
	}
	if len(d.Itens.Rows) != 0 {
		t.Errorf("Itens.Rows = %v, queria vazio (sem cabecalho de tabela, sem itens)", d.Itens.Rows)
	}
}

// TestExtrairDANFENaoConfundeCNPJDoDestinatarioComEmitente cobre a razao
// de parar de procurar CNPJ do emitente assim que a secao do
// destinatario comeca: sem isso, se o CNPJ do emitente nao aparecer antes
// (uma regiao que o detector perdeu, por exemplo), o CNPJ do destinatario
// acabaria virando "do emitente" por engano.
func TestExtrairDANFENaoConfundeCNPJDoDestinatarioComEmitente(t *testing.T) {
	linhas := []layout.Line{
		linhaDeTexto("DESTINATARIO/REMETENTE"),
		linhaDeTexto("NOME/RAZÃO SOCIAL FROTA MACEDO CNPJ/CPF 27363223000170"),
	}
	d := ExtrairDANFE(linhas)
	if d.Emitente.CNPJOuCPF != "" {
		t.Errorf("Emitente.CNPJOuCPF = %q, quero vazio (o unico CNPJ da entrada e do destinatario)", d.Emitente.CNPJOuCPF)
	}
	if want := "27.363.223/0001-70"; d.Destinatario.CNPJOuCPF != want {
		t.Errorf("Destinatario.CNPJOuCPF = %q, quero %q", d.Destinatario.CNPJOuCPF, want)
	}
}
