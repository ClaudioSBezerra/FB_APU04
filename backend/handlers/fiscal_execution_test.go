package handlers

import "testing"

func TestTipoContribuinte(t *testing.T) {
	tests := []struct {
		indIE          string
		indFinal       string
		cfop           string
		modelo         int
		ufOrig, ufDest string
		want           string
	}{
		{"1", "0", "6102", 55, "PB", "SP", "S"},   // contribuinte p/ revenda (interestadual)
		{"2", "0", "6102", 55, "PB", "SP", "S"},   // contribuinte isento de IE
		{"9", "1", "6102", 55, "PB", "SP", "N"},   // NÃO contribuinte (PJ/PF) → caso DIFAL
		{"9", "1", "5102", 65, "PB", "PB", "N"},   // NFC-e não contribuinte
		{" 9 ", "1", "6102", 55, "PB", "SP", "N"}, // com espaços
		{"1", "0", "6108", 55, "PB", "SP", "S"},   // indIEDest tem precedência sobre o CFOP
		{"", "0", "6108", 55, "PB", "SP", "N"},    // sem indIEDest, CFOP 6108 → não contribuinte
		{"", "0", "6107", 55, "PB", "SP", "N"},    // sem indIEDest, CFOP 6107 → não contribuinte
		{"", "0", "6102", 55, "PB", "SP", "S"},    // sem indIEDest, CFOP comum → fallback modelo (NF-e)
		{"", "0", "5102", 65, "PB", "PB", "N"},    // sem indIEDest → fallback por modelo (NFC-e)
		{"", "", "", 0, "", "", "N"},              // desconhecido → default conservador
		// indFinal=1 em operação INTERNA: contribuinte comprando como consumidor
		// final → "N" (adicional/FECOP). Caso NF 572900 (CASA DO ESCAPAMENTO).
		{"1", "1", "5102", 55, "PB", "PB", "N"}, // contribuinte + consumidor final + mesma UF → N
		{"2", "1", "5102", 55, "PB", "PB", "N"}, // isento IE + consumidor final interno → N
		{"1", "1", "6102", 55, "PB", "SP", "S"}, // consumidor final INTERESTADUAL → NÃO vira N (DIFAL) → S
		{"1", "0", "5102", 55, "PB", "PB", "S"}, // contribuinte revenda mesma UF (indFinal=0) → S
	}
	for _, tc := range tests {
		if got := tipoContribuinte(tc.indIE, tc.indFinal, tc.cfop, tc.modelo, tc.ufOrig, tc.ufDest); got != tc.want {
			t.Errorf("tipoContribuinte(%q, %q, %q, %d, %q, %q) = %q, want %q", tc.indIE, tc.indFinal, tc.cfop, tc.modelo, tc.ufOrig, tc.ufDest, got, tc.want)
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
