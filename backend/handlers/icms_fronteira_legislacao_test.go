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

	// A extração nova devolve a tabela estruturada (linhas "NCM: ..."), não o
	// texto linear. Não pode haver fragmentação de caracteres ("D E C R E T O").
	if strings.Contains(texto, "D E C R E T O") {
		t.Errorf("texto contém fragmentação 'D E C R E T O' — extractPDFText regrediu")
	}
	// Palavras de descrição devem aparecer inteiras (não fragmentadas).
	if !strings.Contains(texto, "Querosenes") {
		t.Errorf("palavra 'Querosenes' não encontrada — extração pode ter regredido")
	}

	filtrado := extrairLinhasRelevantes(texto)
	if len(filtrado) < 5_000 {
		t.Fatalf("filtrado muito curto: %d chars (esperado > 5k linhas com NCM/MVA)", len(filtrado))
	}
	t.Logf("OK: texto=%d chars, filtrado=%d chars (%d linhas)",
		len(texto), len(filtrado), strings.Count(filtrado, "\n")+1)
}

func TestParseLegislacaoJSON_Truncado(t *testing.T) {
	// Resposta truncada no meio da 3ª regra (estouro de max_tokens).
	raw := `{
  "resumo": "Decreto BA 18800",
  "regras": [
    {"ncm":"2201.1","regime":"ST","descricao":"Vinhos","mva_original":54.88,"justificativa":"Anexo I"},
    {"ncm":"2202.1","regime":"ST","descricao":"Refrigerantes","mva_original":140},
    {"ncm":"4202.9","regime":"ST","descricao":"Artigos de plástico","mva_7pct":72.68`

	got := parseLegislacaoJSON(raw)
	if got.Resumo != "Decreto BA 18800" {
		t.Errorf("resumo: got %q", got.Resumo)
	}
	// Recupera as 2 regras completas, descarta a 3ª truncada.
	if len(got.Regras) != 2 {
		t.Fatalf("got %d regras, want 2 (3ª truncada deve ser descartada)", len(got.Regras))
	}
	if got.Regras[0].NCM != "2201.1" || got.Regras[1].NCM != "2202.1" {
		t.Errorf("NCMs errados: %q, %q", got.Regras[0].NCM, got.Regras[1].NCM)
	}
}

func TestParseLegislacaoJSON_Completo(t *testing.T) {
	raw := `prefixo lixo {"resumo":"ok","regras":[{"ncm":"8482","regime":"ANTECIPACAO","descricao":"Rolamentos"}]} sufixo`
	got := parseLegislacaoJSON(raw)
	if got.Resumo != "ok" || len(got.Regras) != 1 || got.Regras[0].NCM != "8482" {
		t.Errorf("parse completo falhou: %+v", got)
	}
}

// TestExtractPDFTable_Pareamento é a regressão do bug central: a linearização
// colava o MVA de um NCM no NCM vizinho (3403 herdava 58,54% que era do
// 2710.19.19). A reconstrução por coluna X deve emparelhar corretamente.
func TestExtractPDFTable_Pareamento(t *testing.T) {
	path := "/tmp/Decreto Nº 18800 DE 20_12_2018 - Estadual - Bahia - LegisWeb.pdf"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("PDF de teste ausente: %v", err)
	}
	txt, err := extractPDFText(data)
	if err != nil {
		t.Fatalf("extractPDFText: %v", err)
	}
	lines := strings.Split(txt, "\n")

	find := func(ncm string) string {
		for _, l := range lines {
			if strings.HasPrefix(l, "NCM: "+ncm+" ") {
				return l
			}
		}
		return ""
	}

	// Deve gerar linhas estruturadas (não o texto linear antigo).
	structured := 0
	for _, l := range lines {
		if strings.HasPrefix(l, "NCM: ") {
			structured++
		}
	}
	if structured < 100 {
		t.Fatalf("poucas linhas estruturadas: %d (esperado >100) — reconstrução de tabela não ativou", structured)
	}

	// 2710.19.19 (querosenes) tem MVA original 30% (a ajustada 58,54% não tem
	// rótulo de alíquota no decreto, então não vira par mapeável).
	q := find("2710.19.19")
	if q == "" {
		t.Fatal("NCM 2710.19.19 não encontrado")
	}
	if !strings.Contains(q, "MVA_orig: 30%") {
		t.Errorf("2710.19.19 deveria ter MVA_orig 30%%: %s", q)
	}

	// 3403 (lubrificantes) NÃO pode ter colado o MVA do vizinho 2710.
	l3403 := find("3403")
	if l3403 == "" {
		t.Fatal("NCM 3403 não encontrado")
	}
	if strings.Contains(l3403, "30%") || strings.Contains(l3403, "58,54%") {
		t.Errorf("3403 herdou MVA do vizinho (regressão do desemparelhamento): %s", l3403)
	}
	if !strings.Contains(strings.ToLower(l3403), "lubrificante") &&
		!strings.Contains(strings.ToLower(l3403), "óleos de petróleo") {
		t.Errorf("3403 deveria descrever lubrificantes: %s", l3403)
	}

	// Rótulo de alíquota não deve poluir o MVA (sem "4%" solto onde só deveria
	// haver MVAs com vírgula). 7318 tem 3 MVAs ajustados.
	l7318 := find("7318")
	if l7318 != "" && strings.Contains(l7318, "MVA_aj:") {
		if strings.Contains(l7318, " 4% ") || strings.HasSuffix(l7318, " 4%") {
			t.Errorf("7318 com alíquota poluindo MVA_aj: %s", l7318)
		}
	}
}

