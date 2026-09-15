package read

import (
	"testing"

	"github.com/iafrotamacedo-cloud/era-read/geom"
	"github.com/iafrotamacedo-cloud/era-read/layout"
)

// palavraPosicionada monta uma layout.Word com uma caixa retangular real
// -- ao contrario de linhaDeTexto (so texto, caixa zerada), usada para
// testar valorNaProximaLinha, que precisa de posicao X de verdade.
func palavraPosicionada(texto string, x0, y0, x1, y1 float64) layout.Word {
	return layout.Word{
		Text: texto,
		Box:  geom.Polygon{{X: x0, Y: y0}, {X: x1, Y: y0}, {X: x1, Y: y1}, {X: x0, Y: y1}},
	}
}

func linhaComPalavras(palavras ...layout.Word) layout.Line {
	return layout.Line{Words: palavras}
}

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

// TestExtrairDANFESerieComDoisPontosSolto cobre um formato visto num DANFE
// real (NF 9192, testado em 14/09/2026): a caixa "SÉRIE" do canhoto
// imprimiu "SÉRIE : 1", com um ":" solto entre rótulo e valor que o
// leiaute oficial não tem -- sem o TrimLeft em primeiraPalavraOuFrase, o
// valor extraído ficava ": 1" em vez de "1".
func TestExtrairDANFESerieComDoisPontosSolto(t *testing.T) {
	linhas := []layout.Line{linhaDeTexto("1 / ALGUMACOISA SÉRIE : 1")}
	d := ExtrairDANFE(linhas)
	if want := "1"; d.Serie != want {
		t.Errorf("Serie = %q, quero %q", d.Serie, want)
	}
}

