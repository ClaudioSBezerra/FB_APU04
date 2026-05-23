package handlers

// handlers_guards14_test.go — cobertura phase 08 (CR/WR fixes)
// Cobre: UpdateCompanyHandler (method/id/body guards), IcmsFronteiraRegraUpdateHandler (body guard).

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUpdateCompanyHandler_WrongMethod(t *testing.T) {
	handler := UpdateCompanyHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/companies?id=abc", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("UpdateCompanyHandler GET: got %d, want 405", rr.Code)
	}
}

func TestUpdateCompanyHandler_MissingID(t *testing.T) {
	handler := UpdateCompanyHandler(nil)
	req := httptest.NewRequest(http.MethodPatch, "/api/companies", strings.NewReader(`{"name":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("UpdateCompanyHandler missing id: got %d, want 400", rr.Code)
	}
}

func TestUpdateCompanyHandler_InvalidBody(t *testing.T) {
	handler := UpdateCompanyHandler(nil)
	req := httptest.NewRequest(http.MethodPatch, "/api/companies?id=abc", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("UpdateCompanyHandler invalid body: got %d, want 400", rr.Code)
	}
}

func TestIcmsFronteiraRegraUpdateHandler_WrongMethod(t *testing.T) {
	handler := IcmsFronteiraRegraUpdateHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/icms-fronteira/regras/abc", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("IcmsFronteiraRegraUpdateHandler GET: got %d, want 405", rr.Code)
	}
}

func TestIcmsFronteiraRegraCreateHandler_WrongMethod(t *testing.T) {
	handler := IcmsFronteiraRegraCreateHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/icms-fronteira/regras", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("IcmsFronteiraRegraCreateHandler GET: got %d, want 405", rr.Code)
	}
}

func TestIcmsFronteiraRegrasListHandler_WrongMethod(t *testing.T) {
	handler := IcmsFronteiraRegrasListHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/icms-fronteira/regras", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("IcmsFronteiraRegrasListHandler POST: got %d, want 405", rr.Code)
	}
}
