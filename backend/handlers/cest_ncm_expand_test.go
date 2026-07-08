package handlers

import "testing"

func TestRomanToInt(t *testing.T) {
	cases := map[string]int{
		"II": 2, "VI": 6, "IX": 9, "X": 10, "XIV": 14,
		"XXV": 25, "XXVI": 26, "I": 1, "": 0, "ABC": 0,
	}
	for in, want := range cases {
		if got := romanToInt(in); got != want {
			t.Errorf("romanToInt(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestSegmentoDoAnexo(t *testing.T) {
	cases := []struct {
		ref  string
		want string
	}{
		{"Anexo II do Conv. ICMS 52/2017", "01"}, // autopeças (linear: 2-1)
		{"Anexo VI do Conv. ICMS 52/2017", "05"}, // combustíveis
		{"Anexo XXV do Conv. ICMS 52/17", "24"},  // último linear
		{"Anexo XXVI do Conv. ICMS 52/17", "28"}, // override: porta a porta
		{"anexo ii", "01"},                       // case-insensitive
		{"Anexo I", ""},                          // regras gerais, não é segmento
		{"sem anexo aqui", ""},                   // não reconhece
	}
	for _, c := range cases {
		if got := segmentoDoAnexo(c.ref); got != c.want {
			t.Errorf("segmentoDoAnexo(%q) = %q, want %q", c.ref, got, c.want)
		}
	}
}
