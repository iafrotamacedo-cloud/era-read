package recog

// Charset mapeia o indice de uma classe da rede (uma posicao da saida do
// CTC) para o caractere correspondente. O indice 0 e sempre o token em
// branco do CTC -- convencao do PaddleOCR
// (BaseRecLabelDecode.add_special_char: dict_character = ["blank"] +
// character_str, conferido na fonte primaria em 14/09/2026) -- e nunca
// aparece no texto que DecodeCTC devolve.
//
// A ERA nao empacota dicionario nenhum, pelo mesmo motivo que nao empacota
// peso de modelo (ver "Sem dados embutidos" no README): o dicionario e
// parte do modelo, tem a licenca dele, e a ordem de cada caractere so faz
// sentido para o .onnx exato que foi treinado com aquela ordem -- trocar
// de modelo sem trocar o dicionario junto decodifica lixo em silencio.
type Charset []string

// NewCharset monta o dicionario de decodificacao a partir das linhas do
// arquivo de dicionario do modelo (uma por caractere, na mesma ordem usada
// no treino -- para o latin_PP-OCRv3_mobile_rec, latin_dict.txt do
// PaddleOCR). O token em branco entra sozinho no indice 0; as linhas nao
// devem incluir isso.
func NewCharset(linhas []string) Charset {
	cs := make(Charset, len(linhas)+1)
	copy(cs[1:], linhas)
	return cs
}
