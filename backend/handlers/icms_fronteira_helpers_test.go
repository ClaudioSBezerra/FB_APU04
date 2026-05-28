package handlers

import (
	"strings"
	"testing"
)

// ── htmlEscape ───────────────────────────────────────────────────────────────

func TestHtmlEscape_NoSpecials(t *testing.T) {
	if got := htmlEscape("hello"); got != "hello" {
		t.Errorf("want 'hello', got %q", got)
	}
}

func TestHtmlEscape_Ampersand(t *testing.T) {
	if got := htmlEscape("a&b"); !strings.Contains(got, "&amp;") {
		t.Errorf("expected &amp; in output, got %q", got)
	}
}

func TestHtmlEscape_LtGt(t *testing.T) {
	got := htmlEscape("<script>")
	if !strings.Contains(got, "&lt;") || !strings.Contains(got, "&gt;") {
		t.Errorf("expected escaped lt/gt, got %q", got)
	}
}

func TestHtmlEscape_Quote(t *testing.T) {
	got := htmlEscape(`say "hi"`)
	if !strings.Contains(got, "&#34;") && !strings.Contains(got, "&quot;") {
		t.Errorf("expected escaped quote, got %q", got)
	}
}

// ── itoa ─────────────────────────────────────────────────────────────────────

func TestItoa_Zero(t *testing.T) {
	if got := itoa(0); got != "0" {
		t.Errorf("want '0', got %q", got)
	}
}

func TestItoa_Positive(t *testing.T) {
	if got := itoa(42); got != "42" {
		t.Errorf("want '42', got %q", got)
	}
}

func TestItoa_Negative(t *testing.T) {
	if got := itoa(-7); got != "-7" {
		t.Errorf("want '-7', got %q", got)
	}
}

// ── rowToCSVRecord ───────────────────────────────────────────────────────────

func TestRowToCSVRecord_FieldCount(t *testing.T) {
	row := fronteiraExportRow{
		DataEmissao: "2026-01-15", NumeroNFe: "000123",
		FornNome: "Fornecedor X", FornCNPJ: "12345678000199",
		FornUF: "SP", CFOP: "2102", Regime: "ANTECIPACAO",
		VProd: 1000, VIcms: 120, VBcST: 0, VST: 0,
		AliqInter: 12, AliqInterna: 20.5, IcmsDevidoEst: 85, VIPI: 50,
	}
	rec := rowToCSVRecord(row)
	if len(rec) != len(exportCSVHeaders) {
		t.Errorf("CSV fields (%d) devem casar com headers (%d)", len(rec), len(exportCSVHeaders))
	}
	if len(rec) != 18 {
		t.Errorf("expected 18 CSV fields (Bloco+17 colunas modelo), got %d", len(rec))
	}
}

func TestRowToCSVRecord_Values(t *testing.T) {
	row := fronteiraExportRow{
		DataEmissao: "2026-01-15", NumeroNFe: "99",
		FornNome: "Teste", FornCNPJ: "11111111000100",
		FornUF: "MG", CFOP: "2551", Regime: "DIFAL",
		VProd: 500, VIcms: 60, VBcST: 0, VST: 0,
		AliqInter: 7, AliqInterna: 20.5, IcmsDevidoEst: 67.5,
	}
	rec := rowToCSVRecord(row)
	if rec[7] != "DIFAL" {
		t.Errorf("expected regime 'DIFAL' at index 7, got %q", rec[7])
	}
}

// ── buildExportQuery ─────────────────────────────────────────────────────────

func TestBuildExportQuery_Todos(t *testing.T) {
	q, args := buildExportQuery("todos", "", "", nil)
	if len(args) != 0 {
		t.Errorf("todos: expected 0 extra args, got %d", len(args))
	}
	if !strings.Contains(q, "FROM classified") {
		t.Errorf("todos: expected 'FROM classified' in query")
	}
	if !strings.Contains(q, "v_ipi") {
		t.Errorf("todos: expected 'v_ipi' column in query")
	}
}

func TestBuildExportQuery_Regime(t *testing.T) {
	q, args := buildExportQuery("ST", "", "", nil)
	if len(args) != 1 {
		t.Errorf("ST: expected 1 extra arg, got %d", len(args))
	}
	if args[0] != "ST" {
		t.Errorf("ST: expected arg[0]='ST', got %v", args[0])
	}
	if !strings.Contains(q, "regime") {
		t.Errorf("ST: expected 'regime' filter in query")
	}
}

func TestBuildExportQuery_Antecipacao(t *testing.T) {
	_, args := buildExportQuery("antecipacao", "", "", nil)
	if len(args) != 1 {
		t.Errorf("antecipacao: expected 1 extra arg, got %d", len(args))
	}
}

func TestBuildExportQuery_TodosComFiltro(t *testing.T) {
	// regime "todos" + filtro → cláusula WHERE 1=1 com o filtro anexado
	q, args := buildExportQuery("todos", "", " AND forn_uf = $3", []interface{}{"BA"})
	if !strings.Contains(q, "WHERE 1=1") {
		t.Errorf("todos com filtro: esperava 'WHERE 1=1' na query")
	}
	if len(args) != 1 {
		t.Errorf("todos com filtro: esperava 1 arg, obteve %d", len(args))
	}
}

func TestBuildExportQuery_ComFiltro(t *testing.T) {
	// filtro de fornecedor entra como $4 quando há regime específico
	q, args := buildExportQuery("ST", "", " AND (forn_cnpj ILIKE $4 OR forn_nome ILIKE $4)", []interface{}{"%ACME%"})
	if len(args) != 2 {
		t.Errorf("com filtro: expected 2 args (regime+forn), got %d", len(args))
	}
	if !strings.Contains(q, "forn_cnpj ILIKE $4") {
		t.Errorf("com filtro: expected forn filter in query")
	}
}

// ── brl (formatação de moeda brasileira) ─────────────────────────────────────

func TestBRL(t *testing.T) {
	cases := []struct {
		name string
		in   float64
		want string
	}{
		{"zero", 0, "R$ 0,00"},
		{"centavos", 0.5, "R$ 0,50"},
		{"abaixo_de_mil", 123.45, "R$ 123,45"},
		{"exatamente_mil", 1000, "R$ 1.000,00"},
		{"milhares", 1234.56, "R$ 1.234,56"},
		{"dezenas_de_milhar", 12345.67, "R$ 12.345,67"},
		{"milhoes", 1234567.89, "R$ 1.234.567,89"},
		{"negativo", -1234.56, "-R$ 1.234,56"},
		{"arredondamento", 9.999, "R$ 10,00"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := brl(c.in); got != c.want {
				t.Errorf("brl(%v) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// ── blocoLabel ───────────────────────────────────────────────────────────────

func TestBlocoLabel(t *testing.T) {
	if got := blocoLabel("mes_anterior"); got != "A - Mês Anterior" {
		t.Errorf("mes_anterior: got %q", got)
	}
	if got := blocoLabel("mes_atual"); got != "B - Mês Atual" {
		t.Errorf("mes_atual: got %q", got)
	}
	if got := blocoLabel(""); got != "B - Mês Atual" {
		t.Errorf("default: got %q", got)
	}
}
