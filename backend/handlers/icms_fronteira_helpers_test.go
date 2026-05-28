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
		VProd: 1000, VIcms: 120, VBcST: 0, VBcCalc: 1050, VST: 0,
		AliqInter: 12, AliqInterna: 20.5, IcmsDevidoEst: 85, VIPI: 50,
	}
	rec := rowToCSVRecord(row)
	if len(rec) != len(exportCSVHeaders) {
		t.Errorf("CSV fields (%d) devem casar com headers (%d)", len(rec), len(exportCSVHeaders))
	}
	if len(rec) != 19 {
		t.Errorf("expected 19 CSV fields (Bloco+18 colunas modelo), got %d", len(rec))
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

// ── cteLinkToCSVRecord ───────────────────────────────────────────────────────

func TestCteLinkToCSVRecord_FieldCount(t *testing.T) {
	link := CteLink{
		ChaveCTe: "CTe123", NumeroCTe: "4981207", DataEmissao: "2026-01-10",
		EmitNome: "Transportadora X", EmitCNPJ: "99888777000100",
		VPrest: 155.99, VIcmsCTe: 6.24,
	}
	rec := cteLinkToCSVRecord("B - Mês Atual", link, "Chave-NF-123", 20.5)
	if len(rec) != len(exportCSVHeaders) {
		t.Errorf("CT-e CSV fields (%d) deve casar com headers (%d)", len(rec), len(exportCSVHeaders))
	}
}

func TestCteLinkToCSVRecord_IcmsDevido(t *testing.T) {
	link := CteLink{VPrest: 155.99, VIcmsCTe: 6.24}
	rec := cteLinkToCSVRecord("B - Mês Atual", link, "chave", 20.5)
	// icms_dev = 155.99 × 20.5% − 6.24 ≈ 25.74; agora em índice 16 (nova coluna V.BC Antecip. em 12)
	if rec[16] == "0.00" {
		t.Errorf("ICMS Devido CT-e não pode ser zero quando v_prest > 0")
	}
}

func TestCteLinkToCSVRecord_IcmsDevidoNegativoZerado(t *testing.T) {
	// Quando ICMS CT-e > v_prest × aliq_interna → resultado não pode ser negativo
	link := CteLink{VPrest: 10.0, VIcmsCTe: 999.0}
	rec := cteLinkToCSVRecord("B - Mês Atual", link, "chave", 20.5)
	if rec[16] != "0.00" {
		t.Errorf("ICMS Devido negativo deve virar 0.00, got %q", rec[16])
	}
}

func TestCteLinkToCSVRecord_CFOPIsCTE(t *testing.T) {
	link := CteLink{NumeroCTe: "777"}
	rec := cteLinkToCSVRecord("A - Mês Anterior", link, "chave", 20.5)
	// índice 6 = CFOP; índice 2 = Número NF-e (exibido como "CT-e NNN")
	if rec[6] != "CTE" {
		t.Errorf("CFOP da linha CT-e deve ser 'CTE', got %q", rec[6])
	}
	if rec[2] != "CT-e 777" {
		t.Errorf("Número CT-e deve ser 'CT-e 777', got %q", rec[2])
	}
}

func TestCteLinkToCSVRecord_ChaveNFEPreservada(t *testing.T) {
	link := CteLink{ChaveCTe: "chave-cte-999"}
	rec := cteLinkToCSVRecord("B - Mês Atual", link, "chave-nfe-abc", 20.5)
	// Chave NF-e em índice 17 (col Q), Chave CT-e em índice 18 (col R)
	if rec[17] != "chave-nfe-abc" {
		t.Errorf("Chave NF-e deve ser preservada na coluna Q, got %q", rec[17])
	}
	if rec[18] != "chave-cte-999" {
		t.Errorf("Chave CT-e deve estar na coluna R, got %q", rec[18])
	}
}

// ── fetchCteLinksForNFs — early-return sem DB ────────────────────────────────

func TestFetchCteLinksForNFs_EmptyChaves(t *testing.T) {
	// Com slice vazio, retorna antes de tocar no DB (db pode ser nil).
	result := fetchCteLinksForNFs(nil, "any-company", []string{})
	if len(result) != 0 {
		t.Errorf("esperado mapa vazio para chaves vazio, got %d entradas", len(result))
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

// ── fetchTopNcmByChave — early-return sem DB ─────────────────────────────────

func TestFetchTopNcmByChave_EmptyChaves(t *testing.T) {
	result := fetchTopNcmByChave(nil, "any-company", []string{})
	if len(result) != 0 {
		t.Errorf("esperado mapa vazio para chaves vazio, got %d entradas", len(result))
	}
}

// ── itenRowToCSV ─────────────────────────────────────────────────────────────

func TestItenRowToCSV_FieldCount(t *testing.T) {
	mva := 45.5
	row := FronteiraItemRow{
		DataEmissao: "2026-01-15", NumeroNFe: "001", FornCNPJ: "11111111000100",
		FornNome: "Forn", FornUF: "BA", CFOP: "2102", Regime: "ST",
		NItem: 1, CProd: "P01", XProd: "Produto", NCM: "84714100", CEST: "2103800",
		VProdItem: 100, VIpiItem: 5, VOutroRateado: 2, VOperacao: 107, VIcmsItem: 12,
		AliqInter: 12, AliqInterna: 20.5, BC: 107, IcmsCalculado: 21.94, IcmsRetido: 21.94,
		MvaOriginal: &mva, BcSt: 130.5,
	}
	rec := itenRowToCSV(row)
	if len(rec) != len(itensCSVHeaders) {
		t.Errorf("itenRowToCSV fields (%d) deve casar com headers (%d)", len(rec), len(itensCSVHeaders))
	}
}

func TestItenRowToCSV_DateTruncation(t *testing.T) {
	row := FronteiraItemRow{DataEmissao: "2026-01-15T00:00:00Z"}
	rec := itenRowToCSV(row)
	if rec[0] != "2026-01-15" {
		t.Errorf("data deve ser truncada em 10 chars, got %q", rec[0])
	}
}

func TestItenRowToCSV_NilMVAAndZeroBcSt(t *testing.T) {
	row := FronteiraItemRow{DataEmissao: "2026-01-15", MvaOriginal: nil, BcSt: 0}
	rec := itenRowToCSV(row)
	if rec[20] != "" {
		t.Errorf("MVA nulo deve gerar string vazia, got %q", rec[20])
	}
	if rec[21] != "" {
		t.Errorf("BcSt zero deve gerar string vazia, got %q", rec[21])
	}
}

// ── divRowToCSV ──────────────────────────────────────────────────────────────

func TestDivRowToCSV_FieldCount(t *testing.T) {
	row := DivergenciaRow{
		Periodo: "01/2026", NumeroNF: "123", FornCNPJ: "11111111000100",
		FornNome: "Forn", FornUF: "BA", DataEmissao: "2026-01-15",
		Regime: "ST", IcmsSefaz: 100, IcmsCalculado: 90, Diferenca: 10, Status: "COBRADO_A_MAIS",
	}
	rec := divRowToCSV(row)
	if len(rec) != len(divCSVHeaders) {
		t.Errorf("divRowToCSV fields (%d) deve casar com headers (%d)", len(rec), len(divCSVHeaders))
	}
}

func TestDivRowToCSV_DateTruncation(t *testing.T) {
	row := DivergenciaRow{DataEmissao: "2026-01-15T10:30:00Z"}
	rec := divRowToCSV(row)
	if rec[5] != "2026-01-15" {
		t.Errorf("data deve ser truncada em 10 chars, got %q", rec[5])
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
