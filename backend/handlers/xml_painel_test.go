package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// xml_painel_test.go — guard tests for XML painel handlers (nil db safe)
// All paths below return before any db.Query call.

func TestXMLEntradasInformativosHandler_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/xml/painel/entradas-informativos", nil)
	rr := httptest.NewRecorder()
	XMLEntradasInformativosHandler(nil)(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestXMLEntradasInformativosHandler_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/xml/painel/entradas-informativos", nil)
	rr := httptest.NewRecorder()
	XMLEntradasInformativosHandler(nil)(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestXMLPainelHandlerEntradas_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/xml/painel/entradas", nil)
	rr := httptest.NewRecorder()
	XMLPainelHandler(nil)(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestXMLPainelHandlerEntradas_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/xml/painel/entradas", nil)
	rr := httptest.NewRecorder()
	XMLPainelHandler(nil)(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}
