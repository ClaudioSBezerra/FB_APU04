package handlers

import "testing"

func TestTipoContribuintePorModelo(t *testing.T) {
	tests := []struct {
		modelo int
		want   string
	}{
		{55, "S"}, // NF-e → contribuinte
		{65, "N"}, // NFC-e → consumidor final
		{0, "N"},  // desconhecido → default conservador
	}
	for _, tc := range tests {
		if got := tipoContribuintePorModelo(tc.modelo); got != tc.want {
			t.Errorf("tipoContribuintePorModelo(%d) = %q, want %q", tc.modelo, got, tc.want)
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
