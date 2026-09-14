package recog

import (
	"strings"
	"testing"

	"github.com/iafrotamacedo-cloud/era-read/tensor"
)

// spike monta uma linha de probabilidade de C classes com 1 no indice dado
// e 0 no resto -- um pico limpo, sem ambiguidade de qual e o argmax.
func spike(c, idx int, prob float32) []float32 {
	v := make([]float32, c)
	v[idx] = prob
	return v
}

// seq monta o tensor [1,T,C] de um unico exemplo a partir de uma lista de
// linhas de C classes (uma por passo de tempo).
func seq(passos ...[]float32) *tensor.Tensor {
	c := len(passos[0])
	flat := make([]float32, 0, len(passos)*c)
	for _, p := range passos {
		flat = append(flat, p...)
	}
	return tensor.MustFromSlice(flat, 1, len(passos), c)
}

func TestDecodeCTC(t *testing.T) {
	// dicionario de teste: indice 0 = branco (automatico), 1='a', 2='b', 3='c'
	cs := NewCharset([]string{"a", "b", "c"})
	const c = 4

	t.Run("colapsa repeticao do mesmo indice", func(t *testing.T) {
		// a, a, a -> uma letra so
		x := seq(spike(c, 1, 0.9), spike(c, 1, 0.8), spike(c, 1, 0.7))
		got, err := DecodeCTC(x, cs)
		if err != nil {
			t.Fatal(err)
		}
		if got[0].Texto != "a" {
			t.Errorf("texto = %q, quero %q", got[0].Texto, "a")
		}
	})

	t.Run("branco separa repeticao de verdade", func(t *testing.T) {
		// a, branco, a -> duas letras (a mesma letra duas vezes, de proposito)
		x := seq(spike(c, 1, 0.9), spike(c, 0, 0.99), spike(c, 1, 0.8))
		got, err := DecodeCTC(x, cs)
		if err != nil {
			t.Fatal(err)
		}
		if got[0].Texto != "aa" {
			t.Errorf("texto = %q, quero %q", got[0].Texto, "aa")
		}
	})

	t.Run("branco no inicio e no fim e descartado", func(t *testing.T) {
		x := seq(spike(c, 0, 0.99), spike(c, 1, 0.9), spike(c, 2, 0.9), spike(c, 0, 0.99))
		got, err := DecodeCTC(x, cs)
		if err != nil {
			t.Fatal(err)
		}
		if got[0].Texto != "ab" {
			t.Errorf("texto = %q, quero %q", got[0].Texto, "ab")
		}
	})

	t.Run("sequencia so de branco da texto vazio e confianca zero", func(t *testing.T) {
		x := seq(spike(c, 0, 0.99), spike(c, 0, 0.99))
		got, err := DecodeCTC(x, cs)
		if err != nil {
			t.Fatal(err)
		}
		if got[0].Texto != "" {
			t.Errorf("texto = %q, quero vazio", got[0].Texto)
		}
		if got[0].Confianca != 0 {
			t.Errorf("confianca = %v, quero 0", got[0].Confianca)
		}
	})

	t.Run("confianca e a media dos caracteres emitidos", func(t *testing.T) {
		// 'a' com prob 0.8, 'b' com prob 0.6 -> media 0.7. O branco no meio
		// nao entra na media (nem a repeticao colapsada, que so registra a
		// primeira ocorrencia).
		x := seq(spike(c, 1, 0.8), spike(c, 0, 0.99), spike(c, 2, 0.6))
		got, err := DecodeCTC(x, cs)
		if err != nil {
			t.Fatal(err)
		}
		if d := got[0].Confianca - 0.7; d > 1e-6 || d < -1e-6 {
			t.Errorf("confianca = %v, quero 0.7", got[0].Confianca)
		}
	})

	t.Run("lote com mais de uma amostra", func(t *testing.T) {
		// duas amostras, T=2, C=4, montadas a mao (sem usar seq, que so
		// serve para lote de 1).
		flat := []float32{}
		flat = append(flat, spike(c, 1, 0.9)...) // amostra 0, t=0: 'a'
		flat = append(flat, spike(c, 2, 0.9)...) // amostra 0, t=1: 'b'
		flat = append(flat, spike(c, 3, 0.9)...) // amostra 1, t=0: 'c'
		flat = append(flat, spike(c, 0, 0.9)...) // amostra 1, t=1: branco
		x := tensor.MustFromSlice(flat, 2, 2, c)

		got, err := DecodeCTC(x, cs)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 {
			t.Fatalf("len = %d, quero 2", len(got))
		}
		if got[0].Texto != "ab" {
			t.Errorf("amostra 0: texto = %q, quero %q", got[0].Texto, "ab")
		}
		if got[1].Texto != "c" {
			t.Errorf("amostra 1: texto = %q, quero %q", got[1].Texto, "c")
		}
	})

	t.Run("rank errado da erro", func(t *testing.T) {
		x := tensor.MustFromSlice([]float32{1, 2, 3, 4}, 2, 2)
		if _, err := DecodeCTC(x, cs); err == nil {
			t.Error("forma 2D deveria dar erro")
		}
	})

	t.Run("numero de classes nao bate com o dicionario da erro", func(t *testing.T) {
		x := seq(spike(3, 1, 0.9)) // C=3, mas o dicionario (com o branco) tem 4
		if _, err := DecodeCTC(x, cs); err == nil {
			t.Error("numero de classes divergente deveria dar erro")
		}
	})
}

func TestNewCharset(t *testing.T) {
	cs := NewCharset([]string{"a", "b", "c"})
	if len(cs) != 4 {
		t.Fatalf("len = %d, quero 4 (branco + 3 caracteres)", len(cs))
	}
	if cs[0] != "" {
		t.Errorf("cs[0] (branco) = %q, quero vazio", cs[0])
	}
	if got := strings.Join([]string(cs[1:]), ""); got != "abc" {
		t.Errorf("cs[1:] = %q, quero %q", got, "abc")
	}
}