// TestExtrairDANFEValorAlinhadoNaLinhaSeguinte cobre o jeito de imprimir
// mais comum do DANFE, medido num documento real em 15/09/2026: cada
// quadro tem uma linha só de rótulos (as colunas) e os valores
// correspondentes na linha de baixo, alinhados por posição X -- não
// "rótulo: valor" colado como a DAV ou como o texto sintético de
// linhasDANFEDeExemplo acima. Também cobre o efeito colateral que esse
// jeito de imprimir causa de verdade num documento real: se a busca do
// destinatário continuasse depois da primeira "RAZÃO SOCIAL" achada, o
// quadro do transportador (que tem o mesmo rótulo mais abaixo na página)
// sobrescreveria o nome do destinatário.
func TestExtrairDANFEValorAlinhadoNaLinhaSeguinte(t *testing.T) {
	linhas := []layout.Line{
		linhaComPalavras(
			palavraPosicionada("NATUREZA DA OPERACAO", 10, 100, 180, 130),
			palavraPosicionada("PROTOCOLO DE AUTORIZACAO DE USO", 190, 100, 400, 130),
		),
		linhaComPalavras(
			palavraPosicionada("Venda de mercadoria", 10, 133, 180, 160),
			palavraPosicionada("135260000012345 - 14/09/2026 10:30:00", 190, 133, 400, 160),
		),
		linhaComPalavras(
			palavraPosicionada("NOME/RAZAO SOCIAL", 10, 300, 200, 330),
			palavraPosicionada("DATA DA EMISSAO", 210, 300, 350, 330),
		),
		linhaComPalavras(
			palavraPosicionada("FROTA MACEDO ENGENHARIA EIRELI", 10, 333, 200, 360),
			palavraPosicionada("18/08/2026", 210, 333, 350, 360),
		),
		linhaComPalavras(
			palavraPosicionada("VALOR TOTAL DOS PRODUTOS", 10, 500, 220, 530),
			palavraPosicionada("VALOR TOTAL DA NOTA", 230, 500, 400, 530),
		),
		linhaComPalavras(
			palavraPosicionada("25,00", 10, 533, 220, 560),
			palavraPosicionada("30,00", 230, 533, 400, 560),
		),
		// Distrator: o quadro TRANSPORTADOR/VOLUMES TRANSPORTADOS, bem mais
		// abaixo na página, tem o proprio rotulo "RAZAO SOCIAL" -- longe
		// demais do quadro do destinatario (linhas 2/3 acima) para
		// valorNaProximaLinha confundir os dois, mas so o guard
		// destinatarioPreenchido evita reprocessar esta ocorrencia.
		linhaComPalavras(
			palavraPosicionada("RAZAO SOCIAL", 10, 700, 150, 730),
		),
		linhaComPalavras(
			palavraPosicionada("TRANSPORTADORA XYZ", 10, 733, 150, 760),
		),
	}

	d := ExtrairDANFE(linhas)

	if want := "Venda de mercadoria"; d.NaturezaOperacao != want {
		t.Errorf("NaturezaOperacao = %q, quero %q", d.NaturezaOperacao, want)
	}
	if want := "135260000012345 - 14/09/2026 10:30:00"; d.Protocolo != want {
		t.Errorf("Protocolo = %q, quero %q", d.Protocolo, want)
	}
	if want := "FROTA MACEDO ENGENHARIA EIRELI"; d.Destinatario.Nome != want {
		t.Errorf("Destinatario.Nome = %q, quero %q (nao deveria pegar o da transportadora)", d.Destinatario.Nome, want)
	}
	if d.DataEmissao == nil {
		t.Fatal("DataEmissao = nil, queria uma data")
	}
	if y, m, dia := d.DataEmissao.Date(); y != 2026 || m != 8 || dia != 18 {
		t.Errorf("DataEmissao = %v, quero 2026-08-18", d.DataEmissao)
	}

	quero := map[string]int64{
		"VALOR TOTAL DOS PRODUTOS": 2500,
		"VALOR TOTAL DA NOTA":      3000,
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
}

// TestExtrairDANFEItensQuandoImpostoVemAntes cobre uma ordem de quadros
// vista num DANFE real em 15/09/2026: ao contrário do texto sintético
// acima (impostos DEPOIS da tabela de itens, como na DAV), aquele
// documento imprimiu "CÁLCULO DO IMPOSTO" ANTES de "DADOS DO
// PRODUTO/SERVIÇO" -- usar só "achou um imposto" como fim da tabela (o
// esquema original, copiado da DAV) nunca disparava depois do começo da
// tabela, e ela varria até o fim da página. "DADOS ADICIONAIS" -- que no
// leiaute oficial sempre vem DEPOIS da tabela de itens -- é o marcador de
// fim que resolve esse caso.
func TestExtrairDANFEItensQuandoImpostoVemAntes(t *testing.T) {
	linhas := []layout.Line{
		linhaDeTexto("VALOR TOTAL DA NOTA 50,00"),
		linhaDeTexto("DADOS DO PRODUTO/SERVICO"),
		linhaDeTexto("00001 PRODUTO A UN 2 10,00 20,00"),
		linhaDeTexto("00002 PRODUTO B UN 1 5,00 5,00"),
		linhaDeTexto("DADOS ADICIONAIS"),
		linhaDeTexto("OPTANTE PELO SIMPLES NACIONAL"),
	}
	d := ExtrairDANFE(linhas)

	if got, ok := d.Impostos["VALOR TOTAL DA NOTA"]; !ok || int64(got) != 5000 {
		t.Errorf("Impostos[VALOR TOTAL DA NOTA] = %v (ok=%v), quero 5000 centavos", got, ok)
	}
	if len(d.Itens.Rows) != 2 {
		t.Fatalf("len(Itens.Rows) = %d, quero 2 (nao deveria varrer ate DADOS ADICIONAIS)", len(d.Itens.Rows))
	}
}

// TestExtrairDANFEDestinatarioCNPJForaDaColunaDoNome cobre um caso real
// (nota da Madeireira Rio Branco, testado em 15/09/2026): o CNPJ do
// destinatário saiu certo pelo reconhecedor, na MESMA linha de valor do
// nome, mas numa coluna diferente (fora da faixa X do rótulo "RAZÃO
// SOCIAL", que valorNaProximaLinha usa só para achar o NOME). Sem
// procurar CNPJ no texto inteiro da linha de valor -- não só na faixa
// alinhada com o rótulo -- esse CNPJ ficava de fora.
func TestExtrairDANFEDestinatarioCNPJForaDaColunaDoNome(t *testing.T) {
	linhas := []layout.Line{
		linhaComPalavras(palavraPosicionada("NOME/RAZAO SOCIAL", 10, 100, 200, 130)),
		linhaComPalavras(
			palavraPosicionada("FROTA MACEDO ENGENHARIA EIRELI", 10, 133, 200, 160),
			palavraPosicionada("27.363.223/0001-70", 210, 133, 320, 160),
		),
	}
	d := ExtrairDANFE(linhas)
	if want := "FROTA MACEDO ENGENHARIA EIRELI"; d.Destinatario.Nome != want {
		t.Errorf("Destinatario.Nome = %q, quero %q", d.Destinatario.Nome, want)
	}
	if want := "27.363.223/0001-70"; d.Destinatario.CNPJOuCPF != want {
		t.Errorf("Destinatario.CNPJOuCPF = %q, quero %q", d.Destinatario.CNPJOuCPF, want)
	}
}

// TestExtrairDANFEDestinatarioCNPJNaLinhaDoRotulo cobre outro caso real
// (nota da TIM/JJM, testado em 15/09/2026): o CNPJ do destinatário saiu
// colado na PRÓPRIA linha do rótulo "RAZÃO SOCIAL" (junto com a data de
// emissão), enquanto o nome ficou na linha seguinte -- o inverso da
// suposição original de que rótulo e CNPJ vêm sempre juntos na mesma
// coluna do nome.
func TestExtrairDANFEDestinatarioCNPJNaLinhaDoRotulo(t *testing.T) {
	linhas := []layout.Line{
		linhaComPalavras(
			palavraPosicionada("NOME/RAZAO SOCIAL", 10, 100, 200, 130),
			palavraPosicionada("27.363.223/0001-70", 210, 100, 320, 130),
			palavraPosicionada("07/07/2026", 330, 100, 420, 130),
		),
		linhaComPalavras(
			palavraPosicionada("FROTA MACEDO ENGENHARIA LTDA", 10, 133, 200, 160),
		),
	}
	d := ExtrairDANFE(linhas)
	if want := "FROTA MACEDO ENGENHARIA LTDA"; d.Destinatario.Nome != want {
		t.Errorf("Destinatario.Nome = %q, quero %q", d.Destinatario.Nome, want)
	}
	if want := "27.363.223/0001-70"; d.Destinatario.CNPJOuCPF != want {
		t.Errorf("Destinatario.CNPJOuCPF = %q, quero %q", d.Destinatario.CNPJOuCPF, want)
	}
}

// TestExtrairDANFEDataEmissaoSemDA cobre uma variante de redação real (a
// nota da TIM/JJM, testado em 15/09/2026, imprimiu "DATA EMISSÃO", sem o
// "DA" do leiaute oficial e da nota da Madeireira Rio Branco).
func TestExtrairDANFEDataEmissaoSemDA(t *testing.T) {
	linhas := []layout.Line{linhaDeTexto("NOME/RAZAO SOCIAL CNPJ/CPF DATA EMISSAO 07/07/2026")}
	d := ExtrairDANFE(linhas)
	if d.DataEmissao == nil {
		t.Fatal("DataEmissao = nil, queria uma data")
	}
	if y, m, dia := d.DataEmissao.Date(); y != 2026 || m != 7 || dia != 7 {
		t.Errorf("DataEmissao = %v, quero 2026-07-07", d.DataEmissao)
	}
}

// TestExtrairDANFEEtiquetaCurtaNaoRoubaValorDaMaisLonga cobre um bug real
// achado na nota da Madeireira Rio Branco (testado em 15/09/2026):
// "VALOR DO ICMS" é literalmente um prefixo de "VALOR DO ICMS
// SUBSTITUIÇÃO", e o reconhecedor colou as duas etiquetas (mais texto de
// uma terceira coluna) num único blob de OCR. Sem o guard de
// etiquetaTemIrmaMaisLonga, "VALOR DO ICMS" batia nesse blob, usava a
// faixa X larga demais dele, e pegava o PRIMEIRO valor da linha de baixo
// (0,00) em vez do valor correto (71,71) -- um valor ERRADO, não só
// ausente, o que é pior que os outros gaps documentados.
func TestExtrairDANFEEtiquetaCurtaNaoRoubaValorDaMaisLonga(t *testing.T) {
	linhas := []layout.Line{
		linhaComPalavras(
			palavraPosicionada("VALOR DO ICMS SUBSTITUICAO EXTRA", 10, 100, 400, 130),
		),
		linhaComPalavras(
			palavraPosicionada("0,00", 10, 133, 150, 160),
			palavraPosicionada("71,71", 200, 133, 400, 160),
		),
	}
	d := ExtrairDANFE(linhas)

	if got, ok := d.Impostos["VALOR DO ICMS"]; ok {
		t.Errorf(`Impostos["VALOR DO ICMS"] = %v, queria ausente (o blob e da etiqueta SUBSTITUICAO, nao desta)`, got)
	}
	if got, ok := d.Impostos["VALOR DO ICMS SUBSTITUIÇÃO"]; !ok || int64(got) != 0 {
		t.Errorf(`Impostos["VALOR DO ICMS SUBSTITUIÇÃO"] = %v (ok=%v), quero 0 centavos (a etiqueta mais longa reivindica o blob)`, got, ok)
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
