package handlers

import (
	"bytes"
	"encoding/json"
	"math"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func almostEqual(a, b float64) bool { return math.Abs(a-b) < 0.005 }

func TestParseMoney(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{"R$ 1,774.48", 1774.48}, // US c/ prefixo
		{"R$ 4.13", 4.13},
		{"R$ 10,059.97", 10059.97},
		{"1.774,48", 1774.48}, // BR
		{"R$ 0.00", 0},
		{"", 0},
		{"-R$ 50.00", -50.00},
		{"20.50", 20.50},
		{"4,00", 4.00},
		{"abc", 0},
	}
	for _, c := range cases {
		if got := parseMoney(c.in); !almostEqual(got, c.want) {
			t.Errorf("parseMoney(%q) = %v, quer %v", c.in, got, c.want)
		}
	}
}

func TestDetectColumns_LayoutNovo(t *testing.T) {
	// Export novo (18 colunas, com V.IPI e V.BC Antecip.)
	header := []string{
		"Data Emissão", "Número NF-e", "Fornecedor", "CNPJ", "UF", "CFOP", "Regime",
		"V.Prod", "V.IPI", "ICMS Atual", "V.BC ST", "V.BC Antecip.", "V.ST",
		"Alíq.Inter.%", "Alíq.Interna.%", "ICMS Devido Est.", "Chave NF-e", "Chave CT-e",
	}
	ci := detectColumns(header)
	if ci.chaveNFe != 16 || ci.chaveCTe != 17 || ci.icmsDevido != 15 || ci.vProd != 7 ||
		ci.numeroNFe != 1 || ci.fornecedor != 2 || ci.cfop != 5 ||
		ci.aliqInter != 13 || ci.aliqInterna != 14 || ci.vIPI != 8 {
		t.Errorf("layout novo mapeado errado: %+v", ci)
	}
}

func TestDetectColumns_LayoutAntigo(t *testing.T) {
	// Export antigo do contador (16 colunas, sem V.IPI/V.BC Antecip.)
	header := []string{
		"Data Emissão", "Número NF-e", "Fornecedor", "CNPJ", "UF", "CFOP", "Regime",
		"V.Prod", "ICMS Atual", "V.BC ST", "V.ST",
		"Alíq.Inter.%", "Alíq.Interna.%", "ICMS Devido Est.", "Chave NF-e", "Chave CT-e",
	}
	ci := detectColumns(header)
	if ci.chaveNFe != 14 || ci.chaveCTe != 15 || ci.icmsDevido != 13 || ci.vProd != 7 ||
		ci.aliqInter != 11 || ci.aliqInterna != 12 || ci.vIPI != -1 {
		t.Errorf("layout antigo mapeado errado: %+v", ci)
	}
}

func TestDetectColumns_BlocoC(t *testing.T) {
	header := []string{
		"Data Emissão", "NF-e", "Fornecedor", "CNPJ", "UF", "CFOP Saída", "Regime",
		"V.Operação", "ICMS Est.", "Classificação", "Chave NF-e", "Chave CT-e",
	}
	ci := detectColumns(header)
	if ci.chaveNFe != 10 || ci.chaveCTe != 11 || ci.icmsDevido != 8 || ci.vProd != 7 || ci.numeroNFe != 1 {
		t.Errorf("bloco C mapeado errado: %+v", ci)
	}
}

