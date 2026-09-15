package read

import (
	"testing"

	"github.com/iafrotamacedo-cloud/era-read/layout"
)

// linhaUmaPalavraY monta uma layout.Line de uma palavra só, com posição Y
// real (X fixo, sempre a mesma faixa) -- o bastante para testar
// valorNaProximaLinha quando rótulo e valor são cada um uma linha inteira
// de uma palavra só (o caso comum no DANFSe real conferido em 15/09/2026).
func linhaUmaPalavraY(texto string, y0, y1 float64) layout.Line {
	return linhaComPalavras(palavraPosicionada(texto, 10, y0, 300, y1))
}

// linhasNFSeDeExemplo imita a saída que read.Page devolveria para um
// DANFSe real -- na ordem e com os rótulos confirmados na fonte primária
// (Nota Técnica nº 008 -- Especificações Técnicas do DANFSe, SE/CGNFS-e,
// versão 1.02) e no documento real usado para validar isto (nota de
// Antonio Carlos de Lima, Fortaleza/CE).
func linhasNFSeDeExemplo() []layout.Line {
	return []layout.Line{
		linhaDeTexto("DANFSe v2.0 Documento Auxiliar da NFS-e"),
		linhaDeTexto("Município: FORTALEZA/CE"),
		linhaUmaPalavraY("NÚMERO DA NFS-E", 100, 130),
		linhaUmaPalavraY("1392", 133, 160),
		linhaUmaPalavraY("COMPETÊNCIA DA NFS-E", 200, 230),
		linhaUmaPalavraY("01/07/2026", 233, 260),
		linhaUmaPalavraY("DATA E HORA DA EMISSÃO DA NFS-E", 300, 330),
		linhaUmaPalavraY("15/07/2026 09:34:02", 333, 360),
		linhaDeTexto("23044001215837609000197000000000139226070156271306"),
		linhaUmaPalavraY("CNPJ/CPF/NIF", 400, 430),
		linhaUmaPalavraY("15.837.609/0001-97", 433, 460),
		linhaUmaPalavraY("Nome/Nome Empresarial", 500, 530),
		linhaUmaPalavraY("15.837.609 ANTONIO CARLOS DE LIMA", 533, 560),
		linhaDeTexto("TOMADOR/ADQUIRENTE"),
		linhaUmaPalavraY("CNPJ/CPF/NIF", 600, 630),
		linhaUmaPalavraY("27.363.223/0001-70", 633, 660),
		linhaUmaPalavraY("Nome/Nome Empresarial", 700, 730),
		linhaUmaPalavraY("FROTA MACEDO ENGENHARIA LTDA", 733, 760),
		linhaDeTexto("Descrição do Serviço SERVIÇO DE PINTURA DAS PAREDES"),
		linhaUmaPalavraY("BC ISSQN", 800, 830),
		linhaUmaPalavraY("200,00", 833, 860),
		linhaUmaPalavraY("ISSQN Apurado", 900, 930),
		linhaUmaPalavraY("6,00", 933, 960),
		linhaUmaPalavraY("Valor da Operação/Serviço", 1000, 1030),
		linhaUmaPalavraY("200,00", 1033, 1060),
		linhaUmaPalavraY("Valor Líquido da NFS-e", 1100, 1130),
		linhaUmaPalavraY("194,00", 1133, 1160),
	}
}

