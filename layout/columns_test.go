package layout

import "testing"

// wordAt monta uma Word retangular simples, so para estes testes -- altura
// fixa de 20 (uma "linha" de texto tipica), largura e posicao dadas.
func wordAt(x0 float64, w float64, texto string) Word {
	return Word{Box: caixa(x0, 0, x0+w, 20), Text: texto}
}

func TestSplitCellsVaoPequenoFicaJunto(t *testing.T) {
	// "Bairro:" partido em duas regioes vizinhas pelo detector -- vao de 8px,
	// bem menor que 3x a altura (60).
	l := Line{Words: []Word{wordAt(0, 50, "Bairro:"), wordAt(58, 100, "CIDADE DOS FUNCIONARIOS")}}
	got := l.SplitCells(DefaultGapFactor)
	if len(got) != 1 {
		t.Fatalf("celulas = %d, quero 1 (vao pequeno, mesmo campo)", len(got))
	}
	if want := "Bairro: CIDADE DOS FUNCIONARIOS"; got[0].Text() != want {
		t.Errorf("texto = %q, quero %q", got[0].Text(), want)
	}
}

func TestSplitCellsVaoGrandeSepara(t *testing.T) {
	// "Total a pagar:" e um valor numa coluna bem separada -- vao de 300px,
	// bem maior que 3x a altura (60).
	l := Line{Words: []Word{wordAt(0, 80, "Total a pagar:"), wordAt(380, 40, "22,90")}}
	got := l.SplitCells(DefaultGapFactor)
	if len(got) != 2 {
		t.Fatalf("celulas = %d, quero 2 (vao grande, campos diferentes)", len(got))
	}
	if got[0].Text() != "Total a pagar:" || got[1].Text() != "22,90" {
		t.Errorf("celulas = %q, %q", got[0].Text(), got[1].Text())
	}
}

func TestSplitCellsVariasFronteiras(t *testing.T) {
	l := Line{Words: []Word{
		wordAt(0, 40, "A"), wordAt(45, 40, "B"), // vao 5: junto
		wordAt(300, 40, "C"), // vao 255: separado
		wordAt(345, 40, "D"), // vao 5: junto de C
	}}
	got := l.SplitCells(DefaultGapFactor)
	if len(got) != 2 {
		t.Fatalf("celulas = %d, quero 2", len(got))
	}
	if want := "A B"; got[0].Text() != want {
		t.Errorf("celula 0 = %q, quero %q", got[0].Text(), want)
	}
	if want := "C D"; got[1].Text() != want {
		t.Errorf("celula 1 = %q, quero %q", got[1].Text(), want)
	}
}

func TestSplitCellsLinhaDeUmaPalavraSo(t *testing.T) {
	l := Line{Words: []Word{wordAt(0, 40, "Sozinha")}}
	got := l.SplitCells(DefaultGapFactor)
	if len(got) != 1 || got[0].Text() != "Sozinha" {
		t.Fatalf("celulas = %v, quero 1 celula com \"Sozinha\"", got)
	}
}

func TestSplitCellsLinhaVazia(t *testing.T) {
	if got := (Line{}).SplitCells(DefaultGapFactor); got != nil {
		t.Errorf("celulas = %v, quero nil", got)
	}
}

func TestCellBoundsEnvolveAsPalavras(t *testing.T) {
	c := Cell{Words: []Word{wordAt(0, 40, "A"), wordAt(100, 40, "B")}}
	min, max := c.Bounds()
	if min.X != 0 || max.X != 140 {
		t.Errorf("bounds = %v..%v, quero X de 0 a 140", min, max)
	}
}