func TestDiagnoseCausa(t *testing.T) {
	if c := diagnoseCausa("only_1", parsedRow{}, parsedRow{}); !strings.Contains(c, "conferência") {
		t.Errorf("only_1: %q", c)
	}
	if c := diagnoseCausa("only_2", parsedRow{}, parsedRow{}); !strings.Contains(c, "correta") {
		t.Errorf("only_2: %q", c)
	}
	// Base de cálculo difere (V.Prod diverge)
	r1 := parsedRow{vProd: 89904.99, icmsDevido: 15285.78}
	r2 := parsedRow{vProd: 84828.18, icmsDevido: 14245.10}
	if c := diagnoseCausa("diff", r1, r2); !strings.Contains(c, "Base de cálculo") {
		t.Errorf("base: %q", c)
	}
	// Alíquota interestadual difere (mesma base)
	r1 = parsedRow{vProd: 1000, aliqInter: 4, aliqInterna: 20.5, icmsDevido: 165}
	r2 = parsedRow{vProd: 1000, aliqInter: 12, aliqInterna: 20.5, icmsDevido: 85}
	if c := diagnoseCausa("diff", r1, r2); !strings.Contains(c, "interestadual") {
		t.Errorf("inter: %q", c)
	}
	// IPI na base confirmado: P2 tem V.IPI; Δ = V.IPI × interna
	// V.IPI=411.80, interna=20.5 → efeito 84.42
	r1 = parsedRow{vProd: 1000, aliqInter: 4, aliqInterna: 20.5, icmsDevido: 1000, vIPI: 0}
	r2 = parsedRow{vProd: 1000, aliqInter: 4, aliqInterna: 20.5, icmsDevido: 1084.42, vIPI: 411.80}
	if c := diagnoseCausa("diff", r1, r2); !strings.Contains(c, "IPI na base") {
		t.Errorf("ipi: %q", c)
	}
	// Mesma base e alíquotas → crédito/cálculo
	r1 = parsedRow{vProd: 1000, aliqInter: 4, aliqInterna: 20.5, icmsDevido: 165}
	r2 = parsedRow{vProd: 1000, aliqInter: 4, aliqInterna: 20.5, icmsDevido: 170}
	if c := diagnoseCausa("diff", r1, r2); !strings.Contains(c, "crédito") {
		t.Errorf("credito: %q", c)
	}
}

func TestRoundFloat(t *testing.T) {
	if roundFloat(1.005, 2) != 1.01 && roundFloat(1.004, 2) != 1.00 {
		t.Errorf("roundFloat falhou")
	}
}

