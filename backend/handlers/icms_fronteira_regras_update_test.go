package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestIcmsFronteiraRegraUpdateHandler_Creation verifica que o handler retorna
// uma http.HandlerFunc não-nula mesmo com db=nil (outer func cobre 1 stmt).
func TestIcmsFronteiraRegraUpdateHandler_Creation(t *testing.T) {
	h := IcmsFronteiraRegraUpdateHandler(nil)
	if h == nil {
		t.Error("expected non-nil handler")
	}
}

// TestIcmsFronteiraRegraUpdateHandler_MethodNotAllowed verifica que GET é
// rejeitado com 405 antes de tocar o banco (nil DB é seguro aqui).
func TestIcmsFronteiraRegraUpdateHandler_MethodNotAllowed(t *testing.T) {
	h := IcmsFronteiraRegraUpdateHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/icms-fronteira/regras/abc", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected %d, got %d", http.StatusMethodNotAllowed, rr.Code)
	}
}

// TestRegraUpdateCompanyScope garante a correção do tampering cross-tenant
// (CR-02): o escopo do UPDATE de um usuário comum (não-admin) nunca pode
// alcançar as regras globais (company_id IS NULL); só o admin global pode.
func TestRegraUpdateCompanyScope(t *testing.T) {
	if got := regraUpdateCompanyScope(false); strings.Contains(got, "IS NULL") {
		t.Errorf("escopo de não-admin não pode alcançar regra global, got %q", got)
	}
	if got := regraUpdateCompanyScope(true); !strings.Contains(got, "IS NULL") {
		t.Errorf("escopo de admin deve alcançar regra global, got %q", got)
	}
}
