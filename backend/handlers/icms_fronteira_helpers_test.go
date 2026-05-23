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
		AliqInter: 12, AliqInterna: 20.5, IcmsDevidoEst: 85,
	}
	rec := rowToCSVRecord(row)
	if len(rec) != 14 {
		t.Errorf("expected 14 CSV fields, got %d", len(rec))
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
	if rec[6] != "DIFAL" {
		t.Errorf("expected regime 'DIFAL' at index 6, got %q", rec[6])
	}
}

// ── buildExportQuery ─────────────────────────────────────────────────────────

func TestBuildExportQuery_Todos(t *testing.T) {
	q, args := buildExportQuery("todos", "")
	if len(args) != 0 {
		t.Errorf("todos: expected 0 extra args, got %d", len(args))
	}
	if !strings.Contains(q, "FROM classified") {
		t.Errorf("todos: expected 'FROM classified' in query")
	}
}

func TestBuildExportQuery_Regime(t *testing.T) {
	q, args := buildExportQuery("ST", "")
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
	_, args := buildExportQuery("antecipacao", "")
	if len(args) != 1 {
		t.Errorf("antecipacao: expected 1 extra arg, got %d", len(args))
	}
}
