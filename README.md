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
| 3 | `detect` | DBNet + contornos + expansão de polígono | **funcionando** — validado com imagem real contra o ONNX Runtime (99,999% de similaridade) e testado em dois documentos reais da Frota Macedo (102 e 60 regiões, ver "Testado em documento real" abaixo) |
| 4 | `dewarp` | medidor de deformação (decide N0/N1/N2/N3 a partir dos polígonos), retificação por linha de N2 — e N3 depois | **pronto** (N3 fica para quando entrar rede) |
| 5 | `recog` | SVTR + decodificação CTC, charset pt-BR | **funcionando** — pré-processamento de linha, grafo e decodificação CTC validados com linha de texto real de documento da Frota Macedo (ver "SVTR: o grafo já roda" e "Validado com duas linhas reais" abaixo) |
| 6 | `layout` | linhas, colunas, tabelas, ordem de leitura | **funcionando** — linhas e ordem de leitura de 1 coluna prontas; campo dentro de uma linha por espaçamento (`SplitCells`) validado nos dois documentos reais; colunas entre várias linhas por célula (`GroupTable`) ou por palavra (`GroupTableWords`) **implementadas e testadas**, com a tabela de itens dos documentos reais saindo em colunas reconhecíveis (1→7 colunas no PDF em alta resolução, ver "`GroupTableWords`" abaixo) |
| — | `read` | liga `detect`+`dewarp`+`recog`+`layout` numa passagem só: página inteira → linhas de texto | **funcionando** — roda de ponta a ponta nos dois documentos reais da Frota Macedo já usados nas fases 3 e 5, com texto corretamente legível depois de quatro bugs de integração achados e corrigidos (ver "`read`: a página inteira", "Terceiro bug" e "Quarto bug" abaixo); resta um defeito conhecido, sem relação com os quatro (espaço perdido dentro de uma região que o detector marcou como uma peça só) |
| 7 | `extract` | campos tipados por tipo de documento | parcial — CNPJ, CPF, data e valor monetário **prontos**; `read.ExtrairCampos` (achados livres) e `read.ExtrairDAV` (schema da DAV: emitente, destinatário, número, data, totais, itens em coluna via `GroupTableWords`) ligam isso à saída de `read.Page`, validados no PDF em alta resolução com emitente/destinatário/número/data/total corretos e itens em 7 colunas reconhecíveis (ver "`ExtrairCampos` e `ExtrairDAV`" abaixo); falta schema para outros tipos de documento |

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

`ToLinePolygon` faz a ponte entre o contorno bruto (um vértice por pixel de
borda, formato genérico) e o polígono que `dewarp.ExtractBaseline` espera
(metade dos vértices formando a borda de cima esquerda→direita, a outra
metade a de baixo direita→esquerda).

Do lado de entrada, `detect.Preprocess` redimensiona (lado maior no limite
de 960px, os dois eixos arredondados para múltiplo de 32 — o backbone
reduz e depois amplia por esse fator, e um tamanho que não feche exato
acumula erro de forma a cada estágio) e normaliza. O detalhe que mais
importava acertar: conferido no código-fonte do PaddleOCR, a imagem nunca
é convertida de BGR para RGB — carrega direto via `cv2.imread` (BGR) e
segue assim até a rede. Errar essa ordem não quebra nada visivelmente (a
rede roda, produz saída de forma certa), só detecta pior — o tipo de erro
que só uma imagem real revelaria, não um teste sintético.

Com pré e pós-processamento prontos, a fase 3 está com todas as peças
escritas e testadas de ponta a ponta contra o `.onnx` real — o que falta é
**imagem real**: até aqui só ruído aleatório confirmou que a fiação inteira
funciona sem erro de forma; se os valores saem certos só uma foto de
verdade (e o pré-processamento comparado byte a byte contra a referência
Python) vai provar.

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

As fases 1, 2, 4 e 6, e o pré/pós-processamento da fase 3, são geometria e
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
672 nós, 14 tipos de operação, sem erro de forma nem de op. Com ruído
aleatório isso não provava que os valores saem certos -- a validação de
verdade veio depois, com imagem real (ver abaixo).

### Validado contra o ONNX Runtime, com imagem real -- 14/09/2026

Peguei a imagem de demonstração oficial do PaddleOCR/PaddleX
(`general_ocr_002.png`, um cartão de embarque fotografado, texto em duas
línguas) e comparei a saída deste código contra o ONNX Runtime rodando o
mesmo `.onnx`, com o mesmo pré-processamento reproduzido em Python.

**Primeira rodada, com a imagem em JPEG: só 99,96% de similaridade de
cosseno** -- perto, mas não o suficiente pra confiar. Isolando cada etapa
(comparei o tensor de entrada antes da rede, depois só o canal de cor antes
de redimensionar, depois os pixels crus antes de qualquer conta) achei que
a diferença já existia nos PIXELS DECODIFICADOS do JPEG -- antes de tocar
em uma linha de código deste repositório. Decodificador de JPEG não é
determinístico entre bibliotecas (o IDCT e o upsampling de crominância
variam) -- `image/jpeg` do Go e o libjpeg do OpenCV simplesmente não
concordam bit a bit no mesmo arquivo. Convertendo a mesma imagem para PNG
(sem perdas, decodificação determinística) e repetindo a comparação:
**99,999% de similaridade de cosseno**, diferença média de 0,00007. O
resíduo que sobra é o esperado entre duas implementações numéricas
independentes (Go e ONNX Runtime) acumulando arredondamento de ponto
flutuante em ordem diferente ao longo de 672 nós -- não dá pra eliminar, e
não é o mesmo tipo de erro que uma imagem real revelou de verdade (ver
abaixo).

**Essa mesma investigação achou um bug real:** `528 ÷ 32 = 16,5` exato --
um empate. O `round()` do Python (usado pelo PaddleOCR de verdade) desempata
para o par mais próximo (16); `math.Round` do Go desempata sempre pra cima
(17). Só uma imagem real bateu numa razão exatamente em 0,5 -- nenhum caso
sintético dos testes tinha pego isso por acaso. Corrigido com uma função
que replica o desempate do Python (`arredondarParaParPython`), com teste
dedicado e o caso exato que revelou o bug.

