package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// ── aliqInterestadual ────────────────────────────────────────────────────────

func TestAliqInterestadual_SulSudeste(t *testing.T) {
	for _, uf := range []string{"SP", "MG", "RJ", "PR", "RS", "SC"} {
		if got := aliqInterestadual("", uf); got != 7.0 {
			t.Errorf("aliqInterestadual(%q): want 7.0, got %v", uf, got)
		}
	}
}

func TestAliqInterestadual_Outros(t *testing.T) {
	for _, uf := range []string{"PE", "BA", "GO", "AM", "MT"} {
		if got := aliqInterestadual("", uf); got != 12.0 {
			t.Errorf("aliqInterestadual(%q): want 12.0, got %v", uf, got)
		}
	}
}

func TestAliqInterestadual_CaseInsensitive(t *testing.T) {
	if got := aliqInterestadual("", "sp"); got != 7.0 {
		t.Errorf("aliqInterestadual lowercase 'sp': want 7.0, got %v", got)
	}
}

func TestAliqInterestadual_Importado4Pct(t *testing.T) {
	// CST origens 1,2,3,6,7,8 → sempre 4%, independente da UF do fornecedor
	for _, orig := range []string{"1", "2", "3", "6", "7", "8"} {
		for _, uf := range []string{"SP", "PE", "BA"} {
			if got := aliqInterestadual(orig, uf); got != 4.0 {
				t.Errorf("aliqInterestadual(orig=%q, uf=%q): want 4.0, got %v", orig, uf, got)
			}
		}
	}
}

func TestAliqInterestadual_NacionalNaoPaga4Pct(t *testing.T) {
	// Origens 0, 4, 5 → aplica regra normal (7% ou 12%)
	for _, orig := range []string{"0", "4", "5", ""} {
		if got := aliqInterestadual(orig, "SP"); got != 7.0 {
			t.Errorf("aliqInterestadual(orig=%q, SP): want 7.0, got %v", orig, got)
		}
		if got := aliqInterestadual(orig, "PE"); got != 12.0 {
			t.Errorf("aliqInterestadual(orig=%q, PE): want 12.0, got %v", orig, got)
		}
	}
}

// ── Handler creation guards ──────────────────────────────────────────────────

func TestIcmsFronteiraResumoHandler_Creation(t *testing.T) {
	if IcmsFronteiraResumoHandler(nil) == nil {
		t.Error("expected non-nil handler")
	}
}

func TestIcmsFronteiraAntecipacaoHandler_Creation(t *testing.T) {
	if IcmsFronteiraAntecipacaoHandler(nil) == nil {
		t.Error("expected non-nil handler")
	}
}

func TestIcmsFronteiraSTHandler_Creation(t *testing.T) {
	if IcmsFronteiraSTHandler(nil) == nil {
		t.Error("expected non-nil handler")
	}
}

func TestIcmsFronteiraDIFALHandler_Creation(t *testing.T) {
	if IcmsFronteiraDIFALHandler(nil) == nil {
		t.Error("expected non-nil handler")
	}
}

// ── Method not allowed (405) — no DB or auth touch ──────────────────────────

func TestIcmsFronteiraResumoHandler_MethodNotAllowed(t *testing.T) {
	h := IcmsFronteiraResumoHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/icms-fronteira/resumo", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestIcmsFronteiraAntecipacaoHandler_MethodNotAllowed(t *testing.T) {
	h := IcmsFronteiraAntecipacaoHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/icms-fronteira/antecipacao", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestIcmsFronteiraSTHandler_MethodNotAllowed(t *testing.T) {
	h := IcmsFronteiraSTHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/icms-fronteira/st", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestIcmsFronteiraDIFALHandler_MethodNotAllowed(t *testing.T) {
	h := IcmsFronteiraDIFALHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/icms-fronteira/difal", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

// ── No JWT → 401 via AuthMiddleware ─────────────────────────────────────────

func TestIcmsFronteiraResumoHandler_NoAuth(t *testing.T) {
	h := AuthMiddleware(IcmsFronteiraResumoHandler(nil), "")
	req := httptest.NewRequest(http.MethodGet, "/api/icms-fronteira/resumo", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestIcmsFronteiraAntecipacaoHandler_NoAuth(t *testing.T) {
	h := AuthMiddleware(IcmsFronteiraAntecipacaoHandler(nil), "")
	req := httptest.NewRequest(http.MethodGet, "/api/icms-fronteira/antecipacao", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

// ── No claims in context → 401 (direct call without middleware) ─────────────

func TestIcmsFronteiraResumoHandler_NoClaimsInContext(t *testing.T) {
	h := IcmsFronteiraResumoHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/icms-fronteira/resumo", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestIcmsFronteiraAntecipacaoHandler_NoClaimsInContext(t *testing.T) {
	h := IcmsFronteiraAntecipacaoHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/icms-fronteira/antecipacao", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestIcmsFronteiraSTHandler_NoClaimsInContext(t *testing.T) {
	h := IcmsFronteiraSTHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/icms-fronteira/st", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestIcmsFronteiraDIFALHandler_NoClaimsInContext(t *testing.T) {
	h := IcmsFronteiraDIFALHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/icms-fronteira/difal", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}
