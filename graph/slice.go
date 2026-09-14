package graph

import (
	"fmt"

	"github.com/iafrotamacedo-cloud/era-read/nn"
	"github.com/iafrotamacedo-cloud/era-read/onnx"
	"github.com/iafrotamacedo-cloud/era-read/tensor"
)

// montaSlice recorta um tensor por eixo, com inicio/fim/passo proprios para
// cada um -- a forma como o opset 10+ do ONNX faz (antes disso eram
// atributos fixos; agora sao entradas, o que permite indice calculado em
// tempo de execucao, tipico de reshape dinamico por largura variavel).
//
// Por isso starts/ends/axes/steps sao lidos em EXECUCAO, no proprio exec,
// e nao via b.peso na montagem: eles podem vir de um Shape sobre a entrada
// de verdade, que so existe quando a imagem chega.
func montaSlice(b *builder, n *onnx.Node) (*operation, error) {
	if len(n.Inputs) < 3 || len(n.Inputs) > 5 {
		return nil, fmt.Errorf("Slice espera de 3 a 5 entradas, recebeu %d", len(n.Inputs))
	}

	return novaOp(n, func(ws *nn.Workspace, ins []*tensor.Tensor) ([]*tensor.Tensor, error) {
		x := ins[0]
		r := x.Rank()

		starts := intsDe(ins[1])
		ends := intsDe(ins[2])

		var axes []int
		if len(ins) > 3 && ins[3] != nil {
			axes = intsDe(ins[3])
		} else {
			axes = make([]int, len(starts))
			for i := range axes {
				axes[i] = i
			}
		}
		var steps []int
		if len(ins) > 4 && ins[4] != nil {
			steps = intsDe(ins[4])
		} else {
			steps = make([]int, len(starts))
			for i := range steps {
				steps[i] = 1
			}
		}

		if len(starts) != len(ends) || len(starts) != len(axes) || len(starts) != len(steps) {
			return nil, fmt.Errorf("Slice: starts(%d)/ends(%d)/axes(%d)/steps(%d) com tamanhos diferentes",
				len(starts), len(ends), len(axes), len(steps))
		}

		// Por padrao, cada eixo passa inteiro (equivalente a nao fatiar).
		eStart := make([]int, r)
		eEnd := make([]int, r)
		eStep := make([]int, r)
		for i := 0; i < r; i++ {
			eStart[i], eEnd[i], eStep[i] = 0, x.Shape[i], 1
		}

		for k, axRaw := range axes {
			ax := axRaw
			if ax < 0 {
				ax += r
			}
			if ax < 0 || ax >= r {
				return nil, fmt.Errorf("Slice: eixo %d fora da forma %v", axRaw, x.Shape)
			}
			dim := x.Shape[ax]
			step := steps[k]
			if step == 0 {
				return nil, fmt.Errorf("Slice: passo zero no eixo %d", ax)
			}

			s, e := starts[k], ends[k]
			if s < 0 {
				s += dim
			}
			if e < 0 {
				e += dim
			}

			if step > 0 {
				s = clampInt(s, 0, dim)
				e = clampInt(e, 0, dim)
			} else {
				// Passo negativo percorre de tras para frente; o limite
				// inferior e -1 (um antes do primeiro elemento), nao 0,
				// para poder incluir o indice 0 no resultado.
				s = clampInt(s, -1, dim-1)
				e = clampInt(e, -1, dim-1)
			}

			eStart[ax], eEnd[ax], eStep[ax] = s, e, step
		}

		forma := make([]int, r)
		for i := 0; i < r; i++ {
			forma[i] = contarPassos(eStart[i], eEnd[i], eStep[i])
		}

		out := ws.Tensor(forma...)
		total := out.Size()
		if total == 0 {
			return []*tensor.Tensor{out}, nil
		}

		idx := make([]int, r)
		srcIdx := make([]int, r)
		for i := 0; i < total; i++ {
			for d := 0; d < r; d++ {
				srcIdx[d] = eStart[d] + idx[d]*eStep[d]
			}
			out.Set(x.At(srcIdx...), idx...)

			for d := r - 1; d >= 0; d-- {
				idx[d]++
				if idx[d] < forma[d] {
					break
				}
				idx[d] = 0
			}
		}
		return []*tensor.Tensor{out}, nil
	}), nil
}

// contarPassos devolve quantos indices [start, end) (ou (end, start] se o
// passo for negativo) cabem andando de step em step.
func contarPassos(start, end, step int) int {
	if step > 0 {
		if end <= start {
			return 0
		}
		return (end - start + step - 1) / step
	}
	if start <= end {
		return 0
	}
	return (start - end + (-step) - 1) / (-step)
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// intsDe converte um tensor 1D de float32 (a ERA representa todo indice ou
// forma assim -- ver montaReshape) numa lista de int.
func intsDe(t *tensor.Tensor) []int {
	f := t.Flat()
	out := make([]int, len(f))
	for i, v := range f {
		out[i] = int(v)
	}
	return out
}