Com a correção, rodei o detector completo (`Preprocess` → rede →
`Detect` → `Rescale`) na mesma imagem: **33 regiões de texto encontradas**,
a maioria com confiança acima de 0,98, caindo visivelmente em cima do texto
de verdade -- cabeçalho bilíngue, campos, código de barras, até texto em
cima de uma mancha na foto.

### Testado em documento real da Frota Macedo -- 14/09/2026

Depois da validação numérica contra o ONNX Runtime (acima, com a imagem de
vitrine do PaddleOCR), o passo seguinte foi um documento de verdade da
empresa: uma "Documento Auxiliar de Venda - Pedido" (DAV).

**Teste 1 -- PDF em alta resolução.** Render de `CCF08092026.pdf` via
`pdftoppm -r 200` (1606×2286 px). Rodando o detector completo: **102
regiões de texto encontradas**, cobrindo corretamente cabeçalho, campos,
tabela de itens e totais -- inclusive **excluindo** de forma correta uma
anotação escrita à mão que aparecia sobre o documento (fora de escopo desta
fase, e o detector não confundiu uma coisa com a outra).

**Teste 2 -- print de tela, resolução mais baixa.** O usuário enviou uma
captura de tela de um visualizador (`Relatório SysPDV`) mostrando um
documento parecido, com a moldura do sistema (barra de título, barra de
ferramentas, barra de tarefas do Windows) em volta. Por instrução explícita
do usuário, a página do documento foi recortada da moldura antes do teste
(`nota_whatsapp_crop.png`, 765×803 -- localizado por transição de
brilho nas bordas da página branca, sem retocar o conteúdo). Rodando o
mesmo detector nessa imagem: **60 regiões de texto encontradas**, a maioria
com confiança acima de 0,97.

Uma auditoria visual do resultado achou uma falha real: a descrição de um
item de tabela ("SERVICO DE ENTREGA") ficou sem caixa, enquanto os valores
da mesma linha (unidade, quantidade, preço) foram detectados normalmente --
confirmado ampliando a região em 2× (`zoom_item2.png`). A explicação mais
provável, e a única com evidência a favor, é resolução: o print tem
765×803 contra 1606×2286 do PDF, pouco mais da metade em cada eixo, o que
deixa o traço de uma descrição em fonte pequena mais perto do limiar de
binarização do `detect.Binarize`. Não é erro de pipeline -- o
pré-processamento já está validado byte a byte contra a referência Python
(seção acima) -- é uma limitação real de imagem de entrada pobre, registrada
em vez de escondida.

**O que isso prova, e o que não prova:** o pipeline de detecção funciona em
documento real da empresa, não só na imagem de vitrine, e em duas condições
de captura diferentes (PDF gerado por software vs. foto de tela). Não prova
nada sobre reconhecimento de texto (fase 5, ainda não implementada) -- o que
sai daqui são só posições de texto, não o texto em si.

## SVTR: o grafo já roda -- 14/09/2026

Primeiro passo da fase 5: escolher o modelo de reconhecimento e fazer o
grafo dele montar e executar contra o `.onnx` real, do jeito que a fase 3
fez com o detector -- ops descobertas testando o modelo de verdade, não só
lendo código-fonte.

### Por que não é o mesmo modelo da família do detector

`ch_PP-OCRv4_det` (o detector, já em uso) é praticamente cego a idioma: ele
acha "aqui tem texto", não decodifica caractere nenhum. Quem precisa bater
com o idioma é o *reconhecedor*, porque ele decodifica contra um dicionário
de caracteres fixo -- e o `ch_PP-OCRv4_rec` "irmão" do detector tem
dicionário **chinês**, sem garantia de cobrir acento português (ç, ã, õ),
que aparece direto em nota fiscal ("SERVIÇO", "ENDEREÇO").

Escolhido em vez disso: **`latin_PP-OCRv3_mobile_rec`**, o modelo que o
próprio PaddleOCR usa para francês, espanhol, italiano, português etc. --
mesma arquitetura (SVTR_LCNet + PP-LCNetV3), dicionário diferente
(`latin_dict.txt`, 186 caracteres, conferido na fonte primária do
PaddleOCR no GitHub: cobre à á â ã ç é ê í ó ô õ ú ü, o alfabeto português
inteiro). É exatamente a arquitetura multilíngue oficial do PaddleOCR: um
detector compartilhado entre idiomas, um reconhecedor por idioma.

### Licença: uma divergência entre o checkpoint oficial e o espelho ONNX

O checkpoint original, `PaddlePaddle/latin_PP-OCRv3_mobile_rec` no Hugging
Face (organização oficial do PaddlePaddle) declara **Apache 2.0** -- mas
esse repositório só tem o formato de inferência do Paddle
(`inference.json` + `.pdiparams`), não `.onnx`. O `.onnx` convertido veio de
um espelho de terceiro, `docato/PaddleOCR_Mobile_Models`
(via `paddle2onnx`), cuja própria página declara **MIT**. As duas licenças
não são incompatíveis na prática (MIT é mais permissiva que Apache 2.0), mas
a divergência não foi resolvida -- fica registrada para quem for usar isto
em produção conferir qual das duas vale, em vez de presumir.

### As ops que faltavam, achadas testando o `.onnx` real

Do mesmo jeito que a fase 3: inspecionado com o próprio parser da ERA, não
por leitura de código-fonte. 535 nós, 25 tipos de operação -- 6 não
existiam ainda:

| Op | Ocorrências | Para quê |
|---|---|---|
| `HardSwish` | 27 | ativação do backbone PP-LCNetV3 (`x * HardSigmoid(x)`, alpha/beta fixos) |
| `ReduceMean` | 10 | media -> subtrai -> `Pow` -> media -> `Sqrt` -> `Div`: normalização decomposta manualmente, sem `LayerNormalization` |
| `Pow` | 5 | a mesma decomposição acima (eleva ao quadrado antes de tirar a média) |
| `Sqrt` | 5 | idem |
| `Shape` | 4 | forma da própria entrada, para calcular reshape em tempo de execução |
| `Slice` | 8 | recorte por eixo com índice calculado (dinâmico, não fixo) |

