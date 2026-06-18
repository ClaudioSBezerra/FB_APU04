package handlers

import (
	"os"
	"strings"
	"testing"
)

// TestAuditoriaReal roda a auditoria com os arquivos reais do cliente (se
// presentes em /tmp/TESTE BLOCO E/). Skipa se os arquivos não estiverem lá.
func TestAuditoriaReal(t *testing.T) {
	base := "/tmp/TESTE BLOCO E/"
	sped := base + "SpedEFD-06314327000203-103742352-Remessa de arquivo substituto-mai.2026.txt"
	f, err := os.Open(sped)
	if err != nil {
		t.Skip("arquivos de teste ausentes; skip")
	}
	defer f.Close()

	a, err := parseSpedAuditoria(f)
	if err != nil {
		t.Fatalf("parseSpedAuditoria: %v", err)
	}
	if a.CNPJ != "06314327000203" {
		t.Errorf("CNPJ=%q", a.CNPJ)
	}
	if a.Competencia != "05/2026" {
		t.Errorf("Competencia=%q", a.Competencia)
	}
	if !bateValor(a.E110Recolher, 5807361.65) {
		t.Errorf("E110Recolher=%.2f, quer 5807361.65", a.E110Recolher)
	}
	if !bateValor(a.E115Protege, 1309614.58) {
		t.Errorf("E115Protege=%.2f, quer 1309614.58", a.E115Protege)
	}

	for _, fn := range []string{"ICMS NORMAL 05-2026.pdf", "ICMS NORMAL COMPLEMENTAR 05-2026.pdf", "PROTEGE 05-2026.pdf", "ADICIONAL 2% 05-2026.pdf"} {
		g, err := parseDarePDF(base+fn, fn)
		if err != nil {
			t.Fatalf("parseDarePDF %s: %v", fn, err)
		}
		t.Logf("GUIA %s -> receita=%s valor=%.2f ref=%s comp=%s venc=%s", fn, g.CodReceita, g.ValorOriginal, g.RefCodigo, g.RefCompetencia, g.Vencimento)
		a.Guias = append(a.Guias, g)
	}

	out := auditar(a)
	for _, c := range out.Conciliacao {
		t.Logf("CONCIL %s: EFD=%.2f Guia=%.2f dif=%.2f OK=%v", c.Tributo, c.ValorEFD, c.ValorGuia, c.Diferenca, c.OK)
	}
	for _, d := range out.Divergencias {
		t.Logf("DIVERG: %s", d)
	}

	// Conciliações esperadas (todas batem com a soma correta):
	if g := a.somaGuias("108"); !bateValor(g, 5807361.65) {
		t.Errorf("Σ guias 108=%.2f, quer 5807361.65", g)
	}
	if g := a.somaGuias("4014"); !bateValor(g, 1309614.58) {
		t.Errorf("Σ guias 4014=%.2f, quer 1309614.58", g)
	}
	if g := a.somaGuias("4146"); !bateValor(g, 111117.78) {
		t.Errorf("Σ guias 4146=%.2f, quer 111117.78", g)
	}
	for _, c := range out.Conciliacao {
		if !c.OK {
			t.Errorf("conciliação NÃO bateu: %s (EFD %.2f × Guia %.2f)", c.Tributo, c.ValorEFD, c.ValorGuia)
		}
	}

	// Render não vazio e contém marcadores-chave.
	html := renderAuditoriaHTML(out)
	for _, want := range []string{"DASHBOARD DE AUDITORIA", "ICMS Normal (108)", "PROTEGE", "Adicional ICMS 2%", "06.314.327/0002-03"} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML sem marcador %q", want)
		}
	}
}
