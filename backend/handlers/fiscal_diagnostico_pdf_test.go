// Testes das funções puras e guards do PDF do Diagnóstico + endpoint de
// filiais (2026-07-08).
package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFmtBRLPdf(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "R$ 0,00"},
		{1234.5, "R$ 1.234,50"},
		{1234567.89, "R$ 1.234.567,89"},
		{999.99, "R$ 999,99"},
		{1000, "R$ 1.000,00"},
	}
	for _, c := range cases {
		if got := fmtBRLPdf(c.in); got != c.want {
			t.Errorf("fmtBRLPdf(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestOrDash(t *testing.T) {
	if got := orDash(""); got != "—" {
		t.Errorf("orDash(\"\") = %q, want —", got)
	}
	if got := orDash("   "); got != "—" {
		t.Errorf("orDash(spaces) = %q, want —", got)
	}
	if got := orDash("2026-07-01"); got != "2026-07-01" {
		t.Errorf("orDash kept = %q", got)
	}
}

// ─── FiscalDiagnosticoPDFHandler ─────────────────────────────────────────────

// POST → 405 before any auth or DB touch.
func TestFiscalDiagnosticoPDFHandler_MethodNotAllowed(t *testing.T) {
	handler := FiscalDiagnosticoPDFHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/fiscal/diagnostico/pdf", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

// GET sem claims no contexto → 401 antes de tocar o DB.
func TestFiscalDiagnosticoPDFHandler_NoAuth(t *testing.T) {
	handler := FiscalDiagnosticoPDFHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/fiscal/diagnostico/pdf", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

// ─── FiscalFiliaisHandler ────────────────────────────────────────────────────

// POST → 405 before any auth or DB touch.
func TestFiscalFiliaisHandler_MethodNotAllowed(t *testing.T) {
	handler := FiscalFiliaisHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/fiscal/filiais", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

// GET sem claims → 401.
func TestFiscalFiliaisHandler_NoAuth(t *testing.T) {
	handler := FiscalFiliaisHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/fiscal/filiais", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestSanitizeOracleTarget(t *testing.T) {
	cases := []struct{ in, want string }{
		{"oracle://user:pass@10.136.1.211:1521/fcjpateste", "10.136.1.211:1521/fcjpateste"},
		{"10.136.1.211:1521/fcjpateste", "10.136.1.211:1521/fcjpateste"},
		{"user/senha@10.136.1.211:1521/fcjpateste", "10.136.1.211:1521/fcjpateste"},
		{"  10.136.1.211:1521/fcjpateste  ", "10.136.1.211:1521/fcjpateste"},
		{"", ""},
	}
	for _, c := range cases {
		if got := sanitizeOracleTarget(c.in); got != c.want {
			t.Errorf("sanitizeOracleTarget(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
