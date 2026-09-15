package read

import (
	"testing"

	"github.com/iafrotamacedo-cloud/era-read/extract"
	"github.com/iafrotamacedo-cloud/era-read/layout"
)

// linhaDeTexto monta uma layout.Line de uma palavra so, com o texto dado
// -- estes testes nao se importam com geometria, so com o que
// ExtrairCampos faz com o texto ja juntado.
func linhaDeTexto(texto string) layout.Line {
	return layout.Line{Words: []layout.Word{{Text: texto}}}
}

func TestExtrairCamposCNPJECPF(t *testing.T) {
	linhas := []layout.Line{
		linhaDeTexto("Razao Social: RODRIGUES MATERIAL LTDA CNPJ:14788633000110"),
		linhaDeTexto("Nome: FROTA MACEDO CPF/CNPJ: 27363223000170"),
	}
	c := ExtrairCampos(linhas)

	if len(c.CNPJs) != 2 {
		t.Fatalf("CNPJs = %v, quero 2", c.CNPJs)
	}
	if c.CNPJs[0] != "14.788.633/0001-10" {
		t.Errorf("CNPJs[0] = %q, quero %q", c.CNPJs[0], "14.788.633/0001-10")
	}
	if c.CNPJs[1] != "27.363.223/0001-70" {
		t.Errorf("CNPJs[1] = %q, quero %q", c.CNPJs[1], "27.363.223/0001-70")
	}
	if len(c.CPFs) != 0 {
		t.Errorf("CPFs = %v, quero nenhum (os dois documentos usaram CNPJ)", c.CPFs)
	}
}

func TestExtrairCamposCNPJInvalidoNaoEntra(t *testing.T) {
	// mesma forma de um CNPJ, digito verificador errado -- ruido tipico de
	// reconhecimento (um digito trocado).
	linhas := []layout.Line{linhaDeTexto("CNPJ: 14788633000111")}
	c := ExtrairCampos(linhas)
	if len(c.CNPJs) != 0 {
		t.Errorf("CNPJs = %v, quero nenhum (digito verificador nao bate)", c.CNPJs)
	}
}

func TestExtrairCamposData(t *testing.T) {
	linhas := []layout.Line{
		linhaDeTexto("Dt.Emis:22/07/2026"),
		linhaDeTexto("Total a pagar: 22,90"), // sem data, nao deveria dar falso positivo
	}
	c := ExtrairCampos(linhas)
	if len(c.Datas) != 1 {
		t.Fatalf("Datas = %v, quero 1", c.Datas)
	}
	if y, m, d := c.Datas[0].Date(); y != 2026 || m != 7 || d != 22 {
		t.Errorf("data = %v, quero 2026-07-22", c.Datas[0])
	}
}

func TestExtrairCamposValorPorEtiqueta(t *testing.T) {
	linhas := []layout.Line{
		linhaDeTexto("Total a pagar: 22,90"),
		linhaDeTexto("Valor Produtos: 100,60"),
	}
	c := ExtrairCampos(linhas)

	quero := map[string]extract.Money{
		"Total a pagar":  2290,
		"Valor Produtos": 10060,
	}
	for etiqueta, v := range quero {
		got, ok := c.Valores[etiqueta]
		if !ok {
			t.Errorf("faltou o valor de %q", etiqueta)
			continue
		}
		if got != v {
			t.Errorf("%q = %v, quero %v", etiqueta, got, v)
		}
	}
}

func TestExtrairCamposSoAPrimeiraOcorrenciaDaEtiqueta(t *testing.T) {
	linhas := []layout.Line{
		linhaDeTexto("Total a pagar: 22,90"),
		linhaDeTexto("Total a pagar: 999,00"), // repeticao -- nao deveria substituir
	}
	c := ExtrairCampos(linhas)
	if got := c.Valores["Total a pagar"]; got != 2290 {
		t.Errorf("Total a pagar = %v, quero 22,90 (a primeira ocorrencia)", got)
	}
}

func TestExtrairCamposEtiquetaAusenteNaoEntra(t *testing.T) {
	linhas := []layout.Line{linhaDeTexto("Quantidade: 15,000 Preco Unitario: 1,00")}
	c := ExtrairCampos(linhas)
	if len(c.Valores) != 0 {
		t.Errorf("Valores = %v, quero nenhum (nenhuma etiqueta conhecida na linha)", c.Valores)
	}
}

func TestIndexNormalizadoIgnoraAcentoEMaiuscula(t *testing.T) {
	casos := []struct {
		s, alvo string
		quero   int
	}{
		{"Razão Social: X", "Razao Social", 0},
		{"RAZÃO SOCIAL: X", "razao social", 0},
		{"Identificacäo do Emitente", "identificacao", 0}, // acento errado (ä em vez de ã) tambem cai
		{"CNPJ/CPF: 123", "xyz", -1},
	}
	for _, c := range casos {
		if got := indexNormalizado(c.s, c.alvo); got != c.quero {
			t.Errorf("indexNormalizado(%q, %q) = %d, quero %d", c.s, c.alvo, got, c.quero)
		}
	}
}

// TestApósEtiquetaComAcentoAntesNaoCorrompe cobre o motivo de comparar por
// RUNE, nao por byte: um acento antes do rotulo ocupa mais de 1 byte em
// UTF-8, e cortar por indice de byte no meio de um caractere quebraria o
// texto (ou erraria a posicao) quando o rotulo vem depois de algum acento.
func TestApósEtiquetaComAcentoAntesNaoCorrompe(t *testing.T) {
	got, ok := apósEtiqueta("Razão Social: RODRIGUES LTDA CNPJ:123", "CNPJ:")
	if !ok {
		t.Fatal("deveria ter achado a etiqueta CNPJ:")
	}
	if want := "123"; got != want {
		t.Errorf("resto = %q, quero %q", got, want)
	}
}
