package graph

import (
	"fmt"

	"github.com/iafrotamacedo-cloud/era-read/nn"
	"github.com/iafrotamacedo-cloud/era-read/tensor"
)

// matmulGenerico multiplica dois tensores de rank >= 2 pelas regras de
// transmissao do NumPy: as duas ultimas dimensoes sao a matriz (M,K) e
// (K,N), e tudo antes delas e "lote", transmitido como em Add/Mul.
//
// Existe porque MatMul aparece de duas formas bem diferentes num grafo real:
// como camada densa (um lado e peso fixo -- caminho rapido em montaMatMul,
// via nn.Linear) e como produto de atencao (Q @ K^T, nenhum lado e peso
// treinado -- os dois so existem depois que a imagem chega). Autoatencao
// e exatamente esse segundo caso, e foi o que revelou a limitacao: o
// primeiro MatMul implementado so aceitava peso constante.
func matmulGenerico(ws *nn.Workspace, a0, b0 *tensor.Tensor) (*tensor.Tensor, error) {
	ra, rb := a0.Rank(), b0.Rank()
	if ra < 2 || rb < 2 {
		return nil, fmt.Errorf("MatMul com operando de rank %d/%d (< 2) ainda nao e suportado", ra, rb)
	}

	m, k1 := a0.Shape[ra-2], a0.Shape[ra-1]
	k2, nCols := b0.Shape[rb-2], b0.Shape[rb-1]
	if k1 != k2 {
		return nil, fmt.Errorf("MatMul: dimensao interna nao bate, %d contra %d (formas %v e %v)",
			k1, k2, a0.Shape, b0.Shape)
	}

	// Contiguous() garante que cada lote e um bloco M*K (ou K*N) corrido em
	// Data -- necessario porque um operando pode ser a VIEW de um Transpose,
	// que so troca strides, sem mover memoria.
	a := a0.Contiguous()
	b := b0.Contiguous()

	batchShape, err := formaTransmitida(a.Shape[:ra-2], b.Shape[:rb-2])
	if err != nil {
		return nil, fmt.Errorf("MatMul: lotes %v e %v nao sao compativeis: %w", a.Shape[:ra-2], b.Shape[:rb-2], err)
	}

	// Strides no espaco do lote transmitido -- em unidade de BLOCOS (um
	// bloco = uma matriz inteira), nao de elementos. Reusa a mesma logica
	// de transmissao de Add/Mul: dimensao de tamanho 1 recebe passo zero,
	// o que faz o mesmo bloco repetir.
	sa := stridesTransmitidos(a.Shape[:ra-2], batchShape)
	sb := stridesTransmitidos(b.Shape[:rb-2], batchShape)

	numLotes := 1
	for _, d := range batchShape {
		numLotes *= d
	}

	saida := append(append([]int(nil), batchShape...), m, nCols)
	out := ws.Tensor(saida...)

	blocoA, blocoB, blocoOut := m*k1, k2*nCols, m*nCols

	idx := make([]int, len(batchShape))
	offA, offB := 0, 0
	for lote := 0; lote < numLotes; lote++ {
		ba := a.Data[offA*blocoA : offA*blocoA+blocoA]
		bb := b.Data[offB*blocoB : offB*blocoB+blocoB]
		bo := out.Data[lote*blocoOut : lote*blocoOut+blocoOut]

		for i := 0; i < m; i++ {
			for j := 0; j < nCols; j++ {
				var soma float32
				for k := 0; k < k1; k++ {
					soma += ba[i*k1+k] * bb[k*nCols+j]
				}
				bo[i*nCols+j] = soma
			}
		}

		for d := len(batchShape) - 1; d >= 0; d-- {
			idx[d]++
			offA += sa[d]
			offB += sb[d]
			if idx[d] < batchShape[d] {
				break
			}
			offA -= sa[d] * batchShape[d]
			offB -= sb[d] * batchShape[d]
			idx[d] = 0
		}
	}

	return out, nil
}
