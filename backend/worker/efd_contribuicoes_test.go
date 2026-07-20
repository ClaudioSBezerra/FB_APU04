package worker

import (
	"strings"
	"testing"
)

// TestParseC100Fields_AssumedLayout valida a extração de campos de uma linha
// C100 sintética construída a partir do layout documentado em
// processEFDContribuicoesFile (worker.go) — confirmado em 2026-07-20 contra
// um arquivo real de produção (EFD_CONTRIB_092021_V2.txt), não apenas contra
// o Guia Prático oficial.
func TestParseC100Fields_AssumedLayout(t *testing.T) {
	// |C100|IND_OPER|IND_EMIT|COD_PART|COD_MOD|COD_SIT|SER|NUM_DOC|CHV_NFE|
	// DT_DOC|DT_E_S|VL_DOC|IND_PGTO|VL_DESC|VL_ABAT_NT|VL_MERC|IND_FRT|VL_FRT|
	// VL_SEG|VL_OUT_DA|VL_BC_ICMS|VL_ICMS|VL_BC_ICMS_ST|VL_ICMS_ST|VL_IPI|
	// VL_PIS|VL_COFINS|VL_PIS_ST|VL_COFINS_ST|
	chave := "35200714200166000166550010000123451234567890"
	line := strings.Join([]string{
		"", "C100",
		"0",             // IND_OPER (2) -> entrada
		"0",             // IND_EMIT (3)
		"12345",         // COD_PART (4)
		"55",            // COD_MOD (5)
		"00",            // COD_SIT (6)
		"1",             // SER (7)
		"12345",         // NUM_DOC (8)
		chave,           // CHV_NFE (9)
		"01012026",      // DT_DOC (10)
		"01012026",      // DT_E_S (11)
		"1000,00",       // VL_DOC (12)
		"0",             // IND_PGTO (13)
		"0,00",          // VL_DESC (14)
		"0,00",          // VL_ABAT_NT (15)
		"1000,00",       // VL_MERC (16)
		"0",             // IND_FRT (17)
		"0,00",          // VL_FRT (18)
		"0,00",          // VL_SEG (19)
		"0,00",          // VL_OUT_DA (20)
		"1000,00",       // VL_BC_ICMS (21)
		"180,00",        // VL_ICMS (22)
		"0,00",          // VL_BC_ICMS_ST (23)
		"0,00",          // VL_ICMS_ST (24)
		"0,00",          // VL_IPI (25)
		"16,50",         // VL_PIS (26)
		"76,00",         // VL_COFINS (27)
		"0,00",          // VL_PIS_ST (28) -- kept even though not required by len check
		"0,00",          // VL_COFINS_ST (29)
		"",
	}, "|")

	parts := strings.Split(line, "|")
	fields, ok := parseC100Fields(parts)
	if !ok {
		t.Fatalf("expected parseC100Fields to succeed, got ok=false (len(parts)=%d)", len(parts))
	}
	if fields.IndOper != "0" {
		t.Errorf("IndOper = %q, want %q", fields.IndOper, "0")
	}
	if fields.ChvNFe != chave {
		t.Errorf("ChvNFe = %q, want %q", fields.ChvNFe, chave)
	}
	if fields.VlPis != 16.50 {
		t.Errorf("VlPis = %v, want %v", fields.VlPis, 16.50)
	}
	if fields.VlCofins != 76.00 {
		t.Errorf("VlCofins = %v, want %v", fields.VlCofins, 76.00)
	}
}

// TestParseC100Fields_TooShort garante que uma linha truncada/layout
// inesperado não derruba o job — o chamador deve pular a linha.
func TestParseC100Fields_TooShort(t *testing.T) {
	parts := strings.Split("|C100|0|0|123|", "|")
	_, ok := parseC100Fields(parts)
	if ok {
		t.Fatalf("expected ok=false for a truncated C100 line, got true")
	}
}

// TestParseC100Fields_IndOperSaida garante que IND_OPER="1" (saída) é
// extraído corretamente — o dispatch para nfe_saidas vs nfe_entradas mora no
// chamador (processEFDContribuicoesFile), mas depende deste campo.
func TestParseC100Fields_IndOperSaida(t *testing.T) {
	fieldsRaw := make([]string, 30)
	fieldsRaw[1] = "C100"
	fieldsRaw[2] = "1" // saída
	fieldsRaw[9] = "35200714200166000166550010000123451234567890"
	fieldsRaw[26] = "10,00"
	fieldsRaw[27] = "46,15"
	line := strings.Join(fieldsRaw, "|")
	parts := strings.Split(line, "|")

	fields, ok := parseC100Fields(parts)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if fields.IndOper != "1" {
		t.Errorf("IndOper = %q, want %q", fields.IndOper, "1")
	}
	if fields.VlPis != 10.00 {
		t.Errorf("VlPis = %v, want 10.00", fields.VlPis)
	}
	if fields.VlCofins != 46.15 {
		t.Errorf("VlCofins = %v, want 46.15", fields.VlCofins)
	}
}
