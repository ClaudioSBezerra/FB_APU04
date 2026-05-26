package handlers

import (
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// fronteiraFiltros — monta cláusula WHERE + args a partir da query string.
func TestFronteiraFiltros_Vazio(t *testing.T) {
	r := httptest.NewRequest("GET", "/x", nil)
	sql, args := fronteiraFiltros(r, 3)
	if sql != "" || len(args) != 0 {
		t.Fatalf("esperado vazio, veio sql=%q args=%v", sql, args)
	}
}

func TestFronteiraFiltros_Todos(t *testing.T) {
	r := httptest.NewRequest("GET",
		"/x?uf=pe&forn=acme&num_nota=123&data_ini=2026-01-01&data_fim=2026-01-31", nil)
	sql, args := fronteiraFiltros(r, 3)
	if len(args) != 5 {
		t.Fatalf("esperado 5 args, veio %d (%v)", len(args), args)
	}
	// uf deve ser uppercased
	if args[0] != "PE" {
		t.Errorf("uf deveria virar PE, veio %v", args[0])
	}
	if args[1] != "%acme%" {
		t.Errorf("forn deveria ter wildcards, veio %v", args[1])
	}
	// placeholders numerados a partir de 3
	for _, frag := range []string{"$3", "$4", "$5", "$6", "$7"} {
		if !strings.Contains(sql, frag) {
			t.Errorf("sql não contém placeholder %s: %q", frag, sql)
		}
	}
}

func TestFronteiraFiltros_ApenasUF(t *testing.T) {
	r := httptest.NewRequest("GET", "/x?uf=ba", nil)
	sql, args := fronteiraFiltros(r, 10)
	if len(args) != 1 || args[0] != "BA" {
		t.Fatalf("esperado [BA], veio %v", args)
	}
	if !strings.Contains(sql, "uf_filial = $10") {
		t.Errorf("placeholder deveria começar em $10: %q", sql)
	}
}

// detectFronteiraRegrasColumns — mapeia cabeçalho → índice.
func TestDetectFronteiraRegrasColumns(t *testing.T) {
	header := []string{"NCM", "Descrição", "Cód. Segmento", "Segmento", "UF",
		"Regime", "Alíquota Interna", "MVA Original", "Aliquotas de 4%",
		"Aliquotas de 7%", "Aliquotas de 12%", "CEST", "Redução"}
	idx := detectFronteiraRegrasColumns(header)

	expect := map[string]int{
		"ncm": 0, "descricao": 1, "segmento_codigo": 2, "segmento": 3, "uf": 4,
		"regime": 5, "aliq": 6, "mva_orig": 7, "mva4": 8, "mva7": 9, "mva12": 10,
		"cest": 11, "reducao": 12,
	}
	for k, want := range expect {
		if got, ok := idx[k]; !ok || got != want {
			t.Errorf("coluna %q: esperado %d, veio %d (ok=%v)", k, want, got, ok)
		}
	}
}

func TestDetectFronteiraRegrasColumns_Minimo(t *testing.T) {
	idx := detectFronteiraRegrasColumns([]string{"foo", "bar"})
	if _, ok := idx["ncm"]; ok {
		t.Error("não deveria detectar ncm em cabeçalho sem coluna NCM")
	}
}

// splitNCMs — uma célula pode conter vários NCMs.
func TestSplitNCMs(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"8544", []string{"8544"}},
		{"3815.12.10, 3815.12.90", []string{"38151210", "38151290"}},
		{"8544\n7605\n7614", []string{"8544", "7605", "7614"}},
		{"1234;1234", []string{"1234"}},               // dedup
		{"1234 / etc. / ou", []string{"1234"}},        // descarta lixo não-numérico
		{"  84-71  ", []string{"8471"}},               // remove pontuação/espaço
	}
	for _, c := range cases {
		got := splitNCMs(c.in)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("splitNCMs(%q) = %v, esperado %v", c.in, got, c.want)
		}
	}
}

// dedupStr — remove duplicatas e vazios preservando ordem.
func TestDedupStr(t *testing.T) {
	got := dedupStr([]string{"a", "", "b", "a", " ", "c", "b"})
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("dedupStr = %v, esperado %v", got, want)
	}
}

// parseFloatBR — decimal com vírgula e sufixo %.
func TestParseFloatBR(t *testing.T) {
	cases := map[string]float64{
		"81,64": 81.64, "30": 30, "58,54%": 58.54, "": 0, "abc": 0, " 12,5 ": 12.5,
	}
	for in, want := range cases {
		if got := parseFloatBR(in); got != want {
			t.Errorf("parseFloatBR(%q) = %v, esperado %v", in, got, want)
		}
	}
}

// primeiroPct — maior percentual da célula.
func TestPrimeiroPct(t *testing.T) {
	if got := primeiroPct("75,79% 66,34% 81,64%"); got != 81.64 {
		t.Errorf("primeiroPct = %v, esperado 81.64", got)
	}
	if got := primeiroPct("sem percentual"); got != 0 {
		t.Errorf("primeiroPct sem pct = %v, esperado 0", got)
	}
}

// normalizaCest — remove pontuação.
func TestNormalizaCest(t *testing.T) {
	if got := normalizaCest(" 01.049.00 "); got != "0104900" {
		t.Errorf("normalizaCest = %q, esperado 0104900", got)
	}
	if got := normalizaCest("28-038-00"); got != "2803800" {
		t.Errorf("normalizaCest com hífen = %q, esperado 2803800", got)
	}
}

// parsePctOrNull — vazio/traço/zero → nil; número → *float64 (via interface).
func TestParsePctOrNull(t *testing.T) {
	for _, in := range []string{"", "-", "—", "0", "0,00"} {
		if got := parsePctOrNull(in); got != nil {
			t.Errorf("parsePctOrNull(%q) = %v, esperado nil", in, got)
		}
	}
	if got := parsePctOrNull("112,04"); got != 112.04 {
		t.Errorf("parsePctOrNull(112,04) = %v, esperado 112.04", got)
	}
}

// nullIfEmpty — vazio → nil, senão a própria string.
func TestNullIfEmpty(t *testing.T) {
	if nullIfEmpty("  ") != nil {
		t.Error("nullIfEmpty(espaços) deveria ser nil")
	}
	if got := nullIfEmpty("X"); got != "X" {
		t.Errorf("nullIfEmpty(X) = %v, esperado X", got)
	}
}
