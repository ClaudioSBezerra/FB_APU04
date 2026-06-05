package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Guards (método + auth) dos handlers do demonstrativo ST por item e de fretes
// pendentes. Não tocam o banco: o check de método (405) e o de claims (401)
// acontecem antes de qualquer uso do *sql.DB, então passar nil é seguro.

func guardMethod(t *testing.T, name, path string, h http.HandlerFunc) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("%s POST: got %d, want %d", name, rr.Code, http.StatusMethodNotAllowed)
	}
}

func guardNoAuth(t *testing.T, name, path string, h http.HandlerFunc) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("%s GET no-auth: got %d, want %d", name, rr.Code, http.StatusUnauthorized)
	}
}

func TestSTItensHandlers_Guards(t *testing.T) {
	cases := []struct {
		name string
		path string
		h    http.HandlerFunc
	}{
		{"STItens", "/api/icms-fronteira/st-itens", IcmsFronteiraSTItensHandler(nil)},
		{"STItensXLSX", "/api/icms-fronteira/st-itens/exportar/xlsx", IcmsFronteiraSTItensXLSXHandler(nil)},
		{"STItensHTML", "/api/icms-fronteira/st-itens/exportar/pdf", IcmsFronteiraSTItensHTMLHandler(nil)},
		{"FretesPendentes", "/api/icms-fronteira/fretes-pendentes", IcmsFronteiraFretesPendentesHandler(nil)},
		{"FretesPendentesXLSX", "/api/icms-fronteira/fretes-pendentes/exportar/xlsx", IcmsFronteiraFretesPendentesXLSXHandler(nil)},
	}
	for _, c := range cases {
		guardMethod(t, c.name, c.path, c.h)
		guardNoAuth(t, c.name, c.path, c.h)
	}
}

func almost(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 0.01
}

func TestComputeST(t *testing.T) {
	// 1) Com regra + segmento: base = oper*(1+MVAaj); icms calc = BCred*aliqI - debito; a pagar = calc - retido
	r := STItemRow{VProd: 1000, VIPI: 0, VOutro: 0, TemRegra: true, SegmentoOK: true,
		MVAAjustado: 100, AliqInterna: 20.5, IcmsDebitado: 70, IcmsRetido: 0}
	r.computeST()
	if !almost(r.VOperacao, 1000) || !almost(r.BaseCalculo, 2000) {
		t.Errorf("base: oper=%.2f base=%.2f (want 1000/2000)", r.VOperacao, r.BaseCalculo)
	}
	if !almost(r.IcmsCalculado, 2000*0.205-70) { // 410-70=340
		t.Errorf("icms calc=%.2f want 340", r.IcmsCalculado)
	}
	if !almost(r.IcmsAPagar, 340) {
		t.Errorf("a pagar=%.2f want 340", r.IcmsAPagar)
	}

	// 2) Sem regra/segmento → base e derivados zerados
	r2 := STItemRow{VProd: 500, TemRegra: false, SegmentoOK: false, AliqInterna: 20.5}
	r2.computeST()
	if r2.BaseCalculo != 0 || r2.IcmsCalculado != 0 || r2.IcmsAPagar != 0 {
		t.Errorf("sem regra deveria zerar: base=%.2f calc=%.2f pagar=%.2f", r2.BaseCalculo, r2.IcmsCalculado, r2.IcmsAPagar)
	}

	// 3) Retido >= calculado → a pagar piso 0
	r3 := STItemRow{VProd: 1000, TemRegra: true, SegmentoOK: true, MVAAjustado: 100,
		AliqInterna: 20.5, IcmsDebitado: 70, IcmsRetido: 999}
	r3.computeST()
	if r3.IcmsAPagar != 0 {
		t.Errorf("retido alto: a pagar=%.2f want 0", r3.IcmsAPagar)
	}

	// 4) Redução de BC aplicada
	r4 := STItemRow{VProd: 1000, TemRegra: true, SegmentoOK: true, MVAAjustado: 0,
		AliqInterna: 20.0, ReducaoBC: 50, IcmsDebitado: 0}
	r4.computeST()
	if !almost(r4.BCReduzida, 500) { // base 1000, -50% = 500
		t.Errorf("bc reduzida=%.2f want 500", r4.BCReduzida)
	}
}

func TestGroupSTItens(t *testing.T) {
	rows := []STItemRow{
		{ChaveNFe: "A", NumeroNFe: "1"},
		{ChaveNFe: "A", NumeroNFe: "1"},
		{ChaveNFe: "B", NumeroNFe: "2"},
	}
	g := groupSTItens(rows)
	if len(g) != 2 {
		t.Fatalf("grupos=%d want 2", len(g))
	}
	if g[0].Chave != "A" || len(g[0].Itens) != 2 {
		t.Errorf("grupo A: chave=%s itens=%d", g[0].Chave, len(g[0].Itens))
	}
	if g[1].Chave != "B" || len(g[1].Itens) != 1 {
		t.Errorf("grupo B: chave=%s itens=%d", g[1].Chave, len(g[1].Itens))
	}
}

func TestFmtDataBR(t *testing.T) {
	cases := map[string]string{
		"2026-04-15T10:20:30": "15/04/2026",
		"2026-04-15":          "15/04/2026",
		"2026-12-01 00:00:00": "01/12/2026",
		"":                    "",
		"x":                   "x",
	}
	for in, want := range cases {
		if got := fmtDataBR(in); got != want {
			t.Errorf("fmtDataBR(%q)=%q want %q", in, got, want)
		}
	}
}