// buildXLSX cria um arquivo xlsx em memória com as abas/linhas fornecidas.
func buildXLSX(t *testing.T, sheets map[string][][]string) []byte {
	t.Helper()
	f := excelize.NewFile()
	first := true
	for name, rows := range sheets {
		if first {
			f.SetSheetName("Sheet1", name)
			first = false
		} else {
			f.NewSheet(name)
		}
		for r, row := range rows {
			for c, val := range row {
				cell, _ := excelize.CoordinatesToCellName(c+1, r+1)
				f.SetCellValue(name, cell, val)
			}
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func abHeaderNovo() []string {
	return []string{"Data", "Número NF-e", "Fornecedor", "CNPJ", "UF", "CFOP", "Regime",
		"V.Prod", "V.IPI", "ICMS Atual", "V.BC ST", "V.BC Antecip.", "V.ST",
		"Alíq.Inter.%", "Alíq.Interna.%", "ICMS Devido Est.", "Chave NF-e", "Chave CT-e"}
}
func abHeaderAntigo() []string {
	return []string{"Data", "Número NF-e", "Fornecedor", "CNPJ", "UF", "CFOP", "Regime",
		"V.Prod", "ICMS Atual", "V.BC ST", "V.ST",
		"Alíq.Inter.%", "Alíq.Interna.%", "ICMS Devido Est.", "Chave NF-e", "Chave CT-e"}
}

func TestCompareBlocosEndToEnd(t *testing.T) {
	// Planilha 1 (correta, layout antigo): NF A (igual), NF B (só na 1), NF D (IPI na base)
	p1 := buildXLSX(t, map[string][][]string{
		"B - Mês Atual": {
			abHeaderAntigo(),
			{"d", "100", "FORN A", "c", "PE", "2102", "ANTECIPACAO", "R$ 1,000.00", "R$ 50.00", "R$ 0.00", "R$ 0.00", "4.00", "20.50", "R$ 165.00", "CHAVE_A", ""},
			{"d", "200", "FORN B", "c", "PE", "2102", "ANTECIPACAO", "R$ 500.00", "R$ 20.00", "R$ 0.00", "R$ 0.00", "4.00", "20.50", "R$ 82.50", "CHAVE_B", ""},
			{"d", "400", "FORN D", "c", "PE", "2102", "ANTECIPACAO", "R$ 1,000.00", "R$ 50.00", "R$ 0.00", "R$ 0.00", "4.00", "20.50", "R$ 165.00", "CHAVE_D", ""},
		},
	})
	// Planilha 2 (conferência, layout novo): NF A (igual), NF C (só na 2), NF D (ICMS maior = IPI×20.5%)
	p2 := buildXLSX(t, map[string][][]string{
		"B - Mês Atual": {
			abHeaderNovo(),
			{"d", "100", "FORN A", "c", "PE", "2102", "ANTECIPACAO", "R$ 1,000.00", "R$ 0.00", "R$ 50.00", "R$ 0.00", "R$ 1,000.00", "R$ 0.00", "4.00", "20.50", "R$ 165.00", "CHAVE_A", ""},
			{"d", "300", "FORN C", "c", "PE", "2102", "ANTECIPACAO", "R$ 700.00", "R$ 0.00", "R$ 30.00", "R$ 0.00", "R$ 700.00", "R$ 0.00", "4.00", "20.50", "R$ 115.50", "CHAVE_C", ""},
			{"d", "400", "FORN D", "c", "PE", "2102", "ANTECIPACAO", "R$ 1,000.00", "R$ 100.00", "R$ 50.00", "R$ 0.00", "R$ 1,100.00", "R$ 0.00", "4.00", "20.50", "R$ 185.50", "CHAVE_D", ""},
		},
	})

	f1, err := excelize.OpenReader(bytes.NewReader(p1))
	if err != nil {
		t.Fatal(err)
	}
	defer f1.Close()
	f2, err := excelize.OpenReader(bytes.NewReader(p2))
	if err != nil {
		t.Fatal(err)
	}
	defer f2.Close()

	s1 := findSheetsByKeyword(f1)
	s2 := findSheetsByKeyword(f2)
	diffs := compareBlocos(f1, f2, s1["atual"], s2["atual"])

	byChave := map[string]DiffRow{}
	for _, d := range diffs {
		byChave[d.ChaveNFe] = d
	}

	// A deve estar igual (não aparece)
	if _, ok := byChave["CHAVE_A"]; ok {
		t.Errorf("CHAVE_A igual não deveria aparecer")
	}
	// B só na P1
	if byChave["CHAVE_B"].Status != "only_1" {
		t.Errorf("CHAVE_B esperava only_1, veio %q", byChave["CHAVE_B"].Status)
	}
	// C só na P2
	if byChave["CHAVE_C"].Status != "only_2" {
		t.Errorf("CHAVE_C esperava only_2, veio %q", byChave["CHAVE_C"].Status)
	}
	// D divergente com causa IPI na base (185.50 - 165 = 20.50 = 100 × 20.5%)
	d := byChave["CHAVE_D"]
	if d.Status != "diff" {
		t.Errorf("CHAVE_D esperava diff, veio %q", d.Status)
	}
	if !almostEqual(d.DiffICMS, -20.50) {
		t.Errorf("CHAVE_D diff esperado -20.50, veio %.2f", d.DiffICMS)
	}
	if !strings.Contains(d.Causa, "IPI na base") {
		t.Errorf("CHAVE_D causa esperava IPI na base, veio %q", d.Causa)
	}
}

func TestComparativoHandlerHTTP(t *testing.T) {
	mk := func(sheet string, hdr []string) []byte {
		return buildXLSX(t, map[string][][]string{sheet: {hdr,
			{"d", "100", "FORN A", "c", "PE", "2102", "ANTECIPACAO", "R$ 1,000.00", "R$ 50.00", "R$ 0.00", "R$ 0.00", "4.00", "20.50", "R$ 165.00", "CHAVE_A", ""},
		}})
	}
	p1 := mk("B - Mês Atual", abHeaderAntigo())
	p2 := mk("B - Mês Atual", abHeaderAntigo())

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for field, data := range map[string][]byte{"file1": p1, "file2": p2} {
		fw, _ := mw.CreateFormFile(field, field+".xlsx")
		fw.Write(data)
	}
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/icms-fronteira/comparativo", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()

	IcmsFronteiraComparativoHandler()(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, corpo: %s", rec.Code, rec.Body.String())
	}
	var resp ComparativoResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json inválido: %v", err)
	}
	// planilhas idênticas → sem divergências no bloco B
	if len(resp.BlocoB) != 0 {
		t.Errorf("esperava 0 divergências, veio %d", len(resp.BlocoB))
	}
}

func TestComparativoHandlerMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/icms-fronteira/comparativo", nil)
	rec := httptest.NewRecorder()
	IcmsFronteiraComparativoHandler()(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET esperava 405, veio %d", rec.Code)
	}
}
