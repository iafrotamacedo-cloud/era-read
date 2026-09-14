package detect

// vizinhanca8 sao os 8 deslocamentos ao redor de um pixel.
var vizinhanca8 = [8][2]int{
	{-1, -1}, {0, -1}, {1, -1},
	{-1, 0}, {1, 0},
	{-1, 1}, {0, 1}, {1, 1},
}

// label agrupa pixels de texto vizinhos na mesma mancha, por preenchimento
// por vizinhanca (flood fill). Devolve um rotulo por pixel (0 = fundo,
// 1..n = a mancha a que pertence) e quantas manchas foram achadas.
//
// 8-conectividade: dois pixels que se tocam so na diagonal ainda contam
// como a mesma palavra. Importa porque binarizar um mapa suave produz
// borda levemente serrilhada -- com 4-conectividade, esse serrilhado
// partiria uma palavra em duas manchas.
func label(b *Bitmap) ([]int, int) {
	labels := make([]int, len(b.Pix))
	atual := 0
	pilha := make([]int, 0, 64)

	for y := 0; y < b.H; y++ {
		for x := 0; x < b.W; x++ {
			idx := y*b.W + x
			if !b.Pix[idx] || labels[idx] != 0 {
				continue
			}

			atual++
			labels[idx] = atual
			pilha = append(pilha, idx)

			for len(pilha) > 0 {
				cur := pilha[len(pilha)-1]
				pilha = pilha[:len(pilha)-1]
				cx, cy := cur%b.W, cur/b.W

				for _, d := range vizinhanca8 {
					nx, ny := cx+d[0], cy+d[1]
					if !b.In(nx, ny) {
						continue
					}
					nidx := ny*b.W + nx
					if b.Pix[nidx] && labels[nidx] == 0 {
						labels[nidx] = atual
						pilha = append(pilha, nidx)
					}
				}
			}
		}
	}
	return labels, atual
}
