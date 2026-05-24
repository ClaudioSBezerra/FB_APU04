package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// ---------------------------------------------------------------------------
// calcICMSFrete — lógica pura, sem DB
// ---------------------------------------------------------------------------

func TestCalcICMSFrete_DIFAL(t *testing.T) {
	// DIFAL = vFrete * (aliqInterna - aliqInter) / 100
	got := calcICMSFrete(1000, "DIFAL", 12, 20, 0)
	want := 80.0
	if got != want {
		t.Errorf("DIFAL: got %.2f, want %.2f", got, want)
	}
}

func TestCalcICMSFrete_DIFAL_NegativeFloor(t *testing.T) {
	// aliqInterna < aliqInter → deve retornar 0
	got := calcICMSFrete(1000, "DIFAL", 20, 12, 0)
	if got != 0 {
		t.Errorf("DIFAL negative: got %.2f, want 0", got)
	}
}

func TestCalcICMSFrete_ANTECIPACAO(t *testing.T) {
	// ANTECIPACAO = max(0, vFrete*aliqInterna/100 - vICMSCTe)
	got := calcICMSFrete(1000, "ANTECIPACAO", 12, 20, 50)
	want := 150.0 // 1000*0.20 - 50
	if got != want {
		t.Errorf("ANTECIPACAO: got %.2f, want %.2f", got, want)
	}
}

func TestCalcICMSFrete_ANTECIPACAO_AlreadyPaid(t *testing.T) {
	// vICMSCTe >= due → deve retornar 0
	got := calcICMSFrete(1000, "ANTECIPACAO", 12, 20, 300)
	if got != 0 {
		t.Errorf("ANTECIPACAO already paid: got %.2f, want 0", got)
	}
}

func TestCalcICMSFrete_ST(t *testing.T) {
	// ST usa mesma lógica da antecipação
	got := calcICMSFrete(2000, "ST", 7, 20.5, 100)
	want := 310.0 // 2000*0.205 - 100
	if got != want {
		t.Errorf("ST: got %.2f, want %.2f", got, want)
	}
}

func TestCalcICMSFrete_Unknown(t *testing.T) {
	// regime desconhecido → 0
	got := calcICMSFrete(1000, "NORMAL", 12, 20, 0)
	if got != 0 {
		t.Errorf("NORMAL: got %.2f, want 0", got)
	}
}

// ---------------------------------------------------------------------------
// IcmsFronteiraFretesHandler — guards sem DB
// ---------------------------------------------------------------------------

func TestIcmsFronteiraFretesHandler_Creation(t *testing.T) {
	if IcmsFronteiraFretesHandler(nil) == nil {
		t.Error("expected non-nil handler")
	}
}

func TestIcmsFronteiraFretesHandler_MethodNotAllowed(t *testing.T) {
	h := IcmsFronteiraFretesHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/icms-fronteira/fretes", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST: got %d, want 405", rr.Code)
	}
}

func TestIcmsFronteiraFretesHandler_Unauthorized(t *testing.T) {
	h := IcmsFronteiraFretesHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/icms-fronteira/fretes", nil)
	// sem ClaimsKey no contexto → 401
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("no claims: got %d, want 401", rr.Code)
	}
}