// TestExtractPDFTable_AnexoPendente: segmentos que remetem a anexo externo
// (sem NCM inline) devem virar linhas ANEXO_PENDENTE visíveis, não sumir.
func TestExtractPDFTable_AnexoPendente(t *testing.T) {
	path := "/tmp/Decreto Nº 18800 DE 20_12_2018 - Estadual - Bahia - LegisWeb.pdf"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("PDF de teste ausente: %v", err)
	}
	txt, err := extractPDFText(data)
	if err != nil {
		t.Fatalf("extractPDFText: %v", err)
	}
	var pend []string
	for _, l := range strings.Split(txt, "\n") {
		if strings.HasPrefix(l, "ANEXO_PENDENTE:") {
			pend = append(pend, l)
		}
	}
	if len(pend) == 0 {
		t.Fatal("nenhum ANEXO_PENDENTE capturado — segmentos de anexo estão sendo perdidos")
	}
	// a citação do Conv 52/2017 deve aparecer ao menos uma vez
	all := strings.Join(pend, "\n")
	if !strings.Contains(all, "Conv. ICMS 52") {
		t.Errorf("citação do Conv 52/2017 não reconhecida nas pendências: %v", pend)
	}
	t.Logf("OK: %d segmento(s) ANEXO_PENDENTE", len(pend))
}

func TestParseMVAajPairs(t *testing.T) {
	// ordem embaralhada e rótulos variados ("Alíq."/"Aliq."/só "(N%)")
	cell := "75,79% (Aliq. 7%) 66,34% (12%) 81,64% (Alíq. 4%)"
	got := parseMVAajPairs(cell)
	want := map[int]float64{4: 81.64, 7: 75.79, 12: 66.34}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("aliq %d: got %v, want %v", k, got[k], v)
		}
	}
}

func TestBackCalcAliqInterna(t *testing.T) {
	cases := []struct {
		nome  string
		orig  float64
		pairs map[int]float64
		want  float64
	}{
		// refrigerantes: orig 114%, aj 156,80/148,78/135,40 → interna 20%
		{"refrigerantes", 114, map[int]float64{4: 156.80, 7: 148.78, 12: 135.40}, 20},
		// 7318: orig 55%, aj 81,64/75,79/66,34 → interna 18%
		{"parafusos 7318", 55, map[int]float64{4: 81.64, 7: 75.79, 12: 66.34}, 18},
		// sem dados → 0
		{"sem pares", 30, map[int]float64{}, 0},
		{"sem orig", 0, map[int]float64{7: 50}, 0},
	}
	for _, c := range cases {
		t.Run(c.nome, func(t *testing.T) {
			got := backCalcAliqInterna(c.orig, c.pairs)
			// tolerância de arredondamento do decreto
			if d := got - c.want; d > 0.6 || d < -0.6 {
				t.Errorf("got %v, want ~%v", got, c.want)
			}
		})
	}
}

func TestSplitLinhasEmChunks(t *testing.T) {
	// 95 linhas não-vazias, 30 por chunk → 4 chunks (30,30,30,5)
	var sb strings.Builder
	for i := 0; i < 95; i++ {
		sb.WriteString("NCM 1234 linha\n")
	}
	// intercala linhas vazias que devem ser descartadas
	texto := sb.String() + "\n\n   \n"
	got := splitLinhasEmChunks(texto, 30)
	if len(got) != 4 {
		t.Fatalf("got %d chunks, want 4", len(got))
	}
	// último chunk tem 5 linhas
	if n := strings.Count(got[3], "\n") + 1; n != 5 {
		t.Errorf("último chunk tem %d linhas, want 5", n)
	}
	// chunk cheio tem 30 linhas
	if n := strings.Count(got[0], "\n") + 1; n != 30 {
		t.Errorf("primeiro chunk tem %d linhas, want 30", n)
	}
}

func TestSplitTextoEmChunks(t *testing.T) {
	cases := []struct {
		nome  string
		texto string
		limit int
		want  int
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

func TestFmtPct(t *testing.T) {
	cases := map[float64]string{20.5: "20,5", 12.0: "12", 4.25: "4,25", 0: "0", 7.0: "7"}
	for in, want := range cases {
		if got := fmtPct(in); got != want {
			t.Errorf("fmtPct(%v) = %q, want %q", in, got, want)
		}
	}
}
