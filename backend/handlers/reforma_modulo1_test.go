package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

// ─── Handler creation guards ──────────────────────────────────────────────────

func TestCreditosBloqueadosHandler_Creation(t *testing.T) {
	if CreditosBloqueadosHandler(nil) == nil {
		t.Error("expected non-nil handler")
	}
}

func TestCreditosBloqueadosCSVHandler_Creation(t *testing.T) {
	if CreditosBloqueadosCSVHandler(nil) == nil {
		t.Error("expected non-nil handler")
	}
}

func TestRankingFornecedoresHandler_Creation(t *testing.T) {
	if RankingFornecedoresHandler(nil) == nil {
		t.Error("expected non-nil handler")
	}
}

func TestRankingFornecedoresCSVHandler_Creation(t *testing.T) {
	if RankingFornecedoresCSVHandler(nil) == nil {
		t.Error("expected non-nil handler")
	}
}

func TestReprecificacaoHandler_Creation(t *testing.T) {
	if ReprecificacaoHandler(nil) == nil {
		t.Error("expected non-nil handler")
	}
}

func TestReprecificacaoCSVHandler_Creation(t *testing.T) {
	if ReprecificacaoCSVHandler(nil) == nil {
		t.Error("expected non-nil handler")
	}
}

func TestSplitPaymentHandler_Creation(t *testing.T) {
	if SplitPaymentHandler(nil) == nil {
		t.Error("expected non-nil handler")
	}
}

// ─── Method-not-allowed (POST rejected before any DB touch) ──────────────────

func TestCreditosBloqueadosHandler_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rr := httptest.NewRecorder()
	CreditosBloqueadosHandler(nil)(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestCreditosBloqueadosCSVHandler_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rr := httptest.NewRecorder()
	CreditosBloqueadosCSVHandler(nil)(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestRankingFornecedoresHandler_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rr := httptest.NewRecorder()
	RankingFornecedoresHandler(nil)(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestRankingFornecedoresCSVHandler_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rr := httptest.NewRecorder()
	RankingFornecedoresCSVHandler(nil)(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestReprecificacaoHandler_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rr := httptest.NewRecorder()
	ReprecificacaoHandler(nil)(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestReprecificacaoCSVHandler_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rr := httptest.NewRecorder()
	ReprecificacaoCSVHandler(nil)(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestSplitPaymentHandler_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rr := httptest.NewRecorder()
	SplitPaymentHandler(nil)(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

// ─── No claims in context → 401 ──────────────────────────────────────────────

func TestCreditosBloqueadosHandler_NoClaims(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	CreditosBloqueadosHandler(nil)(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestCreditosBloqueadosCSVHandler_NoClaims(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	CreditosBloqueadosCSVHandler(nil)(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestRankingFornecedoresHandler_NoClaims(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	RankingFornecedoresHandler(nil)(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestRankingFornecedoresCSVHandler_NoClaims(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	RankingFornecedoresCSVHandler(nil)(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestReprecificacaoHandler_NoClaims(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	ReprecificacaoHandler(nil)(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestReprecificacaoCSVHandler_NoClaims(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	ReprecificacaoCSVHandler(nil)(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestSplitPaymentHandler_NoClaims(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	SplitPaymentHandler(nil)(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

// ─── Claims present but user_id missing → 401 (CR-01 path) ──────────────────

func reqWithEmptyClaims(method, path string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	ctx := context.WithValue(req.Context(), ClaimsKey, jwt.MapClaims{})
	return req.WithContext(ctx)
}

func TestCreditosBloqueadosHandler_EmptyUserID(t *testing.T) {
	rr := httptest.NewRecorder()
	CreditosBloqueadosHandler(nil)(rr, reqWithEmptyClaims(http.MethodGet, "/"))
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestRankingFornecedoresHandler_EmptyUserID(t *testing.T) {
	rr := httptest.NewRecorder()
	RankingFornecedoresHandler(nil)(rr, reqWithEmptyClaims(http.MethodGet, "/"))
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestReprecificacaoHandler_EmptyUserID(t *testing.T) {
	rr := httptest.NewRecorder()
	ReprecificacaoHandler(nil)(rr, reqWithEmptyClaims(http.MethodGet, "/"))
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestSplitPaymentHandler_EmptyUserID(t *testing.T) {
	rr := httptest.NewRecorder()
	SplitPaymentHandler(nil)(rr, reqWithEmptyClaims(http.MethodGet, "/"))
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

// ─── reforma_config.go guard tests ───────────────────────────────────────────

func TestGetReformaParametrosHandler_NoClaims(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	GetReformaParametrosHandler(nil)(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestPutReformaParametrosHandler_NoClaims(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/", nil)
	rr := httptest.NewRecorder()
	PutReformaParametrosHandler(nil)(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

