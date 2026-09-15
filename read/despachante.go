package read

import (
	"strings"

	"github.com/iafrotamacedo-cloud/era-read/layout"
)

// TipoDocumento identifica qual dos schemas conhecidos (DAV, DANFE,
// NFSe) um documento é.
type TipoDocumento int

const (
	TipoDesconhecido TipoDocumento = iota
	TipoDAV
	TipoDANFE
	TipoNFSe
)

// String devolve o nome do tipo, para log e mensagem de erro -- nunca
// usado para comparação (compare o valor de TipoDocumento em si).
func (t TipoDocumento) String() string {
	switch t {
	case TipoDAV:
		return "DAV"
	case TipoDANFE:
		return "DANFE"
	case TipoNFSe:
		return "NFSe"
	default:
		return "desconhecido"
	}
}

// IdentificarTipoDocumento decide qual dos três tipos conhecidos as
// linhas representam, procurando um marcador impresso exclusivo de cada
// um -- "DANFSe" (título do documento auxiliar da NFS-e), "DANFE" (título
// do documento auxiliar da NF-e) e "DOCUMENTO AUXILIAR DE VENDA" (título
// da DAV). Nunca um palpite: se nenhum marcador aparecer em nenhuma
// linha, devolve TipoDesconhecido.
//
// A comparação ignora espaço, não só maiúscula/minúscula: um DAV real
// testado em 15/09/2026 saiu do reconhecedor como "DOCUMENTOAUXILIAR
// DEVENDA-PEDIDO" -- sem o espaço entre "DOCUMENTO" e "AUXILIAR", e com o
// espaço solto entre "DE" e "VENDA" em vez de entre "AUXILIAR" e "DE". Um
// marcador de uma palavra só ("DANFE", "DANFSe") não sofre desse
// problema, mas o de três palavras da DAV sofre, e a comparação abaixo
// cobre os dois casos igual.
//
// A ordem de checagem (NFSe antes de DANFE) importa só porque "DANFSe"
// contém as letras D-A-N-F, mas NÃO contém a subsequência "danfe" como
// substring própria ("danfse" ≠ "...danfe...") -- os dois marcadores não
// colidem de fato, a ordem aqui é só precaução.
func IdentificarTipoDocumento(linhas []layout.Line) TipoDocumento {
	for _, l := range linhas {
		texto := semEspacos(strings.ToLower(l.Text()))
		switch {
		case strings.Contains(texto, "danfse"):
			return TipoNFSe
		case strings.Contains(texto, "danfe"):
			return TipoDANFE
		case strings.Contains(texto, semEspacos("documento auxiliar de venda")):
			return TipoDAV
		}
	}
	return TipoDesconhecido
}

// semEspacos remove todo espaço de s -- usado só para achar o marcador
// de tipo de documento (ver comentário de IdentificarTipoDocumento), não
// para cortar rótulo de valor (onde a posição do espaço importa).
func semEspacos(s string) string {
	return strings.ReplaceAll(s, " ", "")
}

// Documento é o resultado de ExtrairDocumento -- exatamente um dos três
// ponteiros vem preenchido, conforme Tipo (os outros dois ficam nil).
// Quando Tipo é TipoDesconhecido, os três ficam nil -- ExtrairDocumento
// nunca adivinha qual ExtrairX chamar.
type Documento struct {
	Tipo  TipoDocumento
	DAV   *DAV
	DANFE *DANFE
	NFSe  *NFSe
}

// ExtrairDocumento identifica o tipo do documento (via
// IdentificarTipoDocumento) e chama o ExtrairX correspondente -- o jeito
// de usar este motor sem precisar saber de antemão qual dos três schemas
// um documento é.
func ExtrairDocumento(linhas []layout.Line) Documento {
	tipo := IdentificarTipoDocumento(linhas)
	switch tipo {
	case TipoDAV:
		d := ExtrairDAV(linhas)
		return Documento{Tipo: tipo, DAV: &d}
	case TipoDANFE:
		d := ExtrairDANFE(linhas)
		return Documento{Tipo: tipo, DANFE: &d}
	case TipoNFSe:
		n := ExtrairNFSe(linhas)
		return Documento{Tipo: tipo, NFSe: &n}
	default:
		return Documento{Tipo: TipoDesconhecido}
	}
}