Implementadas em `graph/elementwise.go` (`HardSwish`, `Sqrt`, `Pow` como
mais um caso de `montaBinario`), `graph/reduce.go` (`ReduceMean`) e
`graph/slice.go` (`Shape` foi para `shape.go`, ao lado de `Reshape`; `Slice`
ganhou arquivo próprio por ser mais envolvido: início/fim/passo por eixo,
índice negativo, passo negativo).

### Dois bugs reais, achados só ao montar o grafo de verdade

Nenhum dos dois apareceu na fase 3 porque o detector é uma rede
convolucional simples -- os dois só existem porque um transformer (SVTR)
aplica reshape e multiplicação de matriz de um jeito que uma CNN não usa.

**1. `Reshape` só aceitava forma constante.** Um exportador que suporte
largura variável (uma linha de texto recortada, cujo comprimento em pixels
depende de quantos caracteres tem) calcula a forma alvo do `Reshape` **em
execução**, via `Shape` sobre a própria entrada -- não como um número fixo
gravado na montagem. `montaReshape` exigia que a forma já estivesse
resolvida como peso constante na montagem (`b.peso`), o que bastava para o
detector (formas sempre fixas) e falhava aqui dizendo que a entrada
"precisa ser um peso constante", mesmo vindo de uma conta legítima sobre o
próprio tensor. Corrigido lendo a forma do tensor em **execução**
(`ins[1]`), não mais na montagem -- generalização, não gambiarra: continua
funcionando para o caso antigo (forma fixa), e passa a funcionar para o
novo.

**2. `MatMul` de peso fixo não desfazia o achatamento.** O caminho rápido de
`MatMul` contra peso constante (vira `nn.Linear`) precisa de entrada 2D;
para uma entrada 3D `[N,T,Cin]` (uma sequência inteira, o caso comum em
bloco de atenção), ele achatava para `[N*T,Cin]`, multiplicava, e devolvia
o resultado **achatado**, sem desfazer. A soma residual logo depois (comum
em bloco de atenção: `x + Atencao(x)`) quebrava comparando `[N,T,Cout]` com
`[N*T,Cout]`. Corrigido devolvendo ao formato original (`[N,T,Cout]`) antes
de sair da operação.

Junto, ainda foi preciso um `MatMul` genérico de verdade
(`graph/matmul.go`, `matmulGenerico`) para o caso em que **nenhum** dos dois
lados é peso -- a auto-atenção calcula `Q @ Kᵀ`, e os dois só existem depois
que a imagem chega. Trata lote e transmissão de forma como `Add`/`Mul` já
tratam, só que para as duas últimas dimensões virarem multiplicação de
matriz de verdade.

### O grafo bate com o ONNX Runtime

Mesma entrada fixa (semente aleatória do NumPy, reproduzível), rodada nos
dois lados:

```
entrada [2,3,48,320] (lote de 2, a arquitetura aceita lote e largura
dinâmicos -- testado nos dois eixos)
saída ONNX Runtime: [2,40,187], soma 80,0 (softmax por posição, 2×40
posições)
saída ERA READ:     [2,40,187]
diferença média: 3,2×10⁻⁹  |  diferença máxima: 8,3×10⁻⁷
similaridade de cosseno: 1,0000001
```

Diferença de ponto flutuante entre duas implementações independentes, não
divergência de lógica -- ordens de grandeza menor que a diferença de
JPEG-vs-PNG que a fase 3 encontrou (99,999%), porque aqui a entrada não
passou por nenhum decodificador de imagem com lugar para variar.

### `recog`: pré-processamento de linha e decodificação CTC -- 14/09/2026

Com o grafo validado, faltavam as duas pontas: preparar a linha de texto
recortada antes de entrar na rede, e traduzir a saída dela de volta para
texto. As duas viraram o pacote `recog`.

**Pré-processamento (`recog.Preprocess`).** Replica `resize_norm_img` de
`tools/infer/predict_rec.py` -- a função que roda de verdade para o
algoritmo `"SVTR_LCNet"` (a família PP-OCRv3/v4, o caso deste modelo).
Existe **outra** função no mesmo arquivo com nome quase igual,
`resize_norm_img_svtr`, mas ela só roda para uma lista diferente de
algoritmos (`"SVTR"`, `"SATRN"`, `"ParseQ"`, `"CPPD"`) -- as duas foram
conferidas lado a lado na fonte para não trocar uma pela outra, o tipo de
detalhe que só aparece lendo o dispatch inteiro, não só a assinatura da
função. Algoritmo: redimensiona para a altura fixa (48px) preservando a
proporção; a largura alvo é `max(320/48, proporção_da_linha) × 48`
arredondado -- 320 é o padrão do modelo, mas uma linha proporcionalmente
mais larga cresce além disso; preenche com zero a área que sobra à
direita. Normalização `(pixel/255 - 0,5) / 0,5` igual nos três canais --
diferente da detecção, que usa média/desvio do ImageNet por canal. Ordem
de canal continua BGR sem conversão, pela mesma razão já documentada em
`detect.Preprocess`.

**Decodificação (`recog.DecodeCTC`).** Replica `CTCLabelDecode.decode` de
`ppocr/postprocess/rec_postprocess.py`: por posição de tempo, pega o
índice de maior probabilidade; colapsa repetição consecutiva do MESMO
índice cru (inclusive quando o índice repetido é o branco); só depois
descarta o branco. A ordem importa -- é o que separa duas letras iguais
seguidas (dois picos do mesmo índice) de letra-branco-letra (a mesma letra
duas vezes, de propósito). `recog.Charset` mapeia índice para caractere;
a ERA não empacota dicionário nenhum (mesmo motivo de não empacotar peso
de modelo) -- quem usa a biblioteca lê o arquivo de dicionário do modelo e
monta o `Charset`.

### Um dicionário incompleto, achado testando linha de texto real

