package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// ─── Handler creation guards ──────────────────────────────────────────────────

func TestCfopAnalysisHandler_Creation(t *testing.T) {
	h := CfopAnalysisHandler(nil)
	if h == nil {
		t.Error("expected non-nil handler")
	}
}

func TestCfopAnalysisCSVHandler_Creation(t *testing.T) {
	h := CfopAnalysisCSVHandler(nil)
	if h == nil {
		t.Error("expected non-nil handler")
	}
}

func TestNcmAnalysisHandler_Creation(t *testing.T) {
	h := NcmAnalysisHandler(nil)
	if h == nil {
		t.Error("expected non-nil handler")
	}
}

func TestNcmAnalysisCSVHandler_Creation(t *testing.T) {
	h := NcmAnalysisCSVHandler(nil)
	if h == nil {
		t.Error("expected non-nil handler")
	}
}

func TestUfDestinoHandler_Creation(t *testing.T) {
	h := UfDestinoHandler(nil)
	if h == nil {
		t.Error("expected non-nil handler")
	}
}

func TestB2bB2cHandler_Creation(t *testing.T) {
	h := B2bB2cHandler(nil)
	if h == nil {
		t.Error("expected non-nil handler")
	}
}

// ─── Method-not-allowed (POST rejeitado antes de qualquer acesso ao DB) ──────

func TestCfopAnalysisHandler_MethodNotAllowed(t *testing.T) {
	h := CfopAnalysisHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/reforma/modulo2/cfop", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("got %d, want 405", rr.Code)
	}
}

func TestCfopAnalysisCSVHandler_MethodNotAllowed(t *testing.T) {
	h := CfopAnalysisCSVHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/reforma/modulo2/cfop/csv", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("got %d, want 405", rr.Code)
	}
}

func TestNcmAnalysisHandler_MethodNotAllowed(t *testing.T) {
	h := NcmAnalysisHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/reforma/modulo2/ncm", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("got %d, want 405", rr.Code)
	}
}

func TestNcmAnalysisCSVHandler_MethodNotAllowed(t *testing.T) {
	h := NcmAnalysisCSVHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/reforma/modulo2/ncm/csv", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("got %d, want 405", rr.Code)
	}
}

func TestUfDestinoHandler_MethodNotAllowed(t *testing.T) {
	h := UfDestinoHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/reforma/modulo2/uf-destino", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("got %d, want 405", rr.Code)
	}
}

func TestB2bB2cHandler_MethodNotAllowed(t *testing.T) {
	h := B2bB2cHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/reforma/modulo2/b2b-b2c", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("got %d, want 405", rr.Code)
	}
}
