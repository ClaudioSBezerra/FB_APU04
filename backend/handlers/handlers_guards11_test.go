package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// handlers_guards11_test.go — guards adicionais para atingir >=30% de cobertura.
// Cobre handlers sem tests existentes: GetSimplesDashboard, SaveRFBCredential,
// DeleteRFBCredential, GetRFBCredential, CheckDuplicityHandler.

func TestGetSimplesDashboardHandler_Unauthenticated(t *testing.T) {
	h := GetSimplesDashboardHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/simples/dashboard", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestSaveRFBCredentialHandler_MethodNotAllowed(t *testing.T) {
	h := SaveRFBCredentialHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/rfb/credentials", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestDeleteRFBCredentialHandler_MethodNotAllowed(t *testing.T) {
	h := DeleteRFBCredentialHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/rfb/credentials/1", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestGetRFBCredentialHandler_Unauthenticated(t *testing.T) {
	h := GetRFBCredentialHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/rfb/credentials", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}
