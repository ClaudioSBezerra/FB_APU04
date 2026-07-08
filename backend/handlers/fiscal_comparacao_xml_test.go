// Guards do handler de visualização de XML (2026-07-08).
package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// POST → 405 antes de qualquer auth/DB.
func TestFiscalComparacaoXMLHandler_MethodNotAllowed(t *testing.T) {
	handler := FiscalComparacaoXMLHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/fiscal/comparacao/xml", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

// GET sem claims → 401 antes de tocar o DB.
func TestFiscalComparacaoXMLHandler_NoAuth(t *testing.T) {
	handler := FiscalComparacaoXMLHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/fiscal/comparacao/xml?nfe_id=x", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}
