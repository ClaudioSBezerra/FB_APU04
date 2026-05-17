package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Guard tests para xml_conciliacao.go — sem DB (handlers retornam antes de tocar DB)

func TestConciliacaoHandler_MethodNotAllowed(t *testing.T) {
	h := ConciliacaoHandler(nil)
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		req := httptest.NewRequest(method, "/api/xml/conciliacao", nil)
		rr := httptest.NewRecorder()
		h(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("method %s: expected 405, got %d", method, rr.Code)
		}
	}
}

func TestConciliacaoHandler_Unauthenticated(t *testing.T) {
	h := ConciliacaoHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/xml/conciliacao", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestCoberturaHandler_MethodNotAllowed(t *testing.T) {
	h := CoberturaHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/xml/cobertura", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestCoberturaHandler_Unauthenticated(t *testing.T) {
	h := CoberturaHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/xml/cobertura", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestConciliacaoCSVHandler_MethodNotAllowed(t *testing.T) {
	h := ConciliacaoCSVHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/xml/conciliacao/csv", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestConciliacaoCSVHandler_Unauthenticated(t *testing.T) {
	h := ConciliacaoCSVHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/xml/conciliacao/csv", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestXMLPainelHandler_MethodNotAllowed(t *testing.T) {
	h := XMLPainelHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/xml/painel/entradas", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestXMLPainelHandler_Unauthenticated(t *testing.T) {
	h := XMLPainelHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/xml/painel/entradas", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}