`latin_dict.txt` (fonte primária do PaddleOCR) tem 185 linhas. Com o token
em branco, `1 + 185 = 186` -- mas a rede tem **187** classes de saída. A
diferença só apareceu rodando texto de verdade (não os testes sintéticos
do `graph`, que não têm dicionário nenhum para conferir): o texto
decodificado saía com os índices deslocados, ilegível.

Causa: o carregador de dicionário do PaddleOCR
(`BaseRecLabelDecode.__init__`) tem uma opção `use_space_char` que, quando
ligada, **acrescenta** um caractere de espaço ao fim da lista -- mesmo o
arquivo já tendo um espaço na primeira linha. O modelo escolhido foi
treinado com essa opção ligada: o dicionário certo é as 185 linhas do
arquivo **mais um espaço no fim**, dois espaços no total (índices
diferentes, mesmo caractere). `1 (branco) + 185 + 1 (espaço extra) = 187`,
batendo exato. Não tem como saber isso só lendo o arquivo do dicionário --
só bate contando as classes da rede.

### Validado com duas linhas reais de um documento da Frota Macedo

Duas linhas recortadas à mão de `nota_whatsapp_crop.png` (o mesmo
documento da fase 3), rodadas pelo pipeline completo
(`recog.Preprocess` → grafo → `recog.DecodeCTC`) e comparadas com a mesma
referência em Python (OpenCV + ONNX Runtime + a mesma regra de
decodificação):

| Linha | Saída da ERA READ | Confiança | Referência Python |
|---|---|---|---|
| `CPF/CNPJ:  27363223000170` | `"CPF/CNPJ: 27363223000170"` | 0,9434 | idêntico, 0,9434605 |
| `Nº do Documento:  0000018355` | `"No do Documento:000018355"` | 0,8387 | idêntico, 0,8394767 |

A primeira linha saiu perfeita. A segunda tem dois erros reais do modelo,
não do código: perdeu o `º` (ordinal) e um dígito de `0000018355`
(virou `000018355`) -- o crop dessa linha é menor e a fonte mais
apertada, no mesmo tipo de limite de resolução que a fase 3 já tinha
documentado para detecção. `recog` não tem como corrigir um erro de
reconhecimento do modelo; só reporta o que ele decodificou.

Comparando o TENSOR de entrada (não só o texto final) contra a mesma
referência: diferença média ~0,0009, máxima ~0,006, numa faixa de
valores em [-1,1] -- residual pequeno mas real, atribuível a
`imgproc.Resize` (interpolação bilinear com centro de pixel) e
`cv2.resize` do OpenCV não serem bit-a-bit idênticos, o mesmo tipo de
diferença que a fase 3 já tinha isolado para o redimensionamento da
detecção. Não impediu o texto de sair certo na linha 1 nem de errar nos
mesmos dois lugares que a referência erraria com a mesma imagem de baixa
resolução -- é ruído de implementação, não divergência de lógica.

**O que a fase 5 ainda não faz sozinha:** decidir onde cortar uma linha
dentro da página, e processar lote de mais de uma linha de uma vez (cada
`Preprocess` produz sua própria largura; lotear linhas de larguras
diferentes exige um preenchimento comum, que ainda não foi escrito porque
nenhum caso de uso pediu velocidade de lote ainda). A primeira metade --
onde cortar -- é o que o pacote `read` resolve, ligando `detect` a
`recog`, a seguir.

## `read`: a página inteira, de ponta a ponta -- 14/09/2026

Com detecção, retificação e reconhecimento cada um funcionando sozinho,
faltava a cola: `read.Page` recebe uma imagem de página inteira e os dois
grafos (detecção e reconhecimento), e devolve linhas de texto já na ordem
de leitura --

```
imagem → detect.Preprocess → grafo de deteccao → detect.Detect
       → para cada regiao: mede a deformacao (dewarp) → retifica
         (homografia se for reta, curva se for N2) → recog.Preprocess
         → grafo de reconhecimento → recog.DecodeCTC
       → layout.GroupLines
```

`ProcessarRegioes` é a metade de baixo desse desenho -- tudo depois da
detecção -- separada para poder testar sem precisar de uma rede de
detecção de verdade: os testes constroem o mapa de probabilidade a mão
(um retângulo com as pontas arredondadas, a forma que uma detecção real
tem depois do `Unclip`) e usam uma rede de reconhecimento de brinquedo
(três camadas, sem pretensão nenhuma de ler caractere de verdade) só para
confirmar que a fiação -- retificar, rodar a rede, decodificar, agrupar --
está correta. A validação de que o reconhecimento *de verdade* funciona já
existe contra o `.onnx` real (fases 3 e 5, acima); aqui o que está sob
teste é a cola, não os modelos.

### Um bug real, achado só ao ligar as duas metades

Testar a fiação com uma detecção sintética -- mas geometricamente
realista -- revelou um bug que nenhuma das duas fases sozinha tinha
como expor: toda região reta que passava por `detect.ToLinePolygon` e
depois por `dewarp.ExtractBaseline`/`MeasureLine` saía classificada como
N3 (curvatura irregular), mesmo sendo perfeitamente reta.

Causa: `ToLinePolygon` divide o contorno denso de uma região em duas
metades ("cima" e "baixo") cortando o mesmo laço em dois arcos que
**começam e terminam exatamente no mesmo par de vértices** -- o ponto mais
à esquerda e o mais à direita do contorno inteiro. Para uma região mais
larga que alta (o caso normal), esses dois vértices compartilhados ficam
perto do **meio da altura** da região, não em cima nem embaixo -- é ali
que ela é mais larga. `ExtractBaseline` pegava esses dois pontos junto
com o resto da borda de baixo, e `MeasureLine` via um salto artificial de
quase a altura inteira da região nas duas pontas de toda baseline, mesmo
de uma linha reta. Uma imagem sintética com uma única detecção não
pegaria isso por acaso -- só apareceu com uma detecção real (via
`detect.Detect`, não um quadrilátero desenhado à mão) processada pelo
resto do pipeline de verdade.

