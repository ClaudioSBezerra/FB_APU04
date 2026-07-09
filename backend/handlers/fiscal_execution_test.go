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

func TestCentrosFiscais(t *testing.T) {
	tests := []struct {
		cfop           string
		mesmaEmpresa   bool
		ufOrig, ufDest string
		first          string
	}{
		{"5152", false, "PB", "PB", "CDNE"},  // transferência → CDNE primeiro
		{"6152", false, "PB", "SP", "CDNE"},  // transferência interestadual
		{"5408", false, "PB", "PB", "CDNE"},  // transferência ST
		{"5102", false, "PB", "PB", "VRJNE"}, // venda MESMA UF (PB→PB) → VRJNE
		{"5405", false, "PB", "PB", "VRJNE"}, // venda ST intraestadual → VRJNE
		{"6102", false, "PB", "SP", "CDNE"},  // venda INTERESTADUAL (PB→SP) → CDNE
		{"6108", false, "PB", "RJ", "CDNE"},  // venda interestadual não contrib. → CDNE
		{"", false, "PB", "PB", "VRJNE"},     // default venda intraestadual
		{"5102", false, "", "", "VRJNE"},     // UF vazia → assume intraestadual (VRJNE)
		{"5949", true, "PB", "SP", "CDNE"},   // saída entre filiais (CNPJ próprio) → CDNE
		{"5102", true, "PB", "PB", "CDNE"},   // qualquer CFOP p/ mesma empresa → CDNE
	}
	for _, tc := range tests {
		got := centrosFiscais(tc.cfop, tc.mesmaEmpresa, tc.ufOrig, tc.ufDest)
		if len(got) != 2 || got[0] != tc.first {
			t.Errorf("centrosFiscais(%q, %v, %q, %q) = %v, want [%s ...] com fallback", tc.cfop, tc.mesmaEmpresa, tc.ufOrig, tc.ufDest, got, tc.first)
		}
		if got[0] == got[1] {
			t.Errorf("centrosFiscais(%q, %v, %q, %q): fallback igual ao primeiro (%v)", tc.cfop, tc.mesmaEmpresa, tc.ufOrig, tc.ufDest, got)
		}
	}
}

func TestMesmaEmpresa(t *testing.T) {
	tests := []struct {
		emit, dest string
		want       bool
	}{
		{"10230480001960", "10230480001536", true},  // filiais Ferreira Costa
		{"10230480001960", "05208211000138", false}, // cliente PJ
		{"10230480001960", "", false},               // consumidor (CPF/sem dest)
		{"", "10230480001536", false},
	}
	for _, tc := range tests {
		if got := mesmaEmpresa(tc.emit, tc.dest); got != tc.want {
			t.Errorf("mesmaEmpresa(%q, %q) = %v, want %v", tc.emit, tc.dest, got, tc.want)
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
