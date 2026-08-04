package handlers

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func TestInstitutoFromSheet(t *testing.T) {
	cases := map[string]string{
		"REGRAS_INAPLICABILIDADE": "ANTECIPACAO",
		"ANT_PARCIAL_INAPLICAB":   "ANT_PARCIAL",
		"ANT_PROPRIA_INAPLICAB":   "ANT_PROPRIA",
		"ST_INAPLICABILIDADE_BA":  "ST",
		"ST_INAPLICABILIDADE_CE":  "ST",
		"MAPEAMENTO_SPED":         "", // não é aba de regras
		"FLUXO_DECISAO":           "",
		"COMPARATIVO_BAHIA":       "",
	}
	for sheet, want := range cases {
		if got := institutoFromSheet(sheet); got != want {
			t.Errorf("institutoFromSheet(%q) = %q, quer %q", sheet, got, want)
		}
	}
}

func TestFindInaplicHeader(t *testing.T) {
	rows := [][]string{
		{"TÍTULO LONGO DA ABA"},
		{"Fonte: ..."},
		{"ID_REGRA", "GRUPO", "TIPO_VERIFICACAO"},
		{"AP01", "1-X", "NCM"},
	}
	if got := findInaplicHeader(rows); got != 2 {
		t.Errorf("findInaplicHeader = %d, quer 2", got)
	}
	if got := findInaplicHeader([][]string{{"a"}, {"b"}}); got != -1 {
		t.Errorf("sem cabeçalho ID deveria dar -1, veio %d", got)
	}
}

func TestDetectInaplicColumns_PE(t *testing.T) {
	hdr := []string{"ID_REGRA", "GRUPO", "TIPO_VERIFICACAO", "REGISTRO_SPED", "CAMPO_SPED",
		"VALORES_GATILHO", "REGISTRO_SPED_2", "CAMPO_SPED_2", "VALORES_SPED_2", "LOGICA",
		"RESULTADO", "INSTRUCAO_SISTEMA", "BASE_LEGAL", "VIGENCIA_INICIO", "VIGENCIA_FIM"}
	ci := detectInaplicColumns(hdr)
	checks := map[string]int{
		"id": 0, "grupo": 1, "tipo_verif": 2, "registro_sped": 3, "campo_sped": 4,
		"valores_gatilho": 5, "registro_sped_2": 6, "campo_sped_2": 7, "valores_2": 8,
		"logica": 9, "resultado": 10, "instrucao": 11, "base_legal": 12,
		"vigencia_inicio": 13, "vigencia_fim": 14,
	}
	for k, want := range checks {
		if ci[k] != want {
			t.Errorf("PE col %q = %d, quer %d", k, ci[k], want)
		}
	}
}

func TestDetectInaplicColumns_BACE(t *testing.T) {
	// BA/CE: usa HIPÓTESE, CONDIÇÃO_PRINCIPAL, COND_ADICIONAL, VIG_FIM (sem VIG_INICIO)
	hdr := []string{"ID_ST_BA", "HIPÓTESE", "TIPO_VERIF", "REGISTRO_SPED", "CAMPO_SPED",
		"CONDIÇÃO_PRINCIPAL", "REG_SPED_2", "CAMPO_SPED_2", "COND_ADICIONAL", "LOGICA",
		"RESULTADO", "INSTRUCAO_SISTEMA", "EXCECAO", "BASE_LEGAL", "VIG_FIM"}
	ci := detectInaplicColumns(hdr)
	if ci["id"] != 0 || ci["hipotese"] != 1 || ci["tipo_verif"] != 2 {
		t.Errorf("BA/CE id/hipotese/tipo errados: %+v", ci)
	}
	if ci["valores_gatilho"] != 5 { // CONDIÇÃO_PRINCIPAL
		t.Errorf("BA/CE gatilho (CONDIÇÃO) = %d, quer 5", ci["valores_gatilho"])
	}
	if ci["valores_2"] != 8 { // COND_ADICIONAL
		t.Errorf("BA/CE valores_2 (COND_ADICIONAL) = %d, quer 8", ci["valores_2"])
	}
	if ci["vigencia_fim"] != 14 || ci["vigencia_inicio"] != -1 {
		t.Errorf("BA/CE vigência: fim=%d (quer 14), inicio=%d (quer -1)", ci["vigencia_fim"], ci["vigencia_inicio"])
	}
}

func TestIsAutoAplicavel(t *testing.T) {
	auto := []struct{ tipo, r1, r2, campo string }{
		{"CST_ICMS", "C170", "", "CST_ICMS"},
		{"CFOP", "C170", "", "CFOP"},
		{"NCM", "0200", "", "COD_NCM"},
		{"VL_ICMS_ST", "C170", "", "VL_ICMS_ST"},
		{"COMBINADA", "0200", "C170", "COD_NCM"},
	}
	for _, c := range auto {
		if !isAutoAplicavel(c.tipo, c.r1, c.r2, c.campo) {
			t.Errorf("esperava auto=true para %+v", c)
		}
	}
	naoAuto := []struct{ tipo, r1, r2, campo string }{
		{"CREDENC", "EXTERNO", "", ""},
		{"CNAE_DEST", "0000", "", "CNAE"},
		{"COMBINADA", "0150", "EXTERNO", "CNPJ[0:8]"},
		{"COMBINADA", "EXTERNO", "0200", "CADASTRO SEFAZ-PE"},
	}
	for _, c := range naoAuto {
		if isAutoAplicavel(c.tipo, c.r1, c.r2, c.campo) {
			t.Errorf("esperava auto=false para %+v", c)
		}
	}
}