func TestExtrairNFSe(t *testing.T) {
	n := ExtrairNFSe(linhasNFSeDeExemplo())

	if want := "FORTALEZA/CE"; n.Municipio != want {
		t.Errorf("Municipio = %q, quero %q", n.Municipio, want)
	}
	if want := "1392"; n.NumeroNFSe != want {
		t.Errorf("NumeroNFSe = %q, quero %q", n.NumeroNFSe, want)
	}
	if want := "23044001215837609000197000000000139226070156271306"; n.ChaveAcesso != want {
		t.Errorf("ChaveAcesso = %q, quero %q (%d dígitos)", n.ChaveAcesso, want, len(n.ChaveAcesso))
	}
	if len(n.ChaveAcesso) != 50 {
		t.Errorf("len(ChaveAcesso) = %d, quero 50", len(n.ChaveAcesso))
	}

	if n.Competencia == nil {
		t.Fatal("Competencia = nil, queria uma data")
	}
	if y, m, d := n.Competencia.Date(); y != 2026 || m != 7 || d != 1 {
		t.Errorf("Competencia = %v, quero 2026-07-01", n.Competencia)
	}
	if n.DataEmissao == nil {
		t.Fatal("DataEmissao = nil, queria uma data")
	}
	if y, m, d := n.DataEmissao.Date(); y != 2026 || m != 7 || d != 15 {
		t.Errorf("DataEmissao = %v, quero 2026-07-15", n.DataEmissao)
	}

	if want := "15.837.609/0001-97"; n.Prestador.CNPJOuCPF != want {
		t.Errorf("Prestador.CNPJOuCPF = %q, quero %q", n.Prestador.CNPJOuCPF, want)
	}
	if want := "15.837.609 ANTONIO CARLOS DE LIMA"; n.Prestador.Nome != want {
		t.Errorf("Prestador.Nome = %q, quero %q", n.Prestador.Nome, want)
	}
	if want := "27.363.223/0001-70"; n.Tomador.CNPJOuCPF != want {
		t.Errorf("Tomador.CNPJOuCPF = %q, quero %q", n.Tomador.CNPJOuCPF, want)
	}
	if want := "FROTA MACEDO ENGENHARIA LTDA"; n.Tomador.Nome != want {
		t.Errorf("Tomador.Nome = %q, quero %q", n.Tomador.Nome, want)
	}

	if want := "SERVIÇO DE PINTURA DAS PAREDES"; n.DescricaoServico != want {
		t.Errorf("DescricaoServico = %q, quero %q", n.DescricaoServico, want)
	}

	if want := int64(20000); int64(n.ValorServico) != want {
		t.Errorf("ValorServico = %v, quero %v centavos", n.ValorServico, want)
	}
	if want := int64(20000); int64(n.BaseCalculoISSQN) != want {
		t.Errorf("BaseCalculoISSQN = %v, quero %v centavos", n.BaseCalculoISSQN, want)
	}
	if want := int64(600); int64(n.ISSQNApurado) != want {
		t.Errorf("ISSQNApurado = %v, quero %v centavos", n.ISSQNApurado, want)
	}
	if want := int64(19400); int64(n.ValorLiquido) != want {
		t.Errorf("ValorLiquido = %v, quero %v centavos", n.ValorLiquido, want)
	}
}

// TestExtrairNFSeSemRotulosConhecidosFicaZerado -- mesma regra das
// outras duas: nenhum rótulo achado, nenhum campo preenchido.
func TestExtrairNFSeSemRotulosConhecidosFicaZerado(t *testing.T) {
	n := ExtrairNFSe([]layout.Line{linhaDeTexto("um texto qualquer sem nenhum rotulo conhecido")})

	if n.Prestador.CNPJOuCPF != "" || n.Tomador.CNPJOuCPF != "" {
		t.Errorf("Prestador/Tomador = %+v / %+v, queria zerados", n.Prestador, n.Tomador)
	}
	if n.NumeroNFSe != "" || n.ChaveAcesso != "" || n.Municipio != "" {
		t.Errorf("NumeroNFSe=%q ChaveAcesso=%q Municipio=%q, queria todos vazios",
			n.NumeroNFSe, n.ChaveAcesso, n.Municipio)
	}
	if n.Competencia != nil || n.DataEmissao != nil {
		t.Error("datas deveriam ser nil")
	}
	if n.ValorServico != 0 || n.BaseCalculoISSQN != 0 || n.ISSQNApurado != 0 || n.ValorLiquido != 0 {
		t.Error("valores deveriam ser zero")
	}
}

// TestExtrairNFSeNaoConfundeCNPJDoTomadorComPrestador cobre a razão de
// parar de tratar linhas como "do prestador" assim que "TOMADOR/
// ADQUIRENTE" aparece: sem isso, se o Prestador ainda não tivesse achado
// nome/CNPJ (uma região perdida pelo reconhecedor, por exemplo), os dados
// do Tomador acabariam virando "do Prestador" por engano -- mesmo bug que
// TestExtrairDANFENaoConfundeCNPJDoDestinatarioComEmitente cobre no
// DANFE.
func TestExtrairNFSeNaoConfundeCNPJDoTomadorComPrestador(t *testing.T) {
	linhas := []layout.Line{
		linhaDeTexto("TOMADOR/ADQUIRENTE"),
		linhaUmaPalavraY("CNPJ/CPF/NIF", 100, 130),
		linhaUmaPalavraY("27.363.223/0001-70", 133, 160),
	}
	n := ExtrairNFSe(linhas)
	if n.Prestador.CNPJOuCPF != "" {
		t.Errorf("Prestador.CNPJOuCPF = %q, quero vazio (o único CNPJ da entrada é do tomador)", n.Prestador.CNPJOuCPF)
	}
	if want := "27.363.223/0001-70"; n.Tomador.CNPJOuCPF != want {
		t.Errorf("Tomador.CNPJOuCPF = %q, quero %q", n.Tomador.CNPJOuCPF, want)
	}
}