Corrigido em `dewarp.ExtractBaseline`: quando a borda de cima e a de baixo
compartilham os vértices das pontas (a assinatura desse corte em dois
arcos), os dois são descartados antes de virar baseline -- só para medir a
deformação. Um quadrilátero simples de 4 vértices (sem esse
compartilhamento) não é afetado; a checagem só dispara para o caso de
contorno denso. Testes antigos de `dewarp` continuam passando sem mudança;
o teste novo que capturou o bug (`TestProcessarRegioesFiacaoCompleta`) só
passou depois do conserto.

### Segundo bug, achado só ao ler um documento de verdade de ponta a ponta

Com o primeiro conserto, a página inteira já classificava certo (N0), mas
o texto lido saía cortado no início ou no fim -- `"AENTOAUXILIARDEVENDA-F"`
em vez de `"DOCUMENTO AUXILIAR DE VENDA-PEDIDO"`. Um teste sintético com
uma região só não pegaria isso: o recorte saía com a FORMA certa (a
proporção largura/altura não parecia errada), só que amostrando o pedaço
errado da imagem original.

Causa: para retificar por homografia (N0/N1), o código pegava os 4
"cantos" do jeito mais óbvio -- os mesmos pontos que sobram depois de
descartar as pontas contaminadas do primeiro bug. Mas esses pontos que
sobram já não são as bordas verdadeiras da região: ficam alguns pixels
para DENTRO da extensão real (a reamostragem de `ToLinePolygon` anda por
índice, não por posição, então o primeiro ponto que sobra depois de
descartar a ponta contaminada já avançou uma fração real do contorno).
Usar esses pontos como canto da homografia mapeava o retângulo de saída
inteiro para essa faixa mais estreita -- esticando-a para preencher o
recorte, e cortando fora o pedaço de fora dela. Os pontos são ótimos para
medir a deformação (o que motivou descartá-los no primeiro bug) e ruins
para definir a área do recorte -- dois usos diferentes do mesmo dado, que
o código tratava como se fossem o mesmo problema.

Corrigido usando o retângulo envolvente da região inteira (`Polygon.Bounds`)
como cantos do recorte, em vez dos pontos "de cima/baixo" -- nunca perde
conteúdo, ao custo de não corrigir a perspectiva de uma linha N1 com
precisão cirúrgica (o recorte de uma linha tortinha inclui uma margem de
fundo a mais nos cantos, em vez de desentortar exatamente). `recog` tolera
bem essa margem extra; não tolera texto cortado.

### Rodado nos dois documentos reais da Frota Macedo

Com os dois modelos reais (`ch_PP-OCRv4_det` + `latin_PP-OCRv3_mobile_rec`)
e o mesmo dicionário da fase 5, `read.Page` nos dois documentos já usados
nas fases 3 e 5, depois dos dois consertos acima:

| Documento | Linhas lidas | Amostra |
|---|---|---|
| PDF em alta resolução (`CCF08092026.pdf`) | 28 | `"DOCUMENTO AUXILIAR DE VENDA - PEDIDO"`, `"Nome: FROTA MACEDO ENGENHARIA EIRELI (000000000000035) CPF/CNPJ: 27363223000170"` |
| Print de tela (`nota_whatsapp_crop.png`) | 22 | `"DOCUMENTO AUXILIAR DE VENDA-PEDIDO"`, `"N° do Documento:0000018355"` |

Cabeçalho, razão social, CNPJ, endereço e a maioria dos itens da tabela
saem certos ou quase certos (erros pontuais de caractere -- "Razäo" em vez
de "Razão", "FR0TA" com zero em vez de "O" -- do reconhecedor, não do
recorte). Um defeito continua conhecido e não escondido:

- **Espaço entre palavras se perde dentro de uma região.** Quando o
  detector marca um campo inteiro como UMA região em vez de uma por
  palavra (comum em campos curtos, como `"Nome:FR0TA MACEDO..."` sem
  espaço depois de "Nome:"), o reconhecedor devolve tudo colado -- ele não
  foi treinado para inserir espaço onde não há um caractere de espaço de
  verdade visível. `layout.GroupLines` já junta várias regiões com espaço
  quando elas chegam separadas; não tem como inserir o que nunca existiu
  como região distinta. A linha do item "SERVICO DE ENTREGA" (a mesma que
  a fase 3 já tinha marcado como limite de resolução) continua saindo
  ilegível pelo mesmo motivo de sempre -- resolução baixa na fonte.

**O que isso prova, e o que não prova:** as quatro fases já prontas
(detecção, retificação, reconhecimento, layout) se conectam e produzem
texto **corretamente legível** de um documento real de ponta a ponta -- o
marco que o README já registrava como "o marco real" da fase 5. Não prova
qualidade de produção: o defeito de espaço acima é conhecido, e erros
pontuais de caractere em campos de baixa resolução continuam acontecendo,
os dois documentados em vez de escondidos.

## `SplitCells`: separar campo por espaçamento -- 14/09/2026

`layout.GroupLines` junta palavras na mesma faixa vertical numa `Line` só
-- e não sabe separar duas colunas lado a lado que caem na mesma altura
(um rótulo e o valor numa tabela, por exemplo). `SplitCells` corta essa
`Line` de volta em `Cell`s por espaçamento horizontal: um vão entre
palavras maior que `DefaultGapFactor` (3×) a altura da própria linha
conta como fronteira de coluna -- relativo à altura, não um número fixo de
pixels, porque o mesmo documento em outra resolução muda todo espaçamento
em pixels na mesma proporção que muda a altura da fonte.

Rodado nos dois documentos reais da Frota Macedo (a saída de `read.Page`,
depois dos consertos acima), separa limpo o caso comum -- código do
produto de um lado, quantidade/preço do outro; rótulo de endereço de um
lado, telefone do outro:

```
"00000000000442 - SERRA STARRETT 1218 1,000"
  -> "00000000000442 - SERRA STARRETT 1218"  |  "1,000"

"Fale Conosco: 85987111007 Dt. Emis: 18/08/2026"
  -> "Fale Conosco: 85987111007"  |  "Dt. Emis: 18/08/2026"
```

Onde não ajuda: linhas onde o próprio reconhecimento já saiu embaralhado
(a área de totais, `"OEOPOOAC 100,60 Valor Produtos: 100,60"`) continuam
embaralhadas depois de cortadas em células -- `SplitCells` separa por
geometria, não conserta caractere errado.

