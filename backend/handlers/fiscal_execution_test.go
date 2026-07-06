package handlers

import "testing"

func TestTipoContribuinte(t *testing.T) {
	tests := []struct {
		indIE  string
		cfop   string
		modelo int
		want   string
	}{
		{"1", "6102", 55, "S"},   // contribuinte ICMS
		{"2", "6102", 55, "S"},   // contribuinte isento de IE
		{"9", "6102", 55, "N"},   // NF-e para NÃO contribuinte (PJ/PF) → caso DIFAL
		{"9", "5102", 65, "N"},   // NFC-e não contribuinte
		{" 9 ", "6102", 55, "N"}, // com espaços
		{"1", "6108", 55, "S"},   // indIEDest tem precedência sobre o CFOP
		{"", "6108", 55, "N"},    // sem indIEDest, CFOP 6108 → não contribuinte
		{"", "6107", 55, "N"},    // sem indIEDest, CFOP 6107 → não contribuinte
		{"", "6102", 55, "S"},    // sem indIEDest, CFOP comum → fallback modelo (NF-e)
		{"", "5102", 65, "N"},    // sem indIEDest → fallback por modelo (NFC-e)
		{"", "", 0, "N"},         // desconhecido → default conservador
	}
	for _, tc := range tests {
		if got := tipoContribuinte(tc.indIE, tc.cfop, tc.modelo); got != tc.want {
			t.Errorf("tipoContribuinte(%q, %q, %d) = %q, want %q", tc.indIE, tc.cfop, tc.modelo, got, tc.want)
		}
	}
}

func TestTipoOperacaoPorCFOP(t *testing.T) {
	tests := []struct {
		cfop string
		want int
	}{
		{"5405", 1},    // venda ST
		{"5102", 1},    // venda
		{"6108", 1},    // venda interestadual
		{"5151", 20},   // transferência
		{"5152", 20},   // transferência
		{"5409", 20},   // transferência ST
		{"6152", 20},   // transferência interestadual
		{" 6408 ", 20}, // com espaços
		{"", 1},        // vazio → venda (default)
	}
	for _, tc := range tests {
		if got := tipoOperacaoPorCFOP(tc.cfop); got != tc.want {
			t.Errorf("tipoOperacaoPorCFOP(%q) = %d, want %d", tc.cfop, got, tc.want)
		}
	}
}
