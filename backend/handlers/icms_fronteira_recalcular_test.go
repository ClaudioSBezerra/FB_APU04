package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// POST sem claims → 401 (não toca o DB).
func TestIcmsFronteiraRecalcularHandler_NoAuth(t *testing.T) {
	h := IcmsFronteiraRecalcularHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/icms-fronteira/recalcular", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("esperado 401, veio %d", rr.Code)
	}
}

// GET → 405 (só POST).
func TestIcmsFronteiraRecalcularHandler_MethodNotAllowed(t *testing.T) {
	h := IcmsFronteiraRecalcularHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/icms-fronteira/recalcular", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("esperado 405, veio %d", rr.Code)
	}
}