## Terceiro bug: campo pequeno virava N3 -- 14/09/2026

Investigando por que `"Total a pagar:"` nunca vinha com o valor ao lado
(o texto reconhecido trazia só o rótulo, a linha toda), o rastro levou a
`dewarp.Classify`, não a `layout`: as duas regiões com o valor
(`"100,60"` e `"+0,00"`, ~30px de altura) **foram detectadas** com
confiança alta (0,996 e 0,999) -- só que classificadas **N3** (curvatura
irregular), que `read.reconhecerRegiao` descarta inteiro por não ter
implementação ainda.

Causa: `dewarp.Thresholds.RetoPx`/`CurvoPx` são limiares em **pixel
absoluto** (1,5px, "ponto de partida", nunca calibrado contra documento
real). Um campo curto (`"100,60"`, 6 caracteres) tem ruído de contorno da
mesma ordem de grandeza ABSOLUTA que uma linha de texto inteira -- alguns
pixels, do jeito que `Unclip` e a rasterização do detector sempre deixam
-- mas esses mesmos pixels são uma fração bem maior da altura de um campo
pequeno. As duas regiões reais mediram `LinearBow` de 7,09px e 4,03px,
folgadamente acima do limiar fixo de 1,5px.

Corrigido em `read.Options.FatorDeformacaoRelativo` (padrão 0,3): antes de
classificar, o limiar vira `max(limiar absoluto, altura da região × 0,3)`
-- um campo de 30px de altura ganha ~9px de folga, uma linha de texto
normal (bem mais alta) ganha proporcionalmente mais, sem nunca ficar mais
apertado que o padrão absoluto. `dewarp.Thresholds` em si não mudou --
o ajuste mora na camada de integração (`read`), que é quem sabe o
tamanho de cada região.

Não é calibração medida contra curvatura de verdade: os dois documentos
reais usados aqui são de papel liso, nenhum tem dobra ou curvatura de
fato para testar se o limiar mais largo deixa passar uma curva que devia
ser N3. Ponto de partida, como o próprio `dewarp.DefaultThresholds` já se
declarava -- agora com um caso real medido por trás do número, não só um
palpite.

## Quarto bug: um item físico virava várias `Line`s -- 14/09/2026

Investigado a pedido explícito, como causa raiz de um sintoma já
registrado (`GroupTable` fundindo colunas erradas nos itens da tabela):
por que o código+descrição de um item e a grade de quantidade/preço ao
lado saem em `Line`s diferentes, se visualmente é a mesma linha impressa
-- **sem relação** com o defeito de "espaço perdido dentro de uma região"
já documentado (aquele é uma região só que o reconhecedor não separou em
palavras; este é mais de uma região que `layout` não juntou numa linha).

Medido com as posições reais de um item (`nota_real-1.png`):

```
"...TIGRE/KR" (descrição)  x0=64    y=461..494  (altura 33)
"UN"                       x0=666   y=467..498  (altura 31)
"Valor Total"              x0=1426  y=447..477  (altura 30)
```

`GroupLines` ordena por Y e ancora cada `Line` na PRIMEIRA palavra, sem
nunca atualizar a âncora -- por desenho, para não deixar uma linha
"derivar" verticalmente e engolir a de baixo. Por ordem de Y, "Valor
Total" (447) vira âncora. A descrição bate 53% de sobreposição contra
essa âncora -- entra. Mas "UN" bate só 33% contra a âncora ORIGINAL
"Valor Total" (embora bata 87% contra a descrição, que já tinha
entrado) -- like a âncora nunca muda, "UN" fica de fora e abre uma
`Line` nova. Um problema de transitividade: A sobrepõe B, B sobreporia
C, mas A não sobrepõe C, e só A conta.

Corrigido em duas partes, as duas testadas com os números reais acima:

1. **A faixa de comparação passa a crescer** conforme cada palavra nova
   entra (até um teto de `MaxDriftFactor`, 2× a altura da âncora original)
   -- resolve a transitividade sem reabrir o risco original de derivar
   sem limite (`TestGroupLinesNaoDerivaSemLimite` prova o teto segurando
   uma cadeia longa).
2. Só isso **causou um novo problema**, medido rodando contra o
   documento real antes de declarar pronto: duas linhas de ITENS
   DIFERENTES, empilhadas de perto (pouco espaço entre elas, comum numa
   tabela), têm sobreposição vertical tão alta quanto o caso de cima --
   geometricamente quase idêntico. `"PANO DE CHAO"` e `"SERRA STARRETT"`
   (dois produtos diferentes) se fundiram numa `Line` só. O que distingue
   os dois casos: palavras da MESMA linha impressa ocupam colunas
   diferentes (faixas de X que não se sobrepõem); linhas empilhadas
   repetem a coluna (o código do próximo item começa na mesma posição X
   do anterior). Corrigido exigindo que a palavra candidata não dispute a
   faixa de X de nenhuma palavra já aceita na linha
   (`TestGroupLinesNaoFundeLinhasEmpilhadas`).

Resultado nos dois documentos reais: itens que antes saíam partidos em 2-3
`Line`s (código+descrição numa, quantidade/preço/desconto noutra) agora
saem inteiros numa `Line` só -- `"00000000002588 - BORRACHA DE SILICONE
INCO SES 2,000 19,90 0,00% 0,00 0,00 % 0,00 39,80"`, por exemplo, onde
antes a parte numérica ficava numa `Line` separada. Ainda sobram alguns
fragmentos residuais soltos (uma célula "0,00 15,90" que não juntou com o
resto do seu item) -- melhora medida, não perfeição alcançada.

## `GroupTable`: colunas entre linhas -- 14/09/2026

`SplitCells` corta uma linha em campos; `layout.GroupTable` tenta ir além,
alinhando células de VÁRIAS linhas na mesma coluna -- juntando os
intervalos horizontais que se sobrepõem entre linhas, num só passo
(célula de uma linha estende a coluna aberta mais recente se a borda
esquerda cair dentro dela; senão abre coluna nova). Testado com grades
sintéticas limpas: linhas com o mesmo número de células nas mesmas
posições viram colunas certas; uma linha com menos células que as outras
deixa a célula que falta vazia, não deslocada; uma coluna cuja largura
varia um pouco de linha para linha (texto mais longo numa, mais curto
noutra) continua alinhada, desde que as faixas se sobreponham.

