package handlers

import "testing"

// onlyDigits — normaliza CNPJ/qualquer string para só dígitos. Usado pelo motor
// de fronteira para casar CNPJ entre prodepe_enquadramentos e import_jobs/nfe_entradas.
func TestOnlyDigits(t *testing.T) {
	cases := map[string]string{
		"01.612.046/0001-24": "01612046000124",
		"01612046000124":     "01612046000124",
		"":                   "",
		"abc":                "",
		"12.345-67.890":      "1234567890",
		"  01612046000124  ": "01612046000124",
		"01a23b45":           "012345",
	}
	for in, want := range cases {
		if got := onlyDigits(in); got != want {
			t.Errorf("onlyDigits(%q) = %q, want %q", in, got, want)
		}
	}
}

// extractNcmsFromText — extrai NCMs do texto livre de decretos. Testa o caminho
// principal (com descrição via "<desc> - NCM <ncm>") e o fallback (NCMs avulsos).
func TestExtractNcmsFromText_DescriptionAndDedup(t *testing.T) {
	text := `
Art. 1º Ficam beneficiados os produtos:
aguarrás mineral - NCM 2710.12.30; desengraxante - NCM 2710.12.49;
verniz - NCM 3208.10.20.
Item adicional sem descrição: 2710.19.19.
Repetido (não deve duplicar): aguarrás - NCM 2710.12.30.
`
	got := extractNcmsFromText(text)
	if len(got) != 4 {
		t.Fatalf("expected 4 unique NCMs, got %d: %+v", len(got), got)
	}

	idx := map[string]string{}
	for _, n := range got {
		idx[n.NCM] = n.Descricao
	}

	// NCMs esperados (8 dígitos, sem pontos)
	for _, ncm := range []string{"27101230", "27101249", "32081020", "27101919"} {
		if _, ok := idx[ncm]; !ok {
			t.Errorf("NCM %q ausente do resultado: %+v", ncm, got)
		}
	}

	// Descrições devem ser capturadas para os NCMs precedidos por "<desc> - NCM"
	if idx["27101230"] == "" {
		t.Errorf("descrição vazia p/ 27101230 — esperava capturar 'aguarrás mineral'")
	}
	// NCM avulso (sem pattern de descrição) deve vir com Descricao vazia
	if idx["27101919"] != "" {
		t.Errorf("27101919 era NCM avulso — descrição deveria ser vazia, veio %q", idx["27101919"])
	}
}

func TestExtractNcmsFromText_EmptyAndNoise(t *testing.T) {
	// Texto sem NCM válido — não deve achar nada
	cases := []string{
		"",
		"texto sem nenhum padrão de ncm aqui",
		"Lei nº 11.675, de 11 de outubro de 1999",         // ano "1999" não é NCM
		"art. 1º, inciso IV — alíquota de 20% sobre o BC", // só prosa
	}
	for _, txt := range cases {
		if got := extractNcmsFromText(txt); len(got) != 0 {
			t.Errorf("extractNcmsFromText(%q) achou %d NCMs falsos: %+v", txt, len(got), got)
		}
	}
}

func TestExtractNcmsFromText_PreservesOrder(t *testing.T) {
	text := `verniz - NCM 3208.10.20; aguarrás - NCM 2710.12.30; diluidor - NCM 3814.00.20.`
	got := extractNcmsFromText(text)
	want := []string{"32081020", "27101230", "38140020"}
	if len(got) != len(want) {
		t.Fatalf("got %d, want %d", len(got), len(want))
	}
	for i, n := range got {
		if n.NCM != want[i] {
			t.Errorf("pos %d: got %q, want %q", i, n.NCM, want[i])
		}
	}
}
