package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// ─── Handler creation guards — outer func returns non-nil http.HandlerFunc ───

func TestCreditosBloqueadosHandler_Creation(t *testing.T) {
	h := CreditosBloqueadosHandler(nil)
	if h == nil {
		t.Error("expected non-nil handler")
	}
}

func TestCreditosBloqueadosCSVHandler_Creation(t *testing.T) {
	h := CreditosBloqueadosCSVHandler(nil)
	if h == nil {
		t.Error("expected non-nil handler")
	}
}

func TestRankingFornecedoresHandler_Creation(t *testing.T) {
	h := RankingFornecedoresHandler(nil)
	if h == nil {
		t.Error("expected non-nil handler")
	}
}

func TestRankingFornecedoresCSVHandler_Creation(t *testing.T) {
	h := RankingFornecedoresCSVHandler(nil)
	if h == nil {
		t.Error("expected non-nil handler")
	}
}

func TestReprecificacaoHandler_Creation(t *testing.T) {
	h := ReprecificacaoHandler(nil)
	if h == nil {
		t.Error("expected non-nil handler")
	}
}

func TestReprecificacaoCSVHandler_Creation(t *testing.T) {
	h := ReprecificacaoCSVHandler(nil)
	if h == nil {
		t.Error("expected non-nil handler")
	}
}

func TestSplitPaymentHandler_Creation(t *testing.T) {
	h := SplitPaymentHandler(nil)
	if h == nil {
		t.Error("expected non-nil handler")
	}
}

// ─── Method-not-allowed guard — POST rejected before DB touch (nil db is safe) ───

func TestCreditosBloqueadosHandler_MethodNotAllowed(t *testing.T) {
	h := CreditosBloqueadosHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/reforma/modulo1/creditos", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected %d, got %d", http.StatusMethodNotAllowed, rr.Code)
	}
}