Rodado nos itens da tabela dos dois documentos reais, depois do quarto
bug corrigido (o item físico agora forma uma `Line` só na maioria dos
casos): **continua não segmentando em colunas** -- mas por um motivo
NOVO e diferente do que causava isso antes. Antes, uma célula
desproporcionalmente larga (código+descrição) fundia duas colunas de
`GroupTable` numa só. Agora que a `Line` do item tem muitas palavras
(descrição, unidade, quantidade, preço, descontos, total, tudo numa linha
só), `SplitCells` não acha nenhum vão isolado grande o bastante entre
elas -- os vãos entre campos vizinhos (unidade→quantidade,
quantidade→preço) são todos parecidos em tamanho, nenhum se destaca como
fronteira de coluna -- e a linha inteira vira UMA célula só, sem
fronteira nenhuma para `GroupTable` alinhar.

### `GroupTableWords`: alinhar por palavra, não por célula

A saída de `SplitCells` não é a única forma de decidir "célula" -- cada
campo de uma linha de item (a unidade, a quantidade, o preço) já chegou
como uma REGIÃO DETECTADA separada, com posição própria, muito antes de
`SplitCells` entrar em cena. `layout.GroupTableWords` pula `SplitCells`
de vez e trata cada `Word` da linha como sua própria célula, alinhando
por posição X do mesmo jeito que `GroupTable` -- só muda o que conta como
"célula" de entrada.

Rodado nos itens dos dois documentos reais (com o mesmo `GroupTable` só
trocado por `GroupTableWords` em `read.ExtrairDAV`): a tabela de itens do
PDF em alta resolução passou de **1 coluna** para **7**, com descrição,
unidade, quantidade e valores razoavelmente alinhados entre os itens:

```
item 0: [00000000001210 - CAP ESG PVC 40MM TIGRE/KR | UN | 1,000 | 3,50 | 0,00 |     | Valor Total]
item 4: [00000000007105 - PANO DE CHAO COLORIDO      | UN | 2,000 | 4,00 | 0,00 | 0,00 % | 0,00]
item 6: [00000000000174 - MASSA CORRIDA BRANCO 1,5K  | UN | 1,000 | 0,00 % | 0,00 | 0,00 % | 13,90]
```

Não é perfeito: algumas linhas têm célula vazia onde uma vizinha tem
valor (a coluna "preço unitário" de `item 2`, por exemplo, ficou sem
nada, porque aquela palavra específica não caiu na mesma faixa de X das
outras linhas) -- o alinhamento depende de a posição X de cada campo
repetir de linha para linha com fidelidade suficiente, e ruído de
detecção quebra isso ocasionalmente. Ainda assim, é a primeira vez que a
tabela de itens sai com estrutura de coluna reconhecível, não um texto
corrido só.

**O que isso prova, e o que não prova:** para as linhas de item já
corrigidas pelo quarto bug, `GroupTableWords` consegue reconstituir
colunas de verdade na maioria dos casos, medido nos dois documentos
reais -- fechando, na prática, boa parte do que faltava na fase 6. Não
prova alinhamento perfeito nem uma API definitiva (`GroupTable` via
`SplitCells` continua certa para texto corrido com dois campos lado a
lado, tipo rótulo+valor; `GroupTableWords` é para quando o bloco de
linhas já se sabe ser tabela) -- células vazias onde deveria ter valor
mostram que o alinhamento por posição ainda erra quando a detecção em si
não é perfeitamente consistente entre linhas.

## `ExtrairCampos` e `ExtrairDAV`: do texto lido ao dado tipado -- 14/09/2026

Com `read.Page` produzindo texto legível, faltava a última cola: rodar
`extract` (fase 7, já pronta) em cima da saída, sem precisar saber a
estrutura do documento. `read.ExtrairCampos` varre as linhas e devolve:

- **CNPJ e CPF**: `extract.FindCNPJs`/`FindCPFs` direto no texto de cada
  linha -- confiança alta, porque o dígito verificador decide sozinho se
  14 (ou 11) dígitos são coincidência ou CNPJ (CPF) de verdade.
- **Data**: `extract.ParseDateBR` tentado em cada linha -- sem dígito
  verificador, então só entra quando o formato bate exato (`DD/MM/AAAA`
  ou por extenso), o que já filtra a maioria do ruído.
- **Valor monetário**: só perto de uma etiqueta conhecida (`EtiquetasValor`
  -- "Total a pagar", "Valor Produtos", "Total Bruto Produtos"), porque
  `extract.ParseMoney` explicitamente recusa ser buscador de texto livre
  (colidiria com quantidade, número de nota, CEP).

`read.ExtrairDAV` vai além: um schema ESPECÍFICO da "Documento Auxiliar de
Venda" que os dois documentos reais são, não um parser universal de nota
fiscal. Reconhece emitente (razão social + CNPJ), destinatário (nome +
CNPJ/CPF), número do documento, data de emissão, os mesmos totais de
`ExtrairCampos`, e monta a tabela de itens via `GroupTable`.

Rodado no PDF em alta resolução, depois do conserto do terceiro bug:

```
CNPJs:   [14.788.633/0001-10 27.363.223/0001-70]
Datas:   [2026-08-18]
Valores: {Total a pagar: R$ 100,60, Valor Produtos: R$ 100,60}

Emitente:     {RODRIGUES MATERIAL DE CONSTRUCOES LTDA-ME (RODRIGUES C1, 14.788.633/0001-10}
Destinatario: {FROTA MACEDO ENGENHARIA EIRELI (000000000000035), 27.363.223/0001-70}
NumeroDocumento: 0000019185
DataEmissao:     2026-08-18
Totais:          {Total a pagar: R$ 100,60, Valor Produtos: R$ 100,60}
```

