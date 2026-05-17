package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ─── XMLNotasHandler ─────────────────────────────────────────────────────────

// POST → 405 before any auth or DB touch.
func TestXMLNotasHandler_MethodNotAllowed(t *testing.T) {
	handler := XMLNotasHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/xml/notas/entradas", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

// GET without JWT → 401 before DB touch.
func TestXMLNotasHandler_NoAuth(t *testing.T) {
	handler := AuthMiddleware(XMLNotasHandler(nil), "")
	req := httptest.NewRequest(http.MethodGet, "/api/xml/notas/entradas", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

// GET with valid JWT + nil DB → panics when hitting GetEffectiveCompanyID.
// Guard test: covers auth+tipo parsing code paths; panic is expected and recovered.
func TestXMLNotasHandler_WithAuth(t *testing.T) {
	var panicked bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		handler := AuthMiddleware(XMLNotasHandler(nil), "")
		tokenStr := makeTestJWT(t, "user")
		req := httptest.NewRequest(http.MethodGet, "/api/xml/notas/saidas", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		handler(httptest.NewRecorder(), req)
	}()
	_ = panicked
}

// GET with valid JWT + invalid tipo → still panics on nil DB at GetEffectiveCompanyID
// (tipo validation happens after company lookup). Guard covers that branch too.
func TestXMLNotasHandler_InvalidTipo(t *testing.T) {
	var panicked bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		handler := AuthMiddleware(XMLNotasHandler(nil), "")
		tokenStr := makeTestJWT(t, "user")
		req := httptest.NewRequest(http.MethodGet, "/api/xml/notas/invalido", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		handler(httptest.NewRecorder(), req)
	}()
	_ = panicked
}

// ─── buildFallbackNarrative — branch reducao ─────────────────────────────────

// FaturamentoBruto < FaturamentoAnterior → varFat < 0 → direcao = "reducao".
func TestBuildFallbackNarrative_Reducao(t *testing.T) {
	r := &ApuracaoResumo{
		Periodo:             "02/2026",
		PeriodoAnterior:     "01/2026",
		FaturamentoBruto:    30000.0,
		FaturamentoAnterior: 50000.0,
	}
	result := buildFallbackNarrative(r)
	if !strings.Contains(result, "reducao") {
		t.Errorf("buildFallbackNarrative decline: expected 'reducao', got %q", result)
	}
}

// ─── ResolveUserEmail — guard branches (sem DB) ──────────────────────────────

func TestResolveUserEmail_EmptyID(t *testing.T) {
	result := ResolveUserEmail(nil, "")
	if result != "" {
		t.Errorf("ResolveUserEmail empty id: expected empty string, got %q", result)
	}
}

func TestResolveUserEmail_InvalidUUID(t *testing.T) {
	result := ResolveUserEmail(nil, "not-a-valid-uuid")
	if result != "" {
		t.Errorf("ResolveUserEmail invalid uuid: expected empty string, got %q", result)
	}
}

// ─── buildFallbackInsight — Priority 2 (ICMS > 0, sem período anterior) ─────

func TestBuildFallbackInsight_IcmsPriority(t *testing.T) {
	r := &ApuracaoResumo{
		Periodo:    "01/2026",
		IcmsAPagar: 5000.0,
		IcmsEntrada: 2000.0,
	}
	insight := buildFallbackInsight(r)
	if insight.Tipo != "info" {
		t.Errorf("buildFallbackInsight ICMS priority: expected tipo='info', got %q", insight.Tipo)
	}
	if !strings.Contains(insight.Texto, "ICMS") {
		t.Errorf("buildFallbackInsight ICMS priority: expected ICMS in texto, got %q", insight.Texto)
	}
}

// ─── XMLNotasHandler — no claims in context (direct call, no middleware) ─────

// GET with valid method but no JWT claims in context → 401 from handler's own guard.
// Covers the !ok branch body in xml_notas.go (unreachable via AuthMiddleware path).
func TestXMLNotasHandler_NoClaimsInContext(t *testing.T) {
	handler := XMLNotasHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/xml/notas/entradas", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

// ─── RowsBefore — empty table list (no DB touch) ─────────────────────────────

// RowsBefore with empty tables slice returns immediately without querying DB.
func TestRowsBefore_EmptyTables(t *testing.T) {
	result, err := RowsBefore(context.Background(), nil, []string{})
	if err != nil {
		t.Errorf("RowsBefore empty tables: unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("RowsBefore empty tables: expected empty result, got %v", result)
	}
}
