package read

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"

	"github.com/iafrotamacedo-cloud/era-read/graph"
	"github.com/iafrotamacedo-cloud/era-read/onnx"
	"github.com/iafrotamacedo-cloud/era-read/recog"
)

// Engine agrupa os grafos ONNX, o dicionario e as opcoes de leitura --
// a forma usual de o FrotaHub (ou o era-regen) chamar o READ depois do
// Melhorador.
type Engine struct {
	detGraph *graph.Graph
	recGraph *graph.Graph
	cs       recog.Charset
	opts     Options
	detHash  string
	recHash  string
}

// EngineConfig aponta para os arquivos que o operador traz (nunca
// commitados neste repo).
type EngineConfig struct {
	DetONNX string
	RecONNX string
	Dict    string
	Opts    Options
	Filtro  *FiltroRead
}

// AbrirEngine carrega os dois .onnx, o dicionario do rec e aplica o
// filtro_read vigente (se houver).
func AbrirEngine(cfg EngineConfig) (*Engine, error) {
	opts := cfg.Opts
	if opts == (Options{}) {
		opts = DefaultOptions()
	}
	if cfg.Filtro != nil {
		opts = AplicarFiltro(opts, *cfg.Filtro)
	}

	detModel, detHash, err := carregarONNX(cfg.DetONNX)
	if err != nil {
		return nil, fmt.Errorf("read: deteccao: %w", err)
	}
	recModel, recHash, err := carregarONNX(cfg.RecONNX)
	if err != nil {
		return nil, fmt.Errorf("read: reconhecimento: %w", err)
	}

	detGraph, err := graph.New(detModel)
	if err != nil {
		return nil, fmt.Errorf("read: grafo deteccao: %w", err)
	}
	recGraph, err := graph.New(recModel)
	if err != nil {
		return nil, fmt.Errorf("read: grafo reconhecimento: %w", err)
	}

	linhas, err := lerLinhasArquivo(cfg.Dict)
	if err != nil {
		return nil, fmt.Errorf("read: dicionario: %w", err)
	}

	return &Engine{
		detGraph: detGraph,
		recGraph: recGraph,
		cs:       recog.NewCharset(linhas),
		opts:     opts,
		detHash:  detHash,
		recHash:  recHash,
	}, nil
}

// Options devolve as opcoes efetivas (ja com filtro aplicado na abertura).
func (e *Engine) Options() Options { return e.opts }

// Identidade descreve esta instancia do motor para gravar em leitura_era.
func (e *Engine) Identidade() Identidade {
	return IdentidadeDe(e.opts, e.detHash, e.recHash)
}

// ScanImage le uma pagina ja decodificada.
func (e *Engine) ScanImage(src image.Image) (Scan, error) {
	return PageScan(src, e.detGraph, e.recGraph, e.cs, e.opts)
}

// ScanBytes decodifica JPEG ou PNG e le a pagina.
func (e *Engine) ScanBytes(raw []byte) (Scan, error) {
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return Scan{}, fmt.Errorf("read: decodificar imagem: %w", err)
	}
	return e.ScanImage(img)
}

func carregarONNX(caminho string) (*onnx.Model, string, error) {
	b, err := os.ReadFile(caminho)
	if err != nil {
		return nil, "", err
	}
	h := sha256.Sum256(b)
	hash := "sha256:" + hex.EncodeToString(h[:])
	m, err := onnx.Parse(b)
	if err != nil {
		return nil, "", err
	}
	return m, hash, nil
}

func lerLinhasArquivo(caminho string) ([]string, error) {
	b, err := os.ReadFile(caminho)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, linha := range strings.Split(string(b), "\n") {
		linha = strings.TrimSpace(linha)
		if linha == "" {
			continue
		}
		out = append(out, linha)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("read: dicionario vazio")
	}
	return out, nil
}
