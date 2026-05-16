package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestERPBridgeBatchImportHandler_MethodNotAllowed(t *testing.T) {
	// nil db seguro: handler retorna antes de acessar DB (método errado → 405)
	handler := ERPBridgeBatchImportHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/erp-bridge/import/batch", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("TestERPBridgeBatchImportHandler_MethodNotAllowed: got status %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestERPBridgeBatchImportHandler_MissingAPIKey(t *testing.T) {
	// nil db seguro: handler retorna antes de acessar DB (X-API-Key ausente → 401)
	handler := ERPBridgeBatchImportHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/erp-bridge/import/batch", nil)
	// X-API-Key header not set
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("TestERPBridgeBatchImportHandler_MissingAPIKey: got status %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}
