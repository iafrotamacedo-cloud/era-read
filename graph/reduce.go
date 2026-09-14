package graph

import (
	"fmt"

	"github.com/iafrotamacedo-cloud/era-read/nn"
	"github.com/iafrotamacedo-cloud/era-read/onnx"
	"github.com/iafrotamacedo-cloud/era-read/tensor"
)

// montaReduceMean tira a media ao longo dos eixos dados, ou de todos se
// nenhum for informado.
//
// Semantica do opset 13-17 do ONNX: axes e atributo (nao entrada -- isso so
// muda no opset 18), keepdims por padrao mantem os eixos reduzidos com
// tamanho 1 em vez de remove-los. Aparece na decomposicao manual de
// normalizacao (media -> subtrai -> Pow(2) -> media -> Sqrt -> Div), quando
// o exportador nao emite LayerNormalization direto.
func montaReduceMean(b *builder, n *onnx.Node) (*operation, error) {
	temAxesAttr := n.Attr("axes") != nil
	axesAttr := n.AttrInts("axes", nil)
	keepdims := n.AttrInt("keepdims", 1) != 0

	return novaOp(n, func(ws *nn.Workspace, ins []*tensor.Tensor) ([]*tensor.Tensor, error) {
		x := ins[0]
		r := x.Rank()

		var axes []int
		if temAxesAttr {
			axes = make([]int, len(axesAttr))
			for i, a := range axesAttr {
				ax := int(a)
				if ax < 0 {
					ax += r
				}
				axes[i] = ax
			}
		} else {
			axes = make([]int, r)
			for i := range axes {
				axes[i] = i
			}
		}

		reduzir := make([]bool, r)
		for _, a := range axes {
			if a < 0 || a >= r {
				return nil, fmt.Errorf("ReduceMean: eixo %d fora da forma %v", a, x.Shape)
			}
			reduzir[a] = true
		}

		formaComEixos := make([]int, r)
		formaSaida := make([]int, 0, r)
		contagem := 1
		for i, d := range x.Shape {
			if reduzir[i] {
				formaComEixos[i] = 1
				contagem *= d
				if keepdims {
					formaSaida = append(formaSaida, 1)
				}
			} else {
				formaComEixos[i] = d
				formaSaida = append(formaSaida, d)
			}
		}
		if contagem == 0 {
			contagem = 1
		}

		// ws.Tensor devolve memoria reciclada, com lixo -- precisa zerar
		// antes de acumular.
		soma := ws.Tensor(formaComEixos...)
		soma.Fill(0)
		total := x.Size()
		srcIdx := make([]int, r)
		dstIdx := make([]int, r)
		for i := 0; i < total; i++ {
			for d := 0; d < r; d++ {
				if reduzir[d] {
					dstIdx[d] = 0
				} else {
					dstIdx[d] = srcIdx[d]
				}
			}
			soma.Set(soma.At(dstIdx...)+x.At(srcIdx...), dstIdx...)

			for d := r - 1; d >= 0; d-- {
				srcIdx[d]++
				if srcIdx[d] < x.Shape[d] {
					break
				}
				srcIdx[d] = 0
			}
		}

		for i := range soma.Data {
			soma.Data[i] /= float32(contagem)
		}

		if keepdims {
			return []*tensor.Tensor{soma}, nil
		}
		out, err := soma.Reshape(formaSaida...)
		if err != nil {
			return nil, err
		}
		return []*tensor.Tensor{out}, nil
	}), nil
}
