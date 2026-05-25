package handlers

import (
	"os"
	"strings"
	"testing"
)

// TestExtractPDFText_DecretoBA garante que a extração nova produz texto
// legível (palavras, não "D E C R E T O") e que o filtro encontra dezenas
// de linhas relevantes. Regressão do bug "0 chars filtrados".
//
// Skippa silenciosamente se o PDF não estiver presente (CI sem o arquivo).
func TestExtractPDFText_DecretoBA(t *testing.T) {
	path := "/tmp/Decreto Nº 18800 DE 20_12_2018 - Estadual - Bahia - LegisWeb.pdf"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("PDF de teste ausente em %s: %v", path, err)
	}

	texto, err := extractPDFText(data)
	if err != nil {
		t.Fatalf("extractPDFText: %v", err)
	}
	if len(texto) < 10_000 {
		t.Fatalf("texto muito curto: %d chars (esperado > 10k)", len(texto))
	}

	// Antes: "D E C R E T O" — depois: "DECRETO"
	if !strings.Contains(texto, "DECRETO") {
		t.Errorf("palavra 'DECRETO' não encontrada — extração ainda fragmenta caracteres")
	}
	if strings.Contains(texto, "D E C R E T O") {
		t.Errorf("texto contém fragmentação 'D E C R E T O' — extractPDFText regrediu")
	}

	filtrado := extrairLinhasRelevantes(texto)
	if len(filtrado) < 5_000 {
		t.Fatalf("filtrado muito curto: %d chars (esperado > 5k linhas com NCM/MVA)", len(filtrado))
	}
	t.Logf("OK: texto=%d chars, filtrado=%d chars (%d linhas)",
		len(texto), len(filtrado), strings.Count(filtrado, "\n")+1)
}

func TestSplitTextoEmChunks(t *testing.T) {
	cases := []struct {
		nome    string
		texto   string
		limit   int
		want    int
	}{
		{"vazio", "", 100, 1},
		{"menor que limit", "abc\ndef", 100, 1},
		{"corta na linha", strings.Repeat("linha\n", 50), 60, 5},
		{"linha gigante única", strings.Repeat("x", 1000), 100, 1},
	}
	for _, c := range cases {
		t.Run(c.nome, func(t *testing.T) {
			got := splitTextoEmChunks(c.texto, c.limit)
			if len(got) != c.want {
				t.Errorf("got %d chunks, want %d", len(got), c.want)
			}
		})
	}
}
