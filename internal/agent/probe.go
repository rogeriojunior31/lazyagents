package agent

import (
	"bytes"
	"io"
	"os"
	"strings"
)

// TranscriptProber é implementado pelos adapters cujo transcript é texto em
// disco: diz, sem decodificar o arquivo, se ele PODE conter query. false é
// certeza de que não contém; true obriga a conferir no Transcript (o texto
// cru traz chaves JSON e escapes). Opcional, por type assertion.
type TranscriptProber interface {
	MayContain(s Session, query string) bool
}

// probeChunk é quanto do arquivo é lido por vez no pré-filtro.
const probeChunk = 1 << 20

// fileMayContain procura query, sem diferenciar maiúsculas, nos bytes crus de
// path. Só vale para ASCII imprimível sem aspas nem barra invertida: o resto
// o JSON pode ter escapado (\u00e9, \"), e aí devolve true e a busca
// completa decide.
func fileMayContain(path, query string) bool {
	if strings.ContainsAny(query, "\"\\") || strings.ContainsFunc(query, func(r rune) bool { return r < ' ' || r > '~' }) {
		return true
	}
	needle := bytes.ToLower([]byte(query))
	if len(needle) == 0 {
		return true
	}
	f, err := os.Open(path)
	if err != nil {
		return true // ilegível: o Transcript reporta o erro
	}
	defer f.Close()
	buf := make([]byte, probeChunk+len(needle))
	keep := 0 // fim do bloco anterior, para achar query que cruza blocos
	for {
		n, err := io.ReadFull(f, buf[keep:])
		if n > 0 && bytes.Contains(bytes.ToLower(buf[:keep+n]), needle) {
			return true
		}
		if err != nil {
			return false
		}
		keep = min(len(needle)-1, keep+n)
		copy(buf, buf[len(buf)-keep:])
	}
}

func (c *Claude) MayContain(s Session, query string) bool { return fileMayContain(s.Path, query) }
func (c *Codex) MayContain(s Session, query string) bool  { return fileMayContain(s.Path, query) }