func TestParseInaplicDate(t *testing.T) {
	if d := parseInaplicDate("01/12/2021"); d == nil {
		t.Error("01/12/2021 deveria parsear")
	} else if tm, ok := d.(time.Time); !ok || tm.Year() != 2021 || tm.Month() != 12 || tm.Day() != 1 {
		t.Errorf("01/12/2021 → %v", d)
	}
	for _, vazio := range []string{"", "—", "-"} {
		if parseInaplicDate(vazio) != nil {
			t.Errorf("%q deveria virar nil", vazio)
		}
	}
	if parseInaplicDate("2025-01-14") == nil {
		t.Error("ISO 2025-01-14 deveria parsear")
	}
}

func TestInaplicCond(t *testing.T) {
	if c := inaplicCond(nil, false); c != "" {
		t.Errorf("sem regras deveria ser vazio, veio %q", c)
	}
	if c := inaplicCond(nil, true); c != "(classified.v_st > 0)" {
		t.Errorf("só VL_ICMS_ST: %q", c)
	}
	c := inaplicCond([]string{"10", "30"}, false)
	if !strings.Contains(c, "ic.cst_icms IN ('10','30')") || !strings.Contains(c, "reg_c170") {
		t.Errorf("CST cond errada: %q", c)
	}
	if strings.Contains(c, "v_st > 0") {
		t.Errorf("não deveria ter VL_ICMS_ST: %q", c)
	}
	both := inaplicCond([]string{"40"}, true)
	if !strings.Contains(both, " OR ") || !strings.Contains(both, "v_st > 0") || !strings.Contains(both, "'40'") {
		t.Errorf("combinada errada: %q", both)
	}
}

func TestIcmsDevidoExpr(t *testing.T) {
	if e := icmsDevidoExpr(""); e != "icms_devido_est" {
		t.Errorf("vazio → %q", e)
	}
	if e := icmsDevidoExpr("X > 0"); e != "CASE WHEN X > 0 THEN 0 ELSE icms_devido_est END" {
		t.Errorf("com cond → %q", e)
	}
}

func TestNzAndGetFirst(t *testing.T) {
	if nz("") != nil {
		t.Error("nz(vazio) deveria ser nil")
	}
	if nz("x") != "x" {
		t.Error("nz(x) deveria ser x")
	}
	if getFirst("", "b") != "b" {
		t.Error("getFirst vazio→b")
	}
	if getFirst("a", "b") != "a" {
		t.Error("getFirst a→a")
	}
}

func TestDetectInaplicUF(t *testing.T) {
	// reaproveita buildXLSX do comparativo_test (mesmo pacote)
	mk := func(sheet string) *excelize.File {
		data := buildXLSX(t, map[string][][]string{sheet: {{"x"}}})
		f, err := excelize.OpenReader(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		return f
	}
	cases := map[string]string{
		"COMPARATIVO_BAHIA": "BA",
		"COMPARATIVO_CEARA": "CE",
	}
	for sheet, want := range cases {
		f := mk(sheet)
		if got := detectInaplicUF(f, "qualquer.xlsx"); got != want {
			t.Errorf("detectInaplicUF(sheet %q) = %q, quer %q", sheet, got, want)
		}
		f.Close()
	}
	// por filename
	f := mk("REGRAS_INAPLICABILIDADE")
	if got := detectInaplicUF(f, "Inaplicabilidade_Antecipacao_ICMS_PE_SPED.xlsx"); got != "PE" {
		t.Errorf("detectInaplicUF por filename PE = %q", got)
	}
	f.Close()

	// PA (e qualquer outra UF fora do trio original PE/BA/CE) deve ser
	// reconhecido — antes desta correção, caía num fallback silencioso "PE".
	fPA := mk("REGRAS_INAPLICABILIDADE")
	if got := detectInaplicUF(fPA, "Inaplicabilidade_Antecipacao_ICMS_PA_SPED.xlsx"); got != "PA" {
		t.Errorf("detectInaplicUF por filename PA = %q, quer PA", got)
	}
	fPA.Close()

	// Arquivo não reconhecido (nenhuma UF no nome/aba) deve retornar "" — não
	// mais "PE" por padrão, para não gravar regras sob a UF errada.
	fUnknown := mk("PLANILHA_SEM_UF")
	if got := detectInaplicUF(fUnknown, "arquivo_generico.xlsx"); got != "" {
		t.Errorf("detectInaplicUF sem UF reconhecível = %q, quer \"\" (não deve assumir PE)", got)
	}
	fUnknown.Close()
}
