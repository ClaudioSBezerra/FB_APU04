package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// xml_reports_test.go — guard tests for XML report handlers (nil db safe)
// All paths below return before any db.Query call.

func TestXMLSaneamentoCCLASSTRIBHandler_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/xml/reports/saneamento", nil)
	rr := httptest.NewRecorder()
	XMLSaneamentoCCLASSTRIBHandler(nil)(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestXMLSaneamentoCCLASSTRIBHandler_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/xml/reports/saneamento", nil)
	rr := httptest.NewRecorder()
	XMLSaneamentoCCLASSTRIBHandler(nil)(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestXMLSaneamentoCSVHandler_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/xml/reports/saneamento/csv", nil)
	rr := httptest.NewRecorder()
	XMLSaneamentoCSVHandler(nil)(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestXMLSaneamentoCSVHandler_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/xml/reports/saneamento/csv", nil)
	rr := httptest.NewRecorder()
	XMLSaneamentoCSVHandler(nil)(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestXMLFornecedoresCCLASSTRIBHandler_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/xml/reports/fornecedores-cclasstrib", nil)
	rr := httptest.NewRecorder()
	XMLFornecedoresCCLASSTRIBHandler(nil)(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestXMLFornecedoresCCLASSTRIBHandler_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/xml/reports/fornecedores-cclasstrib", nil)
	rr := httptest.NewRecorder()
	XMLFornecedoresCCLASSTRIBHandler(nil)(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestMercadoriasXMLReportHandler_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/xml/reports/mercadorias", nil)
	rr := httptest.NewRecorder()
	MercadoriasXMLReportHandler(nil)(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestMercadoriasXMLReportHandler_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/xml/reports/mercadorias", nil)
	rr := httptest.NewRecorder()
	MercadoriasXMLReportHandler(nil)(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}