`"Total a pagar"` -- o valor que faltava antes do terceiro bug ser
corrigido -- agora aparece, certo, nos dois campos que o procuram
(`ExtrairCampos` e `ExtrairDAV.Totais`). Emitente, destinatário, número
do documento e data saem todos corretos e completos.

No print de tela (resolução mais baixa, já registrada como pior desde a
fase 3): achou o CNPJ e o nome do emitente, mas não o CPF/CNPJ do
destinatário -- aquela região específica saiu detectada com uma caixa de
~5px de altura (contra os ~20-30px normais), pequena demais para o
reconhecedor ler qualquer coisa; nem o ajuste do terceiro bug (que é
sobre CLASSIFICAR a deformação, não sobre reconhecer um recorte
ilegível) resolve isso. `Valores`/`Totais` também saíram vazios neste
documento -- o mesmo tipo de linha que, na versão em alta resolução, tem
rótulo e valor juntos, aqui não trouxe o valor junto em nenhuma `Line`.
Registrado, não escondido: é limite de detecção/resolução específico
desta imagem, não bug em `ExtrairCampos`/`ExtrairDAV`.

**O que isso prova, e o que não prova:** a cadeia inteira -- imagem →
detecção → retificação → reconhecimento → layout → campo tipado --
fecha de ponta a ponta num documento real de alta resolução, produzindo o
dado estruturado que o README define como o produto final do motor
(`{fornecedor, cnpj, itens, total}`, não texto corrido), com emitente,
destinatário, número, data e total todos corretos. Não prova extração
confiável em qualquer resolução (o segundo documento mostra os mesmos
limites de detecção já conhecidos desde a fase 3) nem tabela de itens
estruturada (`GroupTable` não segmenta em colunas neste formato de
linha, ver acima) -- os dois registrados como o que realmente falta antes
de virar produto, não escondidos atrás de um resultado só do melhor caso.

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

Modelo escolhido: `latin_PP-OCRv3_mobile_rec` (dicionário latino, cobre
acento português) — não o `ch_PP-OCRv4_rec` da mesma geração do detector,
que é chinês. Detalhe da escolha, da licença (com uma divergência entre
fonte primária e o espelho `.onnx` ainda não resolvida) e da validação do
grafo contra o ONNX Runtime em "SVTR: o grafo já roda" acima.

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

Fases 1, 2, 3 e 4 prontas; fases 5, 6 e 7 parciais (ver Roteiro). A fase 3 (detecção) está validada contra imagem real e o ONNX
Runtime — 99,999% de similaridade de cosseno, 33 regiões de texto
encontradas corretamente numa foto de verdade (`ch_PP-OCRv4_det_infer`, ver
"Validado contra o ONNX Runtime" acima) — e testada em dois documentos reais
da Frota Macedo: 102 regiões num PDF em alta resolução, 60 num print de
tela de resolução mais baixa, com uma falha de detecção identificada e
atribuída à resolução (ver "Testado em documento real" acima). A fase 5
(reconhecimento) tem o grafo do `latin_PP-OCRv3_mobile_rec` batendo com o
ONNX Runtime (similaridade de cosseno 1,0000001) e o pacote `recog`
(pré-processamento de linha + decodificação CTC) validado com duas linhas
reais de um documento da Frota Macedo — uma saiu perfeita, a outra errou
dois caracteres, no mesmo tipo de limite de resolução baixa já documentado
na fase 3 (ver "SVTR: o grafo já roda" e "Validado com duas linhas reais"
acima). O pacote `read` liga detecção, retificação, reconhecimento e
layout numa passagem só e roda de ponta a ponta nos dois documentos reais
da Frota Macedo -- 30 e 22 linhas de texto corretamente legível
("DOCUMENTO AUXILIAR DE VENDA - PEDIDO", CNPJ, endereço, itens da tabela,
agora com código+descrição e quantidade/preço do mesmo item numa linha só),
depois de quatro bugs de integração achados e corrigidos no processo
(`dewarp.ExtractBaseline` classificava linha reta como N3; o recorte para
reconhecimento usava pontos que cortavam o início/fim do texto; campo
pequeno como um valor monetário curto também virava N3 por limiar de
deformação em pixel absoluto, não relativo ao tamanho da região;
`layout.GroupLines` ancorava cada linha na primeira palavra sem nunca
atualizar, quebrando por transitividade um item de tabela em várias
`Line`s -- ver "`read`: a página inteira", "Terceiro bug" e "Quarto bug"
acima). Resta um defeito conhecido, sem relação com os quatro: espaço
perdido dentro de uma região que o detector marcou como uma peça só.
`layout.SplitCells` separa campo por espaçamento horizontal dentro de uma
linha, validado nos dois documentos reais; `layout.GroupTable` alinha
células de várias linhas em coluna (certo para texto corrido, como
rótulo+valor lado a lado), e `layout.GroupTableWords` faz o mesmo por
palavra em vez de célula -- a tabela de itens do PDF em alta resolução
passou de 1 para 7 colunas reconhecíveis, com alguma célula vazia
ocasional onde a posição de um campo não repetiu entre linhas. `read.
ExtrairCampos` e `read.ExtrairDAV` ligam `extract` à saída de `read.Page`
e fecham o ciclo completo pela primeira vez num documento real --
emitente, destinatário, número do documento, data, total e a tabela de
itens em coluna, todos corretos ou reconhecíveis no PDF em alta
resolução; falta schema para outros tipos de documento além da DAV (ver
"`SplitCells`", "Quarto bug", "`GroupTable`"/"`GroupTableWords`" e
"`ExtrairCampos` e `ExtrairDAV`" acima).

## Licença

MIT -- ver [LICENSE](LICENSE). Decidido em 14/09/2026; nem o monorepo `era`
(de onde este motor saiu) tinha decidido isso ainda, então não havia
precedente a seguir.

Os **pesos de modelo** têm licença própria, independente deste código, e não
são distribuídos aqui. Confira a licença do modelo que for usar -- em
particular, a divergência entre o checkpoint oficial do
`latin_PP-OCRv3_mobile_rec` (Apache 2.0) e o espelho `.onnx` usado aqui
(MIT), registrada na seção "SVTR: o grafo já roda" acima, continua sem
resolver.
