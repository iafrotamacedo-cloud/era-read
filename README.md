# ERA READ

[![CI](https://github.com/iafrotamacedo-cloud/era-read/actions/workflows/ci.yml/badge.svg)](https://github.com/iafrotamacedo-cloud/era-read/actions/workflows/ci.yml)

Motor de leitura de documentos em Go puro. Zero dependências.

## O que é

Recebe a foto de um papel — ou a imagem de uma tela — e devolve os dados que
estavam escritos ali.

Não é scanner e não é cópia. A diferença tem um teste objetivo:

| | Entrada | Saída | Se apagar a imagem original |
|---|---|---|---|
| Scanner | papel | imagem | perdeu tudo |
| OCR | imagem | texto corrido | perdeu a estrutura |
| **ERA READ** | imagem | texto com posição, e depois campos tipados | **não perdeu nada** |

O produto final não é uma imagem melhor nem um `.txt` — é dado. Uma nota
fiscal lida vira `{fornecedor, cnpj, numero, data, itens[...], total}`. O
texto corrido é um estágio intermediário, não a entrega.

Não é um sistema. Não tem banco, tela, login nem nuvem. É o motor que outros
sistemas importam.

## Princípios

Cada um destes é verificado pelo CI a cada push. Promessa sem verificação é
só intenção.

- **Zero dependências.** Só a biblioteca padrão do Go.
- **Sem cgo.** Cross-compila para 11 plataformas, do Raspberry Pi ao WASM.
- **Sem dados embutidos.** Você traz o `.onnx` que quiser usar, quando a fase
  de detecção e reconhecimento existir. Mantém a licença do código limpa e
  desacopla o projeto da licença dos modelos.
- **Sem estado.** A biblioteca não guarda nada. Onde salvar é decisão de
  quem usa.
- **Referência antes de otimização.** Toda implementação rápida é conferida
  nos testes contra uma versão óbvia, escrita para ser fácil de ler em vez de
  rápida — ver `imgproc/reference.go`. É o que permite afirmar que uma
  otimização preservou o resultado.

## Escopo

O alvo é **documento físico na condição em que ele aparece na vida real**:
fotografado na mão, torto, com sombra e **curvo** — livro aberto, folha
enrolada, papel amassado.

Dentro:

- papel fotografado ou escaneado, plano **ou curvo**
- tela de computador — o caso fácil: texto renderizado, sem perspectiva, sem sombra

Fora, por ora:

- **escrita à mão** — outro modelo, outro conjunto de treino
- **PDF que já tem texto** — não precisa de OCR nenhum, é parsear. Vale ter
  um dia, e cabe em Go puro (o `compress/zlib` da stdlib cobre os streams; o
  trabalho de verdade são as fontes embutidas com encoding próprio). Fica de
  fora agora porque não é o problema declarado.

## Por que papel curvo tem conserto

Papel é uma **superfície desenvolvível**: enrola e dobra, mas não estica.
Curvatura gaussiana zero em todo ponto — existe um desdobramento exato de
volta ao plano, preservando distâncias. É por isso que o problema é
tratável: a informação não foi destruída, só reparametrizada.

Fotografar um livro aberto produz três deformações ao mesmo tempo, com
dificuldades bem diferentes:

| Deformação | O que acontece | Dificuldade |
|---|---|---|
| Curvatura da linha | as baselines viram arcos | baixa — geometria |
| Compressão em profundidade | perto da lombada a folha inclina; as letras espremem na horizontal | **alta** — é a que quebra o OCR |
| Gradiente de iluminação | a curvatura cria sombra; binarização global falha | baixa — normalização local |

Uma homografia mapeia plano em plano e não representa nenhuma das duas
primeiras. Papel curvo não é "papel torto com mais esforço" — é outra
categoria de problema.

## Quatro níveis de retificação

Em vez de um dewarp único, quatro caminhos de custo crescente. O pipeline
mede a deformação e escolhe sozinho:

| Nível | Para | Como | Precisa de rede? |
|---|---|---|---|
| **N0** | tela, scan plano | nada | não |
| **N1** | papel plano fotografado torto | 4 cantos → homografia | não |
| **N2** | papel curvo suave | **retificação por linha** | não |
| **N3** | amassado, vinco, curvatura forte | campo de deslocamento previsto | sim, `.onnx` opcional |

### N2 — retificar a linha, não a página

Um scanner precisa produzir uma imagem plana bonita. Este motor **não** —
ele só precisa que o reconhecedor consiga ler, e o reconhecedor lê recortes
de linha de ~32 px de altura. Nessa escala, uma linha isolada é quase reta
mesmo numa página bem curva.

Então: agrupa as caixas detectadas em linhas, ajusta uma curva pela baseline,
e amostra uma faixa seguindo a normal local. A linha sai reta — sem
otimização global, sem modelo de superfície, sem rede.

O que o N2 não resolve: a compressão em profundidade continua dentro da
linha. Na prática ela só aperta o bastante para quebrar a leitura na faixa
colada na dobra. É onde o N3 entra, e é o único motivo de o N3 existir.

### N3

Rede que prevê um *backward map* 2D — para cada pixel da saída, onde buscar
na entrada — em resolução baixa, interpolado e amostrado sobre a imagem
original em resolução plena. Por isso a rede pode ser pequena sem custar
detalhe.

## Detectar antes de retificar

Consequência arquitetural, e é contraintuitiva:

```
imagem → detecta texto (na imagem torta)
       → mede a deformação nas próprias caixas detectadas
       → escolhe N0/N1/N2/N3 → retifica
       → lê cada linha → layout → dados
```

Funciona porque o detector devolve **polígonos**, não retângulos: ele acha
texto curvo sem se importar com a curvatura. E as caixas curvas são
exatamente a medida de quanto a página está deformada. A detecção paga por
si duas vezes — alimenta o reconhecedor e decide o dewarp.

## Roteiro

| Fase | Pacote | Entrega | Estado |
|---|---|---|---|
| 1 | `imgproc` | I/O, cinza, normalização de iluminação, amostragem bilinear | **pronto** |
| 2 | `geom` | polígono, homografia (com `RemapHomography`, que já cobre a retificação de N1), ajuste de curva, remap | **pronto** |
| 3 | `detect` | DBNet + contornos + expansão de polígono | parcial — pós-processamento (contornos + expansão) **pronto**; falta o pré-processamento da imagem e a integração fim a fim com um `.onnx` real |
| 4 | `dewarp` | medidor de deformação (decide N0/N1/N2/N3 a partir dos polígonos), retificação por linha de N2 — e N3 depois | **pronto** (N3 fica para quando entrar rede) |
| 5 | `recog` | SVTR + decodificação CTC, charset pt-BR | — |
| 6 | `layout` | linhas, colunas, tabelas, ordem de leitura | parcial — linhas e ordem de leitura de 1 coluna **prontas**; colunas e tabela faltam |
| 7 | `extract` | campos tipados por tipo de documento | parcial — CNPJ, CPF, data e valor monetário **prontos**; ligar aos campos de cada tipo de documento falta |

A fase 4 (`dewarp`) foi adiantada fora de ordem porque só depende de
`geom` — não de rede nem de decisão pendente. `geom.RemapCurve` amostra uma
faixa da imagem seguindo uma curva, em espaçamento igual de comprimento de
arco (não de x, para não comprimir o texto num trecho mais inclinado da
curva) e ao longo da normal local. `RectifyLine` liga isso à baseline de uma
linha de texto: ajusta a curva pelos pontos e delega a amostragem.

Ela consome polígonos, então antes da fase 3 (`detect`) existir só dá para
testar com polígono sintético; o teste com um detector de verdade
alimentando ela fica para quando a fase 3 fechar. N3 (o caso que N2 não
resolve — a compressão em profundidade perto da dobra) continua não
implementado: precisa de rede.

**`detect`** também avançou em parte: a metade que não é rede. A rede (o
grafo do PP-OCRv4, ver "O motor de inferência" abaixo) devolve um mapa de
probabilidade — um valor entre 0 e 1 por pixel, "isto é texto?" — porque é
a única forma de a saída ter tamanho fixo não importando quantas palavras
existam na imagem nem o formato de cada uma (reto ou curvo). Esse mapa
ainda não é útil para recortar e ler; o pacote transforma ele em polígonos
discretos:

```
mapa de probabilidade
  -> Binarize    (corta num limiar: texto ou não)
  -> label        (agrupa pixels vizinhos na mesma mancha, 8-conectado)
  -> FindContours (traça o contorno de cada mancha, algoritmo de Moore)
  -> RegionScore  (mede a confiança média de cada contorno)
  -> filtra       (descarta mancha pequena ou de confiança baixa)
  -> Unclip       (expande o contorno de volta ao tamanho real)
```

O DBNet é treinado para prever cada mancha um pouco *encolhida* — de
propósito, para que duas palavras vizinhas não colem numa mancha só na
binarização. `Unclip` desfaz isso, expandindo pela fórmula do paper
(`distância = área × proporção ÷ perímetro`), empurrando cada vértice pela
bissetriz das duas arestas que se encontram nele (o "miter join" clássico
de desenho de contorno) — conferido com um caso exato (um quadrado cresce
exatamente a distância certa em cada lado, sem aproximação).

Falta um adaptador (`ToLinePolygon`) entre o contorno bruto (um vértice por
pixel de borda, formato genérico) e o poligono que `dewarp.ExtractBaseline`
espera (metade dos vértices formando a borda de cima esquerda→direita, a
outra metade a de baixo direita→esquerda) — esse já existe. O que falta de
verdade na fase 3: o pré-processamento da imagem de entrada (redimensionar,
normalizar) e testar a cadeia inteira com uma imagem real, não só com a
fiação confirmada contra ruído aleatório (ver "O motor de inferência").

As fases 6 e 7 também foram adiantadas em parte, cada uma na fatia que não
depende de reconhecimento nenhum:

**`layout`** organiza palavras soltas (posição + texto) em linhas, só por
geometria — sobreposição vertical das caixas decide o que é a mesma linha;
ordem horizontal dentro da linha e vertical entre linhas dá a ordem de
leitura. Cobre certo um bloco de texto de coluna única, que é a maior parte
dos campos de nota fiscal, boleto e ordem de compra. **Não** cobre
múltiplas colunas nem tabela: separar coluna de linha por geometria pura
exige calibrar um limiar de espaçamento horizontal contra documento real,
que este motor ainda não tem. Fica registrado como pendência em vez de
chutado.

**`extract`** reconhece CNPJ, CPF, data (numérica e por extenso) e valor
monetário dentro de texto já lido. CNPJ e CPF têm dígito verificador — um
algoritmo publicado, não um palpite — então dá para varrer texto livre com
confiança: `FindCNPJs`/`FindCPFs` descartam qualquer sequência de dígitos
que não feche a conta. Data e dinheiro não têm verificação equivalente,
então só há o parser (`ParseDateBR`, `ParseMoney`), aplicado a um campo já
isolado por `layout` ou por uma etiqueta conhecida ("Total:") — nunca um
buscador de texto livre, que erraria demais colidindo com quantidade,
número de pedido, CEP. `Money` é inteiro (centavos), não `float64`: dinheiro
não admite o erro de arredondamento binário. Falta ligar isso aos campos
específicos de cada tipo de documento — o que só faz sentido com a fase 5
(reconhecimento) alimentando de verdade.

A Fase 5 é o marco real: fotografar um papel na mão e o texto sair certo.
Antes disso é infraestrutura.

As fases 1, 2, 4 e 6, e o pós-processamento da fase 3, são geometria e
processamento de imagem — não tocam em rede nenhuma e não dependem de nada
fora da biblioteca padrão.

## O motor de inferência

As fases 3 (detecção) e 5 (reconhecimento) precisam rodar uma rede neural: ler
um `.onnx`, executar o grafo. Este projeto **nasceu dentro do monorepo `era`**
justamente para reusar isso sem duplicar -- `tensor`, `kernel`, `nn`, `onnx` e
`graph` não têm nada de específico de rosto.

**Decisão tomada em 14/09/2026: cópia, não importação.** `tensor/`, `kernel/`,
`nn/`, `onnx/`, `graph/` e `internal/protowire/` vivem aqui dentro, como
pacotes próprios deste repositório -- não como dependência do `era`. A ERA
READ é **100% independente**: zero import de `github.com/iafrotamacedo-cloud/era`,
zero risco de uma mudança ali quebrar isto aqui sem aviso, zero necessidade de
coordenar com outra sessão para tocar no próprio motor de inferência.

O preço é o de sempre em cópia: as duas bases podem divergir com o tempo. Foi
uma escolha deliberada em troca de nunca depender de decisão, de commit ou de
CI de outro repositório para o próprio motor de inferência funcionar.

### O que veio da cópia

Trazido do `era` (`faces/tensor`, `faces/kernel`, `faces/nn`, `faces/onnx`,
`faces/graph`, `internal/protowire`) no estado em que estava em 14/09/2026,
com os caminhos de import reescritos para `github.com/iafrotamacedo-cloud/era-read/...`.
**Não** veio `faces/graph/modelo_real_test.go`: é um teste de integração
específico do SFace (reconhecimento facial) contra o ONNX Runtime, sem
relação com este motor.

O `faces/onnx` e o `faces/graph` do `era` já tinham sido validados contra o
ONNX Runtime na quinta casa decimal (com um modelo de rosto) antes desta
cópia -- essa prova não se repete aqui automaticamente, mas o código é
literalmente o mesmo.

### As ops que faltavam já vieram implementadas

O motor de inferência precisava de duas ops que o `faces/graph` do `era`
ainda não tinha, e das duas o levantamento por leitura de código-fonte só
achou uma -- a lista final só fechou testando o `.onnx` real do candidato
(`ch_PP-OCRv4_det_infer`, espelho `SWHL/RapidOCR` no Hugging Face, sha256
`d2a7720d...42f49da9`, conferido) e lendo as ops com o próprio parser:

| Op | Ocorrências no grafo | Para quê |
|---|---|---|
| `ConvTranspose` | 2 | as duas camadas finais de upsample aprendido do `DBHead` |
| `HardSigmoid` | 10 | ativação do backbone (estilo MobileNetV3/PP-LCNet) -- não aparecia em `db_fpn.py`/`det_db_head.py` porque vem do módulo do backbone, que não tinha sido lido |

O resto do grafo (672 nós, 14 tipos de operação) já estava coberto: `Conv`,
`BatchNormalization`, `Add`, `Mul`, `Div`, `Clip`, `Concat`, `Constant`,
`GlobalAveragePool`, `Relu`, `Resize`, `Sigmoid`.

Implementadas antes da cópia, no `era` (e trazidas juntas): `ConvTranspose2D`
com duas implementações independentes no `kernel` (uma que reúne, outra que
distribui, cross-checadas nos testes) e `HardSigmoid` na forma de
`montaClip`. `graph/` cobre 100% das ops do `ch_PP-OCRv4_det_infer`.

### Um segundo bug, que só o `.onnx` real revelou

Montar o grafo pela primeira vez, contra o modelo de verdade, achou um
problema que nenhuma leitura de código-fonte alcançaria: o exportador
`paddle2onnx` do PaddlePaddle grava **todos os pesos treinados como nós
`Constant`**, não como `initializer` -- 0 initializers, 342 `Constant` no
modelo real. `montaConstant` só registrava o valor como operação de
execução, nunca como peso disponível na montagem; todo `Conv` logo depois
de um `Constant` falhava dizendo que a entrada "precisa ser um peso
constante" -- mesmo sendo, na prática, exatamente isso. Corrigido (uma
linha: `Constant` agora também alimenta o mapa de constantes da montagem,
não só a execução), com teste de regressão em `graph_test.go`.

### O grafo monta e executa de ponta a ponta

Confirmado com este próprio código, sem `replace` para repositório nenhum:

```
Grafo montado com sucesso -- todas as ops foram reconhecidas.
Executando com entrada aleatoria [1,3,640,640]...
Execucao OK em 1.7458687s. Saida "sigmoid_0.tmp_0", forma [1 1 640 640], 409600 elementos.
Faixa [0,1] confere com a saida de um Sigmoid (mapa de probabilidade).
```

**O que isso prova:** o executor aguenta a arquitetura inteira do PP-OCRv4 --
672 nós, 14 tipos de operação, sem erro de forma nem de op. **O que isso não
prova:** que os valores saem certos. A entrada foi ruído aleatório, não uma
imagem real com o pré-processamento que o modelo espera (normalização por
média/desvio-padrão) -- por isso a saída ficou quase toda zero, o esperado
para ruído, não um sinal de acerto ou erro. Validação numérica de verdade
(contra ONNX Runtime, com imagem real, mesma normalização) é trabalho da
fase 3 propriamente dita, não deste teste de fumaça.

## Escolhas de modelo

**Detecção: DBNet, via PP-OCRv4 do PaddleOCR.** Pesquisado em 14/09/2026.
Licença confirmada na fonte primária (`LICENSE` do repositório
`PaddlePaddle/PaddleOCR`, não um agregador): **Apache License 2.0** —
diferente do caso InsightFace (pesos `buffalo_*` restritos a pesquisa), este
não tem essa restrição. O modelo pronto (`ch_PP-OCRv4_det_infer`, exportado
para `.onnx` por terceiros a partir do checkpoint oficial) está disponível
em alguns espelhos no Hugging Face; baixar e conferir a licença do espelho
em si (não só do modelo original) antes de usar — repositório de terceiro
pode reempacotar sob termo diferente.

**Reconhecimento: SVTR, não CRNN.** Contraintuitivo — o transformer parece o
caminho mais pesado. Em Go puro é o inverso:

| | CRNN + CTC | SVTR |
|---|---|---|
| Precisa de | LSTM bidirecional | matmul, softmax, layernorm, GELU |
| Custo aqui | alto — kernel sequencial novo, paraleliza mal | baixo — o `era` já roda matmul a 13 GFLOPS |
| Precisão | boa | melhor |

**Extração: regras e geometria antes de modelo.** A rota "um modelo faz
tudo" custa centenas de MB de pesos e devolve resultado não determinístico.
Para documento estruturado, o CNPJ é um padrão, a data é um padrão, e o item
da tabela é o que está alinhado na coluna. Determinístico, auditável, 0 MB.
Modelo entra depois, só onde a regra falhar.

## Tamanho honesto

OCR em Go puro é um projeto da ordem do `era/faces`. O que reduz o custo não
é otimismo, é reuso do motor de inferência (ver seção acima) — o trabalho
realmente novo é detecção, dewarp, reconhecimento e extração.

## Testes

```
go test ./...              # todos os pacotes
go test -race ./...        # exige cgo e um compilador C
go vet ./...
```

## Estado

Fases 1, 2 e 4 prontas; fases 6 e 7 parciais (ver Roteiro). Fase 3
(detecção) parcial: o motor de inferência está aqui dentro, independente,
rodando o grafo do candidato (`ch_PP-OCRv4_det_infer`) de ponta a ponta; o
pós-processamento (binariza, acha contorno, mede confiança, expande,
adapta pro formato do `dewarp`) está pronto e testado (`detect/`). Falta o
pré-processamento da imagem de entrada e o teste com imagem real, não só
com ruído aleatório confirmando a fiação.

## Licença

A definir antes da primeira release — MIT ou Apache 2.0.

Os **pesos de modelo** têm licença própria, independente deste código, e não
são distribuídos aqui. Confira a licença do modelo que for usar.
