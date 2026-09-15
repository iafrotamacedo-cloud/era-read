package read

import (
	"testing"

	"github.com/iafrotamacedo-cloud/era-read/layout"
)

func TestIdentificarTipoDocumento(t *testing.T) {
	casos := []struct {
		nome  string
		linha string
		quero TipoDocumento
	}{
		{"DAV", "DOCUMENTO AUXILIAR DE VENDA - PEDIDO", TipoDAV},
		{"DANFE", "DANFE Documento Auxiliar da NOTA FISCAL ELETRONICA", TipoDANFE},
		{"NFSe", "DANFSe v2.0 Documento Auxiliar da NFS-e", TipoNFSe},
		{"nenhum rótulo conhecido", "um texto qualquer sem nenhum marcador", TipoDesconhecido},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got := IdentificarTipoDocumento([]layout.Line{linhaDeTexto(c.linha)})
			if got != c.quero {
				t.Errorf("IdentificarTipoDocumento(%q) = %v, quero %v", c.linha, got, c.quero)
			}
		})
	}
}

// TestIdentificarTipoDocumentoDAVComEspacoPerdido cobre um DAV real
// testado em 15/09/2026: o reconhecedor leu o título como
// "DOCUMENTOAUXILIAR DEVENDA-PEDIDO" -- sem o espaço entre "DOCUMENTO" e
// "AUXILIAR" -- e sem a comparação ignorar espaço, IdentificarTipoDocumento
// devolvia TipoDesconhecido para um DAV de verdade.
func TestIdentificarTipoDocumentoDAVComEspacoPerdido(t *testing.T) {
	got := IdentificarTipoDocumento([]layout.Line{linhaDeTexto("DOCUMENTOAUXILIAR DEVENDA-PEDIDO")})
	if got != TipoDAV {
		t.Errorf("IdentificarTipoDocumento(com espaço perdido) = %v, quero TipoDAV", got)
	}
}

// TestIdentificarTipoDocumentoDANFSeNaoViraDANFE cobre o motivo do
// comentário de IdentificarTipoDocumento sobre a ordem de checagem:
// mesmo que a checagem de DANFE viesse primeiro, "DANFSe" não bateria
// nela, porque "danfe" não é substring de "danfse" -- mas o teste trava
// o comportamento observável (o resultado certo), não a ordem interna.
func TestIdentificarTipoDocumentoDANFSeNaoViraDANFE(t *testing.T) {
	got := IdentificarTipoDocumento([]layout.Line{linhaDeTexto("DANFSe v2.0")})
	if got != TipoNFSe {
		t.Errorf("IdentificarTipoDocumento(DANFSe) = %v, quero TipoNFSe", got)
	}
}

func TestExtrairDocumentoDAV(t *testing.T) {
	linhas := []layout.Line{
		linhaDeTexto("DOCUMENTO AUXILIAR DE VENDA - PEDIDO"),
		linhaDeTexto("Razao Social: RODRIGUES MATERIAL LTDA-ME CNPJ:14788633000110"),
	}
	doc := ExtrairDocumento(linhas)
	if doc.Tipo != TipoDAV {
		t.Fatalf("Tipo = %v, quero TipoDAV", doc.Tipo)
	}
	if doc.DAV == nil {
		t.Fatal("DAV = nil, queria preenchido")
	}
	if doc.DANFE != nil || doc.NFSe != nil {
		t.Errorf("DANFE/NFSe deveriam ficar nil quando Tipo é DAV, achei %+v / %+v", doc.DANFE, doc.NFSe)
	}
	if want := "14.788.633/0001-10"; doc.DAV.Emitente.CNPJOuCPF != want {
		t.Errorf("DAV.Emitente.CNPJOuCPF = %q, quero %q", doc.DAV.Emitente.CNPJOuCPF, want)
	}
}

func TestExtrairDocumentoDANFE(t *testing.T) {
	doc := ExtrairDocumento(linhasDANFEDeExemplo())
	if doc.Tipo != TipoDANFE {
		t.Fatalf("Tipo = %v, quero TipoDANFE", doc.Tipo)
	}
	if doc.DANFE == nil {
		t.Fatal("DANFE = nil, queria preenchido")
	}
	if doc.DAV != nil || doc.NFSe != nil {
		t.Errorf("DAV/NFSe deveriam ficar nil quando Tipo é DANFE, achei %+v / %+v", doc.DAV, doc.NFSe)
	}
	if want := "14.788.633/0001-10"; doc.DANFE.Emitente.CNPJOuCPF != want {
		t.Errorf("DANFE.Emitente.CNPJOuCPF = %q, quero %q", doc.DANFE.Emitente.CNPJOuCPF, want)
	}
}

func TestExtrairDocumentoNFSe(t *testing.T) {
	doc := ExtrairDocumento(linhasNFSeDeExemplo())
	if doc.Tipo != TipoNFSe {
		t.Fatalf("Tipo = %v, quero TipoNFSe", doc.Tipo)
	}
	if doc.NFSe == nil {
		t.Fatal("NFSe = nil, queria preenchido")
	}
	if doc.DAV != nil || doc.DANFE != nil {
		t.Errorf("DAV/DANFE deveriam ficar nil quando Tipo é NFSe, achei %+v / %+v", doc.DAV, doc.DANFE)
	}
	if want := "FROTA MACEDO ENGENHARIA LTDA"; doc.NFSe.Tomador.Nome != want {
		t.Errorf("NFSe.Tomador.Nome = %q, quero %q", doc.NFSe.Tomador.Nome, want)
	}
}

// TestExtrairDocumentoDesconhecidoNaoAdivinha cobre a regra central do
// despachante: sem marcador nenhum, nenhum dos três ExtrairX é chamado.
func TestExtrairDocumentoDesconhecidoNaoAdivinha(t *testing.T) {
	linhas := []layout.Line{linhaDeTexto("um texto qualquer sem nenhum marcador conhecido")}
	doc := ExtrairDocumento(linhas)
	if doc.Tipo != TipoDesconhecido {
		t.Fatalf("Tipo = %v, quero TipoDesconhecido", doc.Tipo)
	}
	if doc.DAV != nil || doc.DANFE != nil || doc.NFSe != nil {
		t.Errorf("todos deveriam ficar nil, achei DAV=%+v DANFE=%+v NFSe=%+v", doc.DAV, doc.DANFE, doc.NFSe)
	}
}
