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
		cfop         string
		mesmaEmpresa bool
		first        string
	}{
		{"5152", false, "CDNE"},  // transferência → CDNE primeiro
		{"6152", false, "CDNE"},  // transferência interestadual
		{"5408", false, "CDNE"},  // transferência ST
		{"5102", false, "VRJNE"}, // venda → VRJNE primeiro
		{"5405", false, "VRJNE"}, // venda ST
		{"", false, "VRJNE"},     // default venda
		{"5949", true, "CDNE"},   // outra saída entre filiais (CNPJ próprio) → CDNE
		{"5102", true, "CDNE"},   // qualquer CFOP p/ mesma empresa → CDNE
	}
	for _, tc := range tests {
		got := centrosFiscais(tc.cfop, tc.mesmaEmpresa)
		if len(got) != 2 || got[0] != tc.first {
			t.Errorf("centrosFiscais(%q, %v) = %v, want [%s ...] com fallback", tc.cfop, tc.mesmaEmpresa, got, tc.first)
		}
		if got[0] == got[1] {
			t.Errorf("centrosFiscais(%q, %v): fallback igual ao primeiro (%v)", tc.cfop, tc.mesmaEmpresa, got)
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
