# ERA READ

Motor de leitura de documentos em Go puro. Ver `README.md` para a
arquitetura, o roteiro de fases e as decisões tomadas.

Nasceu dentro do monorepo `era` (`iafrotamacedo-cloud/era`, em
`documents/`) e saiu para repositório próprio em 14/09/2026, por decisão
explícita do usuário. O `era` continua com `faces` e `maps`; o motor de
inferência que as fases 3 e 5 vão precisar (`tensor`, `kernel`, `nn`,
`onnx`, `graph`) mora lá, não aqui — como importar isso através de
repositório é decisão em aberto, registrada no README.

## Ambiente

O Go **não está no PATH do Git Bash** nesta máquina. Exporte antes de usar:

```
export PATH="$PATH:/c/Program Files/Go/bin"
```

Não há compilador C na máquina, então `go test -race` não roda localmente —
quem cobre isso é o CI, no Linux.

**Bloqueio aleatório de binário de teste.** Nesta máquina, uma política de
segurança local (antivírus ou EDR, não identificado com certeza) às vezes
bloqueia a execução do `.test.exe` recém-compilado no diretório temporário
padrão do Go (`AppData\Local\Temp\go-build...`), com o erro "Uma política
de Controle de Aplicativo bloqueou este arquivo". Não é bug de código —
acontece com pacotes diferentes em execuções diferentes, de forma
aparentemente aleatória, e o mesmo binário roda normalmente fora daquele
diretório. Contorne redirecionando o diretório temporário do Go para outro
lugar:

```
mkdir -p /tmp/go-build-era-read
GOTMPDIR=/tmp/go-build-era-read go test ./... -shuffle=on -count=3
```

Se um pacote falhar assim, rode só ele de novo (`go test ./pacote/...`) —
se passar isolado, foi o bloqueio, não o código.

## Comandos

```
go test ./...              # todos os pacotes
go test -race ./...        # exige cgo e um compilador C
go vet ./...
gofmt -l .                 # tem de sair vazio
```

## Convenções

**Comentários e identificadores.** Identificadores em inglês, comentários em
português **sem acento** — o repositório inteiro é assim, e misturar as duas
grafias polui o diff. READMEs, esses sim, levam acento normal. (O pacote
`layout` nasceu quebrando essa regra por descuido e foi corrigido no commit
seguinte — vale conferir de novo se algo escapar.)

**Mensagens de erro** em português, minúsculas, prefixadas com o pacote:
`fmt.Errorf("dewarp: %d pontos na baseline nao bastam: %d", n, min)`.

**Referência antes de otimização.** Toda implementação rápida é conferida
nos testes contra uma versão óbvia — ver `imgproc/reference.go`. Nunca
otimize um arquivo de referência: o valor dele é ser simples o bastante
para dar para ler e afirmar que está certo.

**Meça, não afirme.** Números em README e em comentário saem de teste ou
benchmark que qualquer um pode rodar.

**Teste que não pode falhar não é teste.** Se um teste passaria mesmo com a
função quebrada, ele não está medindo nada — varra um intervalo de entradas
em vez de escolher um valor sortudo.

**Igualdade exata de float em teste é uma armadilha.** Já quebrou o CI uma
vez: um resultado matematicamente zero pode carregar um resto de
arredondamento (~1e-15) que cancela para `0.0` numa arquitetura e não
cancela em outra (ARM64 vs AMD64 é o caso real que aconteceu). Só compare
float com igualdade exata quando o valor não passou por nenhuma conta —
zero-value do Go, cópia direta de outra variável, ou um `return` antes de
qualquer aritmética. Se passou por soma, divisão ou eliminação gaussiana
(`geom.PolyFit`, `geom.SolveHomography4`), use tolerância.

## O que o CI verifica a cada push

- testes nos três sistemas (Linux, Windows, macOS), com `-shuffle=on`
- `-race`, em Ubuntu
- `gofmt`, `go vet`, `go mod tidy` sem diferença
- **zero dependências** — `go list -m all` tem de vir vazio
- **sem cgo** — nenhum `import "C"`
- cross-compile para 11 plataformas, do Raspberry Pi 32 bits ao WASM

Se este projeto um dia precisar mesmo de biblioteca externa, a saída é
`go.mod` com aquela dependência declarada e a checagem de zero-dependências
ajustada para permitir só ela — não afrouxar de forma geral.

## Dados

Este motor não guarda peso de modelo nenhum. Quando a fase 3 ou 5
precisarem de um `.onnx`, ele é trazido por quem usa a biblioteca, nunca
commitado aqui — `*.onnx` já está no `.gitignore`.
