package detect

import (
	"testing"
)

// TestToLinePolygonSeparaEmCimaEBaixo usa um bloco retangular bem mais
// largo que alto -- o formato normal de uma linha de texto -- e confere
// que ToLinePolygon separa a metade de cima (Y menor) da metade de baixo
// (Y maior), cada uma com a contagem pedida.
func TestToLinePolygonSeparaEmCimaEBaixo(t *testing.T) {
	b := retangulo(0, 0, 9, 2) // bloco 10x3
	contorno := tracarContorno(b, 0, 0)

	const pontos = 4
	got, err := ToLinePolygon(contorno, pontos)
	if err != nil {
		t.Fatalf("ToLinePolygon: %v", err)
	}
	if len(got) != 2*pontos {
		t.Fatalf("len = %d, quero %d", len(got), 2*pontos)
	}

	cima := got[:pontos]
	baixo := got[pontos:]

	var mediaCima, mediaBaixo float64
	for _, p := range cima {
		mediaCima += p.Y
	}
	mediaCima /= pontos
	for _, p := range baixo {
		mediaBaixo += p.Y
	}
	mediaBaixo /= pontos

	if mediaCima >= mediaBaixo {
		t.Errorf("media Y de cima (%v) deveria ser menor que a de baixo (%v)", mediaCima, mediaBaixo)
	}

	// dentro de cada borda, o X tem que ser (quase) monotono -- crescente
	// em cima, decrescente embaixo -- coerente com "esquerda pra direita"
	// e "direita pra esquerda".
	for i := 1; i < len(cima); i++ {
		if cima[i].X < cima[i-1].X {
			t.Errorf("borda de cima nao esta em ordem crescente de X: %v", cima)
			break
		}
	}
	for i := 1; i < len(baixo); i++ {
		if baixo[i].X > baixo[i-1].X {
			t.Errorf("borda de baixo nao esta em ordem decrescente de X: %v", baixo)
			break
		}
	}
}

func TestToLinePolygonPoucosVertices(t *testing.T) {
	if _, err := ToLinePolygon(nil, 4); err == nil {
		t.Error("contorno vazio deveria falhar")
	}
}

func TestToLinePolygonPoucosPontosPorBorda(t *testing.T) {
	b := retangulo(0, 0, 9, 2)
	contorno := tracarContorno(b, 0, 0)
	if _, err := ToLinePolygon(contorno, 1); err == nil {
		t.Error("pontos=1 deveria falhar (precisa de pelo menos 2)")
	}
}
